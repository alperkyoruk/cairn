package service

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/alperkyoruk/cairn/internal/id"
	"github.com/alperkyoruk/cairn/internal/model"
	"github.com/alperkyoruk/cairn/internal/store"
)

// slugRE constrains the short name that appears in a task reference like
// "cairn-12", so a reference can always be split back apart on its last dash.
var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

func (s *Service) CreateProject(ctx context.Context, actor Actor, slug, name, description string) (model.Project, error) {
	if err := s.authorize(actor, OpProjectManage); err != nil {
		return model.Project{}, err
	}
	slug = strings.ToLower(strings.TrimSpace(slug))
	if !slugRE.MatchString(slug) {
		return model.Project{}, invalid("project slug must be 1-32 lowercase letters, digits or dashes")
	}
	if strings.TrimSpace(name) == "" {
		return model.Project{}, invalid("project needs a name")
	}

	p := model.Project{
		ID: id.New(), Slug: slug, Name: strings.TrimSpace(name),
		Description: description, CreatedAt: s.now(),
	}
	err := s.write(ctx, func(q store.Queryer) error {
		if _, err := store.GetProjectBySlug(ctx, q, slug); err == nil {
			return conflict("a project with slug %q already exists", slug)
		} else if !errors.Is(err, store.ErrNotFound) {
			return internal(err)
		}
		if err := store.InsertProject(ctx, q, p); err != nil {
			return internal(err)
		}
		return nil
	})
	if err != nil {
		return model.Project{}, err
	}
	return p, nil
}

func (s *Service) ListProjects(ctx context.Context, actor Actor) ([]model.Project, error) {
	if err := s.authorize(actor, OpRead); err != nil {
		return nil, err
	}
	ps, err := store.ListProjects(ctx, s.read())
	if err != nil {
		return nil, internal(err)
	}
	return ps, nil
}

func (s *Service) GetProject(ctx context.Context, actor Actor, projectID string) (model.Project, error) {
	if err := s.authorize(actor, OpRead); err != nil {
		return model.Project{}, err
	}
	p, err := store.GetProject(ctx, s.read(), projectID)
	if errors.Is(err, store.ErrNotFound) {
		return model.Project{}, notFound("no project with id %s", projectID)
	}
	if err != nil {
		return model.Project{}, internal(err)
	}
	return p, nil
}

func (s *Service) UpdateProject(ctx context.Context, actor Actor, projectID, name, description string) (model.Project, error) {
	if err := s.authorize(actor, OpProjectManage); err != nil {
		return model.Project{}, err
	}
	if strings.TrimSpace(name) == "" {
		return model.Project{}, invalid("project needs a name")
	}

	var out model.Project
	err := s.write(ctx, func(q store.Queryer) error {
		p, err := store.GetProject(ctx, q, projectID)
		if errors.Is(err, store.ErrNotFound) {
			return notFound("no project with id %s", projectID)
		}
		if err != nil {
			return internal(err)
		}
		p.Name, p.Description = strings.TrimSpace(name), description
		if err := store.UpdateProject(ctx, q, p); err != nil {
			return internal(err)
		}
		out = p
		return nil
	})
	return out, err
}

// ArchiveProject hides a project without touching its tasks.
func (s *Service) ArchiveProject(ctx context.Context, actor Actor, projectID string, archived bool) error {
	if err := s.authorize(actor, OpProjectManage); err != nil {
		return err
	}
	return s.write(ctx, func(q store.Queryer) error {
		p, err := store.GetProject(ctx, q, projectID)
		if errors.Is(err, store.ErrNotFound) {
			return notFound("no project with id %s", projectID)
		}
		if err != nil {
			return internal(err)
		}
		if archived {
			at := s.now()
			p.ArchivedAt = &at
		} else {
			p.ArchivedAt = nil
		}
		if err := store.UpdateProject(ctx, q, p); err != nil {
			return internal(err)
		}
		return nil
	})
}

// DeleteProject removes an empty project. It refuses while tasks remain rather
// than deleting work as a side effect of tidying up.
func (s *Service) DeleteProject(ctx context.Context, actor Actor, projectID string) error {
	if err := s.authorize(actor, OpProjectManage); err != nil {
		return err
	}
	return s.write(ctx, func(q store.Queryer) error {
		p, err := store.GetProject(ctx, q, projectID)
		if errors.Is(err, store.ErrNotFound) {
			return notFound("no project with id %s", projectID)
		}
		if err != nil {
			return internal(err)
		}
		n, err := store.CountTasksInProject(ctx, q, projectID)
		if err != nil {
			return internal(err)
		}
		if n > 0 {
			return conflict("%s still holds %d task(s); delete them first", p.Slug, n)
		}
		if err := store.DeleteProject(ctx, q, projectID); err != nil {
			return internal(err)
		}
		return nil
	})
}

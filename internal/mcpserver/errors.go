package mcpserver

import (
	"fmt"
	"strings"

	"github.com/alperkyoruk/cairn/internal/model"
)

// unknownProject names what the caller asked for and what actually exists,
// because "project not found" makes an agent guess a second time.
func unknownProject(slug string, known []model.Project) error {
	if len(known) == 0 {
		return fmt.Errorf("no project %q; this Cairn has no projects yet, "+
			"and only the human can create one", slug)
	}
	slugs := make([]string, 0, len(known))
	for _, p := range known {
		slugs = append(slugs, p.Slug)
	}
	return fmt.Errorf("no project %q; the projects here are: %s",
		slug, strings.Join(slugs, ", "))
}

package service

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// methodOps names the operation each exported method authorises against.
// It is not used by the code — it exists so that adding a method to Service
// without deciding who may call it fails the build.
var methodOps = map[string]Op{
	"Board":          OpRead,
	"GetProject":     OpRead,
	"GetTask":        OpRead,
	"GetTaskByRef":   OpRead,
	"ListAgents":     OpRead,
	"ListProjects":   OpRead,
	"ListTasks":      OpRead,
	"AppendWorklog":  OpWorklogAppend,
	"WriteState":     OpStateWrite,
	"Transition":     OpTaskTransition,
	"CreateTask":     OpTaskCreate,
	"UpdateTask":     OpTaskUpdate,
	"DeleteTask":     OpTaskDelete,
	"ArchiveProject": OpProjectManage,
	"CreateProject":  OpProjectManage,
	"DeleteProject":  OpProjectManage,
	"UpdateProject":  OpProjectManage,
	"CreateAgent":    OpAgentManage,
	"IssueToken":     OpAgentManage,
	"ListTokens":     OpAgentManage,
	"RevokeToken":    OpAgentManage,
}

// unauthenticated lists the methods that deliberately take no Actor, with the
// reason each one is safe. Anything added here should be hard to justify.
var noActorNeeded = map[string]string{
	"NeedsSetup":    "asked by the login page before anyone can exist",
	"Setup":         "creates the first and only human; refuses once one exists",
	"Login":         "exchanges a password for a token; that is the authentication",
	"Authenticate":  "resolves a presented credential; this is where Actors come from",
	"Logout":        "revokes the caller's own credential",
	"ResetPassword": "reachable only from the command line; its authorisation is access to the database file",
	"Close":         "lifecycle, not a request",
}

func TestEveryServiceMethodDecidesWhoMayCallIt(t *testing.T) {
	typ := reflect.TypeOf(&Service{})
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		_, guarded := methodOps[name]
		_, exempt := noActorNeeded[name]
		switch {
		case guarded && exempt:
			t.Errorf("%s is listed both as guarded and as unauthenticated", name)
		case !guarded && !exempt:
			t.Errorf("Service.%s has no entry in methodOps or noActorNeeded: "+
				"decide which operation it requires before shipping it", name)
		}
	}
	for name := range methodOps {
		if _, ok := typ.MethodByName(name); !ok {
			t.Errorf("methodOps names %s, which no longer exists", name)
		}
	}
	for name := range noActorNeeded {
		if _, ok := typ.MethodByName(name); !ok {
			t.Errorf("noActorNeeded names %s, which no longer exists", name)
		}
	}
}

func TestEveryOpAppearsInThePolicy(t *testing.T) {
	for name, op := range methodOps {
		if _, ok := policy[op]; !ok {
			t.Errorf("Service.%s requires %q, which the policy table does not define", name, op)
		}
	}
}

// The layering claim, enforced. If an HTTP handler or an MCP tool could reach
// internal/store directly, "every write path goes through the service layer"
// would be a habit rather than a fact.
func TestOnlyServiceReachesTheStore(t *testing.T) {
	const storePkg = "github.com/alperkyoruk/cairn/internal/store"
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == ".git" || name == "node_modules" || name == "web" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		dir := filepath.Dir(path)
		if dir == filepath.Join(root, "internal", "store") || dir == filepath.Join(root, "internal", "service") {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if p == storePkg {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s imports internal/store directly; go through internal/service", rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

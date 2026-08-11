package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRouteManifestMatchesRegisteredAPIInventory(t *testing.T) {
	apiHandler := &API{}
	handler := apiHandler.Routes()
	engine, ok := handler.(*gin.Engine)
	if !ok {
		t.Fatalf("Routes returned %T, want *gin.Engine for route inspection", handler)
	}

	manifest := routeManifest(apiHandler)
	if got := len(manifest); got != 120 {
		t.Fatalf("API route baseline = %d, want 120", got)
	}
	expected := make(map[string]routeSpec, len(manifest))
	owners := map[string]bool{}
	public := map[string]bool{
		http.MethodGet + " /api/health":                 true,
		http.MethodGet + " /api/auth/login":             true,
		http.MethodGet + " /api/auth/callback":          true,
		http.MethodPost + " /api/auth/logout":           true,
		http.MethodGet + " /api/agents/channel":         true,
		http.MethodPost + " /api/workspaces/from-agent": true,
	}
	for _, route := range manifest {
		key := route.method + " /api" + route.path
		if _, exists := expected[key]; exists {
			t.Fatalf("route manifest registers %s more than once", key)
		}
		if route.owner == "" || route.owner == "api" || route.owner == "transport" {
			t.Errorf("route %s has invalid domain owner %q", key, route.owner)
		}
		if wantPublic := public[key]; wantPublic != (route.access == publicRoute) {
			t.Errorf("route %s access = %q, public = %v", key, route.access, wantPublic)
		}
		if route.access != publicRoute && route.access != protectedRoute {
			t.Errorf("route %s has unknown access group %q", key, route.access)
		}
		if route.access == protectedRoute {
			if _, ok := policyForRoute(route); !ok {
				t.Errorf("protected route %s has no explicit capability policy", key)
			}
		}
		expected[key] = route
		owners[route.owner] = true
	}

	actual := map[string]int{}
	for _, route := range engine.Routes() {
		if !strings.HasPrefix(route.Path, "/api") {
			continue
		}
		key := route.Method + " " + route.Path
		actual[key]++
		if _, exists := expected[key]; !exists {
			t.Errorf("registered API route %s is absent from route manifest", key)
		}
	}
	for key := range expected {
		if actual[key] != 1 {
			t.Errorf("route %s registered %d times, want exactly once", key, actual[key])
		}
	}

	for _, owner := range []string{"health", "cloud-auth", "cloud-agent", "cloud-workspace", "cloud-command", "cloud-snapshot", "audit", "navigation", "system", "storage", "canvas", "state", "search", "ai", "workspace", "workspace-files", "workspace-search", "workspace-health", "item", "item-search", "jira", "knowledge", "verification", "git", "workspace-stream"} {
		if !owners[owner] {
			t.Errorf("route family %q owns no manifest route", owner)
		}
	}
}

func TestAPIReceiverMethodsAreTransportOnly(t *testing.T) {
	allowed := map[string]bool{
		"Routes":                true,
		"initializeControllers": true,
		"registerGinRoutes":     true,
	}
	seen := map[string]bool{}
	fset := token.NewFileSet()
	packages, err := parser.ParseDir(fset, ".", func(info fs.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg := packages["api"]
	if pkg == nil {
		t.Fatal("api package not found")
	}
	for _, file := range pkg.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			declaration, ok := node.(*ast.FuncDecl)
			if !ok || declaration.Recv == nil || len(declaration.Recv.List) != 1 {
				return true
			}
			pointer, ok := declaration.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				return true
			}
			receiver, ok := pointer.X.(*ast.Ident)
			if !ok || receiver.Name != "API" {
				return true
			}
			name := declaration.Name.Name
			seen[name] = true
			if !allowed[name] {
				position := fset.Position(declaration.Pos())
				t.Errorf("domain method %s remains on *API at %s", name, position)
			}
			return true
		})
	}
	for method := range allowed {
		if !seen[method] {
			t.Errorf("required transport method %s is missing from *API", method)
		}
	}
}

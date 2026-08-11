package server

import (
	"os"
	"strings"
	"testing"
)

func TestProductionAuditProducersUseSharedRecorderBoundary(t *testing.T) {
	serverSource, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	apiSource, err := os.ReadFile("api/api.go")
	if err != nil {
		t.Fatal(err)
	}
	serverText, apiText := string(serverSource), string(apiSource)
	for _, producer := range []string{
		"auditStore := audit.NewRecorder(state.Audit)",
		"ConfigureLaunch(reg, idx, auditStore, os.TempDir())",
		"ConfigureActions(knowledge.NewDetector(), appgit.NewService(reg, writer, git), auditStore)",
		"Audit:               auditStore",
	} {
		if !strings.Contains(serverText, producer) {
			t.Errorf("production audit producer wiring missing %q", producer)
		}
	}
	if strings.Count(serverText, "state.Audit") != 1 {
		t.Errorf("raw audit repository escapes recorder construction: %d references", strings.Count(serverText, "state.Audit"))
	}
	for _, producer := range []string{
		"Audit: deps.Audit",
		"NewWorkspaceFileService(deps.WorkspaceRepository, workspaceFileAccess, deps.Git, deps.Audit, refresher)",
		"gitController{gitOps:",
		"itemController{items:",
	} {
		if !strings.Contains(apiText, producer) {
			t.Errorf("API audit producer wiring missing %q", producer)
		}
	}
}

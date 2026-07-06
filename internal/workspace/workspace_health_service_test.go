package workspace

// Health service contract tests.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"plan-manager/internal/common/models"
	"plan-manager/internal/item/index"
)

type workspaceStub struct{ workspace models.WorkspaceConfig }

func (s workspaceStub) Get(id string) (models.WorkspaceConfig, bool, error) {
	return s.workspace, s.workspace.ID == id, nil
}

type itemStub struct{ err error }

func (s itemStub) Query(itemindex.Query) ([]models.ItemSummary, error) {
	return []models.ItemSummary{}, s.err
}

type gitStub struct {
	root      string
	rootErr   error
	branchErr error
	status    models.GitStatus
	statusErr error
}

func (s gitStub) WorkspaceRoot(string) (string, error) { return s.root, s.rootErr }
func (s gitStub) ValidateBranch(string, string) error  { return s.branchErr }
func (s gitStub) Status(string, string) (models.GitStatus, error) {
	return s.status, s.statusErr
}

func TestCheckReturnsHealthyWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	workspace := models.WorkspaceConfig{ID: "workspace", Path: root, Sources: []string{"plans"}, BaselineBranch: "main", LastScannedAt: time.Now()}
	service := NewHealthService(workspaceStub{workspace}, itemStub{}, gitStub{root: root})

	health, err := service.Check(workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	if health.Summary != models.HealthStatusOK || len(health.Checks) != 6 {
		t.Fatalf("health = %#v, want six healthy checks", health)
	}
}

func TestCheckReturnsWarningsForRecoverableState(t *testing.T) {
	root := t.TempDir()
	workspace := models.WorkspaceConfig{ID: "workspace", Path: root, BaselineBranch: "main"}
	service := NewHealthService(workspaceStub{workspace}, itemStub{}, gitStub{root: root, branchErr: errors.New("missing"), status: models.GitStatus{Conflicted: true}})

	health, err := service.Check(workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	if health.Summary != models.HealthStatusWarning {
		t.Fatalf("summary = %q, want warning", health.Summary)
	}
	for _, check := range health.Checks {
		if check.Status == models.HealthStatusWarning && check.RecoveryHint == "" {
			t.Fatalf("warning check has no recovery hint: %#v", check)
		}
	}
}

func TestCheckReturnsFailedForMissingWorkspacePath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	workspace := models.WorkspaceConfig{ID: "workspace", Path: root, Sources: []string{"plans"}, BaselineBranch: "main", LastScannedAt: time.Now()}
	service := NewHealthService(workspaceStub{workspace}, itemStub{err: errors.New("corrupt")}, gitStub{root: root, rootErr: errors.New("not git")})

	health, err := service.Check(workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	if health.Summary != models.HealthStatusFailed {
		t.Fatalf("summary = %q, want failed", health.Summary)
	}
}

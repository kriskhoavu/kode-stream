package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	appcanvas "kode-stream/internal/canvas"
	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
)

func TestCanvasAPIDefaultProjectionPlacementConflictAndViewportIndependence(t *testing.T) {
	root := t.TempDir()
	apiCanvasGit(t, root, "init", "-b", "main")
	if err := os.MkdirAll(filepath.Join(root, "plans", "platform", "PM-001"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "platform", "PM-001", "README.md"), []byte("# PM-001: Main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "platform", "PM-001", "plan.yaml"), []byte("plan:\n  status: draft\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("seed"), 0o644); err != nil {
		t.Fatal(err)
	}
	apiCanvasGit(t, root, "add", ".")
	command := exec.Command("git", "-C", root, "commit", "-m", "seed")
	command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	dataDir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dataDir, "workspaces.yaml"), git)
	workspaceConfig, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	items := itemindex.New(filepath.Join(dataDir, "items.yaml"))
	details := []models.ItemDetail{
		{ItemSummary: models.ItemSummary{ID: "main-item", WorkspaceID: workspaceConfig.ID, Branch: "main", Editable: true, Scope: "platform", Identifier: "PM-001", Title: "Main", Status: models.StatusDraft, ItemPath: "plans/platform/PM-001"}},
		{ItemSummary: models.ItemSummary{ID: "other-item", WorkspaceID: workspaceConfig.ID, Branch: "other", Editable: true, Scope: "platform", Identifier: "PM-002", Title: "Other", Status: models.StatusDraft, ItemPath: "plans/platform/PM-002"}},
	}
	if err := items.ReplaceWorkspace(workspaceConfig.ID, details, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	repository := appcanvas.NewFileRepository(filepath.Join(dataDir, "canvas.yaml"))
	service := appcanvas.NewService(repository, reg, items, git, nil, nil, models.RuntimeModeLocal, models.AppStateDatastoreDataDir, nil)
	handler := New(reg, items, scanner.New(git), nil, nil, git, nil).WithCanvas(service).Routes()

	body, _ := json.Marshal(map[string]string{"workspaceId": workspaceConfig.ID, "branchKey": "main"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/canvas/default", bytes.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var projection appcanvas.Projection
	if err := json.Unmarshal(response.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	if len(projection.Nodes) != 2 || len(projection.Unplaced) != 0 {
		t.Fatalf("projection=%#v", projection)
	}
	for _, node := range projection.Nodes {
		if node.EntityRef.ItemID == "other-item" {
			t.Fatal("projection crossed branch scope")
		}
	}
	mismatchBody, _ := json.Marshal(map[string]string{"workspaceId": workspaceConfig.ID, "branchKey": "other"})
	mismatch := httptest.NewRecorder()
	handler.ServeHTTP(mismatch, httptest.NewRequest(http.MethodPost, "/api/canvas/default", bytes.NewReader(mismatchBody)))
	if mismatch.Code != http.StatusConflict || !bytes.Contains(mismatch.Body.Bytes(), []byte(`"code":"canvas_branch_mismatch"`)) {
		t.Fatalf("mismatch status=%d body=%s", mismatch.Code, mismatch.Body.String())
	}

	conflictBody, _ := json.Marshal(map[string]any{"patches": []appcanvas.PlacementPatch{{NodeID: projection.Nodes[0].ID, EntityRef: projection.Nodes[0].EntityRef, Position: appcanvas.Position{X: 5, Y: 5}, ExpectedRevision: 999}}})
	conflict := httptest.NewRecorder()
	handler.ServeHTTP(conflict, httptest.NewRequest(http.MethodPatch, "/api/canvas/layouts/"+projection.Layout.ID+"/placements", bytes.NewReader(conflictBody)))
	if conflict.Code != http.StatusConflict || !bytes.Contains(conflict.Body.Bytes(), []byte(`"code":"placement_conflict"`)) {
		t.Fatalf("status=%d body=%s", conflict.Code, conflict.Body.String())
	}

	viewportBody, _ := json.Marshal(map[string]any{"expectedVersion": projection.Layout.Version, "viewport": appcanvas.Viewport{X: 20, Y: 30, Zoom: 1.1}})
	viewport := httptest.NewRecorder()
	handler.ServeHTTP(viewport, httptest.NewRequest(http.MethodPatch, "/api/canvas/layouts/"+projection.Layout.ID+"/viewport", bytes.NewReader(viewportBody)))
	if viewport.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", viewport.Code, viewport.Body.String())
	}
	var after appcanvas.Projection
	if err := json.Unmarshal(viewport.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.Layout.Version != projection.Layout.Version+1 || after.Nodes[0].Revision != projection.Nodes[0].Revision {
		t.Fatalf("before=%#v after=%#v", projection, after)
	}
}

func apiCanvasGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

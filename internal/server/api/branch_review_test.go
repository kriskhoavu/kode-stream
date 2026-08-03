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

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/content"
	gitadapter "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	itemwriter "kode-stream/internal/item/writer"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
)

func TestCheckoutReviewAndExplicitImportRoutes(t *testing.T) {
	root := t.TempDir()
	branchReviewGit(t, root, "init", "-b", "main")
	branchReviewGit(t, root, "config", "user.name", "Kode Stream")
	branchReviewGit(t, root, "config", "user.email", "kode-stream@example.test")
	branchReviewWrite(t, root, "plans/.keep", "")
	branchReviewGit(t, root, "add", ".")
	branchReviewGit(t, root, "commit", "-m", "main")
	branchReviewGit(t, root, "switch", "-c", "feature")
	branchReviewWrite(t, root, "plans/platform/PM-038/README.md", "# PM-038: Reviewed\n")
	branchReviewWrite(t, root, "plans/platform/PM-038/plan.yaml", "plan:\n  status: draft\n")
	branchReviewGit(t, root, "add", ".")
	branchReviewGit(t, root, "commit", "-m", "reviewed plan")
	branchReviewGit(t, root, "switch", "main")

	dataDir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dataDir, "workspaces.yaml"), git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	idx := itemindex.New(filepath.Join(dataDir, "items.yaml"))
	scan := scanner.New(git)
	files := fileaccess.New()
	writer := itemwriter.New(files, scan, idx, reg)
	handler := New(reg, idx, scan, files, writer, git, nil).Routes()

	checkout := branchReviewRequest(t, handler, http.MethodPost, "/api/workspaces/"+workspace.ID+"/workstream/checkout", `{}`)
	if checkout.Code != http.StatusOK || !bytes.Contains(checkout.Body.Bytes(), []byte(`"sourceMode":"working_tree"`)) {
		t.Fatalf("checkout status=%d body=%s", checkout.Code, checkout.Body.String())
	}
	legacy := branchReviewRequest(t, handler, http.MethodPost, "/api/workspaces/"+workspace.ID+"/workstream/branch", `{"branch":"feature"}`)
	if legacy.Code != http.StatusConflict || !bytes.Contains(legacy.Body.Bytes(), []byte(`"code":"branch_review_required"`)) {
		t.Fatalf("legacy status=%d body=%s", legacy.Code, legacy.Body.String())
	}
	review := branchReviewRequest(t, handler, http.MethodPost, "/api/workspaces/"+workspace.ID+"/reviews/branch", `{"branch":"feature"}`)
	if review.Code != http.StatusOK {
		t.Fatalf("review status=%d body=%s", review.Code, review.Body.String())
	}
	var result models.WorkstreamBranchLoadResult
	if err := json.Unmarshal(review.Body.Bytes(), &result); err != nil || len(result.Items) != 1 || result.SourceMode != "snapshot" {
		t.Fatalf("review=%+v err=%v", result, err)
	}
	mutation := branchReviewRequest(t, handler, http.MethodPatch, "/api/items/"+result.Items[0].ID+"/metadata", `{"status":"review","materializeConfirmed":true}`)
	if mutation.Code != http.StatusConflict || !bytes.Contains(mutation.Body.Bytes(), []byte(`"code":"snapshot_read_only"`)) {
		t.Fatalf("mutation status=%d body=%s", mutation.Code, mutation.Body.String())
	}
	input, _ := json.Marshal(models.ReviewedPlanImportInput{SourceBranch: "feature", ExpectedCommit: result.Commit, ItemID: result.Items[0].ID})
	imported := branchReviewRequest(t, handler, http.MethodPost, "/api/workspaces/"+workspace.ID+"/reviews/import", string(input))
	if imported.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", imported.Code, imported.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, "plans/platform/PM-038/README.md")); err != nil {
		t.Fatal(err)
	}
}

func branchReviewRequest(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, path, bytes.NewBufferString(body)))
	return response
}

func branchReviewWrite(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func branchReviewGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

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

	"kode-stream/internal/audit"
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
	otherRoot := t.TempDir()
	branchReviewGit(t, otherRoot, "init", "-b", "main")
	branchReviewGit(t, otherRoot, "config", "user.name", "Kode Stream")
	branchReviewGit(t, otherRoot, "config", "user.email", "kode-stream@example.test")
	branchReviewWrite(t, otherRoot, "plans/.keep", "")
	branchReviewGit(t, otherRoot, "add", ".")
	branchReviewGit(t, otherRoot, "commit", "-m", "other workspace")
	otherWorkspace, err := reg.Create(models.WorkspaceInput{Name: "Other Workspace", Path: otherRoot, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	idx := itemindex.New(filepath.Join(dataDir, "items.yaml"))
	scan := scanner.New(git)
	files := fileaccess.New()
	writer := itemwriter.New(files, scan, idx, reg)
	auditStore := audit.New(filepath.Join(dataDir, "audit.jsonl"))
	handler := NewWithReliability(reg, idx, scan, files, writer, git, nil, auditStore, nil).Routes()

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
	operationalItems := branchReviewRequest(t, handler, http.MethodGet, "/api/items?workspaceId="+workspace.ID, "")
	if operationalItems.Code != http.StatusOK || string(operationalItems.Body.Bytes()) != "[]\n" {
		t.Fatalf("snapshot leaked into operational items: status=%d body=%s", operationalItems.Code, operationalItems.Body.String())
	}
	snapshotDetail := branchReviewRequest(t, handler, http.MethodGet, "/api/items/"+result.Items[0].ID, "")
	if snapshotDetail.Code != http.StatusConflict || !bytes.Contains(snapshotDetail.Body.Bytes(), []byte(`"code":"snapshot_review_only"`)) {
		t.Fatalf("snapshot detail status=%d body=%s", snapshotDetail.Code, snapshotDetail.Body.String())
	}
	snapshotDiff := branchReviewRequest(t, handler, http.MethodGet, "/api/items/"+result.Items[0].ID+"/diff", "")
	if snapshotDiff.Code != http.StatusConflict || !bytes.Contains(snapshotDiff.Body.Bytes(), []byte(`"code":"snapshot_review_only"`)) {
		t.Fatalf("snapshot diff status=%d body=%s", snapshotDiff.Code, snapshotDiff.Body.String())
	}
	snapshotSearch := branchReviewRequest(t, handler, http.MethodGet, "/api/items/"+result.Items[0].ID+"/content-search?q=reviewed", "")
	if snapshotSearch.Code != http.StatusConflict || !bytes.Contains(snapshotSearch.Body.Bytes(), []byte(`"code":"snapshot_review_only"`)) {
		t.Fatalf("snapshot content search status=%d body=%s", snapshotSearch.Code, snapshotSearch.Body.String())
	}
	snapshotRunbooks := branchReviewRequest(t, handler, http.MethodGet, "/api/items/"+result.Items[0].ID+"/e2e-runbooks", "")
	if snapshotRunbooks.Code != http.StatusConflict || !bytes.Contains(snapshotRunbooks.Body.Bytes(), []byte(`"code":"snapshot_review_only"`)) {
		t.Fatalf("snapshot E2E runbooks status=%d body=%s", snapshotRunbooks.Code, snapshotRunbooks.Body.String())
	}
	itemFilesPath := "/api/items/" + result.Items[0].ID + "/files"
	missingCommit := branchReviewRequest(t, handler, http.MethodGet, itemFilesPath, "")
	if missingCommit.Code != http.StatusConflict || !bytes.Contains(missingCommit.Body.Bytes(), []byte(`"code":"review_commit_moved"`)) {
		t.Fatalf("missing file commit status=%d body=%s", missingCommit.Code, missingCommit.Body.String())
	}
	wrongCommit := branchReviewRequest(t, handler, http.MethodGet, itemFilesPath+"?expectedCommit=wrong", "")
	if wrongCommit.Code != http.StatusConflict || !bytes.Contains(wrongCommit.Body.Bytes(), []byte(`"code":"review_commit_moved"`)) {
		t.Fatalf("wrong file commit status=%d body=%s", wrongCommit.Code, wrongCommit.Body.String())
	}
	matchingFiles := branchReviewRequest(t, handler, http.MethodGet, itemFilesPath+"?expectedCommit="+result.Commit, "")
	var reviewFiles []models.FileNode
	if err := json.Unmarshal(matchingFiles.Body.Bytes(), &reviewFiles); err != nil || matchingFiles.Code != http.StatusOK || len(reviewFiles) == 0 {
		t.Fatalf("matching files status=%d files=%+v err=%v body=%s", matchingFiles.Code, reviewFiles, err, matchingFiles.Body.String())
	}
	matchingContent := branchReviewRequest(t, handler, http.MethodGet, itemFilesPath+"/"+reviewFiles[0].ID+"?expectedCommit="+result.Commit, "")
	if matchingContent.Code != http.StatusOK {
		t.Fatalf("matching content status=%d body=%s", matchingContent.Code, matchingContent.Body.String())
	}
	wrongContent := branchReviewRequest(t, handler, http.MethodGet, itemFilesPath+"/"+reviewFiles[0].ID+"?expectedCommit=wrong", "")
	if wrongContent.Code != http.StatusConflict || !bytes.Contains(wrongContent.Body.Bytes(), []byte(`"code":"review_commit_moved"`)) {
		t.Fatalf("wrong content commit status=%d body=%s", wrongContent.Code, wrongContent.Body.String())
	}
	mutations := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/items/" + result.Items[0].ID + "/files/README_md", `{"content":"blocked","materializeConfirmed":true}`},
		{http.MethodPatch, "/api/items/" + result.Items[0].ID + "/metadata", `{"status":"review","materializeConfirmed":true}`},
		{http.MethodPatch, "/api/items/" + result.Items[0].ID + "/status", `{"status":"review","materializeConfirmed":true}`},
	}
	for _, request := range mutations {
		mutation := branchReviewRequest(t, handler, request.method, request.path, request.body)
		if mutation.Code != http.StatusConflict || !bytes.Contains(mutation.Body.Bytes(), []byte(`"code":"snapshot_read_only"`)) {
			t.Fatalf("mutation %s status=%d body=%s", request.path, mutation.Code, mutation.Body.String())
		}
	}
	auditEvents, err := auditStore.Recent(len(mutations))
	if err != nil {
		t.Fatal(err)
	}
	if len(auditEvents) != len(mutations) {
		t.Fatalf("audit events = %#v", auditEvents)
	}
	for _, event := range auditEvents {
		if event.WorkspaceID != workspace.ID || event.ItemID != result.Items[0].ID {
			t.Fatalf("unscoped snapshot mutation audit event = %#v", event)
		}
		if event.Status != models.AuditStatusBlocked {
			t.Fatalf("snapshot mutation audit status = %q, want blocked: %#v", event.Status, event)
		}
	}
	input, _ := json.Marshal(models.ReviewedPlanImportInput{SourceBranch: "feature", ExpectedCommit: result.Commit, ExpectedCheckoutBranch: "main", ItemID: result.Items[0].ID})
	wrongWorkspace := branchReviewRequest(t, handler, http.MethodPost, "/api/workspaces/"+otherWorkspace.ID+"/reviews/import", string(input))
	if wrongWorkspace.Code != http.StatusNotFound {
		t.Fatalf("cross-workspace import status=%d body=%s", wrongWorkspace.Code, wrongWorkspace.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, "plans/platform/PM-038")); !os.IsNotExist(err) {
		t.Fatalf("cross-workspace import created target: %v", err)
	}
	branchReviewGit(t, root, "branch", "other")
	branchReviewGit(t, root, "switch", "other")
	changedCheckout := branchReviewRequest(t, handler, http.MethodPost, "/api/workspaces/"+workspace.ID+"/reviews/import", string(input))
	if changedCheckout.Code != http.StatusConflict || !bytes.Contains(changedCheckout.Body.Bytes(), []byte(`"code":"review_checkout_moved"`)) {
		t.Fatalf("changed checkout status=%d body=%s", changedCheckout.Code, changedCheckout.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, "plans/platform/PM-038")); !os.IsNotExist(err) {
		t.Fatalf("changed-checkout import created target: %v", err)
	}
	branchReviewGit(t, root, "switch", "main")
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

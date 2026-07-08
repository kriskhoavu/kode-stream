package api

// Package api provides the Server HTTP transport.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gitadapter "kode-stream/internal/git"
	knowledgeindex "kode-stream/internal/knowledge"
	"kode-stream/internal/workspace/registry"
)

func TestKnowledgeRoutesRequireConfiguredService(t *testing.T) {
	response := httptest.NewRecorder()
	(&API{}).Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/knowledge/wikis?workspaceId=ws", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestKnowledgeErrorMapping(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{knowledgeindex.ErrWorkspaceNotFound, http.StatusNotFound},
		{knowledgeindex.ErrWikiNotFound, http.StatusNotFound},
		{knowledgeindex.ErrPageNotFound, http.StatusNotFound},
		{knowledgeindex.ErrUnsafePath, http.StatusBadRequest},
		{knowledgeindex.ErrKnowledgeDisabled, http.StatusConflict},
	}
	api := &API{}
	for _, test := range tests {
		response := httptest.NewRecorder()
		api.respondKnowledge(response, nil, test.err)
		if response.Code != test.status {
			t.Fatalf("err=%v status=%d body=%s", test.err, response.Code, response.Body.String())
		}
	}
}

func TestKnowledgeActionStatusMapping(t *testing.T) {
	api := &API{}
	tests := []struct {
		result knowledgeindex.KnowledgeActionResult
		err    error
		status int
	}{
		{knowledgeindex.KnowledgeActionResult{}, knowledgeindex.ErrConfirmationRequired, http.StatusConflict},
		{knowledgeindex.KnowledgeActionResult{}, knowledgeindex.ErrEnrichNotConfigured, http.StatusConflict},
		{knowledgeindex.KnowledgeActionResult{}, knowledgeindex.ErrKnowledgeDisabled, http.StatusConflict},
		{knowledgeindex.KnowledgeActionResult{OK: false, Operation: "sync", Message: "pull failed"}, nil, http.StatusUnprocessableEntity},
		{knowledgeindex.KnowledgeActionResult{OK: true, Operation: "rescan"}, nil, http.StatusOK},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		api.respondKnowledgeAction(response, test.result, test.err)
		if response.Code != test.status {
			t.Fatalf("result=%#v err=%v status=%d", test.result, test.err, response.Code)
		}
	}
}

func TestKnowledgeRouteCanBeSaved(t *testing.T) {
	if !validAppRoute("/knowledge") || !validAppRoute("/knowledge?workspace=ws&root=docs") {
		t.Fatal("knowledge route should be valid")
	}
}

func TestKnowledgeHTTPContractsReturnListsAndMissingResources(t *testing.T) {
	directory, workspaceRoot := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nslug: index\ntitle: Index\n---\n# Index\n"
	if err := os.WriteFile(filepath.Join(workspaceRoot, "docs", "index.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(directory, "workspaces.yaml")
	if err := os.WriteFile(registryPath, []byte("- id: ws\n  name: Workspace\n  path: "+workspaceRoot+"\n  sources: [docs]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reg := registry.New(registryPath, gitadapter.New())
	store := knowledgeindex.NewStore(filepath.Join(directory, "knowledge-index.yaml"))
	page := knowledgeindex.KnowledgePage{Slug: "index", Title: "Index", Path: "index.md", Domain: "root", Roles: []string{}, Topics: []string{}, Links: []knowledgeindex.KnowledgeLink{}, Backlinks: []string{}}
	if err := store.ReplaceWorkspace("ws", []knowledgeindex.KnowledgeWiki{{Root: "docs", DisplayName: "Docs", Pages: []knowledgeindex.KnowledgePage{page}, Warnings: []knowledgeindex.KnowledgeWarning{}}}); err != nil {
		t.Fatal(err)
	}
	handler := (&API{}).WithKnowledge(knowledgeindex.NewService(reg, store)).Routes()

	for _, request := range []struct {
		path     string
		status   int
		contains string
	}{
		{"/api/knowledge/wikis?workspaceId=ws", http.StatusOK, `"root":"docs"`},
		{"/api/knowledge/wikis/ws/docs/pages", http.StatusOK, `"pages":[`},
		{"/api/knowledge/wikis/ws/docs/pages/index", http.StatusOK, `"kind":"markdown"`},
		{"/api/knowledge/wikis/ws/docs/graph", http.StatusOK, `"nodes":[`},
		{"/api/knowledge/wikis/ws/docs/pages/missing", http.StatusNotFound, `"error"`},
		{"/api/knowledge/wikis/missing/docs/pages", http.StatusNotFound, `"error"`},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, request.path, nil))
		if response.Code != request.status || !strings.Contains(response.Body.String(), request.contains) {
			t.Fatalf("path=%s status=%d body=%s", request.path, response.Code, response.Body.String())
		}
	}
}

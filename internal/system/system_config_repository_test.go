package system

// Configuration repository contract tests.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestResolvePathsIncludesKnowledgeIndexInDataDirectory(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("PLAN_MANAGER_DATA_DIR", directory)

	paths, err := ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.KnowledgeIndexFile != filepath.Join(directory, "knowledge-index.yaml") {
		t.Fatalf("knowledge index = %q", paths.KnowledgeIndexFile)
	}
}

func TestConfigPathsIncludesEffectiveRegistryFile(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("PLAN_MANAGER_DATA_DIR", directory)
	mux := http.NewServeMux()
	NewController(New()).RegisterRoutes(mux)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/system/config-paths", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["registryFile"] != filepath.Join(directory, "workspaces.yaml") {
		t.Fatalf("registryFile = %#v", payload["registryFile"])
	}
}

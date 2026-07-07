package system

// Configuration repository contract tests.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestDefaultDataDirForSupportedOperatingSystems(t *testing.T) {
	tests := []struct {
		name string
		goos string
		env  map[string]string
		home string
		want string
	}{
		{name: "macOS", goos: "darwin", home: "/Users/test", want: filepath.Join("/Users/test", "Library", "Application Support", "plan-manager")},
		{name: "Linux XDG", goos: "linux", env: map[string]string{"XDG_CONFIG_HOME": "/xdg"}, home: "/home/test", want: filepath.Join("/xdg", "plan-manager")},
		{name: "Linux fallback", goos: "linux", home: "/home/test", want: filepath.Join("/home/test", ".config", "plan-manager")},
		{name: "Windows", goos: "windows", env: map[string]string{"AppData": `C:\\Users\\test\\AppData\\Roaming`}, home: `C:\\Users\\test`, want: filepath.Join(`C:\\Users\\test\\AppData\\Roaming`, "plan-manager")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := defaultDataDirForOS(test.goos, func(key string) string { return test.env[key] }, test.home)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("default data dir = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolvePathsPrefersEnvironmentOverride(t *testing.T) {
	override := t.TempDir()
	t.Setenv("PLAN_MANAGER_DATA_DIR", override)
	paths, err := ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.Dir != override || paths.RegistryFile != filepath.Join(override, "workspaces.yaml") {
		t.Fatalf("paths = %+v", paths)
	}
	if _, err := os.Stat(paths.RegistryFile); !os.IsNotExist(err) {
		t.Fatalf("path resolution wrote registry: %v", err)
	}
}

func TestResolvePathsUsesBootstrapOverride(t *testing.T) {
	home := t.TempDir()
	override := filepath.Join(t.TempDir(), "custom-data")
	t.Setenv("HOME", home)
	t.Setenv("PLAN_MANAGER_DATA_DIR", "")
	defaultDir, err := DefaultDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defaultDir, "bootstrap.yaml"), []byte("dataDir: "+override+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, err := ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.Dir != override || paths.RegistryFile != filepath.Join(override, "workspaces.yaml") {
		t.Fatalf("paths = %+v", paths)
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

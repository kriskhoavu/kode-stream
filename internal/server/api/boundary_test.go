package api

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestGinImportsStayInTransportPackage(t *testing.T) {
	root := moduleRoot(t)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == "frontend" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(data), `"github.com/gin-gonic/gin"`) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(filepath.ToSlash(rel), "internal/server/api/") {
			t.Fatalf("Gin import outside transport package: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAPIRoutesDoNotRegisterServeMuxHandlers(t *testing.T) {
	root := moduleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "internal/server/api/api.go"))
	if err != nil {
		t.Fatal(err)
	}
	routePattern := regexp.MustCompile(`mux\.HandleFunc\("(GET|POST|PUT|PATCH|DELETE) (/api/[^"]+)"`)
	if matches := routePattern.FindAllStringSubmatch(string(data), -1); len(matches) > 0 {
		registrations := make([]string, 0, len(matches))
		for _, match := range matches {
			registrations = append(registrations, match[1]+" "+match[2])
		}
		t.Fatalf("API routes must be registered on Gin, found ServeMux registrations: %s", strings.Join(registrations, ", "))
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

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

func TestRouteInventoryCoversServeMuxRoutes(t *testing.T) {
	root := moduleRoot(t)
	inventoryData, err := os.ReadFile(filepath.Join(root, "plans/platform/PM-030/route-inventory.md"))
	if err != nil {
		t.Fatal(err)
	}
	inventory := string(inventoryData)
	routePattern := regexp.MustCompile(`mux\.HandleFunc\("(GET|POST|PUT|PATCH|DELETE) (/api/[^"]+)"`)
	missing := []string{}
	err = filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range routePattern.FindAllStringSubmatch(string(data), -1) {
			rowPattern := regexp.MustCompile(`(?m)^\|[^|]*\|\s*` + regexp.QuoteMeta(match[1]) + `\s*\|\s*` + regexp.QuoteMeta("`"+match[2]+"`") + `\s*\|`)
			if !rowPattern.MatchString(inventory) {
				missing = append(missing, match[1]+" "+match[2])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) > 0 {
		t.Fatalf("route inventory missing routes: %s", strings.Join(missing, ", "))
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

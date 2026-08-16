package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
)

// An item created with scope == source root lands at plans/<identifier>,
// and must be scanned as an item, not mistaken for a scope.
func TestScanSurfacesSourceRootScopeItem(t *testing.T) {
	root := t.TempDir()
	itemRoot := filepath.Join(root, "plans", "DI-510")
	if err := os.MkdirAll(itemRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemRoot, "README.md"), []byte("# DI-510\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// a sibling normal item so the source is not a document collection
	sibling := filepath.Join(root, "plans", "api", "DI-170")
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling, "README.md"), []byte("# DI-170\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := New(gitadapter.New())
	data, err := s.ScanWithRequest(ScanRequest{
		Workspace:  models.WorkspaceConfig{Path: root, Sources: []string{"plans"}},
		Branch:     "master",
		SourceMode: "working_tree",
		Reader:     NewFilesystemSourceReader(root),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range data.Items {
		t.Logf("scanned item: path=%s scope=%s id=%s", it.ItemPath, it.Scope, it.Identifier)
	}
	found := false
	for _, it := range data.Items {
		if it.Identifier == "DI-510" {
			found = true
		}
	}
	if !found {
		t.Fatalf("DI-510 exists on disk at plans/DI-510 but was not scanned (%d items)", len(data.Items))
	}
}

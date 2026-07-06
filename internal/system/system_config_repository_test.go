package system

// Configuration repository contract tests.

import (
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

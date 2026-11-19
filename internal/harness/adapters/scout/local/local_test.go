package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
)

func TestPollReadsEachTaskFileAsOneTask(t *testing.T) {
	root := t.TempDir()
	tasks := filepath.Join(root, "tasks")
	if err := os.Mkdir(tasks, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(tasks, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("SANDBOX-1.md", "# Add a Goodbye function\n\nAdd Goodbye(name).\n")
	write("SANDBOX-2.md", "No heading here.\n")
	write("notes.txt", "# Not a task\n")
	write("not a key.md", "# Not a task either\n")

	scout := New()
	settings := domain.WorkspaceSettings{Local: &domain.LocalSourceSettings{Enabled: true, Dir: "tasks"}}
	workspace := models.WorkspaceConfig{ID: "sandbox", Path: root}
	if !scout.Enabled(workspace, settings) {
		t.Fatal("the local source should be enabled")
	}
	candidates, err := scout.Poll(context.Background(), workspace, settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates = %#v", candidates)
	}
	first := candidates[0]
	if first.Source.Kind != domain.SourceLocal || first.Source.Key != "SANDBOX-1" || first.Title != "Add a Goodbye function" || first.Body != "Add Goodbye(name)." || first.WorkspaceID != "sandbox" {
		t.Fatalf("first = %#v", first)
	}
	if second := candidates[1]; second.Title != "SANDBOX-2" || second.Body != "No heading here." {
		t.Fatalf("second = %#v", second)
	}

	// An absolute folder is read as it is.
	settings.Local.Dir = tasks
	if candidates, err := scout.Poll(context.Background(), models.WorkspaceConfig{ID: "sandbox", Path: "/elsewhere"}, settings); err != nil || len(candidates) != 2 {
		t.Fatalf("absolute: %d candidates, err=%v", len(candidates), err)
	}
}

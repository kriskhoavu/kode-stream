package workspace

import (
	"path/filepath"
	"testing"
	"time"

	"plan-manager/internal/gitadapter"
	"plan-manager/internal/itemindex"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
)

func TestStateReflectsWorkspaceAndItemChanges(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "workspaces.yaml")
	indexPath := filepath.Join(dir, "item-index.yaml")
	reg := registry.New(registryPath, gitadapter.New())
	idx := itemindex.New(indexPath)
	service := New(reg, idx, nil, nil)

	first, err := service.State()
	if err != nil {
		t.Fatal(err)
	}
	if first.WorkspaceCount != 0 || first.ItemCount != 0 {
		t.Fatalf("unexpected empty state: %+v", first)
	}

	updatedAt := time.Date(2026, 6, 20, 1, 2, 3, 0, time.UTC)
	if err := idx.ReplaceWorkspace("workspace-1", []models.ItemDetail{{
		ItemSummary: models.ItemSummary{
			ID:             "item-1",
			WorkspaceID:    "workspace-1",
			WorkspaceName:  "Workspace",
			Branch:         "main",
			Scope:          "platform",
			Identifier:     "PM-003",
			Title:          "Architecture",
			Status:         models.StatusDraft,
			UpdatedAt:      updatedAt,
			MetadataSource: "item.yaml",
		},
	}}, nil, updatedAt); err != nil {
		t.Fatal(err)
	}

	next, err := service.State()
	if err != nil {
		t.Fatal(err)
	}
	if next.ItemCount != 1 {
		t.Fatalf("item count = %d, want 1", next.ItemCount)
	}
	if next.Version == first.Version {
		t.Fatal("state version should change when indexed items change")
	}
}

func TestNonNilWarningsReturnsEmptySlice(t *testing.T) {
	if got := NonNilWarnings(nil); got == nil || len(got) != 0 {
		t.Fatalf("NonNilWarnings(nil) = %#v", got)
	}
}

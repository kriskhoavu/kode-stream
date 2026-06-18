package itemindex

import (
	"path/filepath"
	"testing"
	"time"

	"plan-manager/internal/models"
)

func TestDeleteWorkspaceRemovesPlansAndKeepsOthers(t *testing.T) {
	idx := New(filepath.Join(t.TempDir(), "items.yaml"))
	if err := idx.ReplaceWorkspace("workspace-a", []models.ItemDetail{
		{ItemSummary: models.ItemSummary{ID: "a-1", WorkspaceID: "workspace-a", Title: "A"}},
	}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := idx.ReplaceWorkspace("workspace-b", []models.ItemDetail{
		{ItemSummary: models.ItemSummary{ID: "b-1", WorkspaceID: "workspace-b", Title: "B"}},
	}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}

	if err := idx.DeleteWorkspace("workspace-a"); err != nil {
		t.Fatal(err)
	}

	items, err := idx.Query(Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "b-1" {
		t.Fatalf("items = %#v, want only workspace-b item", items)
	}
	if _, ok, err := idx.Get("a-1"); err != nil || ok {
		t.Fatalf("workspace-a item still exists: ok=%v err=%v", ok, err)
	}
}

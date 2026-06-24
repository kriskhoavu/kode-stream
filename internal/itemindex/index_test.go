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

func TestReplaceWorkspaceBranchPreservesOtherBranches(t *testing.T) {
	idx := New(filepath.Join(t.TempDir(), "items.yaml"))
	now := time.Now().UTC()
	if err := idx.ReplaceWorkspaceBranch("workspace-a", "main", []models.ItemDetail{
		{ItemSummary: models.ItemSummary{ID: "main-1", WorkspaceID: "workspace-a", Branch: "main", Title: "Main"}},
	}, models.BranchScanMetadata{ScannedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := idx.ReplaceWorkspaceBranch("workspace-a", "feature", []models.ItemDetail{
		{ItemSummary: models.ItemSummary{ID: "feature-1", WorkspaceID: "workspace-a", Branch: "feature", Title: "Feature"}},
	}, models.BranchScanMetadata{BranchRef: "refs/heads/feature", Commit: "abc", SourceMode: "snapshot", ScannedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := idx.ReplaceWorkspaceBranch("workspace-a", "main", []models.ItemDetail{
		{ItemSummary: models.ItemSummary{ID: "main-2", WorkspaceID: "workspace-a", Branch: "main", Title: "Main 2"}},
	}, models.BranchScanMetadata{ScannedAt: now.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}

	mainItems, err := idx.BranchItems("workspace-a", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(mainItems) != 1 || mainItems[0].ID != "main-2" {
		t.Fatalf("main items = %#v", mainItems)
	}
	featureItems, err := idx.BranchItems("workspace-a", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if len(featureItems) != 1 || featureItems[0].ID != "feature-1" {
		t.Fatalf("feature items = %#v", featureItems)
	}
	metadata, ok, err := idx.BranchScan("workspace-a", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || metadata.BranchRef != "refs/heads/feature" || metadata.Commit != "abc" || metadata.SourceMode != "snapshot" {
		t.Fatalf("metadata = %#v ok=%v", metadata, ok)
	}
}

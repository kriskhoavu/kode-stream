package itemindex

// Package itemindex persists the Item domain read model.

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"kode-stream/internal/common/models"
)

func TestReplaceWorkspaceBranchRestoresInMemoryStateWhenPersistenceFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.yaml")
	idx := New(path)
	now := time.Now().UTC()
	previousItem := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "main-before", WorkspaceID: "workspace-a", Branch: "main", Title: "Before"}}
	previousMetadata := models.BranchScanMetadata{Commit: "before", SourceMode: "working_tree", Editable: true, ScannedAt: now, Warnings: []models.ScanWarning{{Message: "before warning"}}}
	if err := idx.ReplaceWorkspaceBranch("workspace-a", "main", []models.ItemDetail{previousItem}, previousMetadata); err != nil {
		t.Fatal(err)
	}
	before := cloneState(idx.state)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	err := idx.ReplaceWorkspaceBranch("workspace-a", "main", []models.ItemDetail{{ItemSummary: models.ItemSummary{ID: "ghost-import", WorkspaceID: "workspace-a", Branch: "main", Title: "Ghost"}}}, models.BranchScanMetadata{Commit: "after", SourceMode: "working_tree", Editable: true, ScannedAt: now.Add(time.Second)})
	if err == nil {
		t.Fatal("expected persistence failure")
	}
	if !reflect.DeepEqual(idx.state, before) {
		t.Fatalf("in-memory state changed after persistence failure:\n before=%#v\n after=%#v", before, idx.state)
	}
	items, err := idx.BranchItems("workspace-a", "main")
	if err != nil || len(items) != 1 || items[0].ID != previousItem.ID {
		t.Fatalf("items after failed replacement=%#v err=%v", items, err)
	}
	if _, ok, err := idx.Get("ghost-import"); err != nil || ok {
		t.Fatalf("ghost item remains: ok=%v err=%v", ok, err)
	}
}

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

func TestQueryExcludesSnapshotsUnlessExplicitlyIncluded(t *testing.T) {
	idx := New(filepath.Join(t.TempDir(), "items.yaml"))
	now := time.Now().UTC()
	if err := idx.ReplaceWorkspaceBranch("workspace-a", "main", []models.ItemDetail{{
		ItemSummary: models.ItemSummary{ID: "checkout-item", WorkspaceID: "workspace-a", Branch: "main", SourceMode: "working_tree"},
	}}, models.BranchScanMetadata{SourceMode: "working_tree", ScannedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := idx.ReplaceWorkspaceBranch("workspace-a", "feature", []models.ItemDetail{{
		ItemSummary: models.ItemSummary{ID: "review-item", WorkspaceID: "workspace-a", Branch: "feature", SourceMode: "snapshot"},
	}}, models.BranchScanMetadata{SourceMode: "snapshot", ScannedAt: now}); err != nil {
		t.Fatal(err)
	}

	operational, err := idx.Query(Query{WorkspaceID: "workspace-a"})
	if err != nil || len(operational) != 1 || operational[0].ID != "checkout-item" {
		t.Fatalf("operational items = %#v, err = %v", operational, err)
	}
	review, err := idx.BranchItems("workspace-a", "feature")
	if err != nil || len(review) != 1 || review[0].ID != "review-item" {
		t.Fatalf("review items = %#v, err = %v", review, err)
	}
	all, err := idx.Query(Query{WorkspaceID: "workspace-a", IncludeSnapshots: true})
	if err != nil || len(all) != 2 {
		t.Fatalf("all items = %#v, err = %v", all, err)
	}
}

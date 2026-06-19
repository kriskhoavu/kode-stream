package item

import (
	"path/filepath"
	"testing"
	"time"

	"plan-manager/internal/fileaccess"
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/itemindex"
	"plan-manager/internal/itemwriter"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
	"plan-manager/internal/scanner"
)

func TestDetailNormalizesCollectionsAndReadsFullReadmeDescription(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "plans/platform/PM-003/README.md", "# PM-003\n\nFull paragraph from README.\n")
	registryPath := filepath.Join(root, "workspaces.yaml")
	indexPath := filepath.Join(root, "item-index.yaml")
	reg := registry.New(registryPath, gitadapter.New())
	idx := itemindex.New(indexPath)
	files := fileaccess.New()
	git := gitadapter.New()
	writer := itemwriter.New(files, scanner.New(git), idx, reg)
	service := New(reg, idx, files, writer, git)
	createdAt := time.Date(2026, 6, 20, 1, 0, 0, 0, time.UTC)

	writeFile(t, root, "workspaces.yaml", `- id: workspace-1
  name: Workspace
  path: `+root+`
  baselineBranch: main
  sources:
    - plans
  createdAt: `+createdAt.Format(time.RFC3339)+`
`)
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
			MetadataSource: "item.yaml",
			ItemPath:       "plans/platform/PM-003",
		},
	}}, nil, createdAt); err != nil {
		t.Fatal(err)
	}

	detail, err := service.Detail("item-1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Description != "Full paragraph from README." {
		t.Fatalf("description = %q", detail.Description)
	}
	if detail.Tags == nil || detail.Documents == nil || detail.Metadata == nil {
		t.Fatalf("detail should normalize nil collections: %+v", detail)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := osMkdirAll(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(path, content); err != nil {
		t.Fatal(err)
	}
}

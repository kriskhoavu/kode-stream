package item

// Item service contract tests.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/content"
	gitadapter "kode-stream/internal/git"
	"kode-stream/internal/item/index"
	"kode-stream/internal/item/writer"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
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
			MetadataSource: "plan.yaml",
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

func TestVerificationTestsPersistSelectedSpecs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "plans/platform/PM-029/plan.yaml", "plan:\n  status: draft\nautomation-test-paths:\n  - path: \"\"\n")
	registryPath := filepath.Join(root, "workspaces.yaml")
	indexPath := filepath.Join(root, "item-index.yaml")
	reg := registry.New(registryPath, gitadapter.New())
	idx := itemindex.New(indexPath)
	files := fileaccess.New()
	git := gitadapter.New()
	writer := itemwriter.New(files, scanner.New(git), idx, reg)
	service := New(reg, idx, files, writer, git)
	createdAt := time.Date(2026, 7, 11, 1, 0, 0, 0, time.UTC)

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
			Identifier:     "PM-029",
			Title:          "Automation runner",
			Status:         models.StatusDraft,
			MetadataSource: "plan.yaml",
			ItemPath:       "plans/platform/PM-029",
		},
	}}, nil, createdAt); err != nil {
		t.Fatal(err)
	}

	saved, err := service.SaveVerificationTests("item-1", models.VerificationTestSelection{
		SelectedSpecs: []string{" cypress/e2e/create-offer.cy.ts ", "cypress/e2e/create-offer.cy.ts"},
		Environment:   " nightly ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := saved.Selection.SelectedSpecs; len(got) != 1 || got[0] != "cypress/e2e/create-offer.cy.ts" {
		t.Fatalf("selected specs = %#v", got)
	}
	if saved.Selection.Environment != "nightly" || saved.Selection.UpdatedAt.IsZero() {
		t.Fatalf("selection = %#v", saved.Selection)
	}
	data, err := os.ReadFile(filepath.Join(root, "plans/platform/PM-029/plan.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if text := string(data); !strings.Contains(text, "verificationTests:") || !strings.Contains(text, "cypress/e2e/create-offer.cy.ts") {
		t.Fatalf("plan.yaml =\n%s", text)
	}
	if !strings.Contains(string(data), "automation-test-paths:") {
		t.Fatalf("plan.yaml lost automation-test-paths:\n%s", string(data))
	}
}

func TestDiscoverVerificationSpecsPrefersAutomationPlanYAML(t *testing.T) {
	automationRepo := t.TempDir()
	writeFile(t, automationRepo, "plans/platform/PM-029/plan.yaml", `plan:
  status: draft
automation-test-paths:
  - path: cypress/e2e/create-offer.cy.ts
  - path: ""
  - path: playwright/create-offer.spec.ts
`)
	writeFile(t, automationRepo, "plans/platform/PM-029/test-plan.md", `# PM-029

Spec: cypress/e2e/old-markdown.cy.ts
`)
	workspace := models.WorkspaceConfig{
		Runtime: &models.WorkspaceRuntimeConfig{
			Automation: &models.RuntimeAutomationConfig{
				Enabled:        true,
				RepositoryPath: automationRepo,
				Runner:         models.AutomationRunnerCypress,
			},
		},
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "item-1", Scope: "platform", Identifier: "PM-029", Title: "Automation runner"}}

	specs, err := DiscoverVerificationSpecs(workspace, item)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Fatalf("specs = %#v", specs)
	}
	if specs[0].Path != "cypress/e2e/create-offer.cy.ts" || specs[0].Runner != "cypress" || specs[0].SourcePath != "plans/platform/PM-029/plan.yaml" {
		t.Fatalf("first spec = %#v", specs[0])
	}
	if specs[1].Path != "playwright/create-offer.spec.ts" || specs[1].Runner != "playwright" || specs[1].SourcePath != "plans/platform/PM-029/plan.yaml" {
		t.Fatalf("second spec = %#v", specs[1])
	}
}

func TestDiscoverVerificationSpecsReadsCurrentItemPlanYAML(t *testing.T) {
	workspaceRoot := t.TempDir()
	automationRepo := t.TempDir()
	writeFile(t, workspaceRoot, "plans/api/DI-170/plan.yaml", `plan:
  status: done
automation-test-paths:
  - path: cypress/e2e/01-base/01-logging-to-console.cy.ts
`)
	writeFile(t, automationRepo, "plans/api/DI-170/plan.yaml", `plan:
  status: draft
automation-test-paths:
  - path: ""
`)
	workspace := models.WorkspaceConfig{
		Path: workspaceRoot,
		Runtime: &models.WorkspaceRuntimeConfig{
			Automation: &models.RuntimeAutomationConfig{
				Enabled:        true,
				RepositoryPath: automationRepo,
				Runner:         models.AutomationRunnerCypress,
			},
		},
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "item-1", Scope: "api", Identifier: "DI-170", Title: "Custom Assortment", ItemPath: "plans/api/DI-170"}}

	specs, err := DiscoverVerificationSpecs(workspace, item)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 {
		t.Fatalf("specs = %#v", specs)
	}
	if specs[0].Path != "cypress/e2e/01-base/01-logging-to-console.cy.ts" || specs[0].SourcePath != "plans/api/DI-170/plan.yaml" {
		t.Fatalf("spec = %#v", specs[0])
	}
}

func TestDiscoverVerificationSpecsIgnoresAutomationPlanMarkdown(t *testing.T) {
	automationRepo := t.TempDir()
	writeFile(t, automationRepo, "plans/PM-029/test-plan.md", `# PM-029

Spec: cypress/e2e/create-offer.cy.ts
Future: playwright/create-offer.spec.ts
`)
	workspace := models.WorkspaceConfig{
		Runtime: &models.WorkspaceRuntimeConfig{
			Automation: &models.RuntimeAutomationConfig{
				Enabled:        true,
				RepositoryPath: automationRepo,
				Runner:         models.AutomationRunnerCypress,
			},
		},
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "item-1", Identifier: "PM-029", Title: "Automation runner"}}

	specs, err := DiscoverVerificationSpecs(workspace, item)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 0 {
		t.Fatalf("specs = %#v", specs)
	}
}

func TestSnapshotMaterializationBlocksExistingTargetFiles(t *testing.T) {
	root := newItemGitRepo(t)
	writeItemGitFile(t, root, "plans/platform/PM-013/README.md", "# Existing\n")
	writeItemGitFile(t, root, "plans/platform/PM-013/plan.yaml", "plan:\n  status: draft\n")
	itemGitCommit(t, root, "main item")
	itemGitRun(t, root, "switch", "-c", "feature")
	writeItemGitFile(t, root, "plans/platform/PM-013/README.md", "# Snapshot\n")
	itemGitCommit(t, root, "snapshot item")
	itemGitRun(t, root, "switch", "main")

	registryPath := filepath.Join(t.TempDir(), "workspaces.yaml")
	indexPath := filepath.Join(t.TempDir(), "item-index.yaml")
	git := gitadapter.New()
	reg := registry.New(registryPath, git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	idx := itemindex.New(indexPath)
	files := fileaccess.New()
	writer := itemwriter.New(files, scanner.New(git), idx, reg)
	service := New(reg, idx, files, writer, git)
	ref, commit, err := git.ResolveBranch(root, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.ReplaceWorkspaceBranch(workspace.ID, "feature", []models.ItemDetail{{
		ItemSummary: models.ItemSummary{
			ID:             "snapshot-item",
			WorkspaceID:    workspace.ID,
			WorkspaceName:  workspace.Name,
			Branch:         "feature",
			BranchRef:      ref,
			Commit:         commit,
			SourceMode:     "snapshot",
			Editable:       false,
			Scope:          "platform",
			Identifier:     "PM-013",
			Title:          "Snapshot",
			Status:         models.StatusDraft,
			MetadataSource: "plan.yaml",
			ItemPath:       "plans/platform/PM-013",
		},
	}}, models.BranchScanMetadata{ScannedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}

	_, err = service.SaveMetadata("snapshot-item", models.ItemMetadataUpdateInput{Status: models.StatusReview, MaterializeConfirmed: true})
	if err == nil || !strings.Contains(err.Error(), "files already exist") {
		t.Fatalf("expected conflict error, got %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "plans/platform/PM-013/README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Existing\n" {
		t.Fatalf("existing checkout file was overwritten: %q", data)
	}
}

func TestWorkingTreeWriteRequiresCurrentCheckoutBranch(t *testing.T) {
	root := newItemGitRepo(t)
	writeItemGitFile(t, root, "plans/platform/PM-013/README.md", "# Existing\n")
	writeItemGitFile(t, root, "plans/platform/PM-013/plan.yaml", "plan:\n  status: draft\n")
	itemGitCommit(t, root, "main item")
	itemGitRun(t, root, "branch", "feature")

	registryPath := filepath.Join(t.TempDir(), "workspaces.yaml")
	indexPath := filepath.Join(t.TempDir(), "item-index.yaml")
	git := gitadapter.New()
	reg := registry.New(registryPath, git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	idx := itemindex.New(indexPath)
	files := fileaccess.New()
	writer := itemwriter.New(files, scanner.New(git), idx, reg)
	service := New(reg, idx, files, writer, git)
	if err := idx.ReplaceWorkspaceBranch(workspace.ID, "feature", []models.ItemDetail{{
		ItemSummary: models.ItemSummary{
			ID:             "feature-item",
			WorkspaceID:    workspace.ID,
			WorkspaceName:  workspace.Name,
			Branch:         "feature",
			SourceMode:     "working_tree",
			Editable:       true,
			Scope:          "platform",
			Identifier:     "PM-013",
			Title:          "Feature",
			Status:         models.StatusDraft,
			MetadataSource: "plan.yaml",
			ItemPath:       "plans/platform/PM-013",
		},
	}}, models.BranchScanMetadata{ScannedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}

	_, err = service.SaveMetadata("feature-item", models.ItemMetadataUpdateInput{Status: models.StatusReview})
	if err == nil || !strings.Contains(err.Error(), "not the current checkout branch") {
		t.Fatalf("expected current checkout branch error, got %v", err)
	}
}

func TestSnapshotFileContentResolvesNestedDocsPath(t *testing.T) {
	root := newItemGitRepo(t)
	writeItemGitFile(t, root, "docs/a12/a12-challenges-in-discovery-epsap.md", "# Challenge\n")
	writeItemGitFile(t, root, "docs/a12/a12-in-discovery.md", "# Discovery\n")
	itemGitCommit(t, root, "add docs")

	registryPath := filepath.Join(t.TempDir(), "workspaces.yaml")
	indexPath := filepath.Join(t.TempDir(), "item-index.yaml")
	git := gitadapter.New()
	reg := registry.New(registryPath, git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"docs"}})
	if err != nil {
		t.Fatal(err)
	}
	idx := itemindex.New(indexPath)
	files := fileaccess.New()
	writer := itemwriter.New(files, scanner.New(git), idx, reg)
	service := New(reg, idx, files, writer, git)
	ref, commit, err := git.ResolveBranch(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.ReplaceWorkspaceBranch(workspace.ID, "main", []models.ItemDetail{{
		ItemSummary: models.ItemSummary{
			ID:             "snapshot-docs",
			WorkspaceID:    workspace.ID,
			WorkspaceName:  workspace.Name,
			Branch:         "main",
			BranchRef:      ref,
			Commit:         commit,
			SourceMode:     "snapshot",
			Editable:       false,
			Scope:          "docs",
			Identifier:     "docs",
			Title:          "Docs",
			Status:         models.StatusUnsorted,
			MetadataSource: "docs",
			ItemPath:       "docs",
		},
		Documents: []models.ItemDocument{{Path: "a12/a12-challenges-in-discovery-epsap.md"}, {Path: "a12/a12-in-discovery.md"}},
	}}, models.BranchScanMetadata{ScannedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}

	tree, err := service.Files("snapshot-docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].Path != "a12" || len(tree[0].Children) != 2 {
		t.Fatalf("unexpected tree: %#v", tree)
	}
	if tree[0].Children[0].ID != "a12__a12-challenges-in-discovery-epsap_md" {
		t.Fatalf("unexpected file id: %q", tree[0].Children[0].ID)
	}
	content, err := service.FileContent("snapshot-docs", tree[0].Children[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if content.Path != "a12/a12-challenges-in-discovery-epsap.md" {
		t.Fatalf("path = %q", content.Path)
	}
	if !strings.Contains(content.Content, "Challenge") {
		t.Fatalf("content = %q", content.Content)
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

func newItemGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	itemGitRun(t, root, "config", "user.name", "Kode Stream")
	itemGitRun(t, root, "config", "user.email", "kode-stream@example.test")
	return root
}

func writeItemGitFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func itemGitCommit(t *testing.T, root, message string) {
	t.Helper()
	itemGitRun(t, root, "add", ".")
	itemGitRun(t, root, "commit", "-m", message)
}

func itemGitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}

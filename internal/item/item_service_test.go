package item

// Item service contract tests.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apperrors "kode-stream/internal/common"
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
	writeFile(t, root, "plans/platform/PM-029/plan.yaml", "plan:\n  status: draft\nautomation-test:\n  - path: \"\"\n")
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
	if !strings.Contains(string(data), "automation-test:") {
		t.Fatalf("plan.yaml lost automation-test:\n%s", string(data))
	}
}

func TestDiscoverVerificationSpecsPrefersAutomationPlanYAML(t *testing.T) {
	automationRepo := t.TempDir()
	writeFile(t, automationRepo, "plans/platform/PM-029/plan.yaml", `plan:
  status: draft
automation-test:
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
automation-test:
  - path: cypress/e2e/01-base/01-logging-to-console.cy.ts
`)
	writeFile(t, automationRepo, "plans/api/DI-170/plan.yaml", `plan:
  status: draft
automation-test:
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

func TestSnapshotVerificationTestsReadCapturedCommit(t *testing.T) {
	root := newItemGitRepo(t)
	writeItemGitFile(t, root, "plans/platform/PM-038/plan.yaml", `plan:
  status: draft
verificationTests:
  selectedSpecs:
    - checkout.cy.ts
automation-test:
  - path: checkout.cy.ts
`)
	itemGitCommit(t, root, "checkout plan")
	itemGitRun(t, root, "switch", "-c", "feature")
	writeItemGitFile(t, root, "plans/platform/PM-038/plan.yaml", `plan:
  status: draft
verificationTests:
  selectedSpecs:
    - reviewed.cy.ts
automation-test:
  - path: reviewed.cy.ts
`)
	itemGitCommit(t, root, "captured review")
	git := gitadapter.New()
	ref, commit, err := git.ResolveBranch(root, "feature")
	if err != nil {
		t.Fatal(err)
	}
	writeItemGitFile(t, root, "plans/platform/PM-038/plan.yaml", `plan:
  status: done
verificationTests:
  selectedSpecs:
    - advanced.cy.ts
automation-test:
  - path: advanced.cy.ts
`)
	itemGitCommit(t, root, "advanced review")
	itemGitRun(t, root, "switch", "main")

	dataDir := t.TempDir()
	reg := registry.New(filepath.Join(dataDir, "workspaces.yaml"), git)
	workspace, err := reg.Create(models.WorkspaceInput{
		Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"},
		Runtime: &models.WorkspaceRuntimeConfig{
			Type: models.RuntimeTypeCustom, RebuildPolicy: models.RebuildPolicyNever,
			Commands:   models.RuntimeCommandSet{Up: "true", Down: "true", Verify: models.RuntimeVerifyCommands{Smoke: "true"}},
			Automation: &models.RuntimeAutomationConfig{Enabled: true, RepositoryPath: root, Runner: models.AutomationRunnerCypress},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	idx := itemindex.New(filepath.Join(dataDir, "items.yaml"))
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "snapshot-verification", WorkspaceID: workspace.ID, Branch: "feature", BranchRef: ref, Commit: commit, SourceMode: "snapshot", Scope: "platform", Identifier: "PM-038", MetadataSource: "plan.yaml", ItemPath: "plans/platform/PM-038"}}
	if err := idx.ReplaceWorkspaceBranch(workspace.ID, "feature", []models.ItemDetail{item}, models.BranchScanMetadata{Branch: "feature", Commit: commit, SourceMode: "snapshot", ScannedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	files := fileaccess.New()
	service := New(reg, idx, files, itemwriter.New(files, scanner.New(git), idx, reg), git)

	result, err := service.VerificationTests(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Selection.SelectedSpecs) != 1 || result.Selection.SelectedSpecs[0] != "reviewed.cy.ts" {
		t.Fatalf("snapshot selection followed mutable content: %#v", result.Selection)
	}
	if len(result.DiscoveredSpecs) != 1 || result.DiscoveredSpecs[0].Path != "reviewed.cy.ts" || result.DiscoveredSpecs[0].SourcePath != "plans/platform/PM-038/plan.yaml" {
		t.Fatalf("snapshot discovery followed mutable content: %#v", result.DiscoveredSpecs)
	}
}

func TestSnapshotEditsAreReadOnly(t *testing.T) {
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
	if !errors.Is(err, ErrSnapshotReadOnly) {
		t.Fatalf("expected snapshot read-only error, got %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "plans/platform/PM-013/README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Existing\n" {
		t.Fatalf("existing checkout file was overwritten: %q", data)
	}
	_, err = service.ImportReviewedPlan(workspace.ID, models.ReviewedPlanImportInput{SourceBranch: "feature", ExpectedCommit: commit, ExpectedCheckoutBranch: "main", ItemID: "snapshot-item"})
	if err == nil || !strings.Contains(err.Error(), "already exist") {
		t.Fatalf("expected import conflict, got %v", err)
	}
}

func TestImportReviewedPlanCopiesPinnedStructuredPlan(t *testing.T) {
	root := newItemGitRepo(t)
	writeItemGitFile(t, root, "README.md", "# Workspace\n")
	writeItemGitFile(t, root, "plans/.keep", "")
	itemGitCommit(t, root, "main")
	itemGitRun(t, root, "switch", "-c", "feature")
	writeItemGitFile(t, root, "plans/platform/PM-038/README.md", "# PM-038: Review import\n")
	writeItemGitFile(t, root, "plans/platform/PM-038/plan.yaml", "plan:\n  status: draft\n")
	itemGitCommit(t, root, "reviewed plan")
	itemGitRun(t, root, "switch", "main")

	dir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dir, "workspaces.yaml"), git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	idx := itemindex.New(filepath.Join(dir, "items.yaml"))
	ref, commit, err := git.ResolveBranch(root, "feature")
	if err != nil {
		t.Fatal(err)
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "review-item", WorkspaceID: workspace.ID, WorkspaceName: workspace.Name, Branch: "feature", BranchRef: ref, Commit: commit, SourceMode: "snapshot", Editable: false, Scope: "platform", Identifier: "PM-038", Title: "Review import", Status: models.StatusDraft, MetadataSource: "plan.yaml", ItemPath: "plans/platform/PM-038"}}
	if err := idx.ReplaceWorkspaceBranch(workspace.ID, "feature", []models.ItemDetail{item}, models.BranchScanMetadata{Branch: "feature", Commit: commit, SourceMode: "snapshot", ScannedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	service := New(reg, idx, fileaccess.New(), itemwriter.New(fileaccess.New(), scanner.New(git), idx, reg), git)
	ownershipLocked := make(chan struct{})
	ownershipRelease := make(chan struct{})
	go func() {
		_ = git.WithWorkspaceMutation(root, func() error {
			close(ownershipLocked)
			<-ownershipRelease
			return nil
		})
	}()
	<-ownershipLocked
	wrongWorkspaceDone := make(chan error, 1)
	go func() {
		_, importErr := service.ImportReviewedPlan("other-workspace", models.ReviewedPlanImportInput{SourceBranch: "feature", ExpectedCommit: commit, ExpectedCheckoutBranch: "main", ItemID: item.ID})
		wrongWorkspaceDone <- importErr
	}()
	select {
	case importErr := <-wrongWorkspaceDone:
		if !errors.Is(importErr, apperrors.ErrItemNotFound) {
			t.Fatalf("cross-workspace import error=%v", importErr)
		}
	case <-time.After(50 * time.Millisecond):
		close(ownershipRelease)
		t.Fatal("cross-workspace ownership check waited for the mutation lock")
	}
	close(ownershipRelease)
	itemGitRun(t, root, "branch", "other")
	itemGitRun(t, root, "switch", "other")
	_, err = service.ImportReviewedPlan(workspace.ID, models.ReviewedPlanImportInput{SourceBranch: "feature", ExpectedCommit: commit, ExpectedCheckoutBranch: "main", ItemID: item.ID})
	if !errors.Is(err, ErrReviewCheckoutMoved) {
		t.Fatalf("expected checkout change rejection, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "plans/platform/PM-038")); !os.IsNotExist(statErr) {
		t.Fatalf("changed-checkout import created target, err=%v", statErr)
	}
	itemGitRun(t, root, "switch", "main")
	locked := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = git.WithWorkspaceMutation(root, func() error {
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked
	type importResult struct {
		result models.WriteResult
		err    error
	}
	done := make(chan importResult, 1)
	go func() {
		result, importErr := service.ImportReviewedPlan(workspace.ID, models.ReviewedPlanImportInput{SourceBranch: "feature", ExpectedCommit: commit, ExpectedCheckoutBranch: "main", ItemID: item.ID})
		done <- importResult{result: result, err: importErr}
	}()
	select {
	case outcome := <-done:
		t.Fatalf("import completed while workspace mutation was locked: %v", outcome.err)
	case <-time.After(50 * time.Millisecond):
	}
	if _, statErr := os.Stat(filepath.Join(root, "plans/platform/PM-038")); !os.IsNotExist(statErr) {
		t.Fatalf("blocked import created target, err=%v", statErr)
	}
	close(release)
	outcome := <-done
	result, err := outcome.result, outcome.err
	if err != nil {
		t.Fatal(err)
	}
	if result.Item.Branch != "main" || result.Item.ItemPath != item.ItemPath {
		t.Fatalf("import result=%+v", result.Item)
	}
	data, err := os.ReadFile(filepath.Join(root, "plans/platform/PM-038/README.md"))
	if err != nil || string(data) != "# PM-038: Review import\n" {
		t.Fatalf("imported README=%q err=%v", data, err)
	}
	current, _ := git.CurrentBranch(root)
	if current != "main" {
		t.Fatalf("import switched checkout to %q", current)
	}
}

func TestImportReviewedPlanRejectsOccupiedTargetRootWithoutMatchingFiles(t *testing.T) {
	root := newItemGitRepo(t)
	writeItemGitFile(t, root, "plans/.keep", "")
	itemGitCommit(t, root, "main")
	itemGitRun(t, root, "switch", "-c", "feature")
	writeItemGitFile(t, root, "plans/platform/PM-038/README.md", "# PM-038: Review import\n")
	writeItemGitFile(t, root, "plans/platform/PM-038/plan.yaml", "plan:\n  status: draft\n")
	itemGitCommit(t, root, "reviewed plan")
	itemGitRun(t, root, "switch", "main")
	writeItemGitFile(t, root, "plans/platform/PM-038/unrelated.txt", "preserve me\n")

	dir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dir, "workspaces.yaml"), git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	idx := itemindex.New(filepath.Join(dir, "items.yaml"))
	ref, commit, err := git.ResolveBranch(root, "feature")
	if err != nil {
		t.Fatal(err)
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "review-item", WorkspaceID: workspace.ID, WorkspaceName: workspace.Name, Branch: "feature", BranchRef: ref, Commit: commit, SourceMode: "snapshot", Scope: "platform", Identifier: "PM-038", MetadataSource: "plan.yaml", ItemPath: "plans/platform/PM-038"}}
	if err := idx.ReplaceWorkspaceBranch(workspace.ID, "feature", []models.ItemDetail{item}, models.BranchScanMetadata{Branch: "feature", Commit: commit, SourceMode: "snapshot", ScannedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	service := New(reg, idx, fileaccess.New(), itemwriter.New(fileaccess.New(), scanner.New(git), idx, reg), git)
	_, err = service.ImportReviewedPlan(workspace.ID, models.ReviewedPlanImportInput{SourceBranch: "feature", ExpectedCommit: commit, ExpectedCheckoutBranch: "main", ItemID: item.ID})
	if err == nil || !strings.Contains(err.Error(), "target already exists") {
		t.Fatalf("expected occupied root rejection, got %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(root, "plans/platform/PM-038/unrelated.txt"))
	if readErr != nil || string(data) != "preserve me\n" {
		t.Fatalf("unrelated target content=%q err=%v", data, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "plans/platform/PM-038/README.md")); !os.IsNotExist(statErr) {
		t.Fatalf("import merged into occupied root, err=%v", statErr)
	}
	if err := os.Remove(filepath.Join(root, "plans/platform/PM-038/unrelated.txt")); err != nil {
		t.Fatal(err)
	}
	_, err = service.ImportReviewedPlan(workspace.ID, models.ReviewedPlanImportInput{SourceBranch: "feature", ExpectedCommit: commit, ExpectedCheckoutBranch: "main", ItemID: item.ID})
	if err == nil || !strings.Contains(err.Error(), "target already exists") {
		t.Fatalf("expected empty root rejection, got %v", err)
	}
}

func TestSnapshotImportReadsCapturedCommitAfterBranchAdvances(t *testing.T) {
	root := newItemGitRepo(t)
	writeItemGitFile(t, root, "plans/.keep", "")
	itemGitCommit(t, root, "main")
	itemGitRun(t, root, "switch", "-c", "feature")
	writeItemGitFile(t, root, "plans/platform/PM-038/README.md", "# Captured\n")
	itemGitCommit(t, root, "captured snapshot")
	git := gitadapter.New()
	ref, commit, err := git.ResolveBranch(root, "feature")
	if err != nil {
		t.Fatal(err)
	}
	writeItemGitFile(t, root, "plans/platform/PM-038/README.md", "# Advanced\n")
	itemGitCommit(t, root, "advance branch")
	itemGitRun(t, root, "switch", "main")

	writer := itemwriter.New(fileaccess.New(), scanner.New(git), nil, nil)
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"plans"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{Branch: "feature", BranchRef: ref, Commit: commit, SourceMode: "snapshot", MetadataSource: "plan.yaml", ItemPath: "plans/platform/PM-038"}}
	if err := writer.MaterializeSnapshotItem(workspace, item, ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "plans/platform/PM-038/README.md"))
	if err != nil || string(data) != "# Captured\n" {
		t.Fatalf("imported content=%q err=%v", data, err)
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

	tree, err := service.Files("snapshot-docs", commit)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].Path != "a12" || len(tree[0].Children) != 2 {
		t.Fatalf("unexpected tree: %#v", tree)
	}
	if tree[0].Children[0].ID != "a12__a12-challenges-in-discovery-epsap_md" {
		t.Fatalf("unexpected file id: %q", tree[0].Children[0].ID)
	}
	content, err := service.FileContent("snapshot-docs", tree[0].Children[0].ID, commit)
	if err != nil {
		t.Fatal(err)
	}
	if content.Path != "a12/a12-challenges-in-discovery-epsap.md" {
		t.Fatalf("path = %q", content.Path)
	}
	if !strings.Contains(content.Content, "Challenge") {
		t.Fatalf("content = %q", content.Content)
	}
	writeItemGitFile(t, root, "docs/a12/a12-challenges-in-discovery-epsap.md", "# Advanced\n")
	writeItemGitFile(t, root, "docs/a12/new-after-review.md", "# New\n")
	itemGitCommit(t, root, "advance reviewed branch")
	pinnedTree, err := service.Files("snapshot-docs", commit)
	if err != nil {
		t.Fatal(err)
	}
	if len(pinnedTree) != 1 || len(pinnedTree[0].Children) != 2 {
		t.Fatalf("snapshot tree followed mutable branch: %#v", pinnedTree)
	}
	pinnedContent, err := service.FileContent("snapshot-docs", tree[0].Children[0].ID, commit)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pinnedContent.Content, "Challenge") || strings.Contains(pinnedContent.Content, "Advanced") {
		t.Fatalf("snapshot content followed mutable branch: %q", pinnedContent.Content)
	}
	advancedRef, advancedCommit, err := git.ResolveBranch(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	advancedItem, ok, err := idx.Get("snapshot-docs")
	if err != nil || !ok {
		t.Fatalf("advanced item: ok=%v err=%v", ok, err)
	}
	advancedItem.BranchRef = advancedRef
	advancedItem.Commit = advancedCommit
	if err := idx.ReplaceWorkspaceBranch(workspace.ID, "main", []models.ItemDetail{advancedItem}, models.BranchScanMetadata{Commit: advancedCommit, SourceMode: "snapshot", ScannedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Files("snapshot-docs", commit); !errors.Is(err, ErrReviewCommitMoved) {
		t.Fatalf("stale expected tree commit error=%v", err)
	}
	if _, err := service.FileContent("snapshot-docs", tree[0].Children[0].ID, commit); !errors.Is(err, ErrReviewCommitMoved) {
		t.Fatalf("stale expected content commit error=%v", err)
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

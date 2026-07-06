package knowledge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	"plan-manager/internal/gitadapter"
	knowledgeindex "plan-manager/internal/knowledge"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
)

type stubDetector struct {
	wikis          []knowledgeindex.KnowledgeWiki
	err            error
	calls          int
	workspaceCalls int
	sourceCalls    int
}

func (d *stubDetector) DetectWorkspace(_ context.Context, _ models.WorkspaceConfig) ([]knowledgeindex.KnowledgeWiki, error) {
	d.calls++
	d.workspaceCalls++
	return d.wikis, d.err
}

func (d *stubDetector) DetectSource(_ context.Context, _ models.WorkspaceConfig, source string) (knowledgeindex.KnowledgeWiki, bool, error) {
	d.calls++
	d.sourceCalls++
	if d.err != nil {
		return knowledgeindex.KnowledgeWiki{}, false, d.err
	}
	for _, wiki := range d.wikis {
		if wiki.Root == source {
			return wiki, true, nil
		}
	}
	return knowledgeindex.KnowledgeWiki{}, false, nil
}

type stubPuller struct {
	result models.GitOperationResult
	input  models.GitOperationInput
}

func (p *stubPuller) Pull(_ string, input models.GitOperationInput) models.GitOperationResult {
	p.input = input
	return p.result
}

type stubAudit struct{ events []models.AuditEvent }

func (a *stubAudit) Append(event models.AuditEvent) (models.AuditEvent, error) {
	a.events = append(a.events, event)
	return event, nil
}

func TestQueriesReturnIndexedPagesAndGuardedMarkdown(t *testing.T) {
	service, workspaceID := newKnowledgeService(t)

	wikis, err := service.Wikis(workspaceID)
	if err != nil || len(wikis) != 1 {
		t.Fatalf("wikis=%#v err=%v", wikis, err)
	}
	pages, warnings, err := service.Pages(workspaceID, "docs")
	if err != nil || len(pages) != 2 || warnings == nil {
		t.Fatalf("pages=%#v warnings=%#v err=%v", pages, warnings, err)
	}
	detail, err := service.Page(workspaceID, "docs", "guide")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Content.Kind != "markdown" || detail.Content.Content == "" || detail.Content.Editable {
		t.Fatalf("detail = %#v", detail)
	}
	graph, err := service.Graph(workspaceID, "docs")
	if err != nil || len(graph.Nodes) != 2 || len(graph.Edges) != 1 {
		t.Fatalf("graph=%#v err=%v", graph, err)
	}
}

func TestQueriesReturnStableMissingAndUnsafeErrors(t *testing.T) {
	service, workspaceID := newKnowledgeService(t)
	if _, err := service.Wikis("missing"); err != ErrWorkspaceNotFound {
		t.Fatalf("workspace err = %v", err)
	}
	if _, _, err := service.Pages(workspaceID, "missing"); err != ErrWikiNotFound {
		t.Fatalf("wiki err = %v", err)
	}
	if _, err := service.Page(workspaceID, "docs", "missing"); err != ErrPageNotFound {
		t.Fatalf("page err = %v", err)
	}
	if _, _, err := service.Pages(workspaceID, "../docs"); err != ErrUnsafePath {
		t.Fatalf("unsafe err = %v", err)
	}
}

func TestRescanAndSyncReplaceOnlyAfterSuccessfulDetection(t *testing.T) {
	service, store := newActionService(t, "", nil)
	detector := &stubDetector{wikis: []knowledgeindex.KnowledgeWiki{{Root: "docs", Pages: []knowledgeindex.KnowledgePage{}, Warnings: []knowledgeindex.KnowledgeWarning{}}}}
	puller, audits := &stubPuller{result: models.GitOperationResult{OK: true}}, &stubAudit{}
	service.ConfigureActions(detector, puller, audits)
	result, err := service.Rescan(context.Background(), "ws", "docs")
	if err != nil || !result.OK {
		t.Fatalf("rescan=%#v err=%v", result, err)
	}
	if detector.sourceCalls != 1 || detector.workspaceCalls != 0 {
		t.Fatalf("rescan calls: source=%d workspace=%d", detector.sourceCalls, detector.workspaceCalls)
	}
	if len(audits.events) != 1 || audits.events[0].Operation != "knowledge_rescan" {
		t.Fatalf("audits=%#v", audits.events)
	}
	result, err = service.Sync(context.Background(), "ws", models.GitOperationInput{Confirm: true})
	if err != nil || !result.OK || !puller.input.Confirm {
		t.Fatalf("sync=%#v err=%v input=%#v", result, err, puller.input)
	}

	detector.err = errors.New("scan failed")
	puller.result = models.GitOperationResult{OK: false, Message: "confirm to pull"}
	result, err = service.Sync(context.Background(), "ws", models.GitOperationInput{})
	if err != nil || result.OK || result.Message != "confirm to pull" {
		t.Fatalf("failed sync=%#v err=%v", result, err)
	}
	wikis, err := store.List("ws")
	if err != nil || len(wikis) != 1 || wikis[0].Root != "docs" {
		t.Fatalf("preserved wikis=%#v err=%v", wikis, err)
	}
}

func TestEnrichRequiresConfirmationAndConfiguration(t *testing.T) {
	service, _ := newActionService(t, "", nil)
	if _, err := service.Enrich(context.Background(), "ws", false); err != ErrConfirmationRequired {
		t.Fatalf("confirmation err=%v", err)
	}
	if _, err := service.Enrich(context.Background(), "ws", true); err != ErrEnrichNotConfigured {
		t.Fatalf("configuration err=%v", err)
	}
}

func TestDisabledKnowledgeRejectsActionsAndHidesPersistedWikis(t *testing.T) {
	disabled := false
	directory, workspaceRoot := t.TempDir(), t.TempDir()
	workspace := models.WorkspaceConfig{ID: "ws", Name: "Workspace", Path: workspaceRoot, Sources: []string{"docs"}, Knowledge: &models.KnowledgeSettings{Enabled: &disabled, EnrichExecutable: "/bin/echo"}}
	data, err := yaml.Marshal([]models.WorkspaceConfig{workspace})
	if err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(directory, "workspaces.yaml")
	if err := os.WriteFile(registryPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	store := knowledgeindex.NewStore(filepath.Join(directory, "knowledge-index.yaml"))
	if err := store.ReplaceWorkspace("ws", []knowledgeindex.KnowledgeWiki{{Root: "docs", Pages: []knowledgeindex.KnowledgePage{}, Warnings: []knowledgeindex.KnowledgeWarning{}}}); err != nil {
		t.Fatal(err)
	}
	service := New(registry.New(registryPath, gitadapter.New()), store)
	wikis, err := service.Wikis("ws")
	if err != nil || len(wikis) != 0 {
		t.Fatalf("wikis=%#v err=%v", wikis, err)
	}
	service.ConfigureActions(&stubDetector{}, &stubPuller{}, &stubAudit{})
	if _, err := service.Rescan(context.Background(), "ws", "docs"); err != ErrKnowledgeDisabled {
		t.Fatalf("rescan err=%v", err)
	}
	if _, err := service.Sync(context.Background(), "ws", models.GitOperationInput{}); err != ErrKnowledgeDisabled {
		t.Fatalf("sync err=%v", err)
	}
	if _, err := service.Enrich(context.Background(), "ws", true); err != ErrKnowledgeDisabled {
		t.Fatalf("enrich err=%v", err)
	}
}

func TestEnrichPassesLiteralArgumentsRescansAndBoundsOutput(t *testing.T) {
	workspace := t.TempDir()
	script := filepath.Join(workspace, "enrich.sh")
	content := "#!/bin/sh\nprintf '%s' \"$1\" > args.txt\nhead -c 70000 /dev/zero | tr '\\0' x\n"
	if err := os.WriteFile(script, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	service, _ := newActionServiceAt(t, workspace, script, []string{"literal $HOME ; value"})
	detector, audits := &stubDetector{wikis: []knowledgeindex.KnowledgeWiki{{Root: "docs", Pages: []knowledgeindex.KnowledgePage{}, Warnings: []knowledgeindex.KnowledgeWarning{}}}}, &stubAudit{}
	service.ConfigureActions(detector, nil, audits)
	result, err := service.Enrich(context.Background(), "ws", true)
	if err != nil || !result.OK || !result.LogTruncated || len(result.Log) != maxActionLogBytes {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	argument, err := os.ReadFile(filepath.Join(workspace, "args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(argument) != "literal $HOME ; value" {
		t.Fatalf("argument=%q", argument)
	}
	if detector.calls != 1 || len(audits.events) != 1 || audits.events[0].Status != models.AuditStatusSuccess {
		t.Fatalf("detector=%d audits=%#v", detector.calls, audits.events)
	}
}

func TestEnrichReportsStartExitAndTimeoutFailuresWithoutRescan(t *testing.T) {
	tests := []struct {
		name, script string
		timeout      time.Duration
	}{
		{"start", "", time.Second},
		{"exit", "#!/bin/sh\necho failed\nexit 7\n", time.Second},
		{"timeout", "#!/bin/sh\nsleep 5\n", 20 * time.Millisecond},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			executable := filepath.Join(workspace, "missing")
			if test.script != "" {
				executable = filepath.Join(workspace, "tool.sh")
				if err := os.WriteFile(executable, []byte(test.script), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			service, _ := newActionServiceAt(t, workspace, executable, nil)
			service.enrichTimeout = test.timeout
			detector, audits := &stubDetector{}, &stubAudit{}
			service.ConfigureActions(detector, nil, audits)
			result, err := service.Enrich(context.Background(), "ws", true)
			if err != nil || result.OK || result.Message == "" || detector.calls != 0 {
				t.Fatalf("result=%#v err=%v calls=%d", result, err, detector.calls)
			}
			if len(audits.events) != 1 || audits.events[0].Status != models.AuditStatusFailed {
				t.Fatalf("audits=%#v", audits.events)
			}
		})
	}
}

func newKnowledgeService(t *testing.T) (*Service, string) {
	t.Helper()
	directory, workspaceRoot := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	indexContent := "---\nslug: index\ntitle: Index\n---\n[[guide]]\n"
	guideContent := "---\nslug: guide\ntitle: Guide\n---\n# Guide\n"
	if err := os.WriteFile(filepath.Join(workspaceRoot, "docs", "index.md"), []byte(indexContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "docs", "guide.md"), []byte(guideContent), 0o644); err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(directory, "workspaces.yaml")
	registryData := "- id: ws\n  name: Workspace\n  path: " + workspaceRoot + "\n  baselineBranch: main\n  sources: [docs]\n"
	if err := os.WriteFile(registryPath, []byte(registryData), 0o600); err != nil {
		t.Fatal(err)
	}
	reg := registry.New(registryPath, gitadapter.New())
	store := knowledgeindex.NewStore(filepath.Join(directory, "knowledge-index.yaml"))
	pages := []knowledgeindex.KnowledgePage{
		{Slug: "index", Title: "Index", Path: "index.md", Domain: "root", Roles: []string{}, Topics: []string{}, Links: []knowledgeindex.KnowledgeLink{{SourceSlug: "index", RawTarget: "guide", Resolution: knowledgeindex.LinkResolved, TargetSlug: "guide"}}, Backlinks: []string{}},
		{Slug: "guide", Title: "Guide", Path: "guide.md", Domain: "root", Roles: []string{}, Topics: []string{}, Links: []knowledgeindex.KnowledgeLink{}, Backlinks: []string{"index"}},
	}
	if err := store.ReplaceWorkspace("ws", []knowledgeindex.KnowledgeWiki{{Root: "docs", DisplayName: "Docs", Pages: pages, Warnings: []knowledgeindex.KnowledgeWarning{}}}); err != nil {
		t.Fatal(err)
	}
	return New(reg, store), "ws"
}

func newActionService(t *testing.T, executable string, args []string) (*Service, *knowledgeindex.Store) {
	t.Helper()
	return newActionServiceAt(t, t.TempDir(), executable, args)
}

func newActionServiceAt(t *testing.T, workspaceRoot, executable string, args []string) (*Service, *knowledgeindex.Store) {
	t.Helper()
	directory := t.TempDir()
	settings := &models.KnowledgeSettings{EnrichExecutable: executable, EnrichArgs: args}
	workspace := models.WorkspaceConfig{ID: "ws", Name: "Workspace", Path: workspaceRoot, Sources: []string{"docs"}, Knowledge: settings}
	data, err := yaml.Marshal([]models.WorkspaceConfig{workspace})
	if err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(directory, "workspaces.yaml")
	if err := os.WriteFile(registryPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	store := knowledgeindex.NewStore(filepath.Join(directory, "knowledge-index.yaml"))
	if err := store.ReplaceWorkspace("ws", []knowledgeindex.KnowledgeWiki{{Root: "old", Pages: []knowledgeindex.KnowledgePage{}, Warnings: []knowledgeindex.KnowledgeWarning{}}}); err != nil {
		t.Fatal(err)
	}
	return New(registry.New(registryPath, gitadapter.New()), store), store
}

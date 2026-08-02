package ai

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gitadapter "kode-stream/internal/git"
)

func TestEmbeddedLaunchPersistsSafeRecordAndIsIdempotent(t *testing.T) {
	service, item, workspace, _, _, _ := launchTestService(t, true)
	manager := NewTerminalManager(Config{})
	t.Cleanup(func() { _ = manager.Close() })
	recordPath := filepath.Join(t.TempDir(), "session-records.yaml")
	records := NewFileSessionRecordRepository(recordPath)
	service.ConfigureEmbedded(manager).ConfigureSessionRecords(records, gitadapter.New())

	input := EmbeddedInput{Provider: "test-ai", ContextMode: "card_context", PromptDraft: "PRIVATE-PROMPT-CONTENT", ExpectedWorkspaceID: item.WorkspaceID, ExpectedBranch: item.Branch, IdempotencyKey: "launch-once"}
	first, err := service.StartEmbedded(item.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	gitCommand(t, workspace.Path, "checkout", "-b", "after-launch")
	second, err := service.StartEmbedded(item.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Session.ID == "" || second.Record == nil || second.Record.ID != first.Session.ID || len(manager.List()) != 1 {
		t.Fatalf("first=%#v second=%#v sessions=%#v", first, second, manager.List())
	}
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "PRIVATE-PROMPT-CONTENT") {
		t.Fatalf("durable record leaked prompt: %s", data)
	}
	if !strings.Contains(string(data), "launch-once") || !strings.Contains(string(data), item.ItemPath) {
		t.Fatalf("durable record lacks safe references: %s", data)
	}
}

func TestEmbeddedLaunchRejectsBranchMismatchWithRecoveryDetails(t *testing.T) {
	service, item, workspace, _, _, _ := launchTestService(t, true)
	manager := NewTerminalManager(Config{})
	t.Cleanup(func() { _ = manager.Close() })
	records := NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "session-records.yaml"))
	service.ConfigureEmbedded(manager).ConfigureSessionRecords(records, gitadapter.New())
	gitCommand(t, workspace.Path, "checkout", "-b", "other")
	if err := os.WriteFile(filepath.Join(workspace.Path, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := service.StartEmbedded(item.ID, EmbeddedInput{Provider: "test-ai", ContextMode: "card_context", ExpectedWorkspaceID: workspace.ID, ExpectedBranch: "main"})
	var launchErr *LaunchError
	if !errors.As(err, &launchErr) || launchErr.Code != "terminal_branch_mismatch" {
		t.Fatalf("err=%#v", err)
	}
	if launchErr.Details["expectedBranch"] != "main" || launchErr.Details["currentBranch"] != "other" || launchErr.Details["dirty"] != "true" {
		t.Fatalf("details=%#v", launchErr.Details)
	}
	if len(manager.List()) != 0 {
		t.Fatalf("branch mismatch started sessions: %#v", manager.List())
	}
}

func TestEmbeddedLaunchRejectsRevisionMismatch(t *testing.T) {
	service, item, _, _, _, _ := launchTestService(t, true)
	manager := NewTerminalManager(Config{})
	t.Cleanup(func() { _ = manager.Close() })
	service.ConfigureEmbedded(manager).ConfigureSessionRecords(NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "session-records.yaml")), gitadapter.New())

	_, err := service.StartEmbedded(item.ID, EmbeddedInput{Provider: "test-ai", ContextMode: "card_context", ExpectedBranch: "main", ObservedCommit: "stale-commit"})
	var launchErr *LaunchError
	if !errors.As(err, &launchErr) || launchErr.Code != "terminal_revision_mismatch" || launchErr.Details["currentCommit"] == "" {
		t.Fatalf("err=%#v", err)
	}
	if len(manager.List()) != 0 {
		t.Fatalf("revision mismatch started sessions: %#v", manager.List())
	}
}

func TestSessionRecordReconciliationMarksOrphanedProcessInterrupted(t *testing.T) {
	repository := NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "session-records.yaml"))
	now := time.Now().UTC()
	_, err := repository.Upsert(SessionRecord{ID: "orphan", WorkspaceID: "workspace", Provider: "codex", Intent: "workspace_only", RequestedBranch: "main", State: StateRunning, StartedAt: now, LastKnownAt: now})
	if err != nil {
		t.Fatal(err)
	}
	service := New(NewSettingsRepository(filepath.Join(t.TempDir(), "ai-settings.yaml"))).ConfigureEmbedded(NewTerminalManager(Config{})).ConfigureSessionRecords(repository, nil)
	t.Cleanup(func() { _ = service.EmbeddedManager().Close() })
	record, found, err := repository.Get("orphan")
	if err != nil || !found || record.State != StateInterrupted || record.EndedAt.IsZero() {
		t.Fatalf("record=%#v found=%v err=%v", record, found, err)
	}
}

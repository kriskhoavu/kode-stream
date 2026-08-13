package ai

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestFailedEmbeddedReservationDoesNotPoisonRetryKey(t *testing.T) {
	service, item, workspace, _, _, _ := launchTestService(t, true)
	manager := NewTerminalManager(Config{MaxSessions: 1})
	t.Cleanup(func() { _ = manager.Close() })
	if _, _, err := manager.Start(StartRequest{ID: "occupying", WorkspaceID: workspace.ID, Executable: "/bin/sh", Args: []string{"-c", "sleep 10"}, Dir: workspace.Path}); err != nil {
		t.Fatal(err)
	}
	records := NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "records.yaml"))
	service.ConfigureEmbedded(manager).ConfigureSessionRecords(records, gitadapter.New())
	_, err := service.StartEmbedded(item.ID, EmbeddedInput{Provider: "test-ai", ContextMode: "card_context", ExpectedWorkspaceID: workspace.ID, ExpectedBranch: item.Branch, IdempotencyKey: "retry-key"})
	if err == nil {
		t.Fatal("expected session limit failure")
	}
	if _, found, findErr := records.FindByIdempotency(workspace.ID, "retry-key"); findErr != nil || found {
		t.Fatalf("failed reservation found=%v err=%v", found, findErr)
	}
}

func TestConcurrentSameKeyEmbeddedLaunchesCoalesceAndRetryAfterFailure(t *testing.T) {
	service, item, workspace, _, _, _ := launchTestService(t, true)
	manager := NewTerminalManager(Config{})
	t.Cleanup(func() { _ = manager.Close() })
	records := NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "records.yaml"))
	service.ConfigureEmbedded(manager).ConfigureSessionRecords(records, gitadapter.New())
	input := EmbeddedInput{Provider: "test-ai", ContextMode: "card_context", ExpectedWorkspaceID: workspace.ID, ExpectedBranch: item.Branch, IdempotencyKey: "same-key"}
	results := make([]EmbeddedResult, 2)
	errs := make([]error, 2)
	var group sync.WaitGroup
	for index := range results {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			results[index], errs[index] = service.StartEmbedded(item.ID, input)
		}(index)
	}
	group.Wait()
	if errs[0] != nil || errs[1] != nil || results[0].Session.ID == "" || results[0].Session.ID != results[1].Session.ID || len(manager.List()) != 1 {
		t.Fatalf("results=%#v errs=%v sessions=%#v", results, errs, manager.List())
	}
}

type failNthRecordRepository struct {
	SessionRecordRepository
	mu            sync.Mutex
	failAt        int
	calls         int
	failRunningAt int
	runningCalls  int
}

func (r *failNthRecordRepository) Upsert(record SessionRecord) (SessionRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.calls == r.failAt {
		return SessionRecord{}, errors.New("injected record write failure")
	}
	if record.State == StateRunning {
		r.runningCalls++
		if r.runningCalls == r.failRunningAt {
			return SessionRecord{}, errors.New("injected running record write failure")
		}
	}
	return r.SessionRecordRepository.Upsert(record)
}

func TestEmbeddedLaunchObserverFailureCancelsAndRetryWorksForItemAndWorkspace(t *testing.T) {
	service, item, workspace, _, _, _ := launchTestService(t, true)
	settings, err := service.Settings()
	if err != nil {
		t.Fatal(err)
	}
	executable := settings.Providers["test-ai"].Executable
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nsleep 10\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	manager := NewTerminalManager(Config{})
	t.Cleanup(func() { _ = manager.Close() })
	base := NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "records.yaml"))
	// reserve succeeds; running-state observer fails; terminal cancellation and
	// failed-record compensation follow, then the same key can be retried.
	records := &failNthRecordRepository{SessionRecordRepository: base, failRunningAt: 2}
	service.ConfigureEmbedded(manager).ConfigureSessionRecords(records, gitadapter.New())
	itemInput := EmbeddedInput{Provider: "test-ai", ContextMode: "card_context", ExpectedWorkspaceID: workspace.ID, ExpectedBranch: item.Branch, IdempotencyKey: "item-retry"}
	if _, err := service.StartEmbedded(item.ID, itemInput); err == nil {
		t.Fatal("expected item observer failure")
	}
	if len(manager.List()) != 1 || manager.List()[0].State != StateCancelled {
		t.Fatalf("orphan process state=%#v", manager.List())
	}
	result, err := service.StartEmbedded(item.ID, itemInput)
	if err != nil || result.Session.ID == "" || result.Grant.Token == "" {
		t.Fatalf("item retry result=%#v err=%v", result, err)
	}

	workspaceInput := EmbeddedInput{Provider: "test-ai", ContextMode: "workspace_only", ExpectedWorkspaceID: workspace.ID, ExpectedBranch: item.Branch, IdempotencyKey: "workspace-retry"}
	workspaceResult, workspaceErr := service.StartEmbeddedWorkspace(workspace.ID, workspaceInput)
	if workspaceErr != nil || workspaceResult.Session.ID == "" || workspaceResult.Grant.Token == "" {
		t.Fatalf("workspace retry result=%#v err=%v", workspaceResult, workspaceErr)
	}
}

func TestEmbeddedReservationFailureStartsNoProcessForItemOrWorkspace(t *testing.T) {
	service, item, workspace, _, _, _ := launchTestService(t, true)
	manager := NewTerminalManager(Config{})
	t.Cleanup(func() { _ = manager.Close() })
	for _, configureAndLaunch := range []func() error{
		func() error {
			service.ConfigureSessionRecords(&failNthRecordRepository{SessionRecordRepository: NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "item.yaml")), failAt: 1}, gitadapter.New())
			_, err := service.StartEmbedded(item.ID, EmbeddedInput{Provider: "test-ai", ContextMode: "card_context", ExpectedWorkspaceID: workspace.ID, ExpectedBranch: item.Branch})
			return err
		},
		func() error {
			service.ConfigureSessionRecords(&failNthRecordRepository{SessionRecordRepository: NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "workspace.yaml")), failAt: 1}, gitadapter.New())
			_, err := service.StartEmbeddedWorkspace(workspace.ID, EmbeddedInput{Provider: "test-ai", ContextMode: "workspace_only", ExpectedWorkspaceID: workspace.ID, ExpectedBranch: item.Branch})
			return err
		},
	} {
		if err := configureAndLaunch(); err == nil {
			t.Fatal("expected reservation failure")
		}
		if len(manager.List()) != 0 {
			t.Fatalf("reservation failure started a process: %#v", manager.List())
		}
	}
}

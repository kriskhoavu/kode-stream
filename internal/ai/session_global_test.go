package ai

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
)

type fakeSessionRecordRepository struct {
	records []SessionRecord
}

func (f *fakeSessionRecordRepository) Get(id string) (SessionRecord, bool, error) {
	for _, record := range f.records {
		if record.ID == id {
			return record, true, nil
		}
	}
	return SessionRecord{}, false, nil
}
func (f *fakeSessionRecordRepository) FindByIdempotency(string, string) (SessionRecord, bool, error) {
	return SessionRecord{}, false, nil
}
func (f *fakeSessionRecordRepository) List(workspaceID, branch string) ([]SessionRecord, error) {
	result := []SessionRecord{}
	for _, record := range f.records {
		if workspaceID != "" && record.WorkspaceID != workspaceID {
			continue
		}
		if branch != "" && record.RequestedBranch != branch {
			continue
		}
		result = append(result, record)
	}
	return result, nil
}
func (f *fakeSessionRecordRepository) Upsert(record SessionRecord) (SessionRecord, error) {
	for i := range f.records {
		if f.records[i].ID == record.ID {
			f.records[i] = record
			return record, nil
		}
	}
	f.records = append(f.records, record)
	return record, nil
}
func (f *fakeSessionRecordRepository) Delete(id string) (bool, error) {
	for index, record := range f.records {
		if record.ID == id {
			f.records = append(f.records[:index], f.records[index+1:]...)
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeSessionRecordRepository) Snapshot() ([]SessionRecord, error) {
	return append([]SessionRecord(nil), f.records...), nil
}
func (f *fakeSessionRecordRepository) ReplaceAll(records []SessionRecord) error {
	f.records = records
	return nil
}

func globalTestService(t *testing.T, workspaces map[string]models.WorkspaceConfig, records *fakeSessionRecordRepository, index *fakeItemIndex, git *fakeChangeGit) *Service {
	t.Helper()
	dataDir := t.TempDir()
	reg := &fakeWorkspaceRegistry{workspaces: workspaces}
	if index == nil {
		index = &fakeItemIndex{items: map[string]models.ItemDetail{}}
	}
	service := New(NewSettingsRepository(filepath.Join(dataDir, "ai-settings.yaml")))
	service.ConfigureLaunch(reg, index, nil, t.TempDir())
	service.ConfigureSessionRecords(records, git)
	return service
}

// The process starts before the session records are wired, so its record reconciles as live.
func liveGlobalTestService(t *testing.T, workspace models.WorkspaceConfig, records *fakeSessionRecordRepository, index *fakeItemIndex, git *fakeChangeGit, liveSessionID string) *Service {
	t.Helper()
	dataDir := t.TempDir()
	reg := &fakeWorkspaceRegistry{workspaces: map[string]models.WorkspaceConfig{workspace.ID: workspace}}
	if index == nil {
		index = &fakeItemIndex{items: map[string]models.ItemDetail{}}
	}
	service := New(NewSettingsRepository(filepath.Join(dataDir, "ai-settings.yaml")))
	service.ConfigureLaunch(reg, index, nil, t.TempDir())
	manager := NewTerminalManager(Config{})
	t.Cleanup(func() { _ = manager.Close() })
	service.ConfigureEmbedded(manager)
	if _, _, err := manager.Start(StartRequest{ID: liveSessionID, WorkspaceID: workspace.ID, Executable: "/bin/sh", Args: []string{"-c", "sleep 5"}, Dir: workspace.Path}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = manager.Cancel(liveSessionID) })
	service.ConfigureSessionRecords(records, git)
	return service
}

func TestSessionRecordsAcrossWorkspacesHidesDeniedWorkspaces(t *testing.T) {
	now := time.Now().UTC()
	workspaces := map[string]models.WorkspaceConfig{
		"allowed": {ID: "allowed", Name: "Allowed", Path: t.TempDir()},
		"denied":  {ID: "denied", Name: "Denied", Path: t.TempDir()},
	}
	records := &fakeSessionRecordRepository{records: []SessionRecord{
		{ID: "s-allowed", WorkspaceID: "allowed", Provider: "claude", Intent: "workspace_only", RequestedBranch: "main", State: StateExited, StartedAt: now, LastKnownAt: now},
		{ID: "s-denied", WorkspaceID: "denied", Provider: "claude", Intent: "workspace_only", RequestedBranch: "main", State: StateExited, StartedAt: now, LastKnownAt: now},
	}}
	service := globalTestService(t, workspaces, records, nil, &fakeChangeGit{})
	mayRead := func(workspaceID string) bool { return workspaceID == "allowed" }
	rows, truncated, err := service.SessionRecordsAcrossWorkspaces(context.Background(), SessionScopeAll, mayRead)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if truncated {
		t.Fatalf("truncated=%v", truncated)
	}
	if len(rows) != 1 || rows[0].Record.ID != "s-allowed" || rows[0].WorkspaceName != "Allowed" {
		t.Fatalf("rows=%#v", rows)
	}
}

func TestSessionRecordsAcrossWorkspacesScopeActiveFiltersLiveOnly(t *testing.T) {
	now := time.Now().UTC()
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "WS", Path: t.TempDir()}
	records := &fakeSessionRecordRepository{records: []SessionRecord{
		{ID: "s-live", WorkspaceID: "ws-1", Provider: "claude", Intent: "workspace_only", RequestedBranch: "main", State: StateRunning, StartedAt: now, LastKnownAt: now},
		{ID: "s-exited", WorkspaceID: "ws-1", Provider: "claude", Intent: "workspace_only", RequestedBranch: "main", State: StateExited, StartedAt: now.Add(time.Second), LastKnownAt: now},
	}}
	service := liveGlobalTestService(t, workspace, records, nil, &fakeChangeGit{}, "s-live")

	rows, _, err := service.SessionRecordsAcrossWorkspaces(context.Background(), SessionScopeActive, func(string) bool { return true })
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(rows) != 1 || rows[0].Record.ID != "s-live" || !rows[0].Record.Live {
		t.Fatalf("rows=%#v, want only the live running session", rows)
	}

	all, _, err := service.SessionRecordsAcrossWorkspaces(context.Background(), SessionScopeAll, func(string) bool { return true })
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(all) != 2 {
		t.Fatalf("all=%#v", all)
	}
}

func TestSessionRecordsAcrossWorkspacesAnnotatesPlanTitleAndBranch(t *testing.T) {
	now := time.Now().UTC()
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "WS", Path: t.TempDir()}
	records := &fakeSessionRecordRepository{records: []SessionRecord{
		{
			ID: "s-1", WorkspaceID: "ws-1", Provider: "claude", Intent: "card_context", RequestedBranch: "main",
			PlanRef: &SessionPlanRef{ItemID: "item-1", ItemPath: "plans/x", BranchKey: "main"},
			State:   StateExited, StartedAt: now, LastKnownAt: now,
		},
	}}
	index := &fakeItemIndex{items: map[string]models.ItemDetail{
		"item-1": {ItemSummary: models.ItemSummary{ID: "item-1", WorkspaceID: "ws-1", Title: "Plan Title"}},
	}}
	git := &fakeChangeGit{currentBranch: "feature-x"}
	service := globalTestService(t, map[string]models.WorkspaceConfig{"ws-1": workspace}, records, index, git)
	rows, _, err := service.SessionRecordsAcrossWorkspaces(context.Background(), SessionScopeAll, func(string) bool { return true })
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(rows) != 1 || rows[0].PlanTitle != "Plan Title" || rows[0].CheckoutBranch != "feature-x" {
		t.Fatalf("rows=%#v", rows)
	}
	if rows[0].ChangesCount != nil {
		t.Fatalf("changesCount=%v, want nil for scope=all", rows[0].ChangesCount)
	}
}

func TestSessionRecordsAcrossWorkspacesComputesChangesCountForActiveOnly(t *testing.T) {
	now := time.Now().UTC()
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "WS", Path: t.TempDir()}
	records := &fakeSessionRecordRepository{records: []SessionRecord{
		{ID: "s-1", WorkspaceID: "ws-1", Provider: "claude", Intent: "workspace_only", RequestedBranch: "main", ObservedCommit: "launch-sha", State: StateRunning, StartedAt: now, LastKnownAt: now},
	}}
	git := &fakeChangeGit{
		currentBranch:  "main",
		resolvedCommit: "head-sha",
		nameStatus:     []gitadapter.NameStatusEntry{{Path: "a.go", Status: "M"}, {Path: "b.go", Status: "A"}},
		revListCount:   1,
	}
	service := liveGlobalTestService(t, workspace, records, nil, git, "s-1")

	active, _, err := service.SessionRecordsAcrossWorkspaces(context.Background(), SessionScopeActive, func(string) bool { return true })
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(active) != 1 || active[0].ChangesCount == nil || *active[0].ChangesCount != 2 {
		t.Fatalf("active=%#v", active)
	}

	all, _, err := service.SessionRecordsAcrossWorkspaces(context.Background(), SessionScopeAll, func(string) bool { return true })
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(all) != 1 || all[0].ChangesCount != nil {
		t.Fatalf("all=%#v, want changesCount omitted outside scope=active", all)
	}
}

func TestSessionRecordsAcrossWorkspacesResolvesBranchOncePerWorkspace(t *testing.T) {
	now := time.Now().UTC()
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "WS", Path: t.TempDir()}
	records := &fakeSessionRecordRepository{records: []SessionRecord{
		{ID: "s-1", WorkspaceID: "ws-1", Provider: "claude", Intent: "workspace_only", RequestedBranch: "main", State: StateExited, StartedAt: now, LastKnownAt: now},
		{ID: "s-2", WorkspaceID: "ws-1", Provider: "claude", Intent: "workspace_only", RequestedBranch: "main", State: StateExited, StartedAt: now.Add(time.Second), LastKnownAt: now},
		{ID: "s-3", WorkspaceID: "ws-1", Provider: "claude", Intent: "workspace_only", RequestedBranch: "main", State: StateExited, StartedAt: now.Add(2 * time.Second), LastKnownAt: now},
	}}
	git := &fakeChangeGit{currentBranch: "feature-x"}
	service := globalTestService(t, map[string]models.WorkspaceConfig{"ws-1": workspace}, records, nil, git)
	rows, _, err := service.SessionRecordsAcrossWorkspaces(context.Background(), SessionScopeAll, func(string) bool { return true })
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows=%#v", rows)
	}
	for _, row := range rows {
		if row.CheckoutBranch != "feature-x" {
			t.Fatalf("row=%#v", row)
		}
	}
	if git.currentBranchCalls != 1 {
		t.Fatalf("currentBranchCalls=%d, want 1 (resolved once per workspace, not per session row)", git.currentBranchCalls)
	}
}

func TestSessionRecordsAcrossWorkspacesRowSurvivesPerRowGitFailure(t *testing.T) {
	now := time.Now().UTC()
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "WS", Path: t.TempDir()}
	records := &fakeSessionRecordRepository{records: []SessionRecord{
		{ID: "s-1", WorkspaceID: "ws-1", Provider: "claude", Intent: "workspace_only", RequestedBranch: "main", ObservedCommit: "launch-sha", State: StateRunning, StartedAt: now, LastKnownAt: now},
	}}
	git := &fakeChangeGit{
		currentBranch:  "main",
		resolvedCommit: "head-sha",
		nameStatusErr:  errors.New("git: name-status failed"),
	}
	service := liveGlobalTestService(t, workspace, records, nil, git, "s-1")

	rows, truncated, err := service.SessionRecordsAcrossWorkspaces(context.Background(), SessionScopeActive, func(string) bool { return true })
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if truncated {
		t.Fatalf("truncated=%v", truncated)
	}
	if len(rows) != 1 || rows[0].Record.ID != "s-1" {
		t.Fatalf("rows=%#v, want the row to survive the per-row Git failure", rows)
	}
	if rows[0].ChangesCount != nil {
		t.Fatalf("changesCount=%v, want nil when the per-row Git call fails", rows[0].ChangesCount)
	}
}

func TestSessionRecordsAcrossWorkspacesOrdersNewestFirstAndTruncates(t *testing.T) {
	now := time.Now().UTC()
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "WS", Path: t.TempDir()}
	total := maxGlobalSessionRows + 5
	sessionRecords := make([]SessionRecord, 0, total)
	for i := 0; i < total; i++ {
		sessionRecords = append(sessionRecords, SessionRecord{
			ID: fmt.Sprintf("s-%03d", i), WorkspaceID: "ws-1", Provider: "claude", Intent: "workspace_only",
			RequestedBranch: "main", State: StateExited,
			StartedAt: now.Add(time.Duration(i) * time.Second), LastKnownAt: now,
		})
	}
	records := &fakeSessionRecordRepository{records: sessionRecords}
	service := globalTestService(t, map[string]models.WorkspaceConfig{"ws-1": workspace}, records, nil, &fakeChangeGit{})
	rows, truncated, err := service.SessionRecordsAcrossWorkspaces(context.Background(), SessionScopeAll, func(string) bool { return true })
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !truncated {
		t.Fatalf("truncated=%v, want true", truncated)
	}
	if len(rows) != maxGlobalSessionRows {
		t.Fatalf("len(rows)=%d, want %d", len(rows), maxGlobalSessionRows)
	}
	if rows[0].Record.ID != fmt.Sprintf("s-%03d", total-1) {
		t.Fatalf("rows[0]=%#v", rows[0])
	}
}

func TestSessionRecordsAcrossWorkspacesPropagatesRegistryError(t *testing.T) {
	records := &fakeSessionRecordRepository{}
	reg := &fakeWorkspaceRegistry{listErr: errors.New("registry unavailable")}
	dataDir := t.TempDir()
	service := New(NewSettingsRepository(filepath.Join(dataDir, "ai-settings.yaml")))
	service.ConfigureLaunch(reg, &fakeItemIndex{items: map[string]models.ItemDetail{}}, nil, t.TempDir())
	service.ConfigureSessionRecords(records, &fakeChangeGit{})
	if _, _, err := service.SessionRecordsAcrossWorkspaces(context.Background(), SessionScopeAll, func(string) bool { return true }); err == nil {
		t.Fatal("expected registry error to propagate")
	}
}

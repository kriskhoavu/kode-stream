package harness

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/storage"
)

type fakeWorkspaces map[string]models.WorkspaceConfig

func (f fakeWorkspaces) Get(id string) (models.WorkspaceConfig, bool, error) {
	workspace, ok := f[id]
	return workspace, ok, nil
}

func (f fakeWorkspaces) List() ([]models.WorkspaceConfig, error) {
	var out []models.WorkspaceConfig
	for _, workspace := range f {
		out = append(out, workspace)
	}
	return out, nil
}

func newTestRepository(t *testing.T) domain.Repository {
	t.Helper()
	repository, closeStore, err := storage.OpenSQLiteHarness(filepath.Join(t.TempDir(), "harness.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closeStore() })
	return repository
}

var testNow = time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)

func newTestService(t *testing.T, repository domain.Repository) *Service {
	t.Helper()
	return New(Dependencies{Repository: repository, Workspaces: fakeWorkspaces{"ws": {ID: "ws", Path: t.TempDir(), BaselineBranch: "main"}}, Now: func() time.Time { return testNow }, Config: Config{LocalMode: true}})
}

func TestServiceWithoutStorageReportsUnsupportedAndRefusesChanges(t *testing.T) {
	service := New(Dependencies{Config: Config{LocalMode: true}})
	status, err := service.Status(context.Background())
	if err != nil || status.StorageSupported || status.Hint != StorageHint {
		t.Fatalf("status = %#v err=%v", status, err)
	}
	if _, err := service.SaveSettings(context.Background(), domain.DefaultSettings(), 0, domain.Actor{}); !IsUnsupported(err) {
		t.Fatalf("save err = %v", err)
	}
	tasks, err := service.Tasks(context.Background(), domain.TaskFilter{})
	if err != nil || len(tasks) != 0 {
		t.Fatalf("tasks = %#v err=%v", tasks, err)
	}
}

func TestServiceInCloudModeRefusesChanges(t *testing.T) {
	service := New(Dependencies{Repository: newTestRepository(t), Config: Config{LocalMode: false}})
	if _, err := service.SaveSettings(context.Background(), domain.DefaultSettings(), 0, domain.Actor{}); !IsUnavailable(err) {
		t.Fatalf("save err = %v", err)
	}
	status, err := service.Status(context.Background())
	if err != nil || status.Mode != "cloud" || !status.StorageSupported {
		t.Fatalf("status = %#v err=%v", status, err)
	}
}

func TestSaveSettingsValidatesWorkspacesAndEmitsAnEvent(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t)
	service := newTestService(t, repository)
	subscription := service.Subscribe(4)
	defer subscription.Cancel()

	settings := domain.DefaultSettings()
	settings.Workspaces = []domain.WorkspaceSettings{{WorkspaceID: "missing"}}
	if _, err := service.SaveSettings(ctx, settings, 0, domain.Actor{}); !IsInvalid(err) {
		t.Fatalf("unknown workspace err = %v", err)
	}
	settings.Enabled = true
	settings.Workspaces = []domain.WorkspaceSettings{{WorkspaceID: "ws"}}
	saved, err := service.SaveSettings(ctx, settings, 0, domain.Actor{ActorID: "local", DisplayName: "Local user"})
	if err != nil || saved.Revision != 1 || saved.Workspaces[0].MaxConcurrentRuns != 1 {
		t.Fatalf("saved = %#v err=%v", saved, err)
	}
	if _, err := service.SaveSettings(ctx, settings, 0, domain.Actor{}); !IsConflict(err) {
		t.Fatalf("stale save err = %v", err)
	}
	select {
	case event := <-subscription.Events:
		if event.Kind != domain.EventSettingsUpdated || event.Seq == 0 || event.Actor == nil {
			t.Fatalf("event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("no settings event")
	}
	status, err := service.Status(ctx)
	if err != nil || !status.Enabled || len(status.Roles) != len(domain.RoleKeys) || status.LastEventSeq == 0 {
		t.Fatalf("status = %#v err=%v", status, err)
	}
}

func TestTaskListsStripBodiesAndRunViewGroupsAgentRuns(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t)
	service := newTestService(t, repository)
	task := domain.Task{ID: "tsk-1", WorkspaceID: "ws", Source: domain.Source{Kind: domain.SourceJira, Key: "PM-1"}, Title: "One", Body: "untrusted body", State: domain.TaskRanked, DiscoveredAt: testNow, UpdatedAt: testNow}
	if err := repository.InsertTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	tasks, err := service.Queue(ctx, "", 10)
	if err != nil || len(tasks) != 1 || tasks[0].Body != "" {
		t.Fatalf("queue = %#v err=%v", tasks, err)
	}
	detail, err := service.Task(ctx, "tsk-1")
	if err != nil || detail.Body != "untrusted body" {
		t.Fatalf("detail = %#v err=%v", detail, err)
	}
	run := domain.Run{ID: "run-1", TaskID: "tsk-1", WorkspaceID: "ws", State: domain.RunRunning, StartedAt: testNow}
	stage := domain.NewStage(run.ID, domain.StagePlan, 0)
	if err := repository.InsertRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if err := repository.InsertStage(ctx, stage); err != nil {
		t.Fatal(err)
	}
	if err := repository.InsertAgentRun(ctx, domain.AgentRun{ID: "agr-1", RunID: run.ID, StageID: stage.ID, Role: domain.RolePlanner, Engine: "claudecode", Model: "opus", State: domain.AgentQueued, QueuedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	view, err := service.Run(ctx, "run-1")
	if err != nil || len(view.Stages) != 1 || len(view.Stages[0].AgentRuns) != 1 || view.Stages[0].AgentRuns[0].Role != domain.RolePlanner {
		t.Fatalf("view = %#v err=%v", view, err)
	}
	if _, err := service.Run(ctx, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing run err = %v", err)
	}
}

func TestResolveConfigReadsTheEnvironment(t *testing.T) {
	env := map[string]string{EnvBotToken: " token ", EnvWebhookURL: "http://bot", EnvDashboardURL: "http://ks/"}
	config := ResolveConfig(func(key string) string { return env[key] }, "/data", true)
	if config.BotToken != "token" || config.WebhookURL != "http://bot" || config.DashboardURL != "http://ks" || config.WorktreeRoot != filepath.Join("/data", "harness", "worktrees") {
		t.Fatalf("config = %#v", config)
	}
}

func errorsAs(err error, target any) bool { return errors.As(err, target) }

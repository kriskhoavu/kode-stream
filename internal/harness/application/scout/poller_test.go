package scout

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/storage"
)

type workspaces map[string]models.WorkspaceConfig

func (w workspaces) Get(id string) (models.WorkspaceConfig, bool, error) {
	workspace, ok := w[id]
	return workspace, ok, nil
}

func (w workspaces) List() ([]models.WorkspaceConfig, error) { return nil, nil }

type fakeScout struct {
	candidates []domain.Candidate
	err        error
}

func (f *fakeScout) Kind() domain.SourceKind { return domain.SourceJira }
func (f *fakeScout) Enabled(_ models.WorkspaceConfig, settings domain.WorkspaceSettings) bool {
	return settings.Jira != nil && settings.Jira.Enabled
}
func (f *fakeScout) Poll(context.Context, models.WorkspaceConfig, domain.WorkspaceSettings) ([]domain.Candidate, error) {
	return f.candidates, f.err
}

func candidate(key, title string) domain.Candidate {
	return domain.Candidate{WorkspaceID: "ws", Source: domain.Source{Kind: domain.SourceJira, Key: key}, Title: title, Body: strings.Repeat("é", domain.MaxTaskBodyBytes)}
}

func TestPollerUpsertsRefreshesAndAgesTasks(t *testing.T) {
	ctx := context.Background()
	repository, closeStore, err := storage.OpenSQLiteHarness(filepath.Join(t.TempDir(), "h.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore()
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	var events []domain.Event
	scout := &fakeScout{candidates: []domain.Candidate{candidate("PM-1", "One"), candidate("PM-2", "Two")}}
	poller := NewPoller(repository, workspaces{"ws": {ID: "ws"}}, nil, func() time.Time { return now }, func(_ context.Context, event domain.Event) domain.Event {
		events = append(events, event)
		return event
	})
	poller.scouts = append(poller.scouts, scout)
	settings := domain.DefaultSettings()
	settings.Workspaces = []domain.WorkspaceSettings{{WorkspaceID: "ws", Jira: &domain.JiraSourceSettings{Enabled: true, JQL: "x"}}, {WorkspaceID: "gone", Jira: &domain.JiraSourceSettings{Enabled: true, JQL: "x"}}}

	polled, err := poller.Poll(ctx, settings, "")
	if err != nil || len(polled) != 1 || polled[0].Added != 2 || polled[0].Found != 2 {
		t.Fatalf("polled = %#v err=%v", polled, err)
	}
	first, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-1")
	if first.State != domain.TaskDiscovered || len(first.Body) > domain.MaxTaskBodyBytes || len(events) != 2 {
		t.Fatalf("first = %#v events=%d", first, len(events))
	}

	scout.candidates = []domain.Candidate{candidate("PM-1", "One, renamed")}
	if polled, _ := poller.Poll(ctx, settings, "ws"); polled[0].Added != 0 {
		t.Fatalf("second poll added = %d", polled[0].Added)
	}
	renamed, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-1")
	if renamed.Title != "One, renamed" || events[len(events)-1].Kind != domain.EventTaskUpdated {
		t.Fatalf("renamed = %#v", renamed)
	}
	for i := 0; i < StaleAfterMissedPolls-1; i++ {
		_, _ = poller.Poll(ctx, settings, "")
	}
	missing, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-2")
	if missing.State != domain.TaskStale {
		t.Fatalf("missing state = %s after %d missed polls", missing.State, missing.MissedPolls)
	}
	scout.candidates = []domain.Candidate{candidate("PM-1", "One, renamed"), candidate("PM-2", "Two")}
	_, _ = poller.Poll(ctx, settings, "")
	back, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-2")
	if back.State != domain.TaskDiscovered || back.MissedPolls != 0 {
		t.Fatalf("reappeared = %#v", back)
	}

	expect := domain.Expect{State: string(back.State), Revision: back.Revision}
	back.State = domain.TaskDismissed
	if _, err := repository.SaveTask(ctx, back, expect); err != nil {
		t.Fatal(err)
	}
	_, _ = poller.Poll(ctx, settings, "")
	if again, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-2"); again.State != domain.TaskDismissed {
		t.Fatalf("a dismissed task reopened: %s", again.State)
	}

	scout.err = errors.New("Jira is unavailable")
	polled, _ = poller.Poll(ctx, settings, "")
	if polled[0].LastError == "" || len(poller.Health()) != 1 {
		t.Fatalf("failure health = %#v", polled)
	}
	if still, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-1"); still.MissedPolls != 0 {
		t.Fatal("a failed poll aged tasks")
	}
}

func TestPollerKeepsAStoppedTaskOpenUntilItsSourceClosesIt(t *testing.T) {
	ctx := context.Background()
	repository, closeStore, err := storage.OpenSQLiteHarness(filepath.Join(t.TempDir(), "h.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore()
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	scout := &fakeScout{candidates: []domain.Candidate{candidate("PM-1", "One")}}
	poller := NewPoller(repository, workspaces{"ws": {ID: "ws"}}, nil, func() time.Time { return now }, func(_ context.Context, event domain.Event) domain.Event { return event })
	poller.scouts = append(poller.scouts, scout)
	settings := domain.DefaultSettings()
	settings.Workspaces = []domain.WorkspaceSettings{{WorkspaceID: "ws", Jira: &domain.JiraSourceSettings{Enabled: true, JQL: "x"}}}
	if _, err := poller.Poll(ctx, settings, ""); err != nil {
		t.Fatal(err)
	}
	task, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-1")
	expect := domain.Expect{State: string(task.State), Revision: task.Revision}
	task.State = domain.TaskCancelled
	if _, err := repository.SaveTask(ctx, task, expect); err != nil {
		t.Fatal(err)
	}

	scout.candidates = []domain.Candidate{candidate("PM-1", "One, renamed")}
	_, _ = poller.Poll(ctx, settings, "")
	stopped, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-1")
	if stopped.State != domain.TaskCancelled || stopped.Title != "One, renamed" {
		t.Fatalf("stopped task = %s %q", stopped.State, stopped.Title)
	}

	scout.candidates = nil
	for i := 0; i < StaleAfterMissedPolls; i++ {
		_, _ = poller.Poll(ctx, settings, "")
	}
	if closed, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-1"); closed.State != domain.TaskStale {
		t.Fatalf("a stopped task whose issue closed is %s", closed.State)
	}
}

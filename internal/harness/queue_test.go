package harness

import (
	"context"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

type staticScout struct{ candidates []domain.Candidate }

func (s *staticScout) Kind() domain.SourceKind { return domain.SourceJira }
func (s *staticScout) Enabled(_ models.WorkspaceConfig, settings domain.WorkspaceSettings) bool {
	return settings.Jira != nil && settings.Jira.Enabled
}
func (s *staticScout) Poll(context.Context, models.WorkspaceConfig, domain.WorkspaceSettings) ([]domain.Candidate, error) {
	return s.candidates, nil
}

func enabledSettings(t *testing.T, service *Service) domain.Settings {
	t.Helper()
	settings := domain.DefaultSettings()
	settings.Enabled = true
	settings.Workspaces = []domain.WorkspaceSettings{{WorkspaceID: "ws", Jira: &domain.JiraSourceSettings{Enabled: true, JQL: "project = PM"}}}
	saved, err := service.SaveSettings(context.Background(), settings, 0, domain.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

func newQueueService(t *testing.T, candidates ...domain.Candidate) (*Service, domain.Repository) {
	t.Helper()
	repository := newTestRepository(t)
	service := New(Dependencies{Repository: repository, Workspaces: fakeWorkspaces{"ws": {ID: "ws"}}, Scouts: []ports.Scout{&staticScout{candidates: candidates}}, Now: func() time.Time { return testNow }, Config: Config{LocalMode: true}})
	enabledSettings(t, service)
	return service, repository
}

func jiraCandidate(key, priority string, blockedBy ...string) domain.Candidate {
	return domain.Candidate{WorkspaceID: "ws", Source: domain.Source{Kind: domain.SourceJira, Key: key, Priority: priority, Labels: []string{}}, Title: key, DependsOnKeys: blockedBy}
}

func TestPollNowRanksTasksAndResolvesDependencies(t *testing.T) {
	ctx := context.Background()
	service, repository := newQueueService(t, jiraCandidate("PM-1", "Low"), jiraCandidate("PM-2", "High", "PM-1", "PM-404"))
	sources, err := service.PollNow(ctx, "ws")
	if err != nil || len(sources) != 1 || sources[0].Added != 2 {
		t.Fatalf("sources = %#v err=%v", sources, err)
	}
	queue, err := service.Queue(ctx, "", 10)
	if err != nil || len(queue) != 2 || queue[0].Source.Key != "PM-2" || queue[0].State != domain.TaskRanked || len(queue[0].ScoreReasons) == 0 {
		t.Fatalf("queue = %#v err=%v", queue, err)
	}
	first, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-1")
	if len(queue[0].DependsOn) != 1 || queue[0].DependsOn[0] != first.ID {
		t.Fatalf("dependsOn = %#v, want [%s]", queue[0].DependsOn, first.ID)
	}
	if _, err := service.PollNow(ctx, "other"); !IsInvalid(err) {
		t.Fatalf("unknown workspace err = %v", err)
	}
}

func TestPinPriorityDismissAndStartChangeTheQueue(t *testing.T) {
	ctx := context.Background()
	service, repository := newQueueService(t, jiraCandidate("PM-1", "Low"), jiraCandidate("PM-2", "High"))
	if _, err := service.PollNow(ctx, ""); err != nil {
		t.Fatal(err)
	}
	low, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-1")
	if _, err := service.PinTask(ctx, low.ID, true, low.Revision+5, domain.Actor{}); !IsConflict(err) {
		t.Fatalf("stale pin err = %v", err)
	}
	pinned, err := service.PinTask(ctx, low.ID, true, low.Revision, domain.Actor{ActorID: "local"})
	if err != nil || !pinned.Pinned || pinned.Score < 1000 {
		t.Fatalf("pinned = %#v err=%v", pinned, err)
	}
	queue, _ := service.Queue(ctx, "", 10)
	if queue[0].ID != low.ID {
		t.Fatal("a pinned task is not first")
	}
	if _, err := service.SetPriority(ctx, low.ID, 9999, pinned.Revision, domain.Actor{}); !IsInvalid(err) {
		t.Fatalf("huge override err = %v", err)
	}
	raised, err := service.SetPriority(ctx, low.ID, 7, pinned.Revision, domain.Actor{})
	if err != nil || raised.PriorityOverride != 7 {
		t.Fatalf("raised = %#v err=%v", raised, err)
	}

	high, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-2")
	started, err := service.StartTask(ctx, high.ID, high.Revision, "start-key", domain.Actor{ActorID: "local", DisplayName: "Local user"}, domain.ChannelDashboard)
	if err != nil || !started.StartRequested {
		t.Fatalf("started = %#v err=%v", started, err)
	}
	replay, err := service.StartTask(ctx, high.ID, 0, "start-key", domain.Actor{}, domain.ChannelDashboard)
	if err != nil || replay.Revision != started.Revision {
		t.Fatalf("replay = %#v err=%v", replay, err)
	}
	approvals, _ := service.Approvals(ctx, domain.ApprovalFilter{TaskID: high.ID, States: []domain.ApprovalState{domain.ApprovalApproved}})
	if len(approvals) != 1 || approvals[0].Gate != domain.GateStart || approvals[0].DecidedBy == nil {
		t.Fatalf("approvals = %#v", approvals)
	}

	dismissed, err := service.DismissTask(ctx, raised.ID, "duplicate of PM-2", raised.Revision, domain.Actor{})
	if err != nil || dismissed.State != domain.TaskDismissed || dismissed.Note != "duplicate of PM-2" {
		t.Fatalf("dismissed = %#v err=%v", dismissed, err)
	}
	var illegal domain.IllegalTransitionError
	if _, err := service.PinTask(ctx, dismissed.ID, false, dismissed.Revision, domain.Actor{}); err == nil || !errorsAs(err, &illegal) {
		t.Fatalf("pin dismissed err = %v", err)
	}
}

func TestClearQueueDeletesTasksNoRunTouchedAndTheFinderRefillsIt(t *testing.T) {
	ctx := context.Background()
	service, repository := newQueueService(t, jiraCandidate("PM-1", "Low"), jiraCandidate("PM-2", "High"))
	if _, err := service.PollNow(ctx, ""); err != nil {
		t.Fatal(err)
	}
	low, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-1")
	if _, err := service.DismissTask(ctx, low.ID, "not now", low.Revision, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ClearQueue(ctx, "other", domain.Actor{}); !IsInvalid(err) {
		t.Fatalf("unknown workspace err = %v", err)
	}
	removed, err := service.ClearQueue(ctx, "ws", domain.Actor{ActorID: "local"})
	if err != nil || removed != 2 {
		t.Fatalf("removed = %d err = %v", removed, err)
	}
	if left, _ := repository.ListTasks(ctx, domain.TaskFilter{}); len(left) != 0 {
		t.Fatalf("tasks left = %#v", left)
	}
	events, _ := repository.EventsAfter(ctx, 0, 100)
	gone := 0
	for _, event := range events {
		if event.Kind == domain.EventTaskRemoved {
			gone++
		}
	}
	if gone != 2 {
		t.Fatalf("task.removed events = %d", gone)
	}
	if _, err := service.PollNow(ctx, ""); err != nil {
		t.Fatal(err)
	}
	back, _ := repository.ListTasks(ctx, domain.TaskFilter{})
	if len(back) != 2 || back[0].State == domain.TaskDismissed || back[1].State == domain.TaskDismissed {
		t.Fatalf("after poll = %#v", back)
	}
}

func TestPausingARoleIsRecordedAndReported(t *testing.T) {
	ctx := context.Background()
	service, _ := newQueueService(t)
	health, err := service.SetRolePaused(ctx, domain.RoleKeyCoder, true, domain.Actor{})
	if err != nil || !health.Paused || health.Role != domain.RoleKeyCoder {
		t.Fatalf("health = %#v err=%v", health, err)
	}
	settings, _ := service.Settings(ctx)
	if !settings.Paused[domain.RoleKeyCoder] {
		t.Fatal("pause not saved")
	}
	if health, err := service.SetRolePaused(ctx, domain.RoleKeyCoder, false, domain.Actor{}); err != nil || health.Paused {
		t.Fatalf("resume = %#v err=%v", health, err)
	}
	if _, err := service.SetRolePaused(ctx, "janitor", true, domain.Actor{}); !IsNotFound(err) {
		t.Fatalf("unknown role err = %v", err)
	}
}

func TestStartRunsTheScoutAndSorterLoopsUntilStop(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t)
	service := New(Dependencies{Repository: repository, Workspaces: fakeWorkspaces{"ws": {ID: "ws"}}, Scouts: []ports.Scout{&staticScout{candidates: []domain.Candidate{jiraCandidate("PM-7", "High")}}}, Config: Config{LocalMode: true}})
	enabledSettings(t, service)
	if err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer service.Stop(time.Second)
	deadline := time.Now().Add(5 * time.Second)
	for {
		task, found, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-7")
		if found && task.State == domain.TaskRanked {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task not ranked by the loops: found=%v state=%s", found, task.State)
		}
		time.Sleep(20 * time.Millisecond)
	}
	status, err := service.Status(ctx)
	if err != nil || !status.Running || len(status.Sources) != 1 {
		t.Fatalf("status = %#v err=%v", status, err)
	}
	service.Stop(time.Second)
	if status, _ := service.Status(ctx); status.Running {
		t.Fatal("still running after Stop")
	}
}

func TestCreateTaskFromACardRanksAndStartsIt(t *testing.T) {
	ctx := context.Background()
	service, repository := newQueueService(t)
	actor := domain.Actor{ActorID: "local", DisplayName: "Local user"}
	if _, err := service.CreateTask(ctx, CreateTaskInput{WorkspaceID: "ws", Title: "No key"}, actor, domain.ChannelDashboard); !IsInvalid(err) {
		t.Fatalf("no key err = %v", err)
	}
	if _, err := service.CreateTask(ctx, CreateTaskInput{WorkspaceID: "other", Key: "PM-7"}, actor, domain.ChannelDashboard); !IsInvalid(err) {
		t.Fatalf("unknown workspace err = %v", err)
	}
	queued, err := service.CreateTask(ctx, CreateTaskInput{WorkspaceID: "ws", Key: " PM-7 ", Title: "Seven", Body: "The ticket's text"}, actor, domain.ChannelDashboard)
	if err != nil || queued.State != domain.TaskRanked || queued.StartRequested || queued.Source.Key != "PM-7" || queued.Source.Kind != domain.SourceJira || queued.Body != "" {
		t.Fatalf("queued = %#v err=%v", queued, err)
	}
	stored, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-7")
	if stored.Body != "The ticket's text" || stored.Title != "Seven" {
		t.Fatalf("stored = %#v", stored)
	}
	started, err := service.CreateTask(ctx, CreateTaskInput{WorkspaceID: "ws", Key: "PM-7", StartNow: true, IdempotencyKey: "card-7"}, actor, domain.ChannelDashboard)
	if err != nil || started.ID != queued.ID || !started.StartRequested {
		t.Fatalf("started = %#v err=%v", started, err)
	}
	approvals, _ := service.Approvals(ctx, domain.ApprovalFilter{TaskID: queued.ID, States: []domain.ApprovalState{domain.ApprovalApproved}})
	if len(approvals) != 1 || approvals[0].Gate != domain.GateStart || approvals[0].DecidedBy == nil || approvals[0].DecidedBy.ActorID != "local" {
		t.Fatalf("approvals = %#v", approvals)
	}
	fresh, err := service.CreateTask(ctx, CreateTaskInput{WorkspaceID: "ws", Key: "PM-8", Title: "Eight", StartNow: true}, actor, domain.ChannelDashboard)
	if err != nil || !fresh.StartRequested || fresh.State != domain.TaskRanked {
		t.Fatalf("fresh = %#v err=%v", fresh, err)
	}
	dismissed, err := service.DismissTask(ctx, fresh.ID, "not now", fresh.Revision, actor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateTask(ctx, CreateTaskInput{WorkspaceID: "ws", Key: "PM-8", StartNow: true}, actor, domain.ChannelDashboard); !IsInvalid(err) {
		t.Fatalf("dismissed again err = %v (state %s)", err, dismissed.State)
	}
}

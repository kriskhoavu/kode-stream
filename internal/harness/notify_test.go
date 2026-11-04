package harness

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"kode-stream/internal/harness/application/notify"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

type fakeNotifier struct {
	mu         sync.Mutex
	fail       bool
	deliveries []ports.Delivery
}

func (f *fakeNotifier) Deliver(_ context.Context, delivery ports.Delivery) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deliveries = append(f.deliveries, delivery)
	if f.fail {
		return errors.New("bot down")
	}
	return nil
}

func TestNotifiableEventsAreDeliveredAtLeastOnceWithBackoff(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t)
	notifier := &fakeNotifier{fail: true}
	clock := testNow
	service := New(Dependencies{Repository: repository, Workspaces: fakeWorkspaces{"ws": {ID: "ws"}}, Notifier: notifier, Now: func() time.Time { return clock }, Config: Config{LocalMode: true, DashboardURL: "http://ks"}})
	task := domain.Task{ID: "tsk-1", WorkspaceID: "ws", Source: domain.Source{Kind: domain.SourceJira, Key: "PM-1", URL: "https://jira/PM-1"}, Title: "One", State: domain.TaskRunning, DiscoveredAt: testNow, UpdatedAt: testNow}
	run := domain.Run{ID: "run-1", TaskID: task.ID, WorkspaceID: "ws", State: domain.RunRunning, StartedAt: testNow, Totals: domain.Totals{CostUSD: 1.42, Turns: 37}}
	approval := domain.Approval{ID: "apr-1", RunID: run.ID, TaskID: task.ID, Gate: domain.GatePlan, State: domain.ApprovalRequested, Summary: "Three phases.", RequestedAt: testNow}
	for _, err := range []error{repository.InsertTask(ctx, task), repository.InsertRun(ctx, run), repository.InsertApproval(ctx, approval)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	service.emit(ctx, domain.Event{Kind: domain.EventTaskRanked, TaskID: task.ID})
	requested := service.emit(ctx, domain.Event{Kind: domain.EventApprovalRequested, TaskID: task.ID, RunID: run.ID, ApprovalID: approval.ID, Payload: payload(map[string]any{"summary": "Three phases."})})

	for attempt := 1; attempt <= notify.MaxAttempts; attempt++ {
		if err := service.deliverDue(ctx); err != nil {
			t.Fatal(err)
		}
		if len(notifier.deliveries) != attempt {
			t.Fatalf("attempt %d: deliveries = %d", attempt, len(notifier.deliveries))
		}
		if err := service.deliverDue(ctx); err != nil || len(notifier.deliveries) != attempt {
			t.Fatalf("a row was retried before its backoff: %d", len(notifier.deliveries))
		}
		clock = clock.Add(notify.Backoff(attempt))
	}
	last := notifier.deliveries[len(notifier.deliveries)-1]
	if last.EventID != notify.EventID(requested.Seq) || last.Attempt != notify.MaxAttempts {
		t.Fatalf("last delivery = %#v", last)
	}
	var body notify.Payload
	if err := json.Unmarshal(last.Body, &body); err != nil || body.Kind != domain.EventApprovalRequested || body.Task.SourceKey != "PM-1" || body.Run.CostUSD != 1.42 || body.Approval.Gate != "plan" || body.Summary != "Three phases." || body.Links["dashboard"] != "http://ks/harness/runs/run-1" {
		t.Fatalf("body = %s err=%v", last.Body, err)
	}
	events, _ := repository.EventsAfter(ctx, requested.Seq, 10)
	if len(events) != 1 || events[0].Kind != domain.EventNotificationFailed {
		t.Fatalf("events after = %#v", events)
	}

	notifier.fail = false
	service.emit(ctx, domain.Event{Kind: domain.EventRunFinished, TaskID: task.ID, RunID: run.ID})
	if err := service.deliverDue(ctx); err != nil {
		t.Fatal(err)
	}
	if due, _ := repository.DueNotifications(ctx, clock.Add(time.Hour), 10); len(due) != 0 {
		t.Fatalf("delivered rows still due: %#v", due)
	}

	quiet := New(Dependencies{Repository: repository, Config: Config{LocalMode: true}})
	if err := quiet.deliverDue(ctx); err != nil {
		t.Fatal(err)
	}
}

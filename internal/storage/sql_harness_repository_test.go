package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kode-stream/internal/harness/domain"
)

func TestSQLiteHarnessRepositoryContract(t *testing.T) {
	repository, closeStore, err := OpenSQLiteHarness(filepath.Join(t.TempDir(), "harness.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore()
	exerciseHarnessRepository(t, repository)
}

func TestPostgresHarnessRepositoryContract(t *testing.T) {
	databaseURL := os.Getenv(EnvDatabaseURL)
	if databaseURL == "" {
		t.Skip("set KODE_STREAM_DATABASE_URL to run the Postgres harness contract")
	}
	admin, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("kode_stream_harness_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA "` + schema + `"`); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	defer admin.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`)
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	store, err := openSQLStore(Config{Driver: StorageDriverPostgres, DatabaseURL: parsed.String(), Migrations: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	exerciseHarnessRepository(t, newSQLHarnessRepository(store))
}

func exerciseHarnessRepository(t *testing.T, repository domain.Repository) {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)

	task := domain.Task{ID: "tsk-1", WorkspaceID: "ws", Source: domain.Source{Kind: domain.SourceJira, Key: "PM-1", Labels: []string{"bug"}}, Title: "First", State: domain.TaskDiscovered, DiscoveredAt: now, UpdatedAt: now}
	if err := repository.InsertTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	duplicate := task
	duplicate.ID = "tsk-dup"
	if err := repository.InsertTask(ctx, duplicate); !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("duplicate source insert err = %v, want ErrDuplicate", err)
	}
	found, ok, err := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-1")
	if err != nil || !ok || found.ID != "tsk-1" || found.Source.Labels[0] != "bug" {
		t.Fatalf("find = %#v ok=%v err=%v", found, ok, err)
	}

	found.State = domain.TaskRanked
	found.Score = 30
	saved, err := repository.SaveTask(ctx, found, domain.Expect{State: string(domain.TaskDiscovered), Revision: 0})
	if err != nil || saved.Revision != 1 {
		t.Fatalf("save = %#v err=%v", saved, err)
	}
	if _, err := repository.SaveTask(ctx, found, domain.Expect{State: string(domain.TaskDiscovered), Revision: 0}); !errors.Is(err, domain.ErrStateMoved) {
		t.Fatalf("stale save err = %v, want ErrStateMoved", err)
	}
	second := domain.Task{ID: "tsk-2", WorkspaceID: "ws", Source: domain.Source{Kind: domain.SourceGitHub, Key: "o/r#2"}, Title: "Second", State: domain.TaskRanked, Score: 50, Pinned: true, DiscoveredAt: now, UpdatedAt: now}
	if err := repository.InsertTask(ctx, second); err != nil {
		t.Fatal(err)
	}
	tasks, err := repository.ListTasks(ctx, domain.TaskFilter{States: []domain.TaskState{domain.TaskRanked}})
	if err != nil || len(tasks) != 2 || tasks[0].ID != "tsk-2" {
		t.Fatalf("queue order = %#v err=%v", tasks, err)
	}

	run := domain.Run{ID: "run-1", TaskID: "tsk-1", WorkspaceID: "ws", SourceKey: "PM-1", Branch: "harness/PM-1-abc", State: domain.RunRunning, StartedAt: now}
	stage := domain.NewStage(run.ID, domain.StageQA, 3)
	err = repository.Atomic(ctx, func(tx domain.Repository) error {
		if err := tx.InsertRun(ctx, run); err != nil {
			return err
		}
		return tx.InsertStage(ctx, stage)
	})
	if err != nil {
		t.Fatal(err)
	}
	rolledBack := domain.Run{ID: "run-rollback", TaskID: "tsk-2", WorkspaceID: "ws", State: domain.RunRunning, StartedAt: now}
	failure := errors.New("abort")
	if err := repository.Atomic(ctx, func(tx domain.Repository) error {
		if err := tx.InsertRun(ctx, rolledBack); err != nil {
			return err
		}
		return failure
	}); !errors.Is(err, failure) {
		t.Fatalf("atomic err = %v", err)
	}
	if _, err := repository.GetRun(ctx, "run-rollback"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("rolled back run err = %v, want ErrNotFound", err)
	}

	claimed, ok, err := repository.ClaimStage(ctx, []domain.StageKind{domain.StageQA}, domain.Claim{Owner: "qa", LeaseUntil: now.Add(time.Minute), Now: now})
	if err != nil || !ok || claimed.State != domain.StageRunning || claimed.LeaseOwner != "qa" {
		t.Fatalf("claim stage = %#v ok=%v err=%v", claimed, ok, err)
	}
	if _, ok, _ := repository.ClaimStage(ctx, []domain.StageKind{domain.StageQA}, domain.Claim{Owner: "qa", LeaseUntil: now.Add(time.Minute), Now: now}); ok {
		t.Fatal("a running stage was claimed twice")
	}

	agentRun := domain.AgentRun{ID: "agr-1", RunID: run.ID, StageID: stage.ID, Role: domain.RolePlanner, Engine: "claudecode", Model: "opus", State: domain.AgentQueued, Attempt: 1, QueuedAt: now}
	if err := repository.InsertAgentRun(ctx, agentRun); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := repository.ClaimAgentRun(ctx, []domain.Role{domain.RoleCoder}, domain.Claim{Owner: "coder", LeaseUntil: now, Now: now}); ok {
		t.Fatal("a coder claimed a planner run")
	}
	claimedRun, ok, err := repository.ClaimAgentRun(ctx, []domain.Role{domain.RolePlanner}, domain.Claim{Owner: "planner-0", LeaseUntil: now.Add(time.Second), Now: now})
	if err != nil || !ok || claimedRun.State != domain.AgentRunning {
		t.Fatalf("claim agent run = %#v ok=%v err=%v", claimedRun, ok, err)
	}
	recoveredRuns, recoveredStages, err := repository.RecoverExpiredLeases(ctx, now.Add(2*time.Minute))
	if err != nil || len(recoveredRuns) != 1 || recoveredRuns[0].State != domain.AgentInterrupted || len(recoveredStages) != 1 || recoveredStages[0].State != domain.StagePending {
		t.Fatalf("recover runs=%#v stages=%#v err=%v", recoveredRuns, recoveredStages, err)
	}

	finished := recoveredRuns[0]
	expect := domain.Expect{State: string(finished.State), Revision: finished.Revision}
	finished.State = domain.AgentQueued
	if finished, err = repository.SaveAgentRun(ctx, finished, expect); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repository.ClaimAgentRun(ctx, []domain.Role{domain.RolePlanner}, domain.Claim{Owner: "planner-0", LeaseUntil: now.Add(time.Hour), Now: now}); err != nil || !ok {
		t.Fatalf("re-claim ok=%v err=%v", ok, err)
	}
	interrupted, err := repository.InterruptRunning(ctx, now)
	if err != nil || len(interrupted) != 1 {
		t.Fatalf("interrupt = %#v err=%v", interrupted, err)
	}
	done := interrupted[0]
	expect = domain.Expect{State: string(done.State), Revision: done.Revision}
	done.State = domain.AgentFailed
	done.CostUSD = 1.25
	done.Turns = 7
	finishedAt := now.Add(time.Minute)
	done.FinishedAt = &finishedAt
	if _, err := repository.SaveAgentRun(ctx, done, expect); err != nil {
		t.Fatal(err)
	}
	second2 := domain.AgentRun{ID: "agr-2", RunID: run.ID, StageID: stage.ID, Role: domain.RoleCoder, Engine: "claudecode", Model: "sonnet", State: domain.AgentQueued, QueuedAt: now}
	if err := repository.InsertAgentRun(ctx, second2); err != nil {
		t.Fatal(err)
	}
	cost, err := repository.CostSummary(ctx, domain.CostQuery{GroupBy: domain.CostByRole, From: now.Add(-time.Hour), To: now.Add(time.Hour)})
	if err != nil || cost.TotalUSD != 1.25 || len(cost.Rows) != 2 {
		t.Fatalf("cost = %#v err=%v", cost, err)
	}
	if _, err := repository.CostSummary(ctx, domain.CostQuery{GroupBy: "week"}); err == nil {
		t.Fatal("unknown cost group was accepted")
	}

	approval := domain.Approval{ID: "apr-1", RunID: run.ID, TaskID: "tsk-1", Gate: domain.GatePlan, State: domain.ApprovalRequested, RequestedAt: now}
	if err := repository.InsertApproval(ctx, approval); err != nil {
		t.Fatal(err)
	}
	approval.State = domain.ApprovalApproved
	approval.IdempotencyKey = "key-1"
	approval.Channel = domain.ChannelWebex
	decided, err := repository.SaveApproval(ctx, approval, domain.Expect{State: string(domain.ApprovalRequested), Revision: 0})
	if err != nil {
		t.Fatal(err)
	}
	replayed, ok, err := repository.FindApprovalByIdempotencyKey(ctx, "key-1")
	if err != nil || !ok || replayed.ID != decided.ID || replayed.IdempotencyKey != "key-1" || replayed.Channel != domain.ChannelWebex {
		t.Fatalf("by key = %#v ok=%v err=%v", replayed, ok, err)
	}
	other := domain.Approval{ID: "apr-2", TaskID: "tsk-2", Gate: domain.GateStart, State: domain.ApprovalApproved, IdempotencyKey: "key-1", RequestedAt: now}
	if err := repository.InsertApproval(ctx, other); !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("duplicate key err = %v, want ErrDuplicate", err)
	}

	first, err := repository.AppendEvent(ctx, domain.Event{Kind: domain.EventTaskDiscovered, At: now, TaskID: "tsk-1"})
	if err != nil {
		t.Fatal(err)
	}
	secondEvent, err := repository.AppendEvent(ctx, domain.Event{Kind: domain.EventRunStarted, At: now, RunID: "run-1", Payload: []byte(`{"branch":"harness/PM-1-abc"}`)})
	if err != nil || secondEvent.Seq <= first.Seq {
		t.Fatalf("seq first=%d second=%d err=%v", first.Seq, secondEvent.Seq, err)
	}
	events, err := repository.EventsAfter(ctx, first.Seq, 10)
	if err != nil || len(events) != 1 || events[0].RunID != "run-1" || string(events[0].Payload) != `{"branch":"harness/PM-1-abc"}` {
		t.Fatalf("events = %#v err=%v", events, err)
	}
	if last, err := repository.LastEventSeq(ctx); err != nil || last != secondEvent.Seq {
		t.Fatalf("last = %d err=%v", last, err)
	}

	notification := domain.Notification{ID: "ntf-1", EventSeq: secondEvent.Seq, Target: "webhook", State: domain.NotificationPending, NextAt: now}
	if err := repository.EnqueueNotification(ctx, notification); err != nil {
		t.Fatal(err)
	}
	due, err := repository.DueNotifications(ctx, now, 10)
	if err != nil || len(due) != 1 {
		t.Fatalf("due = %#v err=%v", due, err)
	}
	due[0].State = domain.NotificationDelivered
	if err := repository.SaveNotification(ctx, due[0]); err != nil {
		t.Fatal(err)
	}
	if due, _ := repository.DueNotifications(ctx, now, 10); len(due) != 0 {
		t.Fatalf("delivered notification still due: %#v", due)
	}

	settings, err := repository.LoadSettings(ctx)
	if err != nil || settings.Enabled || settings.Revision != 0 {
		t.Fatalf("default settings = %#v err=%v", settings, err)
	}
	settings.Enabled = true
	saved1, err := repository.SaveSettings(ctx, settings, 0)
	if err != nil || saved1.Revision != 1 {
		t.Fatalf("save settings = %#v err=%v", saved1, err)
	}
	if _, err := repository.SaveSettings(ctx, settings, 0); !errors.Is(err, domain.ErrStateMoved) {
		t.Fatalf("stale settings err = %v", err)
	}
	loaded, err := repository.LoadSettings(ctx)
	if err != nil || !loaded.Enabled || loaded.Revision != 1 {
		t.Fatalf("loaded = %#v err=%v", loaded, err)
	}
}

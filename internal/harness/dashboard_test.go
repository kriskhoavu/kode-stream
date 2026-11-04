package harness

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"kode-stream/internal/harness/domain"
)

func TestDashboardViewsCarryWhatTheScreensRead(t *testing.T) {
	ctx := context.Background()
	fixture := newPipeline(t, happyScript)
	fixture.settle(t)
	view := fixture.run(t)

	detail, err := fixture.service.DashboardRunDetail(ctx, view.ID)
	if err != nil || detail.Task.ID != fixture.task.ID || detail.Task.Body != "" || len(detail.Stages) != 2 || len(detail.AgentRuns) != 2 || len(detail.Approvals) != 1 || detail.Run.CurrentStage == nil || detail.Run.CurrentStage.Kind != domain.StagePlanApproval {
		t.Fatalf("detail = %#v err=%v", detail, err)
	}
	var reviewer DashboardAgentRun
	for _, agentRun := range detail.AgentRuns {
		if agentRun.Role == domain.RolePlanReviewer {
			reviewer = agentRun
		}
	}
	if reviewer.Verdict != domain.VerdictApprove || reviewer.Summary != "Fine." {
		t.Fatalf("reviewer = %#v", reviewer)
	}
	approval := detail.Approvals[0]
	if approval.Context.Difficulty != "hard" || approval.Context.Subtasks != 2 || approval.Context.CostUSD != 0.5 || approval.Context.Verdict != domain.VerdictApprove {
		t.Fatalf("approval context = %#v", approval.Context)
	}
	runs, err := fixture.service.DashboardRuns(ctx, domain.RunFilter{})
	if err != nil || len(runs) != 1 || runs[0].CurrentStage == nil {
		t.Fatalf("runs = %#v err=%v", runs, err)
	}
	wrapped, err := fixture.service.DashboardAgentRun(ctx, reviewer.ID)
	if err != nil || wrapped.AgentRun.ID != reviewer.ID {
		t.Fatalf("agent run = %#v err=%v", wrapped, err)
	}

	events, _ := fixture.repository.EventsAfter(ctx, 0, 500)
	shown := map[string]bool{}
	for _, event := range events {
		mapped, ok := fixture.service.DashboardEvent(ctx, event)
		if !ok {
			continue
		}
		data, _ := json.Marshal(mapped)
		for _, key := range []string{"task", "run", "stage", "agentRun", "approval"} {
			if strings.Contains(string(data), `"`+key+`":{`) {
				shown[key] = true
			}
		}
	}
	// This fixture stores its task directly, so no task event exists here;
	// the transport test covers task events.
	for _, key := range []string{"run", "stage", "agentRun", "approval"} {
		if !shown[key] {
			t.Fatalf("no event carried a %s; saw %v", key, shown)
		}
	}
	progress, ok := fixture.service.DashboardEvent(ctx, domain.Event{Kind: domain.EventAgentProgress, AgentRunID: reviewer.ID, Payload: []byte(`{"progress":"reading","turns":99}`)})
	if agentRun, _ := progress["agentRun"].(DashboardAgentRun); !ok || agentRun.Progress != "reading" || agentRun.Turns != 99 {
		t.Fatalf("progress = %#v", progress)
	}
	if _, ok := fixture.service.DashboardEvent(ctx, domain.Event{Kind: domain.EventSettingsUpdated}); ok {
		t.Fatal("settings.updated reached the dashboard feed")
	}

	status, err := fixture.service.DashboardStatus(ctx)
	roles := map[string]DashboardRole{}
	for _, role := range status.Roles {
		roles[role.Role] = role
	}
	if err != nil || status.WaitingCount != 1 || roles["qa"].Engine != "claudecode" || roles["reviewer"].Model != "opus" || len(status.Engines) != 1 || status.Engines[0].Limits == nil {
		t.Fatalf("status = %#v err=%v", status, err)
	}
}

func TestDashboardSettingsRoundTripKeepsWhatTheScreenDoesNotShow(t *testing.T) {
	ctx := context.Background()
	service, _ := newQueueService(t)
	stored, _ := service.Settings(ctx)
	stored.Workspaces[0].Scope = []string{"internal/"}
	stored.Workspaces[0].VerificationCommands = []string{"go test ./..."}
	stored.Workspaces[0].Bitbucket = &domain.BitbucketSourceSettings{Enabled: false, Workspace: "PROJ", Repo: "app", BaseURL: "https://bb.example", TokenEnvVar: "BB"}
	stored, err := service.SaveSettings(ctx, stored, stored.Revision, domain.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	view, err := service.DashboardSettings(ctx)
	if err != nil || len(view.Sources) != 2 || view.Sources[1].Query != "PROJ/app" {
		t.Fatalf("view = %#v err=%v", view, err)
	}
	view.Sources[0].Query = "project = NEW"
	view.Sources = append(view.Sources, DashboardSource{WorkspaceID: "ws", Kind: "github", Enabled: true, Query: "acme/app", Labels: []string{"harness"}})
	view.Slots["qa"] = 2
	saved, err := service.SaveDashboardSettings(ctx, view, domain.Actor{})
	if err != nil || saved.Slots["qa"] != 2 || len(saved.Sources) != 3 {
		t.Fatalf("saved = %#v err=%v", saved, err)
	}
	after, _ := service.Settings(ctx)
	workspace := after.Workspaces[0]
	if workspace.Jira.JQL != "project = NEW" || workspace.GitHub.Repo != "acme/app" || workspace.Bitbucket.Cloud || workspace.Bitbucket.BaseURL != "https://bb.example" || workspace.Scope[0] != "internal/" || workspace.VerificationCommands[0] != "go test ./..." || after.Slots.QAFixer != 2 {
		t.Fatalf("stored = %#v", workspace)
	}
	if _, err := service.SaveDashboardSettings(ctx, view, domain.Actor{}); !IsConflict(err) {
		t.Fatalf("stale save err = %v", err)
	}
}

func TestMoveBeforeRanksATaskJustAboveAnother(t *testing.T) {
	ctx := context.Background()
	service, repository := newQueueService(t, jiraCandidate("PM-1", "Low"), jiraCandidate("PM-2", "High"))
	if _, err := service.PollNow(ctx, ""); err != nil {
		t.Fatal(err)
	}
	low, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-1")
	high, _, _ := repository.FindTaskBySource(ctx, domain.SourceJira, "PM-2")
	moved, err := service.MoveBefore(ctx, low.ID, high.ID, low.Revision, domain.Actor{})
	if err != nil || moved.Score != high.Score+0.5 {
		t.Fatalf("moved = %v (above %v) err=%v", moved.Score, high.Score, err)
	}
	if _, err := service.MoveBefore(ctx, low.ID, low.ID, moved.Revision, domain.Actor{}); !IsInvalid(err) {
		t.Fatalf("self move err = %v", err)
	}
}

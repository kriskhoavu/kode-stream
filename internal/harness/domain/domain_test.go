package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestTaskStateMachineRefusesReopeningTerminalStates(t *testing.T) {
	for _, terminal := range []TaskState{TaskDone, TaskDismissed, TaskCancelled} {
		if err := CheckTask(terminal, TaskRanked); err == nil {
			t.Fatalf("%s reopened", terminal)
		}
		if !TaskTerminal(terminal) {
			t.Fatalf("%s not terminal", terminal)
		}
	}
	for _, step := range [][2]TaskState{{TaskDiscovered, TaskRanked}, {TaskRanked, TaskRunning}, {TaskRunning, TaskBlocked}, {TaskBlocked, TaskRunning}, {TaskRunning, TaskDone}, {TaskStale, TaskDiscovered}} {
		if err := CheckTask(step[0], step[1]); err != nil {
			t.Fatalf("%s -> %s refused: %v", step[0], step[1], err)
		}
	}
	var illegal IllegalTransitionError
	if err := CheckAgentRun(AgentSucceeded, AgentQueued); !errors.As(err, &illegal) {
		t.Fatalf("succeeded agent run requeued: %v", err)
	}
	if err := CheckAgentRun(AgentInterrupted, AgentQueued); err != nil {
		t.Fatal(err)
	}
	if err := CheckApproval(ApprovalApproved, ApprovalRejected); err == nil {
		t.Fatal("a decided approval changed its decision")
	}
}

func TestStageSequenceSkipsPRApprovalUnlessRequired(t *testing.T) {
	if got := NextStage(StageQA, false); got != StageDraftPR {
		t.Fatalf("after qa = %s", got)
	}
	if got := NextStage(StageQA, true); got != StagePRApproval {
		t.Fatalf("after qa with approval = %s", got)
	}
	if got := NextStage(StageDraftPR, true); got != "" {
		t.Fatalf("after draft_pr = %s", got)
	}
	if MaxRoundsOf(StagePlan) != 3 || MaxRoundsOf(StageQA) != 2 || MaxRoundsOf(StagePlanApproval) != 1 {
		t.Fatal("round bounds changed")
	}
}

func TestSettingsNormalizeFillsDefaultsAndCapsKnobs(t *testing.T) {
	settings, err := Settings{}.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Slots.Coder != 1 || settings.Intervals.LeaseSeconds != 2700 || settings.RetentionDays != 3 {
		t.Fatalf("defaults = %#v", settings)
	}
	if settings.ModelPolicy.Roles[RoleCoder].HighDifficultyModel != "opus" || settings.ModelPolicy.ModelFor(RoleCoder, "high") != "opus" || settings.ModelPolicy.ModelFor(RoleCoder, "low") != "sonnet" {
		t.Fatalf("coder policy = %#v", settings.ModelPolicy.Roles[RoleCoder])
	}
	for _, tool := range AlwaysDisallowedTools {
		found := false
		for _, disallowed := range settings.ModelPolicy.DisallowedTools {
			found = found || disallowed == tool
		}
		if !found {
			t.Fatalf("always-disallowed %s missing", tool)
		}
	}

	cases := map[string]func(*Settings){
		"slots.coder":  func(s *Settings) { s.Slots.Coder = 99 },
		"leaseSeconds": func(s *Settings) { s.Intervals.LeaseSeconds = 5 },
		"maxTurns":     func(s *Settings) { s.ModelPolicy.Roles[RolePlanner] = RolePolicy{MaxTurns: 500} },
		"may not allow": func(s *Settings) {
			s.ModelPolicy.Roles[RoleCoder] = RolePolicy{AllowedTools: []string{"Bash(git push:*)"}}
		},
		"read-only":      func(s *Settings) { s.ModelPolicy.Roles[RoleCodeReviewer] = RolePolicy{PermissionMode: "acceptEdits"} },
		"perAgentRunUsd": func(s *Settings) { s.ModelPolicy.PerAgentRunUSD = 100 },
		"jira.jql": func(s *Settings) {
			s.Workspaces = []WorkspaceSettings{{WorkspaceID: "ws", Jira: &JiraSourceSettings{Enabled: true}}}
		},
		"listed twice": func(s *Settings) { s.Workspaces = []WorkspaceSettings{{WorkspaceID: "ws"}, {WorkspaceID: "ws"}} },
		"owner/name": func(s *Settings) {
			s.Workspaces = []WorkspaceSettings{{WorkspaceID: "ws", GitHub: &GitHubSourceSettings{Enabled: true, Repo: "nope"}}}
		},
		"unknown role":    func(s *Settings) { s.Paused = map[RoleKey]bool{"janitor": true} },
		"ranker.llm.topN": func(s *Settings) { s.Ranker.LLM.TopN = 1 },
	}
	for want, mutate := range cases {
		candidate := DefaultSettings()
		candidate.ModelPolicy.Roles = map[Role]RolePolicy{}
		mutate(&candidate)
		_, err := candidate.Normalize()
		var invalid InvalidError
		if !errors.As(err, &invalid) || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: err = %v", want, err)
		}
	}
}

func TestPoolsShareTheReviewerSlot(t *testing.T) {
	if PoolOf(RolePlanReviewer) != RoleKeyReviewer || PoolOf(RoleCodeReviewer) != RoleKeyReviewer {
		t.Fatal("reviewers do not share one pool")
	}
	if len(RolesOf(RoleKeyReviewer)) != 2 || len(RolesOf(RoleKeyScout)) != 0 {
		t.Fatal("pool roles changed")
	}
}

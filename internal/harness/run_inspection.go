package harness

import (
	"context"
	"errors"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

type RunChanges struct {
	Branch     string           `json:"branch"`
	BaseCommit string           `json:"baseCommit"`
	Files      []ports.DiffFile `json:"files"`
	DiffStat   string           `json:"diffStat"`
	Available  bool             `json:"available"`
	// From says where the files were read: "worktree", "branch", or "".
	From string `json:"from"`
}

// RunFileDiff is the diff of one path a run changed.
type RunFileDiff struct {
	Path      string `json:"path"`
	Diff      string `json:"diff"`
	Truncated bool   `json:"truncated"`
	From      string `json:"from"`
}

// maxRunDiffBytes bounds one file's diff a person reads.
const maxRunDiffBytes = 512 << 10

// runSource is where a run's files are read: its workspace and its worktree
// while it has one.
func (s *Service) runSource(ctx context.Context, runID string) (domain.Run, models.WorkspaceConfig, string, bool, error) {
	if !s.Supported() {
		return domain.Run{}, models.WorkspaceConfig{}, "", false, domain.ErrNotFound
	}
	run, err := s.repository.GetRun(ctx, runID)
	if err != nil {
		return domain.Run{}, models.WorkspaceConfig{}, "", false, err
	}
	if s.worktrees == nil || s.workspaces == nil {
		return run, models.WorkspaceConfig{}, "", false, nil
	}
	workspace, found, err := s.workspaces.Get(run.WorkspaceID)
	if err != nil || !found {
		return run, workspace, "", false, err
	}
	worktree := run.WorktreePath
	if run.WorktreeRemoved {
		worktree = ""
	}
	return run, workspace, worktree, true, nil
}

// FileDiff returns the diff of one path the run changed: live from its
// worktree, else from its branch.
func (s *Service) FileDiff(ctx context.Context, runID, path string) (RunFileDiff, error) {
	run, workspace, worktree, ok, err := s.runSource(ctx, runID)
	if err != nil {
		return RunFileDiff{}, err
	}
	out := RunFileDiff{Path: path}
	if !ok {
		return out, nil
	}
	diff, truncated, from, err := s.worktrees.FileDiff(ctx, workspace, worktree, run.BaseCommit, run.Branch, path, maxRunDiffBytes)
	if errors.Is(err, ports.ErrNotChanged) {
		return out, domain.ErrNotFound
	}
	if err != nil {
		return out, err
	}
	out.Diff, out.Truncated, out.From = diff, truncated, from
	return out, nil
}

// Changes lists what a run changed in its worktree against its base.
func (s *Service) Changes(ctx context.Context, runID string) (RunChanges, error) {
	run, workspace, worktree, ok, err := s.runSource(ctx, runID)
	if err != nil {
		return RunChanges{}, err
	}
	changes := RunChanges{Branch: run.Branch, BaseCommit: run.BaseCommit, Files: []ports.DiffFile{}}
	if !ok {
		return changes, nil
	}
	files, from, err := s.worktrees.ChangedFiles(ctx, workspace, worktree, run.BaseCommit, run.Branch)
	if err != nil || from == "" {
		return changes, nil
	}
	changes.Files, changes.From, changes.Available = files, from, true
	return changes, nil
}

// RunPlan is the plan a run wrote, readable before it is merged.
type RunPlan struct {
	Branch string          `json:"branch"`
	From   string          `json:"from"`
	Files  []ports.RunFile `json:"files"`
}

// maxPlanFileBytes bounds one plan file a person reads.
const maxPlanFileBytes = 256 << 10

// Plan reads the files a run wrote under the plan folder: live from its
// worktree while it has one, else from its branch. A run whose worktree and
// branch are both gone has no plan to show here; once merged, it is on the
// board.
func (s *Service) Plan(ctx context.Context, runID string) (RunPlan, error) {
	run, workspace, worktree, ok, err := s.runSource(ctx, runID)
	if err != nil {
		return RunPlan{}, err
	}
	plan := RunPlan{Branch: run.Branch, Files: []ports.RunFile{}}
	if !ok {
		return plan, nil
	}
	files, from, err := s.worktrees.ReadRunFiles(ctx, workspace, worktree, run.BaseCommit, run.Branch, planScope[0], maxPlanFileBytes)
	if err != nil {
		return plan, nil
	}
	plan.Files, plan.From = files, from
	return plan, nil
}

// ReviewTarget is where a person's Review session runs: the run's own
// worktree, on its branch, with a brief that says what they are looking at.
type ReviewTarget struct {
	WorkspaceID string
	Dir         string
	Branch      string
	Prompt      string
}

// ReviewSession names the worktree of a run waiting at Review. A run at any
// other stage, or without a worktree, has none.
func (s *Service) ReviewSession(ctx context.Context, runID string) (ReviewTarget, error) {
	if !s.Supported() {
		return ReviewTarget{}, domain.ErrNotFound
	}
	run, err := s.repository.GetRun(ctx, runID)
	if err != nil {
		return ReviewTarget{}, err
	}
	if run.State != domain.RunRunning || run.CurrentStageKind != domain.StageReview {
		return ReviewTarget{}, domain.IllegalTransitionError{Entity: "run", From: string(run.CurrentStageKind), To: "review session"}
	}
	if run.WorktreePath == "" || run.WorktreeRemoved {
		return ReviewTarget{}, domain.Invalid("this run has no worktree left to open")
	}
	task, err := s.repository.GetTask(ctx, run.TaskID)
	if err != nil {
		return ReviewTarget{}, err
	}
	var prompt strings.Builder
	prompt.WriteString("You are helping a person review a change the harness made for " + task.Source.Key + ": " + task.Title + ".\n")
	prompt.WriteString("This folder is the run's own worktree, on branch " + run.Branch + ". The change passed its tests.\n")
	if run.PlanPath != "" {
		prompt.WriteString("The approved plan is " + run.PlanPath + ".\n")
	}
	prompt.WriteString("Start by summarising what changed (git log and git diff against the base). Then help the person try it and make the changes they ask for. Do not push. The harness commits what is left when the person hands the change off.")
	return ReviewTarget{WorkspaceID: run.WorkspaceID, Dir: run.WorktreePath, Branch: run.Branch, Prompt: prompt.String()}, nil
}

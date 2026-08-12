package git

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/pathguard"
	"kode-stream/internal/filesystem/writeguard"
)

type WorkspaceRepository interface {
	Get(string) (models.WorkspaceConfig, bool, error)
}

type ItemRefresher interface {
	RefreshWorkspace(models.WorkspaceConfig) (models.ScanResult, error)
}

type GitRepository interface {
	WithWorkspaceMutation(string, func() error) error
	Status(string, string) (models.GitStatus, error)
	Activity(string, string, int) ([]models.GitActivityEntry, error)
	CurrentBranch(string) (string, error)
	ListBranches(string) ([]string, error)
	ListStashes(string) ([]models.GitStashEntry, error)
	ApplyStash(string, string) error
	Fetch(string) error
	Pull(string) error
	Push(string) error
	Commit(string, string, []string) error
	CreateBranch(string, string, string, bool) error
	SwitchBranch(string, string) error
	SwitchBranchSafely(string, string, string, string) (string, error)
	CanCarryChanges(string, string, models.GitStatus) (bool, error)
}

type GitService struct {
	registry WorkspaceRepository
	writer   ItemRefresher
	git      GitRepository
}

type Service = GitService

type workspaceMutationContextKey struct{}

func NewService(reg WorkspaceRepository, writer ItemRefresher, git GitRepository) *GitService {
	return &GitService{registry: reg, writer: writer, git: git}
}

func (s *Service) withMutation(ctx context.Context, workspacePath string, action func() error) error {
	if held, _ := ctx.Value(workspaceMutationContextKey{}).(string); held == workspacePath {
		return action()
	}
	if git, ok := s.git.(interface {
		WithWorkspaceMutationContext(context.Context, string, func() error) error
	}); ok {
		return git.WithWorkspaceMutationContext(ctx, workspacePath, action)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.git.WithWorkspaceMutation(workspacePath, action)
}

// WithWorkspaceMutationContext exposes the established workspace mutation
// boundary to cohesive services that need to perform a multi-step operation.
// The context passed to action is marked as already holding the lock so the
// service's own mutation methods can safely participate without re-locking.
func (s *Service) WithWorkspaceMutationContext(ctx context.Context, workspaceID string, action func(context.Context) error) error {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return err
	}
	return s.withMutation(ctx, workspace.Path, func() error {
		return action(context.WithValue(ctx, workspaceMutationContextKey{}, workspace.Path))
	})
}

func (s *Service) Status(workspaceID string) (models.GitStatus, error) {
	return s.StatusContext(context.Background(), workspaceID)
}
func (s *Service) StatusContext(ctx context.Context, workspaceID string) (models.GitStatus, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.GitStatus{}, err
	}
	if git, ok := s.git.(interface {
		StatusContext(context.Context, string, string) (models.GitStatus, error)
	}); ok {
		return git.StatusContext(ctx, workspace.ID, workspace.Path)
	}
	if err := ctx.Err(); err != nil {
		return models.GitStatus{}, err
	}
	return s.git.Status(workspace.ID, workspace.Path)
}

func (s *Service) Branches(workspaceID string) (models.WorkspaceBranches, error) {
	return s.BranchesContext(context.Background(), workspaceID)
}
func (s *Service) BranchesContext(ctx context.Context, workspaceID string) (models.WorkspaceBranches, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.WorkspaceBranches{}, err
	}
	var current string
	if git, ok := s.git.(interface {
		CurrentBranchContext(context.Context, string) (string, error)
	}); ok {
		current, err = git.CurrentBranchContext(ctx, workspace.Path)
	} else if err = ctx.Err(); err == nil {
		current, err = s.git.CurrentBranch(workspace.Path)
	}
	if err != nil {
		return models.WorkspaceBranches{}, err
	}
	var branches []string
	if git, ok := s.git.(interface {
		ListBranchesContext(context.Context, string) ([]string, error)
	}); ok {
		branches, err = git.ListBranchesContext(ctx, workspace.Path)
	} else if err = ctx.Err(); err == nil {
		branches, err = s.git.ListBranches(workspace.Path)
	}
	if err != nil {
		return models.WorkspaceBranches{}, err
	}
	return normalizeWorkspaceBranches(workspace.ID, current, branches), nil
}

func (s *Service) Stashes(workspaceID string) ([]models.GitStashEntry, error) {
	return s.StashesContext(context.Background(), workspaceID)
}
func (s *Service) StashesContext(ctx context.Context, workspaceID string) ([]models.GitStashEntry, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if git, ok := s.git.(interface {
		ListStashesContext(context.Context, string) ([]models.GitStashEntry, error)
	}); ok {
		return git.ListStashesContext(ctx, workspace.Path)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.git.ListStashes(workspace.Path)
}

func (s *Service) ApplyStash(workspaceID, ref string) models.GitOperationResult {
	return s.ApplyStashContext(context.Background(), workspaceID, ref)
}
func (s *Service) ApplyStashContext(ctx context.Context, workspaceID, ref string) models.GitOperationResult {
	workspace, err := s.workspace(workspaceID)
	if err == nil {
		err = s.withMutation(ctx, workspace.Path, func() error {
			if git, ok := s.git.(interface {
				ApplyStashContext(context.Context, string, string) error
			}); ok {
				return git.ApplyStashContext(ctx, workspace.Path, ref)
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return s.git.ApplyStash(workspace.Path, ref)
		})
	}
	return s.mutationResult(ctx, workspace, err == nil, err)
}

func (s *Service) Activity(workspaceID, relPath string, limit int) ([]models.GitActivityEntry, error) {
	return s.ActivityContext(context.Background(), workspaceID, relPath, limit)
}
func (s *Service) ActivityContext(ctx context.Context, workspaceID, relPath string, limit int) ([]models.GitActivityEntry, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return nil, err
	}
	cleanPath := ""
	if strings.TrimSpace(relPath) != "" {
		cleanPath, err = pathguard.CleanRelative(relPath)
		if err != nil {
			return nil, err
		}
	}
	if git, ok := s.git.(interface {
		ActivityContext(context.Context, string, string, int) ([]models.GitActivityEntry, error)
	}); ok {
		return git.ActivityContext(ctx, workspace.Path, cleanPath, limit)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.git.Activity(workspace.Path, cleanPath, limit)
}

func normalizeWorkspaceBranches(workspaceID, current string, branches []string) models.WorkspaceBranches {
	unique := make(map[string]struct{}, len(branches)+1)
	for _, branch := range branches {
		if branch = strings.TrimSpace(branch); branch != "" {
			unique[branch] = struct{}{}
		}
	}
	if current = strings.TrimSpace(current); current != "" {
		unique[current] = struct{}{}
	}
	names := make([]string, 0, len(unique))
	for branch := range unique {
		names = append(names, branch)
	}
	sort.Strings(names)
	return models.WorkspaceBranches{WorkspaceID: workspaceID, Current: current, Branches: names}
}

func (s *Service) Fetch(workspaceID string, _ models.GitOperationInput) models.GitOperationResult {
	return s.FetchContext(context.Background(), workspaceID)
}
func (s *Service) FetchContext(ctx context.Context, workspaceID string) models.GitOperationResult {
	workspace, err := s.workspace(workspaceID)
	if err == nil {
		err = s.withMutation(ctx, workspace.Path, func() error {
			if git, ok := s.git.(interface {
				FetchContext(context.Context, string) error
			}); ok {
				return git.FetchContext(ctx, workspace.Path)
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return s.git.Fetch(workspace.Path)
		})
	}
	return s.mutationResult(ctx, workspace, err == nil, err)
}

func (s *Service) Pull(workspaceID string, input models.GitOperationInput) models.GitOperationResult {
	return s.PullContext(context.Background(), workspaceID, input)
}
func (s *Service) PullContext(ctx context.Context, workspaceID string, input models.GitOperationInput) models.GitOperationResult {
	workspace, err := s.workspace(workspaceID)
	if err == nil {
		err = s.withMutation(ctx, workspace.Path, func() error {
			status, statusErr := s.StatusContext(ctx, workspace.ID)
			if statusErr != nil {
				return statusErr
			}
			if status.Dirty || status.Conflicted {
				return fmt.Errorf("working tree has local changes; stash or commit before pulling")
			}
			if git, ok := s.git.(interface {
				PullContext(context.Context, string) error
			}); ok {
				return git.PullContext(ctx, workspace.Path)
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return s.git.Pull(workspace.Path)
		})
	}
	return s.mutationResult(ctx, workspace, err == nil, err)
}

func (s *Service) Push(workspaceID string, _ models.GitOperationInput) models.GitOperationResult {
	return s.PushContext(context.Background(), workspaceID)
}
func (s *Service) PushContext(ctx context.Context, workspaceID string) models.GitOperationResult {
	workspace, err := s.workspace(workspaceID)
	if err == nil {
		err = s.withMutation(ctx, workspace.Path, func() error {
			if git, ok := s.git.(interface {
				PushContext(context.Context, string) error
			}); ok {
				return git.PushContext(ctx, workspace.Path)
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return s.git.Push(workspace.Path)
		})
	}
	return s.mutationResult(ctx, workspace, err == nil, err)
}

func (s *Service) Commit(workspaceID string, input models.GitCommitInput) models.GitOperationResult {
	return s.CommitContext(context.Background(), workspaceID, input)
}
func (s *Service) CommitContext(ctx context.Context, workspaceID string, input models.GitCommitInput) models.GitOperationResult {
	workspace, err := s.workspace(workspaceID)
	if err == nil {
		if err = writeguard.ValidateCommitMessage(input.Message); err == nil {
			err = ValidatePaths(workspace, input.Paths)
		}
		if err == nil {
			err = s.withMutation(ctx, workspace.Path, func() error {
				if git, ok := s.git.(interface {
					CommitContext(context.Context, string, string, []string) error
				}); ok {
					return git.CommitContext(ctx, workspace.Path, input.Message, input.Paths)
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				return s.git.Commit(workspace.Path, input.Message, input.Paths)
			})
		}
	}
	return s.mutationResult(ctx, workspace, err == nil, err)
}

func (s *Service) CreateBranch(workspaceID string, input models.BranchCreateInput) models.GitOperationResult {
	return s.CreateBranchContext(context.Background(), workspaceID, input)
}
func (s *Service) CreateBranchContext(ctx context.Context, workspaceID string, input models.BranchCreateInput) models.GitOperationResult {
	workspace, err := s.workspace(workspaceID)
	if err == nil {
		if err = writeguard.ValidateBranchName(input.Name); err == nil {
			err = s.withMutation(ctx, workspace.Path, func() error {
				if git, ok := s.git.(interface {
					CreateBranchContext(context.Context, string, string, string, bool) error
				}); ok {
					return git.CreateBranchContext(ctx, workspace.Path, input.Name, input.StartPoint, input.Checkout)
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				return s.git.CreateBranch(workspace.Path, input.Name, input.StartPoint, input.Checkout)
			})
		}
	}
	return s.mutationResult(ctx, workspace, err == nil, err)
}

func (s *Service) SwitchBranch(workspaceID string, input models.BranchSwitchInput) models.GitOperationResult {
	return s.SwitchBranchContext(context.Background(), workspaceID, input)
}
func (s *Service) SwitchBranchContext(ctx context.Context, workspaceID string, input models.BranchSwitchInput) models.GitOperationResult {
	workspace, err := s.workspace(workspaceID)
	var stashRef string
	if err == nil {
		if err = writeguard.ValidateBranchName(input.Name); err == nil {
			err = s.withMutation(ctx, workspace.Path, func() error {
				if git, ok := s.git.(interface {
					SwitchBranchSafelyContext(context.Context, string, string, string, string) (string, error)
				}); ok {
					stashRef, err = git.SwitchBranchSafelyContext(ctx, workspace.Path, input.Name, input.Strategy, input.StashMessage)
				} else {
					if err = ctx.Err(); err == nil {
						stashRef, err = s.git.SwitchBranchSafely(workspace.Path, input.Name, input.Strategy, input.StashMessage)
					}
				}
				return err
			})
		}
	}
	result := s.mutationResult(ctx, workspace, err == nil, err)
	result.StashRef, result.StashMessage = stashRef, input.StashMessage
	if errors.Is(err, ErrBranchSwitchDecisionRequired) || errors.Is(err, ErrCarryChangesUnsafe) {
		result.Code = "branch_switch_decision_required"
		status, statusErr := s.StatusContext(ctx, workspace.ID)
		if statusErr == nil {
			canCarry := false
			if git, ok := s.git.(interface {
				CanCarryChangesContext(context.Context, string, string, models.GitStatus) (bool, error)
			}); ok {
				canCarry, _ = git.CanCarryChangesContext(ctx, workspace.Path, input.Name, status)
			} else if ctx.Err() == nil {
				canCarry, _ = s.git.CanCarryChanges(workspace.Path, input.Name, status)
			}
			result.Decision = &models.BranchSwitchDecision{SourceBranch: status.Branch, TargetBranch: input.Name, CanCarryChanges: canCarry}
			result.Details = map[string]string{"sourceBranch": status.Branch, "targetBranch": input.Name, "canCarryChanges": strconv.FormatBool(canCarry)}
		}
	}
	if stashRef != "" && err != nil {
		result.RecoveryHint = "Your local changes were stashed as " + stashRef + ". Apply it manually when ready."
	}
	return result
}

func (s *Service) mutationResult(ctx context.Context, workspace models.WorkspaceConfig, committed bool, opErr error) models.GitOperationResult {
	if opErr != nil {
		result := s.result(workspace, opErr)
		var partial partialOperationError
		if errors.As(opErr, &partial) {
			result.Partial = true
			result.RecoveryHint = partial.recovery
		}
		return result
	}
	result := models.GitOperationResult{OK: true, Committed: committed}
	if s.writer != nil {
		if _, err := s.writer.RefreshWorkspace(workspace); err != nil {
			result.RefreshRequired, result.RefreshError, result.RecoveryHint = true, err.Error(), "Git operation completed; reload the workspace to refresh its index."
		}
	}
	status, err := s.StatusContext(ctx, workspace.ID)
	if err != nil {
		result.RefreshRequired = true
		if result.RefreshError == "" {
			result.RefreshError = err.Error()
		}
		result.RecoveryHint = "Git operation completed; reload the workspace to refresh its status."
	} else {
		result.Status = status
	}
	return result
}

func (s *Service) workspace(workspaceID string) (models.WorkspaceConfig, error) {
	workspace, ok, err := s.registry.Get(workspaceID)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	if !ok {
		return models.WorkspaceConfig{}, apperrors.ErrWorkspaceNotFound
	}
	return workspace, nil
}

func (s *Service) result(workspace models.WorkspaceConfig, opErr error) models.GitOperationResult {
	status := models.GitStatus{}
	if workspace.ID != "" {
		statusResult, statusErr := s.git.Status(workspace.ID, workspace.Path)
		status = statusResult
		if statusErr != nil && opErr == nil {
			opErr = statusErr
		}
	}
	result := models.GitOperationResult{OK: opErr == nil, Status: status}
	if opErr != nil {
		result.Message = opErr.Error()
	}
	return result
}

func ValidatePaths(workspace models.WorkspaceConfig, paths []string) error {
	return pathguard.ValidateSourcePaths(workspace.Sources, paths)
}

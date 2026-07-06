package git

import (
	"fmt"
	"sort"
	"strings"

	apperrors "plan-manager/internal/common"
	"plan-manager/internal/common/models"
	"plan-manager/internal/filesystem/pathguard"
	"plan-manager/internal/filesystem/writeguard"
)

type WorkspaceRepository interface {
	Get(string) (models.WorkspaceConfig, bool, error)
}

type ItemRefresher interface {
	RefreshWorkspace(models.WorkspaceConfig) (models.ScanResult, error)
}

type GitRepository interface {
	Status(string, string) (models.GitStatus, error)
	Activity(string, string, int) ([]models.GitActivityEntry, error)
	CurrentBranch(string) (string, error)
	ListBranches(string) ([]string, error)
	Fetch(string) error
	Pull(string) error
	Push(string) error
	Commit(string, string, []string) error
	CreateBranch(string, string, string, bool) error
	SwitchBranch(string, string) error
}

type GitService struct {
	registry WorkspaceRepository
	writer   ItemRefresher
	git      GitRepository
}

type Service = GitService

func NewService(reg WorkspaceRepository, writer ItemRefresher, git GitRepository) *GitService {
	return &GitService{registry: reg, writer: writer, git: git}
}

func (s *Service) Status(workspaceID string) (models.GitStatus, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.GitStatus{}, err
	}
	return s.git.Status(workspace.ID, workspace.Path)
}

func (s *Service) Branches(workspaceID string) (models.WorkspaceBranches, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.WorkspaceBranches{}, err
	}
	current, err := s.git.CurrentBranch(workspace.Path)
	if err != nil {
		return models.WorkspaceBranches{}, err
	}
	branches, err := s.git.ListBranches(workspace.Path)
	if err != nil {
		return models.WorkspaceBranches{}, err
	}
	return normalizeWorkspaceBranches(workspace.ID, current, branches), nil
}

func (s *Service) Activity(workspaceID, relPath string, limit int) ([]models.GitActivityEntry, error) {
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
	workspace, err := s.workspace(workspaceID)
	if err == nil {
		err = s.git.Fetch(workspace.Path)
	}
	return s.result(workspace, err)
}

func (s *Service) Pull(workspaceID string, input models.GitOperationInput) models.GitOperationResult {
	workspace, err := s.workspace(workspaceID)
	if err == nil {
		status, statusErr := s.git.Status(workspace.ID, workspace.Path)
		if statusErr != nil {
			err = statusErr
		} else if (status.Dirty || status.Conflicted) && !input.Confirm {
			err = fmt.Errorf("working tree has local changes; confirm to pull")
		} else if err = s.git.Pull(workspace.Path); err == nil {
			_, err = s.writer.RefreshWorkspace(workspace)
		}
	}
	return s.result(workspace, err)
}

func (s *Service) Push(workspaceID string, _ models.GitOperationInput) models.GitOperationResult {
	workspace, err := s.workspace(workspaceID)
	if err == nil {
		err = s.git.Push(workspace.Path)
	}
	return s.result(workspace, err)
}

func (s *Service) Commit(workspaceID string, input models.GitCommitInput) models.GitOperationResult {
	workspace, err := s.workspace(workspaceID)
	if err == nil {
		if err = writeguard.ValidateCommitMessage(input.Message); err == nil {
			err = ValidatePaths(workspace, input.Paths)
		}
		if err == nil {
			err = s.git.Commit(workspace.Path, input.Message, input.Paths)
		}
		if err == nil {
			_, err = s.writer.RefreshWorkspace(workspace)
		}
	}
	return s.result(workspace, err)
}

func (s *Service) CreateBranch(workspaceID string, input models.BranchCreateInput) models.GitOperationResult {
	workspace, err := s.workspace(workspaceID)
	if err == nil {
		if err = writeguard.ValidateBranchName(input.Name); err == nil {
			err = s.git.CreateBranch(workspace.Path, input.Name, input.StartPoint, input.Checkout)
		}
		if err == nil && input.Checkout {
			_, err = s.writer.RefreshWorkspace(workspace)
		}
	}
	return s.result(workspace, err)
}

func (s *Service) SwitchBranch(workspaceID string, input models.BranchSwitchInput) models.GitOperationResult {
	workspace, err := s.workspace(workspaceID)
	if err == nil {
		if err = writeguard.ValidateBranchName(input.Name); err == nil {
			var status models.GitStatus
			status, err = s.git.Status(workspace.ID, workspace.Path)
			if err == nil && (status.Dirty || status.Conflicted) && !input.Confirm {
				err = fmt.Errorf("working tree has local changes; confirm to switch branches")
			}
		}
		if err == nil {
			err = s.git.SwitchBranch(workspace.Path, input.Name)
		}
		if err == nil {
			_, err = s.writer.RefreshWorkspace(workspace)
		}
	}
	return s.result(workspace, err)
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

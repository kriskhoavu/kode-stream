package git

import (
	"fmt"
	"path/filepath"
	"strings"

	"plan-manager/internal/application/apperrors"
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/itemwriter"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
	"plan-manager/internal/writeguard"
)

type Service struct {
	registry *registry.Registry
	writer   *itemwriter.Writer
	git      *gitadapter.GitAdapter
}

func New(reg *registry.Registry, writer *itemwriter.Writer, git *gitadapter.GitAdapter) *Service {
	return &Service{registry: reg, writer: writer, git: git}
}

func (s *Service) Status(workspaceID string) (models.GitStatus, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.GitStatus{}, err
	}
	return s.git.Status(workspace.ID, workspace.Path)
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
	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}
	for _, path := range paths {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
		if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "../") || clean == ".." {
			return fmt.Errorf("path %q is invalid", path)
		}
		allowed := false
		for _, dir := range workspace.Sources {
			if clean == dir || strings.HasPrefix(clean, dir+"/") {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("path %q is outside configured sources", path)
		}
	}
	return nil
}

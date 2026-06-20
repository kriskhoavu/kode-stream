package workspacefiles

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"plan-manager/internal/application/apperrors"
	"plan-manager/internal/models"
	workspaceaccess "plan-manager/internal/workspacefiles"
)

type Registry interface {
	Get(id string) (models.WorkspaceConfig, bool, error)
}

type Access interface {
	List(workspace models.WorkspaceConfig, path string, includeIgnored bool) (models.WorkspaceDirectoryListing, error)
	Read(workspace models.WorkspaceConfig, path string) (models.FileContent, error)
	WriteMarkdown(workspace models.WorkspaceConfig, input models.WorkspaceFileSaveInput) (models.FileContent, error)
	ResolveFile(workspace models.WorkspaceConfig, path string) (string, string, error)
}

type Git interface {
	Diff(workspacePath, relPath string) (string, error)
	RevertPaths(workspacePath string, paths []string) error
}

type Audit interface {
	Append(event models.AuditEvent) (models.AuditEvent, error)
}

type Refresher interface {
	RefreshWorkspace(workspace models.WorkspaceConfig) (models.ScanResult, error)
}

type Service struct {
	registry  Registry
	files     Access
	git       Git
	audit     Audit
	refresher Refresher
}

func New(registry Registry, files Access, git Git, audit Audit, refresher Refresher) *Service {
	return &Service{registry: registry, files: files, git: git, audit: audit, refresher: refresher}
}

func (s *Service) List(workspaceID, path string, includeIgnored bool) (models.WorkspaceDirectoryListing, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.WorkspaceDirectoryListing{}, err
	}
	return s.files.List(workspace, path, includeIgnored)
}

func (s *Service) Read(workspaceID, path string) (models.FileContent, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.FileContent{}, err
	}
	return s.files.Read(workspace, path)
}

func (s *Service) Save(workspaceID string, input models.WorkspaceFileSaveInput) (models.WorkspaceFileWriteResult, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.WorkspaceFileWriteResult{}, err
	}
	started := time.Now()
	file, err := s.files.WriteMarkdown(workspace, input)
	if err != nil {
		s.record(workspace.ID, "workspace_file_save", input.Path, started, err)
		return models.WorkspaceFileWriteResult{}, err
	}
	refreshed, err := s.refreshIfSource(workspace, file.Path)
	s.record(workspace.ID, "workspace_file_save", file.Path, started, err)
	if err != nil {
		return models.WorkspaceFileWriteResult{}, err
	}
	return models.WorkspaceFileWriteResult{File: file, Refreshed: refreshed}, nil
}

func (s *Service) Diff(workspaceID, path string) (string, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return "", err
	}
	clean, _, err := s.files.ResolveFile(workspace, path)
	if err != nil {
		return "", err
	}
	diff, err := s.git.Diff(workspace.Path, clean)
	if err != nil {
		return "", fmt.Errorf("diff unavailable: %w", err)
	}
	return diff, nil
}

func (s *Service) Revert(workspaceID string, input models.WorkspaceFileRevertInput) (models.WorkspaceFileWriteResult, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.WorkspaceFileWriteResult{}, err
	}
	clean, _, err := s.files.ResolveFile(workspace, input.Path)
	if err != nil {
		return models.WorkspaceFileWriteResult{}, err
	}
	started := time.Now()
	if err := s.git.RevertPaths(workspace.Path, []string{clean}); err != nil {
		s.record(workspace.ID, "workspace_file_revert", clean, started, err)
		return models.WorkspaceFileWriteResult{}, err
	}
	file, err := s.files.Read(workspace, clean)
	if err != nil {
		s.record(workspace.ID, "workspace_file_revert", clean, started, err)
		return models.WorkspaceFileWriteResult{}, err
	}
	refreshed, err := s.refreshIfSource(workspace, clean)
	s.record(workspace.ID, "workspace_file_revert", clean, started, err)
	if err != nil {
		return models.WorkspaceFileWriteResult{}, err
	}
	return models.WorkspaceFileWriteResult{File: file, Refreshed: refreshed}, nil
}

func (s *Service) workspace(id string) (models.WorkspaceConfig, error) {
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	if !ok {
		return models.WorkspaceConfig{}, apperrors.ErrWorkspaceNotFound
	}
	return workspace, nil
}

func (s *Service) refreshIfSource(workspace models.WorkspaceConfig, path string) (bool, error) {
	clean := filepath.ToSlash(filepath.Clean(path))
	for _, source := range workspace.Sources {
		source = strings.TrimSuffix(filepath.ToSlash(filepath.Clean(source)), "/")
		if clean == source || strings.HasPrefix(clean, source+"/") {
			if s.refresher == nil {
				return false, nil
			}
			_, err := s.refresher.RefreshWorkspace(workspace)
			return err == nil, err
		}
	}
	return false, nil
}

func (s *Service) record(workspaceID, operation, path string, started time.Time, opErr error) {
	if s.audit == nil {
		return
	}
	status := models.AuditStatusSuccess
	event := models.AuditEvent{
		WorkspaceID: workspaceID,
		Operation:   operation,
		Status:      status,
		Message:     operation,
		Paths:       []string{path},
		DurationMS:  time.Since(started).Milliseconds(),
	}
	if opErr != nil {
		event.Status = models.AuditStatusFailed
		event.Error = opErr.Error()
	}
	_, _ = s.audit.Append(event)
}

var _ Access = (*workspaceaccess.Access)(nil)

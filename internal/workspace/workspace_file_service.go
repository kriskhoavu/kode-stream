package workspace

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	workspaceaccess "kode-stream/internal/workspace/files"
)

type Registry interface {
	Get(id string) (models.WorkspaceConfig, bool, error)
	List() ([]models.WorkspaceConfig, error)
}

type Access interface {
	List(workspace models.WorkspaceConfig, path string, includeIgnored bool) (models.WorkspaceDirectoryListing, error)
	Read(workspace models.WorkspaceConfig, path string) (models.FileContent, error)
	WriteMarkdown(workspace models.WorkspaceConfig, input models.WorkspaceFileSaveInput) (models.FileContent, error)
	ResolveFile(workspace models.WorkspaceConfig, path string) (string, string, error)
	Search(workspace models.WorkspaceConfig, query string, includeIgnored bool) (models.WorkspacePathSearchResponse, error)
	CreateFile(workspace models.WorkspaceConfig, input models.WorkspaceFileCreateInput) (models.WorkspacePathMutationResult, error)
	CreateDirectory(workspace models.WorkspaceConfig, input models.WorkspaceDirectoryCreateInput) (models.WorkspacePathMutationResult, error)
	Rename(workspace models.WorkspaceConfig, input models.WorkspacePathRenameInput) (models.WorkspacePathMutationResult, error)
}
type contextSearchAccess interface {
	SearchContext(context.Context, models.WorkspaceConfig, string, bool) (models.WorkspacePathSearchResponse, error)
}

type Git interface {
	WithWorkspaceMutation(workspacePath string, action func() error) error
	Diff(workspacePath, relPath string) (string, error)
	RevertPaths(workspacePath string, paths []string) error
	PathStates(workspaceID, workspacePath string) ([]models.WorkspacePathGitState, error)
}
type contextGit interface {
	WithWorkspaceMutationContext(context.Context, string, func() error) error
	DiffContext(context.Context, string, string) (string, error)
	RevertPathsContext(context.Context, string, []string) error
}

type Audit interface {
	Append(event models.AuditEvent) (models.AuditEvent, error)
}

type Refresher interface {
	RefreshWorkspace(workspace models.WorkspaceConfig) (models.ScanResult, error)
}

type WorkspaceFileService struct {
	registry  Registry
	files     Access
	git       Git
	audit     Audit
	refresher Refresher
}

func NewWorkspaceFileService(registry Registry, files Access, git Git, audit Audit, refresher Refresher) *WorkspaceFileService {
	return &WorkspaceFileService{registry: registry, files: files, git: git, audit: audit, refresher: refresher}
}

func (s *WorkspaceFileService) List(workspaceID, path string, includeIgnored bool) (models.WorkspaceDirectoryListing, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.WorkspaceDirectoryListing{}, err
	}
	return s.files.List(workspace, path, includeIgnored)
}

func (s *WorkspaceFileService) Read(workspaceID, path string) (models.FileContent, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.FileContent{}, err
	}
	return s.files.Read(workspace, path)
}

func (s *WorkspaceFileService) Search(query, workspaceID string, includeIgnored bool) (models.WorkspacePathSearchResponse, error) {
	return s.SearchContext(context.Background(), query, workspaceID, includeIgnored)
}

func (s *WorkspaceFileService) SearchContext(ctx context.Context, query, workspaceID string, includeIgnored bool) (models.WorkspacePathSearchResponse, error) {
	if err := workspaceaccess.ValidateSearchQuery(query); err != nil {
		return models.WorkspacePathSearchResponse{}, err
	}
	var workspaces []models.WorkspaceConfig
	if workspaceID != "" {
		workspace, err := s.workspace(workspaceID)
		if err != nil {
			return models.WorkspacePathSearchResponse{}, err
		}
		workspaces = []models.WorkspaceConfig{workspace}
	} else {
		var err error
		workspaces, err = s.registry.List()
		if err != nil {
			return models.WorkspacePathSearchResponse{}, err
		}
	}
	response := models.WorkspacePathSearchResponse{Results: []models.WorkspacePathSearchResult{}}
	for _, workspace := range workspaces {
		if err := ctx.Err(); err != nil {
			return models.WorkspacePathSearchResponse{}, err
		}
		var (
			result models.WorkspacePathSearchResponse
			err    error
		)
		if contextual, ok := s.files.(contextSearchAccess); ok {
			result, err = contextual.SearchContext(ctx, workspace, query, includeIgnored)
		} else {
			result, err = s.files.Search(workspace, query, includeIgnored)
		}
		if err != nil {
			return models.WorkspacePathSearchResponse{}, err
		}
		response.Results = append(response.Results, result.Results...)
		response.Truncated = response.Truncated || result.Truncated
		if len(response.Results) >= workspaceaccess.MaxSearchResults {
			response.Results = response.Results[:workspaceaccess.MaxSearchResults]
			response.Truncated = true
			break
		}
	}
	return response, nil
}

func (s *WorkspaceFileService) PathStates(workspaceID string) ([]models.WorkspacePathGitState, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return nil, err
	}
	return s.git.PathStates(workspace.ID, workspace.Path)
}

func (s *WorkspaceFileService) CreateFile(workspaceID string, input models.WorkspaceFileCreateInput) (models.WorkspacePathMutationResult, error) {
	return s.mutate(workspaceID, "workspace_file_create", []string{joinInputPath(input.ParentPath, input.Name)}, func(workspace models.WorkspaceConfig) (models.WorkspacePathMutationResult, error) {
		return s.files.CreateFile(workspace, input)
	})
}

func (s *WorkspaceFileService) CreateDirectory(workspaceID string, input models.WorkspaceDirectoryCreateInput) (models.WorkspacePathMutationResult, error) {
	return s.mutate(workspaceID, "workspace_directory_create", []string{joinInputPath(input.ParentPath, input.Name)}, func(workspace models.WorkspaceConfig) (models.WorkspacePathMutationResult, error) {
		return s.files.CreateDirectory(workspace, input)
	})
}

func (s *WorkspaceFileService) Rename(workspaceID string, input models.WorkspacePathRenameInput) (models.WorkspacePathMutationResult, error) {
	return s.mutate(workspaceID, "workspace_path_rename", []string{input.Path, input.DestinationPath}, func(workspace models.WorkspaceConfig) (models.WorkspacePathMutationResult, error) {
		return s.files.Rename(workspace, input)
	})
}

func (s *WorkspaceFileService) mutate(workspaceID, operation string, auditPaths []string, run func(models.WorkspaceConfig) (models.WorkspacePathMutationResult, error)) (models.WorkspacePathMutationResult, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.WorkspacePathMutationResult{}, err
	}
	started := time.Now()
	result, err := run(workspace)
	if err != nil {
		s.record(workspace.ID, operation, auditPaths, started, err)
		return models.WorkspacePathMutationResult{}, err
	}
	paths := append([]string(nil), auditPaths...)
	refreshed, err := s.refreshIfAnySource(workspace, paths)
	result.Refreshed = refreshed
	result.Committed = true
	if err != nil {
		result.RefreshRequired = true
		result.RefreshError = err.Error()
		s.recordCommittedRefreshFailure(workspace.ID, operation, auditPaths, started, err)
		return result, nil
	}
	s.record(workspace.ID, operation, auditPaths, started, nil)
	return result, nil
}

func (s *WorkspaceFileService) Save(workspaceID string, input models.WorkspaceFileSaveInput) (models.WorkspaceFileWriteResult, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.WorkspaceFileWriteResult{}, err
	}
	started := time.Now()
	file, err := s.files.WriteMarkdown(workspace, input)
	if err != nil {
		s.record(workspace.ID, "workspace_file_save", []string{input.Path}, started, err)
		return models.WorkspaceFileWriteResult{}, err
	}
	refreshed, err := s.refreshIfSource(workspace, file.Path)
	result := models.WorkspaceFileWriteResult{File: file, Refreshed: refreshed, Committed: true}
	if err != nil {
		result.RefreshRequired = true
		result.RefreshError = err.Error()
		s.recordCommittedRefreshFailure(workspace.ID, "workspace_file_save", []string{file.Path}, started, err)
		return result, nil
	}
	s.record(workspace.ID, "workspace_file_save", []string{file.Path}, started, nil)
	return result, nil
}

func (s *WorkspaceFileService) Diff(workspaceID, path string) (string, error) {
	return s.DiffContext(context.Background(), workspaceID, path)
}
func (s *WorkspaceFileService) DiffContext(ctx context.Context, workspaceID, path string) (string, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return "", err
	}
	clean, _, err := s.files.ResolveFile(workspace, path)
	if err != nil {
		return "", err
	}
	diff, err := s.git.Diff(workspace.Path, clean)
	if git, ok := s.git.(contextGit); ok {
		diff, err = git.DiffContext(ctx, workspace.Path, clean)
	} else if err = ctx.Err(); err != nil {
		return "", err
	}
	if err != nil {
		return "", fmt.Errorf("diff unavailable: %w", err)
	}
	return diff, nil
}

func (s *WorkspaceFileService) Revert(workspaceID string, input models.WorkspaceFileRevertInput) (models.WorkspaceFileWriteResult, error) {
	return s.RevertContext(context.Background(), workspaceID, input)
}
func (s *WorkspaceFileService) RevertContext(ctx context.Context, workspaceID string, input models.WorkspaceFileRevertInput) (models.WorkspaceFileWriteResult, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.WorkspaceFileWriteResult{}, err
	}
	clean, _, err := s.files.ResolveFile(workspace, input.Path)
	if err != nil {
		return models.WorkspaceFileWriteResult{}, err
	}
	started := time.Now()
	var file models.FileContent
	var refreshed bool
	action := func() error {
		if git, ok := s.git.(contextGit); ok {
			err = git.RevertPathsContext(ctx, workspace.Path, []string{clean})
		} else if err = ctx.Err(); err == nil {
			err = s.git.RevertPaths(workspace.Path, []string{clean})
		}
		if err != nil {
			return err
		}
		file, err = s.files.Read(workspace, clean)
		if err != nil {
			return err
		}
		refreshed, err = s.refreshIfSource(workspace, clean)
		return err
	}
	if git, ok := s.git.(contextGit); ok {
		err = git.WithWorkspaceMutationContext(ctx, workspace.Path, action)
	} else if err = ctx.Err(); err == nil {
		err = s.git.WithWorkspaceMutation(workspace.Path, action)
	}
	result := models.WorkspaceFileWriteResult{File: file, Refreshed: refreshed, Committed: true}
	if err != nil {
		if file.Path != "" {
			result.RefreshRequired, result.RefreshError = true, err.Error()
			s.recordCommittedRefreshFailure(workspace.ID, "workspace_file_revert", []string{clean}, started, err)
			return result, nil
		}
		s.record(workspace.ID, "workspace_file_revert", []string{clean}, started, err)
		return models.WorkspaceFileWriteResult{}, err
	}
	s.record(workspace.ID, "workspace_file_revert", []string{clean}, started, nil)
	return result, nil
}

func (s *WorkspaceFileService) workspace(id string) (models.WorkspaceConfig, error) {
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	if !ok {
		return models.WorkspaceConfig{}, apperrors.ErrWorkspaceNotFound
	}
	return workspace, nil
}

func (s *WorkspaceFileService) refreshIfSource(workspace models.WorkspaceConfig, path string) (bool, error) {
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

func (s *WorkspaceFileService) refreshIfAnySource(workspace models.WorkspaceConfig, paths []string) (bool, error) {
	for _, path := range paths {
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
	}
	return false, nil
}

func (s *WorkspaceFileService) record(workspaceID, operation string, paths []string, started time.Time, opErr error) {
	if s.audit == nil {
		return
	}
	status := models.AuditStatusSuccess
	event := models.AuditEvent{
		WorkspaceID: workspaceID,
		Operation:   operation,
		Status:      status,
		Message:     operation,
		Paths:       paths,
		DurationMS:  time.Since(started).Milliseconds(),
	}
	if opErr != nil {
		event.Status = models.AuditStatusFailed
		if isBlockedMutation(opErr) {
			event.Status = models.AuditStatusBlocked
		}
		event.Error = opErr.Error()
	}
	_, _ = s.audit.Append(event)
}

func (s *WorkspaceFileService) recordCommittedRefreshFailure(workspaceID, operation string, paths []string, started time.Time, refreshErr error) {
	if s.audit == nil {
		return
	}
	_, _ = s.audit.Append(models.AuditEvent{
		WorkspaceID: workspaceID, Operation: operation, Status: models.AuditStatusSuccess,
		Message: operation + " committed; derived index refresh is required", Paths: paths,
		Error: refreshErr.Error(), DurationMS: time.Since(started).Milliseconds(),
	})
}

func isBlockedMutation(err error) bool {
	return errors.Is(err, workspaceaccess.ErrInvalidName) || errors.Is(err, workspaceaccess.ErrDestinationExists) ||
		errors.Is(err, workspaceaccess.ErrRootMutation) || errors.Is(err, workspaceaccess.ErrSymlinkMutation) ||
		errors.Is(err, workspaceaccess.ErrInvalidPath) || errors.Is(err, workspaceaccess.ErrProtectedPath) ||
		errors.Is(err, workspaceaccess.ErrOutsideRoot)
}

func joinInputPath(parent, name string) string {
	parent = strings.Trim(filepath.ToSlash(parent), "/")
	if parent == "" {
		return strings.TrimSpace(name)
	}
	return parent + "/" + strings.TrimSpace(name)
}

var _ Access = (*workspaceaccess.Access)(nil)

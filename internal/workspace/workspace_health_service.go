package workspace

// This package owns health checks and their application workflow.

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	apperrors "plan-manager/internal/common"
	"plan-manager/internal/common/models"
	"plan-manager/internal/item/index"
)

type workspaceReader interface {
	Get(string) (models.WorkspaceConfig, bool, error)
}

type itemReader interface {
	Query(itemindex.Query) ([]models.ItemSummary, error)
}

type gitReader interface {
	WorkspaceRoot(string) (string, error)
	ValidateBranch(string, string) error
	Status(string, string) (models.GitStatus, error)
}

type HealthService struct {
	workspaces workspaceReader
	items      itemReader
	git        gitReader
	now        func() time.Time
}

func NewHealthService(workspaces workspaceReader, items itemReader, git gitReader) *HealthService {
	return &HealthService{workspaces: workspaces, items: items, git: git, now: time.Now}
}

func (s *HealthService) Check(workspaceID string) (models.WorkspaceHealth, error) {
	workspace, ok, err := s.workspaces.Get(workspaceID)
	if err != nil {
		return models.WorkspaceHealth{}, err
	}
	if !ok {
		return models.WorkspaceHealth{}, apperrors.ErrWorkspaceNotFound
	}

	checks := []models.HealthCheck{s.pathCheck(workspace), s.sourcesCheck(workspace), s.gitCheck(workspace), s.branchCheck(workspace), s.permissionsCheck(workspace), s.indexCheck(workspace)}
	return models.WorkspaceHealth{
		WorkspaceID: workspace.ID,
		CheckedAt:   s.now().UTC(),
		Checks:      checks,
		Summary:     summarize(checks),
	}, nil
}

func (s *HealthService) pathCheck(workspace models.WorkspaceConfig) models.HealthCheck {
	info, err := os.Stat(workspace.Path)
	if err != nil || !info.IsDir() {
		return failed("workspace_path", "Workspace path is not available.", "Restore the path or update the workspace registration.")
	}
	return ok("workspace_path", "Workspace path is available.")
}

func (s *HealthService) sourcesCheck(workspace models.WorkspaceConfig) models.HealthCheck {
	if len(workspace.Sources) == 0 {
		return warning("sources", "No sources are configured.", "Add at least one source to scan items.")
	}
	for _, source := range workspace.Sources {
		info, err := os.Stat(filepath.Join(workspace.Path, filepath.FromSlash(source)))
		if err != nil || !info.IsDir() {
			return failed("sources", fmt.Sprintf("Source %q is not available.", source), "Restore the source directory or update workspace sources.")
		}
	}
	return ok("sources", "All configured sources are available.")
}

func (s *HealthService) gitCheck(workspace models.WorkspaceConfig) models.HealthCheck {
	root, err := s.git.WorkspaceRoot(workspace.Path)
	if err != nil || filepath.Clean(root) != filepath.Clean(workspace.Path) {
		return failed("git_root", "Workspace is not a valid Git root.", "Restore the Git repository or update the workspace path.")
	}
	status, err := s.git.Status(workspace.ID, workspace.Path)
	if err != nil {
		return failed("git_status", "Git status is unavailable.", "Run git status in the workspace and resolve the reported error.")
	}
	if status.Conflicted {
		return warning("git_status", "Git has unresolved conflicts.", "Resolve or abort the current Git operation before writing files.")
	}
	return ok("git_status", "Git status is available.")
}

func (s *HealthService) branchCheck(workspace models.WorkspaceConfig) models.HealthCheck {
	if err := s.git.ValidateBranch(workspace.Path, workspace.BaselineBranch); err != nil {
		return warning("baseline_branch", fmt.Sprintf("Baseline branch %q is unavailable.", workspace.BaselineBranch), "Create the branch or update the workspace baseline.")
	}
	return ok("baseline_branch", "Baseline branch is available.")
}

func (s *HealthService) permissionsCheck(workspace models.WorkspaceConfig) models.HealthCheck {
	info, err := os.Stat(workspace.Path)
	if err != nil {
		return failed("permissions", "Workspace permissions cannot be read.", "Restore read and write access to the workspace.")
	}
	if info.Mode().Perm()&0o400 == 0 {
		return failed("permissions", "Workspace is not readable.", "Grant read access to the workspace.")
	}
	if info.Mode().Perm()&0o200 == 0 {
		return warning("permissions", "Workspace is read-only.", "Grant write access before editing or running Git operations.")
	}
	return ok("permissions", "Workspace is readable and writable.")
}

func (s *HealthService) indexCheck(workspace models.WorkspaceConfig) models.HealthCheck {
	items, err := s.items.Query(itemindex.Query{WorkspaceID: workspace.ID})
	if err != nil {
		return failed("item_index", "Item index is unavailable.", "Run a workspace scan to rebuild the item index.")
	}
	if workspace.LastScannedAt.IsZero() {
		return warning("item_index", "Workspace has not been scanned.", "Scan the workspace to build its item index.")
	}
	return ok("item_index", fmt.Sprintf("Item index contains %d items.", len(items)))
}

func summarize(checks []models.HealthCheck) models.HealthStatus {
	summary := models.HealthStatusOK
	for _, check := range checks {
		if check.Status == models.HealthStatusFailed {
			return models.HealthStatusFailed
		}
		if check.Status == models.HealthStatusWarning {
			summary = models.HealthStatusWarning
		}
	}
	return summary
}

func ok(name, message string) models.HealthCheck {
	return models.HealthCheck{Name: name, Status: models.HealthStatusOK, Message: message}
}

func warning(name, message, hint string) models.HealthCheck {
	return models.HealthCheck{Name: name, Status: models.HealthStatusWarning, Message: message, RecoveryHint: hint}
}

func failed(name, message, hint string) models.HealthCheck {
	return models.HealthCheck{Name: name, Status: models.HealthStatusFailed, Message: message, RecoveryHint: hint}
}

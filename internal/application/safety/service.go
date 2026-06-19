package safety

import (
	"fmt"
	"os"

	"plan-manager/internal/models"
	"plan-manager/internal/security/pathguard"
)

type Service struct{}

func New() *Service { return &Service{} }

func (s *Service) Workspace(workspace models.WorkspaceConfig) models.SafetyCheck {
	info, err := os.Stat(workspace.Path)
	if err != nil || !info.IsDir() {
		return blocked("Workspace path is not available.", "Restore the path or update the workspace registration.")
	}
	if info.Mode().Perm()&0o200 == 0 {
		return blocked("Workspace is read-only.", "Grant write access before running this operation.")
	}
	return allowed()
}

func (s *Service) WritePath(workspace models.WorkspaceConfig, path string) models.SafetyCheck {
	if check := s.Workspace(workspace); !check.OK {
		return check
	}
	if _, err := pathguard.ValidateSourcePath(workspace.Sources, path); err != nil {
		return blocked(err.Error(), "Choose a path inside a configured workspace source.")
	}
	return allowed()
}

func (s *Service) Git(status models.GitStatus, confirm bool, operation string) models.SafetyCheck {
	if status.Conflicted {
		return blocked("Git has unresolved conflicts.", "Resolve or abort the current Git operation before continuing.")
	}
	if status.Dirty && !confirm {
		return blocked(fmt.Sprintf("Working tree has local changes; confirm to %s.", operation), "Review local changes, then confirm the operation or commit them first.")
	}
	return allowed()
}

func allowed() models.SafetyCheck { return models.SafetyCheck{OK: true} }

func blocked(message, hint string) models.SafetyCheck {
	return models.SafetyCheck{OK: false, Message: message, RecoveryHint: hint}
}

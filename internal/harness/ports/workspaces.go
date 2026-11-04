// Package ports declares what the harness needs from the outside: workspaces,
// coding CLIs, issue sources, git worktrees, pull requests and notifications.
package ports

import "kode-stream/internal/common/models"

// Workspaces reads the workspace registry. registry.Repository satisfies it.
type Workspaces interface {
	Get(string) (models.WorkspaceConfig, bool, error)
	List() ([]models.WorkspaceConfig, error)
}

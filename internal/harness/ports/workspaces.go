package ports

import "kode-stream/internal/common/models"

type Workspaces interface {
	Get(string) (models.WorkspaceConfig, bool, error)
	List() ([]models.WorkspaceConfig, error)
}

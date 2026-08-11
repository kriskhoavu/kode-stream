package api

import (
	"kode-stream/internal/audit"
	"kode-stream/internal/system"
)

type cloudController struct {
	runtimeConfig system.RuntimeConfig
	agents        *cloudAgentStore
	workspaces    *cloudWorkspaceStore
	providers     *cloudProviderStore
	audit         audit.Repository
	available     bool
}

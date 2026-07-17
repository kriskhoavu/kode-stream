package api

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	"kode-stream/internal/navigation"
	"kode-stream/internal/system"
)

type auditEventReader interface {
	RecentContext(context.Context, int) ([]models.AuditEvent, error)
}

func (a *API) registerGinRoutes(api *gin.RouterGroup) {
	api.GET("/health", a.ginHealth)
	a.registerCloudAuthRoutes(api)
	api.GET("/agents/channel", ginHTTPHandler(a.cloudAgentChannel))
	api.POST("/workspaces/from-agent", ginHTTPHandler(a.registerCloudWorkspaceFromAgent))
	api.Use(a.cloudAuthMiddleware())
	api.GET("/audit-events", a.ginAuditEvents)
	a.registerNavigationRoutes(api)
	a.registerSystemRoutes(api)
	a.registerStorageRoutes(api)
	a.registerStateSearchAIRoutes(api)
	a.registerCloudAgentRoutes(api)
	a.registerWorkspaceReadRoutes(api)
	a.registerItemReadRoutes(api)
	a.registerWorkspaceItemWriteRoutes(api)
	a.registerKnowledgeVerificationRoutes(api)
	a.registerGitRoutes(api)
	a.registerStreamingRoutes(api)
}

func (a *API) registerStorageRoutes(api *gin.RouterGroup) {
	api.GET("/storage/status", ginHTTPHandler(a.storageStatusRoute))
	api.PUT("/storage/option", ginHTTPHandler(a.storageOptionRoute))
	api.POST("/storage/sync", ginHTTPHandler(a.storageSyncRoute))
}

func (a *API) registerCloudAgentRoutes(api *gin.RouterGroup) {
	api.POST("/agents/connect-token", ginHTTPHandler(a.cloudAgentConnectToken))
	api.GET("/agents", ginHTTPHandler(a.cloudAgents))
	api.POST("/workspaces/:id/commands", ginHTTPHandler(a.cloudWorkspaceCommand))
}

func (a *API) registerNavigationRoutes(api *gin.RouterGroup) {
	controller := navigation.NewController(a.navigation, a.items)
	api.GET("/saved-filters", ginHTTPHandler(controller.Filters))
	api.POST("/saved-filters", ginHTTPHandler(controller.SaveFilter))
	api.DELETE("/saved-filters/:id", ginHTTPHandler(controller.DeleteFilter))
	api.GET("/recent-items", ginHTTPHandler(controller.Recents))
	api.POST("/recent-items", ginHTTPHandler(controller.RecordRecent))
}

func (a *API) registerSystemRoutes(api *gin.RouterGroup) {
	controller := system.NewController(a.dialog)
	api.POST("/system/select-directory", ginHTTPHandler(controller.SelectDirectory))
	api.POST("/system/select-file", ginHTTPHandler(controller.SelectFile))
	api.POST("/system/open-path", ginHTTPHandler(controller.OpenPath))
	api.GET("/system/config-paths", ginHTTPHandler(controller.ConfigPaths))
	api.PUT("/system/config-paths", ginHTTPHandler(controller.UpdateConfigPaths))
}

func (a *API) registerStateSearchAIRoutes(api *gin.RouterGroup) {
	api.GET("/state", ginHTTPHandler(a.state))
	api.GET("/search", ginHTTPHandler(a.searchItems))
	api.GET("/ai/capabilities", ginHTTPHandler(a.aiCapabilities))
	api.GET("/ai/presets", ginHTTPHandler(a.aiPresets))
	api.GET("/ai/providers/:id/capabilities", ginHTTPHandler(a.aiProviderCapabilities))
	api.GET("/ai/settings", ginHTTPHandler(a.aiSettings))
	api.PUT("/ai/settings", ginHTTPHandler(a.saveAISettings))
}

func (a *API) registerWorkspaceReadRoutes(api *gin.RouterGroup) {
	api.GET("/workspaces", ginHTTPHandler(a.listWorkspaces))
	api.GET("/workspaces/files/search", ginHTTPHandler(a.workspacePathSearch))
	api.GET("/workspaces/files/content-search", ginHTTPHandler(a.workspaceContentSearch))
	api.GET("/workspaces/:id/runtime", ginHTTPHandler(a.workspaceRuntime))
	api.GET("/workspaces/:id/health", ginHTTPHandler(a.workspaceHealth))
	api.GET("/workspaces/:id/source-structure", ginHTTPHandler(a.getSourceStructure))
	api.GET("/workspaces/:id/tree", ginHTTPHandler(a.workspaceTree))
	api.GET("/workspaces/:id/files", ginHTTPHandler(a.workspaceFile))
	api.GET("/workspaces/:id/files/diff", ginHTTPHandler(a.workspaceFileDiff))
	api.GET("/workspaces/:id/git/path-status", ginHTTPHandler(a.workspacePathGitStates))
}

func (a *API) registerItemReadRoutes(api *gin.RouterGroup) {
	api.GET("/items", ginHTTPHandler(a.listItems))
	api.GET("/items/:id", ginHTTPHandler(a.itemDetail))
	api.GET("/items/:id/ai-session-eligibility", ginHTTPHandler(a.aiSessionEligibility))
	api.GET("/items/:id/jira", ginHTTPHandler(a.jiraIssue))
	api.GET("/items/:id/jira/attachments/:attachmentId", ginHTTPHandler(a.jiraAttachment))
	api.GET("/items/:id/verification-tests", ginHTTPHandler(a.itemVerificationTests))
	api.GET("/items/:id/files", ginHTTPHandler(a.itemFiles))
	api.GET("/items/:id/content-search", ginHTTPHandler(a.itemContentSearch))
	api.GET("/items/:id/files/:fileID", ginHTTPHandler(a.itemFileContent))
	api.GET("/items/:id/diff", ginHTTPHandler(a.itemDiff))
	api.GET("/workspaces/:id/jira/issues/:issueKey", ginHTTPHandler(a.workspaceJiraIssue))
}

func (a *API) registerWorkspaceItemWriteRoutes(api *gin.RouterGroup) {
	api.POST("/workspaces", ginHTTPHandler(a.createWorkspace))
	api.POST("/workspaces/import-preview", ginHTTPHandler(a.previewWorkspaceImport))
	api.POST("/workspaces/import", ginHTTPHandler(a.importWorkspaces))
	api.PUT("/workspaces/:id", ginHTTPHandler(a.updateWorkspace))
	api.DELETE("/workspaces/:id", ginHTTPHandler(a.deleteWorkspace))
	api.POST("/workspaces/:id/scan", ginHTTPHandler(a.scanWorkspace))
	api.POST("/workspaces/:id/jira/test", ginHTTPHandler(a.testJiraConnection))
	api.PUT("/workspaces/:id/runtime", ginHTTPHandler(a.saveWorkspaceRuntime))
	api.POST("/workspaces/:id/workstream/branch", ginHTTPHandler(a.loadWorkstreamBranch))
	api.PUT("/workspaces/:id/source-structure", ginHTTPHandler(a.saveSourceStructure))
	api.DELETE("/workspaces/:id/source-structure", ginHTTPHandler(a.resetSourceStructure))
	api.PUT("/workspaces/:id/files", ginHTTPHandler(a.saveWorkspaceFile))
	api.POST("/workspaces/:id/files", ginHTTPHandler(a.createWorkspaceFile))
	api.POST("/workspaces/:id/directories", ginHTTPHandler(a.createWorkspaceDirectory))
	api.POST("/workspaces/:id/paths/rename", ginHTTPHandler(a.renameWorkspacePath))
	api.POST("/workspaces/:id/files/revert", ginHTTPHandler(a.revertWorkspaceFile))
	api.POST("/items/:id/jira/refresh", ginHTTPHandler(a.refreshJiraIssue))
	api.PUT("/items/:id/verification-tests", ginHTTPHandler(a.saveItemVerificationTests))
	api.POST("/items/:id/files/:fileID", ginHTTPHandler(a.saveItemFile))
	api.POST("/items/:id/files/:fileID/revert", ginHTTPHandler(a.revertItemFile))
	api.PATCH("/items/:id/metadata", ginHTTPHandler(a.saveItemMetadata))
	api.PATCH("/items/:id/status", ginHTTPHandler(a.updateItemStatus))
	api.POST("/items", ginHTTPHandler(a.createItem))
}

func (a *API) registerKnowledgeVerificationRoutes(api *gin.RouterGroup) {
	api.GET("/knowledge/wikis", ginHTTPHandler(a.knowledgeWikis))
	api.GET("/knowledge/wikis/:workspaceID/:root/pages", ginHTTPHandler(a.knowledgePages))
	api.GET("/knowledge/wikis/:workspaceID/:root/pages/:slug", ginHTTPHandler(a.knowledgePage))
	api.GET("/knowledge/wikis/:workspaceID/:root/graph", ginHTTPHandler(a.knowledgeGraph))
	api.POST("/knowledge/wikis/:workspaceID/:root/rescan", ginHTTPHandler(a.knowledgeRescan))
	api.POST("/knowledge/workspaces/:workspaceID/sync", ginHTTPHandler(a.knowledgeSync))
	api.POST("/knowledge/workspaces/:workspaceID/enrich", ginHTTPHandler(a.knowledgeEnrich))
	api.POST("/workspaces/:id/verification-jobs", ginHTTPHandler(a.createVerificationJob))
	api.POST("/workspaces/:id/verification-checkpoints", ginHTTPHandler(a.ingestVerificationCheckpoint))
	api.GET("/workspaces/:id/verification-jobs/:jobId", ginHTTPHandler(a.verificationJob))
	api.GET("/workspaces/:id/verification-jobs/:jobId/artifacts", ginHTTPHandler(a.verificationArtifacts))
	api.POST("/workspaces/:id/verification-jobs/:jobId/rerun", ginHTTPHandler(a.rerunVerificationJob))
}

func (a *API) registerGitRoutes(api *gin.RouterGroup) {
	api.GET("/workspaces/:id/git/status", ginHTTPHandler(a.gitStatus))
	api.GET("/workspaces/:id/git/activity", ginHTTPHandler(a.gitActivity))
	api.GET("/workspaces/:id/git/branches", ginHTTPHandler(a.gitBranches))
	api.POST("/workspaces/:id/git/fetch", ginHTTPHandler(a.gitFetch))
	api.POST("/workspaces/:id/git/pull", ginHTTPHandler(a.gitPull))
	api.POST("/workspaces/:id/git/push", ginHTTPHandler(a.gitPush))
	api.POST("/workspaces/:id/git/commit", ginHTTPHandler(a.gitCommit))
	api.POST("/workspaces/:id/git/branches", ginHTTPHandler(a.gitCreateBranch))
	api.POST("/workspaces/:id/git/switch", ginHTTPHandler(a.gitSwitchBranch))
}

func (a *API) registerStreamingRoutes(api *gin.RouterGroup) {
	api.POST("/workspaces/stream-create", ginHTTPHandler(a.createWorkspaceStream))
	api.POST("/items/:id/ai-sessions", ginHTTPHandler(a.launchAISession))
	api.POST("/items/:id/ai-sessions/embedded", ginHTTPHandler(a.startEmbeddedAISession))
	api.GET("/ai/sessions/:sessionId", ginHTTPHandler(a.embeddedAISession))
	api.DELETE("/ai/sessions/:sessionId", ginHTTPHandler(a.cancelEmbeddedAISession))
	api.GET("/ai/sessions/:sessionId/channel", ginHTTPHandler(a.embeddedAISessionChannel))
}

func (a *API) ginHealth(c *gin.Context) {
	payload, status := a.healthPayload(c.Request.Context())
	ginJSON(c, status, payload)
}

func (a *API) ginAuditEvents(c *gin.Context) {
	if a.auditReader == nil {
		ginJSON(c, 200, []models.AuditEvent{})
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	events, err := a.auditReader.RecentContext(c.Request.Context(), limit*2)
	if err != nil {
		ginAppError(c, apperrors.Infra(err.Error(), err))
		return
	}
	workspaceID := c.Query("workspaceId")
	if workspaceID != "" {
		filtered := make([]models.AuditEvent, 0, limit)
		for _, event := range events {
			if event.WorkspaceID == workspaceID {
				filtered = append(filtered, event)
				if len(filtered) == limit {
					break
				}
			}
		}
		events = filtered
	} else if len(events) > limit {
		events = events[:limit]
	}
	ginJSON(c, 200, events)
}

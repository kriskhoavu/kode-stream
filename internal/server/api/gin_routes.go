package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"kode-stream/internal/common/models"
)

type routeAccess string

const (
	publicRoute    routeAccess = "public"
	protectedRoute routeAccess = "protected"
)

type routeSpec struct {
	method  string
	path    string
	owner   string
	access  routeAccess
	handler gin.HandlerFunc
}

type routePolicy struct {
	capability models.Capability
	csrf       bool
}

func (a *API) registerGinRoutes(api *gin.RouterGroup) {
	routes := routeManifest(a)
	for _, route := range routes {
		if route.access == publicRoute {
			api.Handle(route.method, route.path, route.handler)
		}
	}
	for _, route := range routes {
		if route.access == protectedRoute {
			policy, ok := policyForRoute(route)
			if !ok {
				panic("protected API route has no authorization policy: " + route.method + " " + route.path)
			}
			api.Handle(route.method, route.path, a.cloud.cloudAuthMiddleware(policy), route.handler)
		}
	}
}

// policyForRoute is the capability inventory for protected route families.
// New owners cannot inherit access merely because they use GET: registration
// fails until their policy is declared here.
func policyForRoute(route routeSpec) (routePolicy, bool) {
	policy := routePolicy{capability: models.CapabilityRead, csrf: isMutatingMethod(route.method)}
	switch route.owner {
	case "system", "storage":
		policy.capability = models.CapabilitySystem
	case "ai":
		if isMutatingMethod(route.method) {
			policy.capability = models.CapabilityAI
		}
	case "verification":
		if isMutatingMethod(route.method) {
			policy.capability = models.CapabilityVerification
		}
	case "git":
		policy.capability = models.CapabilityGit
	case "cloud-agent":
		// Authenticated users may mint a connect token and list their own agents.
		policy.capability = models.CapabilityRead
	case "audit", "navigation", "canvas", "state", "search", "cloud-command", "cloud-snapshot", "workspace", "workspace-files", "workspace-search", "workspace-health", "item", "item-search", "jira", "knowledge", "workspace-stream":
		if isMutatingMethod(route.method) {
			policy.capability = models.CapabilityWrite
		}
	default:
		return routePolicy{}, false
	}
	return policy, true
}

// routeManifest is the single route inventory used by registration and tests.
// Ownership and auth grouping therefore cannot drift from the running router.
func routeManifest(a *API) []routeSpec {
	return []routeSpec{
		{http.MethodGet, "/health", "health", publicRoute, ginHTTPHandler(a.health.health)},
		{http.MethodGet, "/auth/login", "cloud-auth", publicRoute, a.cloud.route(a.cloud.cloudLogin)},
		{http.MethodGet, "/auth/callback", "cloud-auth", publicRoute, a.cloud.route(a.cloud.cloudCallback)},
		{http.MethodPost, "/auth/logout", "cloud-auth", publicRoute, a.cloud.route(a.cloud.cloudLogout)},
		{http.MethodGet, "/agents/channel", "cloud-agent", publicRoute, a.cloud.route(a.cloud.cloudAgentChannel)},
		{http.MethodPost, "/workspaces/from-agent", "cloud-workspace", publicRoute, a.cloud.route(a.cloud.registerCloudWorkspaceFromAgent)},

		{http.MethodGet, "/audit-events", "audit", protectedRoute, ginHTTPHandler(a.audit.auditEvents)},
		{http.MethodGet, "/saved-filters", "navigation", protectedRoute, ginHTTPHandler(a.navigation.Filters)},
		{http.MethodPost, "/saved-filters", "navigation", protectedRoute, ginHTTPHandler(a.navigation.SaveFilter)},
		{http.MethodDelete, "/saved-filters/:id", "navigation", protectedRoute, ginHTTPHandler(a.navigation.DeleteFilter)},
		{http.MethodGet, "/recent-items", "navigation", protectedRoute, ginHTTPHandler(a.navigation.Recents)},
		{http.MethodPost, "/recent-items", "navigation", protectedRoute, ginHTTPHandler(a.navigation.RecordRecent)},
		{http.MethodPost, "/system/select-directory", "system", protectedRoute, ginHTTPHandler(a.system.SelectDirectory)},
		{http.MethodPost, "/system/select-file", "system", protectedRoute, ginHTTPHandler(a.system.SelectFile)},
		{http.MethodPost, "/system/open-path", "system", protectedRoute, ginHTTPHandler(a.system.OpenPath)},
		{http.MethodGet, "/system/config-paths", "system", protectedRoute, ginHTTPHandler(a.system.ConfigPaths)},
		{http.MethodPut, "/system/config-paths", "system", protectedRoute, ginHTTPHandler(a.system.UpdateConfigPaths)},
		{http.MethodGet, "/storage/status", "storage", protectedRoute, ginHTTPHandler(a.storage.storageStatusRoute)},
		{http.MethodPut, "/storage/option", "storage", protectedRoute, ginHTTPHandler(a.storage.storageOptionRoute)},
		{http.MethodPost, "/storage/sync", "storage", protectedRoute, ginHTTPHandler(a.storage.storageSyncRoute)},
		{http.MethodPost, "/canvas/default", "canvas", protectedRoute, ginHTTPHandler(a.canvas.resolveDefaultCanvas)},
		{http.MethodGet, "/canvas/layouts/:id", "canvas", protectedRoute, ginHTTPHandler(a.canvas.canvasLayout)},
		{http.MethodPatch, "/canvas/layouts/:id/placements", "canvas", protectedRoute, ginHTTPHandler(a.canvas.patchCanvasPlacements)},
		{http.MethodPatch, "/canvas/layouts/:id/viewport", "canvas", protectedRoute, ginHTTPHandler(a.canvas.patchCanvasViewport)},
		{http.MethodDelete, "/canvas/layouts/:id/placements/:nodeId", "canvas", protectedRoute, ginHTTPHandler(a.canvas.removeCanvasPlacement)},
		{http.MethodGet, "/state", "state", protectedRoute, a.state.route(a.state.state)},
		{http.MethodGet, "/search", "search", protectedRoute, ginHTTPHandler(a.search.searchItems)},
		{http.MethodGet, "/ai/capabilities", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.aiCapabilities)},
		{http.MethodGet, "/ai/presets", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.aiPresets)},
		{http.MethodGet, "/ai/providers/:id/capabilities", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.aiProviderCapabilities)},
		{http.MethodGet, "/ai/settings", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.aiSettings)},
		{http.MethodPut, "/ai/settings", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.saveAISettings)},
		{http.MethodPost, "/agents/connect-token", "cloud-agent", protectedRoute, a.cloud.route(a.cloud.cloudAgentConnectToken)},
		{http.MethodGet, "/agents", "cloud-agent", protectedRoute, a.cloud.route(a.cloud.cloudAgents)},
		{http.MethodPost, "/workspaces/:id/commands", "cloud-command", protectedRoute, a.cloud.route(a.cloud.cloudWorkspaceCommand)},
		{http.MethodGet, "/workspaces/:id/snapshot", "cloud-snapshot", protectedRoute, a.cloud.route(a.cloud.cloudSnapshotInfo)},
		{http.MethodGet, "/workspaces/:id/snapshot/tree", "cloud-snapshot", protectedRoute, a.cloud.route(a.cloud.cloudSnapshotTree)},
		{http.MethodGet, "/workspaces/:id/snapshot/files", "cloud-snapshot", protectedRoute, a.cloud.route(a.cloud.cloudSnapshotFile)},
		{http.MethodGet, "/workspaces", "workspace", protectedRoute, a.workspace.route(a.workspace.listWorkspaces)},
		{http.MethodGet, "/workspaces/files/search", "workspace-files", protectedRoute, a.workspace.filesRoute(a.workspace.workspacePathSearch)},
		{http.MethodGet, "/workspaces/files/content-search", "workspace-search", protectedRoute, a.workspace.searchRoute(a.workspace.workspaceContentSearch)},
		{http.MethodGet, "/workspaces/:id/runtime", "verification", protectedRoute, ginHTTPHandler(a.verification.workspaceRuntime)},
		{http.MethodGet, "/workspaces/:id/health", "workspace-health", protectedRoute, a.workspace.healthRoute(a.workspace.workspaceHealth)},
		{http.MethodGet, "/workspaces/:id/source-structure", "workspace", protectedRoute, a.workspace.route(a.workspace.getSourceStructure)},
		{http.MethodGet, "/workspaces/:id/tree", "workspace-files", protectedRoute, a.workspace.filesRoute(a.workspace.workspaceTree)},
		{http.MethodGet, "/workspaces/:id/files", "workspace-files", protectedRoute, a.workspace.filesRoute(a.workspace.workspaceFile)},
		{http.MethodGet, "/workspaces/:id/files/diff", "workspace-files", protectedRoute, a.workspace.filesRoute(a.workspace.workspaceFileDiff)},
		{http.MethodGet, "/workspaces/:id/git/path-status", "workspace-files", protectedRoute, a.workspace.filesRoute(a.workspace.workspacePathGitStates)},
		{http.MethodGet, "/items", "item", protectedRoute, a.item.route(a.item.listItems)},
		{http.MethodGet, "/items/:id", "item", protectedRoute, a.item.route(a.item.itemDetail)},
		{http.MethodGet, "/items/:id/ai-session-eligibility", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.aiSessionEligibility)},
		{http.MethodGet, "/items/:id/jira", "jira", protectedRoute, ginHTTPHandler(a.jira.jiraIssue)},
		{http.MethodGet, "/items/:id/jira/attachments/:attachmentId", "jira", protectedRoute, ginHTTPHandler(a.jira.jiraAttachment)},
		{http.MethodGet, "/items/:id/verification-tests", "item", protectedRoute, a.item.route(a.item.itemVerificationTests)},
		{http.MethodGet, "/items/:id/e2e-runbooks", "item", protectedRoute, a.item.route(a.item.itemE2ERunbooks)},
		{http.MethodGet, "/items/:id/files", "item", protectedRoute, a.item.route(a.item.itemFiles)},
		{http.MethodGet, "/items/:id/content-search", "item-search", protectedRoute, a.item.searchRoute(a.item.itemContentSearch)},
		{http.MethodGet, "/items/:id/files/:fileID", "item", protectedRoute, a.item.route(a.item.itemFileContent)},
		{http.MethodGet, "/items/:id/diff", "item", protectedRoute, a.item.route(a.item.itemDiff)},
		{http.MethodGet, "/workspaces/:id/jira/issues/:issueKey", "jira", protectedRoute, ginHTTPHandler(a.jira.workspaceJiraIssue)},
		{http.MethodPost, "/workspaces", "workspace", protectedRoute, a.workspace.route(a.workspace.createWorkspace)},
		{http.MethodPost, "/workspaces/import-preview", "workspace", protectedRoute, a.workspace.route(a.workspace.previewWorkspaceImport)},
		{http.MethodPost, "/workspaces/import", "workspace", protectedRoute, a.workspace.route(a.workspace.importWorkspaces)},
		{http.MethodPut, "/workspaces/:id", "workspace", protectedRoute, a.workspace.route(a.workspace.updateWorkspace)},
		{http.MethodDelete, "/workspaces/:id", "workspace", protectedRoute, a.workspace.route(a.workspace.deleteWorkspace)},
		{http.MethodPost, "/workspaces/:id/scan", "workspace", protectedRoute, a.workspace.route(a.workspace.scanWorkspace)},
		{http.MethodPost, "/workspaces/:id/jira/test", "jira", protectedRoute, ginHTTPHandler(a.jira.testJiraConnection)},
		{http.MethodPut, "/workspaces/:id/runtime", "verification", protectedRoute, ginHTTPHandler(a.verification.saveWorkspaceRuntime)},
		{http.MethodPost, "/workspaces/:id/workstream/branch", "workspace", protectedRoute, a.workspace.workstreamRoute(a.workspace.loadWorkstreamBranch)},
		{http.MethodPost, "/workspaces/:id/workstream/checkout", "workspace", protectedRoute, a.workspace.workstreamRoute(a.workspace.loadWorkstreamCheckout)},
		{http.MethodPost, "/workspaces/:id/reviews/branch", "workspace", protectedRoute, a.workspace.workstreamRoute(a.workspace.loadBranchReview)},
		{http.MethodPost, "/workspaces/:id/reviews/import", "item", protectedRoute, a.item.route(a.item.importReviewedPlan)},
		{http.MethodPut, "/workspaces/:id/source-structure", "workspace", protectedRoute, a.workspace.route(a.workspace.saveSourceStructure)},
		{http.MethodDelete, "/workspaces/:id/source-structure", "workspace", protectedRoute, a.workspace.route(a.workspace.resetSourceStructure)},
		{http.MethodPut, "/workspaces/:id/files", "workspace-files", protectedRoute, a.workspace.filesRoute(a.workspace.saveWorkspaceFile)},
		{http.MethodPost, "/workspaces/:id/files", "workspace-files", protectedRoute, a.workspace.filesRoute(a.workspace.createWorkspaceFile)},
		{http.MethodPost, "/workspaces/:id/directories", "workspace-files", protectedRoute, a.workspace.filesRoute(a.workspace.createWorkspaceDirectory)},
		{http.MethodPost, "/workspaces/:id/paths/rename", "workspace-files", protectedRoute, a.workspace.filesRoute(a.workspace.renameWorkspacePath)},
		{http.MethodPost, "/workspaces/:id/files/revert", "workspace-files", protectedRoute, a.workspace.filesRoute(a.workspace.revertWorkspaceFile)},
		{http.MethodPost, "/items/:id/jira/refresh", "jira", protectedRoute, ginHTTPHandler(a.jira.refreshJiraIssue)},
		{http.MethodPut, "/items/:id/verification-tests", "item", protectedRoute, a.item.route(a.item.saveItemVerificationTests)},
		{http.MethodPost, "/items/:id/files/:fileID", "item", protectedRoute, a.item.route(a.item.saveItemFile)},
		{http.MethodPost, "/items/:id/files/:fileID/revert", "item", protectedRoute, a.item.route(a.item.revertItemFile)},
		{http.MethodPatch, "/items/:id/metadata", "item", protectedRoute, a.item.route(a.item.saveItemMetadata)},
		{http.MethodPatch, "/items/:id/status", "item", protectedRoute, a.item.route(a.item.updateItemStatus)},
		{http.MethodPost, "/items", "item", protectedRoute, a.item.route(a.item.createItem)},
		{http.MethodGet, "/knowledge/wikis", "knowledge", protectedRoute, ginHTTPHandler(a.knowledge.knowledgeWikis)},
		{http.MethodGet, "/knowledge/wikis/:workspaceID/:root/pages", "knowledge", protectedRoute, ginHTTPHandler(a.knowledge.knowledgePages)},
		{http.MethodGet, "/knowledge/wikis/:workspaceID/:root/pages/:slug", "knowledge", protectedRoute, ginHTTPHandler(a.knowledge.knowledgePage)},
		{http.MethodGet, "/knowledge/wikis/:workspaceID/:root/pages/:slug/e2e-runbook", "knowledge", protectedRoute, ginHTTPHandler(a.knowledge.knowledgeE2ERunbook)},
		{http.MethodGet, "/knowledge/wikis/:workspaceID/:root/graph", "knowledge", protectedRoute, ginHTTPHandler(a.knowledge.knowledgeGraph)},
		{http.MethodPost, "/knowledge/wikis/:workspaceID/:root/rescan", "knowledge", protectedRoute, ginHTTPHandler(a.knowledge.knowledgeRescan)},
		{http.MethodPost, "/knowledge/workspaces/:workspaceID/sync", "knowledge", protectedRoute, ginHTTPHandler(a.knowledge.knowledgeSync)},
		{http.MethodPost, "/knowledge/workspaces/:workspaceID/enrich", "knowledge", protectedRoute, ginHTTPHandler(a.knowledge.knowledgeEnrich)},
		{http.MethodPost, "/workspaces/:id/verification-jobs", "verification", protectedRoute, ginHTTPHandler(a.verification.createVerificationJob)},
		{http.MethodPost, "/workspaces/:id/verification-checkpoints", "verification", protectedRoute, ginHTTPHandler(a.verification.ingestVerificationCheckpoint)},
		{http.MethodGet, "/workspaces/:id/verification-jobs/:jobId", "verification", protectedRoute, ginHTTPHandler(a.verification.verificationJob)},
		{http.MethodGet, "/workspaces/:id/verification-jobs/:jobId/artifacts", "verification", protectedRoute, ginHTTPHandler(a.verification.verificationArtifacts)},
		{http.MethodPost, "/workspaces/:id/verification-jobs/:jobId/rerun", "verification", protectedRoute, ginHTTPHandler(a.verification.rerunVerificationJob)},
		{http.MethodGet, "/workspaces/:id/git/status", "git", protectedRoute, a.git.route(a.git.gitStatus)},
		{http.MethodGet, "/workspaces/:id/git/activity", "git", protectedRoute, a.git.route(a.git.gitActivity)},
		{http.MethodGet, "/workspaces/:id/git/branches", "git", protectedRoute, a.git.route(a.git.gitBranches)},
		{http.MethodGet, "/workspaces/:id/git/stashes", "git", protectedRoute, a.git.route(a.git.gitStashes)},
		{http.MethodPost, "/workspaces/:id/git/stashes/:ref/apply", "git", protectedRoute, a.git.route(a.git.gitApplyStash)},
		{http.MethodPost, "/workspaces/:id/git/fetch", "git", protectedRoute, a.git.route(a.git.gitFetch)},
		{http.MethodPost, "/workspaces/:id/git/pull", "git", protectedRoute, a.git.route(a.git.gitPull)},
		{http.MethodPost, "/workspaces/:id/git/push", "git", protectedRoute, a.git.route(a.git.gitPush)},
		{http.MethodPost, "/workspaces/:id/git/commit", "git", protectedRoute, a.git.route(a.git.gitCommit)},
		{http.MethodPost, "/workspaces/:id/git/branches", "git", protectedRoute, a.git.route(a.git.gitCreateBranch)},
		{http.MethodPost, "/workspaces/:id/git/switch", "git", protectedRoute, a.git.route(a.git.gitSwitchBranch)},
		{http.MethodPost, "/workspaces/stream-create", "workspace-stream", protectedRoute, a.workspace.route(a.workspace.createWorkspaceStream)},
		{http.MethodGet, "/ai/session-records", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.aiSessionRecords)},
		{http.MethodPost, "/items/:id/ai-sessions", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.launchAISession)},
		{http.MethodPost, "/items/:id/ai-sessions/embedded", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.startEmbeddedAISession)},
		{http.MethodPost, "/workspaces/:id/ai-sessions", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.launchWorkspaceAISession)},
		{http.MethodPost, "/workspaces/:id/ai-sessions/embedded", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.startEmbeddedWorkspaceAISession)},
		{http.MethodGet, "/ai/sessions/:sessionId", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.embeddedAISession)},
		{http.MethodPost, "/ai/sessions/:sessionId/grant", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.embeddedAISessionGrant)},
		{http.MethodDelete, "/ai/sessions/:sessionId", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.cancelEmbeddedAISession)},
		{http.MethodGet, "/ai/sessions/:sessionId/channel", "ai", protectedRoute, ginHTTPHandler(a.aiSessions.embeddedAISessionChannel)},
	}
}

func guardedHTTPHandler(available bool, message string, handler http.HandlerFunc) gin.HandlerFunc {
	if available {
		return ginHTTPHandler(handler)
	}
	return ginHTTPHandler(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusServiceUnavailable, message)
	})
}

func (a *workspaceController) route(handler http.HandlerFunc) gin.HandlerFunc {
	return guardedHTTPHandler(a.workspaces != nil, "workspace service is unavailable", handler)
}

func (a *workspaceController) filesRoute(handler http.HandlerFunc) gin.HandlerFunc {
	return guardedHTTPHandler(a.files != nil, "workspace files are unavailable", handler)
}

func (a *workspaceController) searchRoute(handler http.HandlerFunc) gin.HandlerFunc {
	return guardedHTTPHandler(a.contentSearch != nil, "workspace content search is unavailable", handler)
}

func (a *workspaceController) healthRoute(handler http.HandlerFunc) gin.HandlerFunc {
	return guardedHTTPHandler(a.health != nil, "workspace health is unavailable", handler)
}

func (a *workspaceController) workstreamRoute(handler http.HandlerFunc) gin.HandlerFunc {
	return guardedHTTPHandler(a.workstream != nil, "workstream service is unavailable", handler)
}

func (a *itemController) route(handler http.HandlerFunc) gin.HandlerFunc {
	return guardedHTTPHandler(a.items != nil, "item service is unavailable", handler)
}

func (a *itemController) searchRoute(handler http.HandlerFunc) gin.HandlerFunc {
	return guardedHTTPHandler(a.contentSearch != nil, "item content search is unavailable", handler)
}

func (a *gitController) route(handler http.HandlerFunc) gin.HandlerFunc {
	return guardedHTTPHandler(a.gitOps != nil, "Git service is unavailable", handler)
}

func (a *cloudController) route(handler http.HandlerFunc) gin.HandlerFunc {
	return guardedHTTPHandler(a.available, "Cloud transport is unavailable", handler)
}

func (a *stateController) route(handler http.HandlerFunc) gin.HandlerFunc {
	return guardedHTTPHandler(a.workspaces != nil, "workspace state is unavailable", handler)
}

package workspace

import (
	"strings"

	"kode-stream/internal/common/models"
)

type ActionCapabilityInput struct {
	Axes               models.WorkspaceProviderAxes
	Authorization      map[models.Capability]bool
	ContentAvailable   bool
	ExecutionAvailable bool
	Writable           bool
	CurrentBranch      string
	TargetBranch       string
}

func ProviderAxes(mode models.RuntimeMode, datastore models.AppStateDatastore, workspace models.WorkspaceConfig) models.WorkspaceProviderAxes {
	axes := models.WorkspaceProviderAxes{
		Topology:          models.DeploymentTopologyLocalApplication,
		ContentProvider:   models.WorkspaceContentProviderLocalCheckout,
		ExecutionProvider: models.WorkspaceExecutionProviderLocalProcess,
		Datastore:         datastore,
	}
	if mode == models.RuntimeModeCloud {
		axes.Topology = models.DeploymentTopologyCloudControlPlane
	}
	if workspace.AccessMode == models.WorkspaceAccessModeRemoteSnapshot || workspace.Location == models.WorkspaceLocationCloudRemoteSnapshot {
		axes.ContentProvider = models.WorkspaceContentProviderProviderSnapshot
		axes.ExecutionProvider = models.WorkspaceExecutionProviderNone
		return axes
	}
	if workspace.Location == models.WorkspaceLocationCloudAgent || (mode == models.RuntimeModeCloud && workspace.AccessMode == models.WorkspaceAccessModeAgentBacked) {
		axes.ContentProvider = models.WorkspaceContentProviderAgentCheckout
		axes.ExecutionProvider = models.WorkspaceExecutionProviderCloudAgent
	}
	return axes
}

func ResolveActionCapabilities(input ActionCapabilityInput) map[models.WorkspaceAction]models.ActionCapability {
	result := map[models.WorkspaceAction]models.ActionCapability{}
	result[models.WorkspaceActionLayoutMove] = resolveLayoutMove(input)
	result[models.WorkspaceActionRepositoryRead] = resolveRepositoryRead(input)
	result[models.WorkspaceActionGitStatus] = resolveGitStatus(input)
	result[models.WorkspaceActionTerminalLaunch] = resolveTerminalLaunch(input)
	result[models.WorkspaceActionVerificationRun] = resolveVerificationRun(input)
	return result
}

func resolveLayoutMove(input ActionCapabilityInput) models.ActionCapability {
	action := models.WorkspaceActionLayoutMove
	if !authorized(input.Authorization, models.CapabilityWrite) {
		return denied(action, models.ActionCapabilityForbidden, "layout_write_forbidden", "Your role cannot change this Canvas layout.")
	}
	return available(action)
}

func resolveRepositoryRead(input ActionCapabilityInput) models.ActionCapability {
	action := models.WorkspaceActionRepositoryRead
	if !contentSupported(input.Axes.ContentProvider) {
		return denied(action, models.ActionCapabilityUnsupported, "content_provider_unsupported", "This workspace has no readable content provider.")
	}
	if !input.ContentAvailable {
		return withRecovery(denied(action, models.ActionCapabilityUnavailable, "content_provider_unavailable", "Workspace content is temporarily unavailable."), "refresh", "Refresh workspace")
	}
	if !authorized(input.Authorization, models.CapabilityRead) {
		return denied(action, models.ActionCapabilityForbidden, "repository_read_forbidden", "Your role cannot read this workspace.")
	}
	return available(action)
}

func resolveGitStatus(input ActionCapabilityInput) models.ActionCapability {
	action := models.WorkspaceActionGitStatus
	if input.Axes.ContentProvider == models.WorkspaceContentProviderProviderSnapshot {
		return denied(action, models.ActionCapabilityUnsupported, "git_status_unsupported", "Git working-tree status is not available for this content provider.")
	}
	if !contentSupported(input.Axes.ContentProvider) {
		return denied(action, models.ActionCapabilityUnsupported, "content_provider_unsupported", "This workspace has no Git-capable content provider.")
	}
	if !input.ContentAvailable {
		return withRecovery(denied(action, models.ActionCapabilityUnavailable, "content_provider_unavailable", "Git status is temporarily unavailable."), "refresh", "Refresh workspace")
	}
	if !authorized(input.Authorization, models.CapabilityGit) {
		return denied(action, models.ActionCapabilityForbidden, "git_status_forbidden", "Your role cannot inspect Git status.")
	}
	return available(action)
}

func resolveTerminalLaunch(input ActionCapabilityInput) models.ActionCapability {
	action := models.WorkspaceActionTerminalLaunch
	if input.Axes.ExecutionProvider == models.WorkspaceExecutionProviderNone || input.Axes.ExecutionProvider == "" {
		return withRecovery(denied(action, models.ActionCapabilityUnsupported, "execution_provider_missing", "This workspace has no terminal execution provider."), "choose_executable_workspace", "Choose executable workspace")
	}
	if !executionSupported(input.Axes.ExecutionProvider) {
		return denied(action, models.ActionCapabilityUnsupported, "execution_provider_unsupported", "This terminal execution provider is unsupported.")
	}
	if !input.ExecutionAvailable {
		return withRecovery(denied(action, models.ActionCapabilityUnavailable, "execution_provider_unavailable", "The terminal execution provider is temporarily unavailable."), "reconnect_execution_provider", "Reconnect execution provider")
	}
	if !authorized(input.Authorization, models.CapabilityTerminal) {
		return denied(action, models.ActionCapabilityForbidden, "terminal_launch_forbidden", "Your role cannot launch terminal sessions.")
	}
	if !input.Writable {
		return denied(action, models.ActionCapabilityConflicted, "workspace_read_only", "Terminal launch requires an editable workspace checkout.")
	}
	current, target := strings.TrimSpace(input.CurrentBranch), strings.TrimSpace(input.TargetBranch)
	if current != "" && target != "" && current != target {
		capability := denied(action, models.ActionCapabilityConflicted, "branch_mismatch", "The plan branch does not match the current checkout branch.")
		return withRecovery(capability, "open_branch_controls", "Open branch controls")
	}
	return available(action)
}

func resolveVerificationRun(input ActionCapabilityInput) models.ActionCapability {
	action := models.WorkspaceActionVerificationRun
	if input.Axes.ExecutionProvider == models.WorkspaceExecutionProviderNone || input.Axes.ExecutionProvider == "" {
		return withRecovery(denied(action, models.ActionCapabilityUnsupported, "execution_provider_missing", "This workspace has no verification execution provider."), "choose_executable_workspace", "Choose executable workspace")
	}
	if !executionSupported(input.Axes.ExecutionProvider) {
		return denied(action, models.ActionCapabilityUnsupported, "execution_provider_unsupported", "This verification execution provider is unsupported.")
	}
	if !input.ExecutionAvailable {
		return withRecovery(denied(action, models.ActionCapabilityUnavailable, "execution_provider_unavailable", "The verification execution provider is temporarily unavailable."), "reconnect_execution_provider", "Reconnect execution provider")
	}
	if !authorized(input.Authorization, models.CapabilityVerification) {
		return denied(action, models.ActionCapabilityForbidden, "verification_run_forbidden", "Your role cannot run verification.")
	}
	return available(action)
}

func contentSupported(provider models.WorkspaceContentProvider) bool {
	switch provider {
	case models.WorkspaceContentProviderLocalCheckout, models.WorkspaceContentProviderAgentCheckout, models.WorkspaceContentProviderProviderSnapshot:
		return true
	default:
		return false
	}
}

func executionSupported(provider models.WorkspaceExecutionProvider) bool {
	return provider == models.WorkspaceExecutionProviderLocalProcess || provider == models.WorkspaceExecutionProviderCloudAgent
}

func authorized(policy map[models.Capability]bool, capability models.Capability) bool {
	if policy == nil {
		return true
	}
	return policy[capability]
}

func available(action models.WorkspaceAction) models.ActionCapability {
	return models.ActionCapability{Action: action, State: models.ActionCapabilityAvailable, RecoveryActions: []models.CapabilityRecoveryAction{}}
}

func denied(action models.WorkspaceAction, state models.ActionCapabilityState, reason, message string) models.ActionCapability {
	return models.ActionCapability{Action: action, State: state, ReasonCode: reason, Message: message, RecoveryActions: []models.CapabilityRecoveryAction{}}
}

func withRecovery(capability models.ActionCapability, action, label string) models.ActionCapability {
	capability.RecoveryActions = append(capability.RecoveryActions, models.CapabilityRecoveryAction{Action: action, Label: label})
	return capability
}

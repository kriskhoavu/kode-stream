package api

import (
	"net/http"

	"kode-stream/internal/common/models"
	workspacecap "kode-stream/internal/workspace"
)

type cloudCommandInput struct {
	Type       string            `json:"type"`
	Capability models.Capability `json:"capability"`
	Payload    map[string]string `json:"payload"`
	Log        string            `json:"log"`
}

type workspaceAccessAdapter interface {
	Command(cloudSession, models.WorkspaceConfig, cloudCommandInput) (models.CommandResult, int, string)
}

type agentAccessAdapter struct {
	agents *cloudAgentStore
}

func (a agentAccessAdapter) Command(session cloudSession, workspace models.WorkspaceConfig, input cloudCommandInput) (models.CommandResult, int, string) {
	capability := input.Capability
	if capability == "" {
		capability = capabilityForCommandType(input.Type)
	}
	connected := a.agents.HasConnected(session.User.ID, workspace.AgentID)
	if action, ok := commandAction(capability); ok {
		resolved := workspacecap.ResolveActionCapabilities(workspacecap.ActionCapabilityInput{
			Axes:               workspacecap.ProviderAxes(models.RuntimeModeCloud, models.AppStateDatastorePostgres, workspace),
			Authorization:      roleCapabilities(session.User.Role),
			ContentAvailable:   connected,
			ExecutionAvailable: connected,
			Writable:           true,
		})[action]
		if resolved.State != models.ActionCapabilityAvailable {
			return models.CommandResult{}, actionCapabilityHTTPStatus(resolved.State), resolved.Message
		}
	} else if !connected {
		return models.CommandResult{}, http.StatusServiceUnavailable, "Cloud Agent is offline"
	}
	if !roleCapabilities(session.User.Role)[capability] {
		return models.CommandResult{}, http.StatusForbidden, "role cannot run this command"
	}
	// A connected socket is not yet a durable command queue.  Refuse rather
	// than acknowledge work that cannot be delivered/correlated exactly once.
	// This is intentionally stable until a persisted command lifecycle exists.
	return models.CommandResult{}, http.StatusServiceUnavailable, "Cloud Agent command delivery is unavailable"
}

func commandAction(capability models.Capability) (models.WorkspaceAction, bool) {
	switch capability {
	case models.CapabilityRead:
		return models.WorkspaceActionRepositoryRead, true
	case models.CapabilityGit:
		return models.WorkspaceActionGitStatus, true
	case models.CapabilityTerminal:
		return models.WorkspaceActionTerminalLaunch, true
	case models.CapabilityVerification:
		return models.WorkspaceActionVerificationRun, true
	default:
		return "", false
	}
}

func actionCapabilityHTTPStatus(state models.ActionCapabilityState) int {
	switch state {
	case models.ActionCapabilityUnavailable:
		return http.StatusServiceUnavailable
	case models.ActionCapabilityForbidden:
		return http.StatusForbidden
	case models.ActionCapabilityConflicted:
		return http.StatusConflict
	default:
		return http.StatusUnprocessableEntity
	}
}

func (a *cloudController) workspaceAccessAdapter(workspace models.WorkspaceConfig) (workspaceAccessAdapter, int, string) {
	switch workspace.AccessMode {
	case "", models.WorkspaceAccessModeAgentBacked:
		return agentAccessAdapter{agents: a.agents}, http.StatusOK, ""
	case models.WorkspaceAccessModeRemoteSnapshot:
		return remoteSnapshotAdapter{providers: a.providers}, http.StatusOK, ""
	default:
		return nil, http.StatusBadRequest, "unsupported workspace access mode"
	}
}

package api

import (
	"net/http"
	"strings"

	"kode-stream/internal/common/models"
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
	if !a.agents.HasConnected(session.User.ID, workspace.AgentID) {
		return models.CommandResult{}, http.StatusServiceUnavailable, "Cloud Agent is offline"
	}
	capability := input.Capability
	if capability == "" {
		capability = capabilityForCommandType(input.Type)
	}
	if !roleCapabilities(session.User.Role)[capability] {
		return models.CommandResult{}, http.StatusForbidden, "role cannot run this command"
	}
	command := models.CommandEnvelope{
		ID:          stableCloudUserID(session.User.ID + ":" + workspace.ID + ":" + input.Type),
		Type:        strings.TrimSpace(input.Type),
		WorkspaceID: workspace.ID,
		UserID:      session.User.ID,
		AgentID:     workspace.AgentID,
		Capability:  capability,
		Payload:     input.Payload,
	}
	return models.CommandResult{Accepted: true, Command: command, Log: redactCommandLog(input.Log)}, http.StatusAccepted, ""
}

func (a *API) workspaceAccessAdapter(workspace models.WorkspaceConfig) (workspaceAccessAdapter, int, string) {
	switch workspace.AccessMode {
	case "", models.WorkspaceAccessModeAgentBacked:
		return agentAccessAdapter{agents: a.agentStore}, http.StatusOK, ""
	case models.WorkspaceAccessModeRemoteSnapshot:
		return remoteSnapshotAdapter{providers: a.cloudProviders}, http.StatusOK, ""
	default:
		return nil, http.StatusBadRequest, "unsupported workspace access mode"
	}
}

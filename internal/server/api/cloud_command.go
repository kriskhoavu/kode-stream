package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"kode-stream/internal/common/models"
)

var secretPattern = regexp.MustCompile(`(?i)(token|secret|password|key)=(\S+)`)

func (a *API) cloudWorkspaceCommand(w http.ResponseWriter, r *http.Request) {
	session, ok := cloudSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Cloud session is required")
		return
	}
	workspace, ok := a.cloudWorkspaces.Get(session.User.ID, r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	if !a.agentStore.HasConnected(session.User.ID, workspace.AgentID) {
		writeError(w, http.StatusServiceUnavailable, "Cloud Agent is offline")
		return
	}
	var input struct {
		Type       string            `json:"type"`
		Capability models.Capability `json:"capability"`
		Payload    map[string]string `json:"payload"`
		Log        string            `json:"log"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	capability := input.Capability
	if capability == "" {
		capability = capabilityForCommandType(input.Type)
	}
	if !roleCapabilities(session.User.Role)[capability] {
		writeError(w, http.StatusForbidden, "role cannot run this command")
		return
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
	writeJSON(w, http.StatusAccepted, models.CommandResult{Accepted: true, Command: command, Log: redactCommandLog(input.Log)})
}

func capabilityForCommandType(commandType string) models.Capability {
	switch strings.TrimSpace(commandType) {
	case "file":
		return models.CapabilityWrite
	case "git":
		return models.CapabilityGit
	case "terminal":
		return models.CapabilityTerminal
	case "ai":
		return models.CapabilityAI
	case "runtime":
		return models.CapabilityRuntime
	case "verification":
		return models.CapabilityVerification
	default:
		return models.CapabilityRead
	}
}

func redactCommandLog(log string) string {
	return secretPattern.ReplaceAllString(log, "$1=[REDACTED]")
}

func cloudHostedExecutionRoute(method, path string) bool {
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
		return false
	}
	if strings.HasSuffix(path, "/commands") || strings.Contains(path, "/auth/") || strings.Contains(path, "/agents/") || strings.HasSuffix(path, "/workspaces/from-agent") {
		return false
	}
	return strings.Contains(path, "/git/") ||
		strings.Contains(path, "/files") ||
		strings.Contains(path, "/ai-sessions") ||
		strings.Contains(path, "/verification-") ||
		strings.Contains(path, "/runtime") ||
		strings.HasSuffix(path, "/scan") ||
		strings.Contains(path, "/system/")
}

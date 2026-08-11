package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"kode-stream/internal/common/models"
)

var secretPattern = regexp.MustCompile(`(?i)(token|secret|password|key)=(\S+)`)

func (a *cloudController) cloudWorkspaceCommand(w http.ResponseWriter, r *http.Request) {
	session, ok := cloudSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Cloud session is required")
		return
	}
	workspace, ok, err := a.workspaces.Get(r.Context(), session.User.ID, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Cloud workspace persistence is unavailable")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	var input cloudCommandInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		a.recordCloudCommand(session, r.PathValue("id"), "unknown", models.AuditStatusBlocked, "invalid command request")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	adapter, status, message := a.workspaceAccessAdapter(workspace)
	if message != "" {
		a.recordCloudCommand(session, workspace.ID, input.Type, models.AuditStatusBlocked, "Cloud command was blocked")
		writeError(w, status, message)
		return
	}
	result, status, message := adapter.Command(session, workspace, input)
	if message != "" {
		if status == 0 {
			status = http.StatusConflict
		}
		a.recordCloudCommand(session, workspace.ID, input.Type, models.AuditStatusFailed, "Cloud command failed")
		writeError(w, status, message)
		return
	}
	a.recordCloudCommand(session, workspace.ID, input.Type, models.AuditStatusSuccess, "Cloud command accepted")
	writeJSON(w, status, result)
}

func (a *cloudController) recordCloudCommand(session cloudSession, workspaceID, commandType string, status models.AuditStatus, message string) {
	if a.audit == nil {
		return
	}
	_, _ = a.audit.Append(models.AuditEvent{OwnerUserID: session.User.ID, ActorUserID: session.User.ID, WorkspaceID: workspaceID, Operation: "cloud_command_" + strings.TrimSpace(commandType), Status: status, Message: message, Time: time.Now().UTC(), Paths: []string{}})
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

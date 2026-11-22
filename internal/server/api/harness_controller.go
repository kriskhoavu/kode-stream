package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"kode-stream/internal/harness"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/workspace/registry"
)

const maxHarnessBodyBytes int64 = 256 << 10

// harnessController serves the dashboard's harness routes. It translates HTTP
// to calls on the harness service and errors back to HTTP, nothing else.
type harnessController struct {
	harness *harness.Service
	// ai starts a person's session at a run's Review stage, and holds the
	// workspace's default provider a settings file exports and imports.
	ai *aiController
	// workspaces finds the checkout a workspace's settings file lives in.
	workspaces registry.Repository
}

func (a *harnessController) route(handler http.HandlerFunc) gin.HandlerFunc {
	return guardedHTTPHandler(a.harness != nil, "the harness is unavailable", handler)
}

func writeHarnessError(w http.ResponseWriter, err error) {
	var illegal domain.IllegalTransitionError
	switch {
	case errors.As(err, &illegal), harness.IsConflict(err):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error(), "code": "conflict", "recoveryHint": "Reload to see the latest state, then try again."})
	case harness.IsAlreadyDecided(err):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error(), "code": "already_decided"})
	case harness.IsInvalid(err):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error(), "code": "invalid_request"})
	case harness.IsNotFound(err):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error(), "code": "not_found"})
	case harness.IsUnsupported(err):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error(), "code": "unavailable", "recoveryHint": harness.StorageHint})
	case harness.IsUnavailable(err):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error(), "code": "unavailable", "recoveryHint": harness.CloudHint})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error(), "code": "internal"})
	}
}

func writeHarnessInvalid(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": message, "code": "invalid_request"})
}

// harnessActor names the person behind a dashboard request.
func harnessActor(r *http.Request) domain.Actor {
	if session, ok := cloudSessionFromContext(r.Context()); ok {
		name := session.User.Name
		if name == "" {
			name = session.User.Email
		}
		return domain.Actor{ActorID: session.User.ID, DisplayName: name, Email: session.User.Email}
	}
	return domain.Actor{ActorID: "local", DisplayName: "Local user"}
}

func queryLimit(r *http.Request, fallback, max int) int {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func queryStates[S ~string](r *http.Request, name string) []S {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil
	}
	var states []S
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			states = append(states, S(part))
		}
	}
	return states
}

func (a *harnessController) status(w http.ResponseWriter, r *http.Request) {
	status, err := a.harness.DashboardStatus(r.Context())
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *harnessController) settings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.harness.DashboardSettings(r.Context())
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *harnessController) saveSettings(w http.ResponseWriter, r *http.Request) {
	var request harness.DashboardSettings
	if !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, false) {
		return
	}
	saved, err := a.harness.SaveDashboardSettings(r.Context(), request, harnessActor(r))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (a *harnessController) tasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := a.harness.Tasks(r.Context(), domain.TaskFilter{WorkspaceID: r.URL.Query().Get("workspaceId"), States: queryStates[domain.TaskState](r, "state"), Limit: queryLimit(r, 500, 1000)})
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": harness.DashboardTasks(tasks)})
}

func (a *harnessController) task(w http.ResponseWriter, r *http.Request) {
	task, err := a.harness.Task(r.Context(), r.PathValue("id"))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, harness.DashboardTaskWithBody(task))
}

func (a *harnessController) queue(w http.ResponseWriter, r *http.Request) {
	tasks, err := a.harness.Queue(r.Context(), r.URL.Query().Get("workspaceId"), queryLimit(r, 500, 1000))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": harness.DashboardTasks(tasks)})
}

func (a *harnessController) runs(w http.ResponseWriter, r *http.Request) {
	runs, err := a.harness.DashboardRuns(r.Context(), domain.RunFilter{TaskID: r.URL.Query().Get("taskId"), WorkspaceID: r.URL.Query().Get("workspaceId"), States: queryStates[domain.RunState](r, "state"), Limit: queryLimit(r, 100, 500)})
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (a *harnessController) run(w http.ResponseWriter, r *http.Request) {
	run, err := a.harness.DashboardRunDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (a *harnessController) agentRun(w http.ResponseWriter, r *http.Request) {
	agentRun, err := a.harness.DashboardAgentRun(r.Context(), r.PathValue("id"))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, agentRun)
}

func (a *harnessController) agentRunTranscript(w http.ResponseWriter, r *http.Request) {
	transcript, err := a.harness.AgentRunTranscript(r.Context(), r.PathValue("id"), queryLimit(r, 50, 200))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, transcript)
}

func (a *harnessController) approvals(w http.ResponseWriter, r *http.Request) {
	states := queryStates[domain.ApprovalState](r, "state")
	if states == nil {
		states = []domain.ApprovalState{domain.ApprovalRequested}
	}
	if len(states) == 1 && states[0] == "all" {
		states = nil
	}
	approvals, err := a.harness.DashboardApprovals(r.Context(), domain.ApprovalFilter{RunID: r.URL.Query().Get("runId"), TaskID: r.URL.Query().Get("taskId"), States: states, Limit: queryLimit(r, 100, 500)})
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"approvals": approvals})
}

func (a *harnessController) pauseRole(w http.ResponseWriter, r *http.Request) {
	a.setRolePaused(w, r, true)
}

func (a *harnessController) resumeRole(w http.ResponseWriter, r *http.Request) {
	a.setRolePaused(w, r, false)
}

func (a *harnessController) setRolePaused(w http.ResponseWriter, r *http.Request, paused bool) {
	health, err := a.harness.SetRolePaused(r.Context(), harness.RoleKeyFromDashboard(r.PathValue("role")), paused, harnessActor(r))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, harness.DashboardRoleHealth(health))
}

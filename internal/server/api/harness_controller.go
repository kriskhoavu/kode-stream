package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"kode-stream/internal/harness"
	"kode-stream/internal/harness/domain"
)

const maxHarnessBodyBytes int64 = 256 << 10

// harnessController serves the dashboard's harness routes. It translates HTTP
// to calls on the harness service and errors back to HTTP, nothing else.
type harnessController struct {
	harness *harness.Service
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

type harnessPinRequest struct {
	Pinned           bool  `json:"pinned"`
	ExpectedRevision int64 `json:"expectedRevision"`
}

func (a *harnessController) pinTask(w http.ResponseWriter, r *http.Request) {
	var request harnessPinRequest
	if !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return
	}
	task, err := a.harness.PinTask(r.Context(), r.PathValue("id"), request.Pinned, request.ExpectedRevision, harnessActor(r))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, harness.DashboardTasks([]domain.Task{task})[0])
}

type harnessPriorityRequest struct {
	PriorityOverride *float64 `json:"priorityOverride"`
	Before           string   `json:"before"`
	ExpectedRevision int64    `json:"expectedRevision"`
}

func (a *harnessController) setPriority(w http.ResponseWriter, r *http.Request) {
	var request harnessPriorityRequest
	if !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return
	}
	var task domain.Task
	var err error
	switch {
	case request.Before != "":
		task, err = a.harness.MoveBefore(r.Context(), r.PathValue("id"), request.Before, request.ExpectedRevision, harnessActor(r))
	case request.PriorityOverride != nil:
		task, err = a.harness.SetPriority(r.Context(), r.PathValue("id"), *request.PriorityOverride, request.ExpectedRevision, harnessActor(r))
	default:
		writeHarnessInvalid(w, "priorityOverride or before is required")
		return
	}
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, harness.DashboardTasks([]domain.Task{task})[0])
}

type harnessDismissRequest struct {
	Reason           string `json:"reason"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

func (a *harnessController) dismissTask(w http.ResponseWriter, r *http.Request) {
	var request harnessDismissRequest
	if !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return
	}
	task, err := a.harness.DismissTask(r.Context(), r.PathValue("id"), request.Reason, request.ExpectedRevision, harnessActor(r))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, harness.DashboardTasks([]domain.Task{task})[0])
}

type harnessStartRequest struct {
	ExpectedRevision int64  `json:"expectedRevision"`
	IdempotencyKey   string `json:"idempotencyKey"`
}

func (a *harnessController) startTask(w http.ResponseWriter, r *http.Request) {
	var request harnessStartRequest
	if !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return
	}
	task, err := a.harness.StartTask(r.Context(), r.PathValue("id"), request.ExpectedRevision, request.IdempotencyKey, harnessActor(r), domain.ChannelDashboard)
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, harness.DashboardTasks([]domain.Task{task})[0])
}

type harnessCreateTaskRequest struct {
	WorkspaceID    string `json:"workspaceId"`
	Key            string `json:"key"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	URL            string `json:"url"`
	StartNow       *bool  `json:"startNow"`
	IdempotencyKey string `json:"idempotencyKey"`
}

// createTask puts a card's ticket in the queue, started at once unless the
// request says to queue it.
func (a *harnessController) createTask(w http.ResponseWriter, r *http.Request) {
	var request harnessCreateTaskRequest
	if !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return
	}
	if strings.TrimSpace(request.Key) == "" {
		writeHarnessInvalid(w, "a Jira key is required")
		return
	}
	startNow := request.StartNow == nil || *request.StartNow
	task, err := a.harness.CreateTask(r.Context(), harness.CreateTaskInput{WorkspaceID: request.WorkspaceID, Key: request.Key, Title: request.Title, Body: request.Body, URL: request.URL, StartNow: startNow, IdempotencyKey: request.IdempotencyKey}, harnessActor(r), domain.ChannelDashboard)
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, harness.DashboardTasks([]domain.Task{task})[0])
}

type harnessQuestionsRequest struct {
	AskQuestions     *bool `json:"askQuestions"`
	ExpectedRevision int64 `json:"expectedRevision"`
}

func (a *harnessController) setTaskQuestions(w http.ResponseWriter, r *http.Request) {
	var request harnessQuestionsRequest
	if !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return
	}
	task, err := a.harness.SetTaskQuestions(r.Context(), r.PathValue("id"), request.AskQuestions, request.ExpectedRevision, harnessActor(r))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, harness.DashboardTasks([]domain.Task{task})[0])
}

type harnessClearRequest struct {
	WorkspaceID string `json:"workspaceId"`
}

func (a *harnessController) clearQueue(w http.ResponseWriter, r *http.Request) {
	var request harnessClearRequest
	if r.ContentLength != 0 && !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return
	}
	removed, err := a.harness.ClearQueue(r.Context(), request.WorkspaceID, harnessActor(r))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

type harnessPollRequest struct {
	WorkspaceID string `json:"workspaceId"`
}

func (a *harnessController) pollScouts(w http.ResponseWriter, r *http.Request) {
	var request harnessPollRequest
	if r.ContentLength != 0 && !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return
	}
	sources, err := a.harness.PollNow(r.Context(), request.WorkspaceID)
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"started": true, "sources": sources})
}

type harnessDecideRequest struct {
	Decision         string `json:"decision"`
	Reason           string `json:"reason"`
	IdempotencyKey   string `json:"idempotencyKey"`
	ExpectedRevision *int64 `json:"expectedRevision"`
}

func writeHarnessDecision(w http.ResponseWriter, approval domain.Approval, err error) {
	var already harness.AlreadyDecidedError
	if errors.As(err, &already) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "already_decided", "approval": already.Approval})
		return
	}
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, approval)
}

func (a *harnessController) decide(w http.ResponseWriter, r *http.Request) {
	var request harnessDecideRequest
	if !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return
	}
	approval, err := a.harness.Decide(r.Context(), r.PathValue("id"), harness.DecideInput{Decision: harness.Decision(request.Decision), Reason: request.Reason, IdempotencyKey: request.IdempotencyKey, ExpectedRevision: request.ExpectedRevision, RequestID: r.Header.Get(requestIDHeader), Actor: harnessActor(r), Channel: domain.ChannelDashboard})
	var already harness.AlreadyDecidedError
	switch {
	case errors.As(err, &already):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "already_decided", "approval": a.harness.DashboardDecision(r.Context(), already.Approval)})
	case err != nil:
		writeHarnessError(w, err)
	default:
		writeJSON(w, http.StatusOK, a.harness.DashboardDecision(r.Context(), approval))
	}
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

func (a *harnessController) cost(w http.ResponseWriter, r *http.Request) {
	parse := func(name string) (time.Time, bool) {
		raw := strings.TrimSpace(r.URL.Query().Get(name))
		if raw == "" {
			return time.Time{}, true
		}
		if value, err := time.Parse(time.RFC3339, raw); err == nil {
			return value.UTC(), true
		}
		if value, err := time.Parse("2006-01-02", raw); err == nil {
			return value.UTC(), true
		}
		return time.Time{}, false
	}
	from, okFrom := parse("from")
	to, okTo := parse("to")
	if !okFrom || !okTo {
		writeHarnessInvalid(w, "from and to must be RFC 3339 timestamps or dates")
		return
	}
	summary, err := a.harness.CostSummary(r.Context(), domain.CostGroup(r.URL.Query().Get("groupBy")), from, to)
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, harness.DashboardCostOf(summary))
}

func (a *harnessController) runPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := a.harness.Plan(r.Context(), r.PathValue("id"))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (a *harnessController) runChanges(w http.ResponseWriter, r *http.Request) {
	changes, err := a.harness.DashboardChanges(r.Context(), r.PathValue("id"))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, changes)
}

type harnessCancelRequest struct {
	Reason           string `json:"reason"`
	ExpectedRevision *int64 `json:"expectedRevision"`
}

func (a *harnessController) cancelRun(w http.ResponseWriter, r *http.Request) {
	var request harnessCancelRequest
	if r.ContentLength != 0 && !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return
	}
	run, err := a.harness.CancelRun(r.Context(), r.PathValue("id"), request.Reason, request.ExpectedRevision, harnessActor(r))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	runs, err := a.harness.DashboardRuns(r.Context(), domain.RunFilter{TaskID: run.TaskID, Limit: 50})
	for _, view := range runs {
		if err == nil && view.ID == run.ID {
			writeJSON(w, http.StatusOK, view)
			return
		}
	}
	writeJSON(w, http.StatusOK, run)
}

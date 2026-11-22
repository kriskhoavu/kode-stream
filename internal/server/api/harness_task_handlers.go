package api

import (
	"net/http"
	"strings"

	"kode-stream/internal/harness"
	"kode-stream/internal/harness/domain"
)

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

type harnessRemoveRequest struct {
	ExpectedRevision int64 `json:"expectedRevision"`
}

func (a *harnessController) removeTask(w http.ResponseWriter, r *http.Request) {
	var request harnessRemoveRequest
	if !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return
	}
	task, err := a.harness.RemoveTask(r.Context(), r.PathValue("id"), request.ExpectedRevision, harnessActor(r))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, harness.DashboardTasks([]domain.Task{task})[0])
}

func (a *harnessController) resetTask(w http.ResponseWriter, r *http.Request) {
	var request harnessRemoveRequest
	if !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return
	}
	task, err := a.harness.ResetTask(r.Context(), r.PathValue("id"), request.ExpectedRevision, harnessActor(r))
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

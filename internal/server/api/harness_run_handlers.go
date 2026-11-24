package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	appaisession "kode-stream/internal/ai"
	"kode-stream/internal/harness"
	"kode-stream/internal/harness/domain"
)

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

type harnessReviewSessionRequest struct {
	Provider       string `json:"provider"`
	IdempotencyKey string `json:"idempotencyKey"`
	Columns        uint16 `json:"columns"`
	Rows           uint16 `json:"rows"`
}

func (a *harnessController) reviewSession(w http.ResponseWriter, r *http.Request) {
	var request harnessReviewSessionRequest
	if !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return
	}
	if request.Columns < 20 || request.Columns > maxTerminalDimension || request.Rows < 5 || request.Rows > 200 || len(request.Provider) > 64 || len(request.IdempotencyKey) > 200 {
		writeError(w, http.StatusBadRequest, "review session input exceeds the allowed limits")
		return
	}
	if a.ai == nil || a.ai.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI sessions are unavailable")
		return
	}
	target, err := a.harness.ReviewSession(r.Context(), r.PathValue("id"))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	if !a.ai.authorizeWorkspace(w, r, target.WorkspaceID) {
		return
	}
	result, err := a.ai.aiSessions.StartEmbeddedReview(appaisession.ReviewInput{WorkspaceID: target.WorkspaceID, Dir: target.Dir, Branch: target.Branch, Prompt: target.Prompt, Provider: request.Provider, IdempotencyKey: request.IdempotencyKey, Columns: request.Columns, Rows: request.Rows})
	if err != nil {
		writeEmbeddedLaunchError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
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

func (a *harnessController) runFileDiff(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}
	diff, err := a.harness.FileDiff(r.Context(), r.PathValue("id"), path)
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, diff)
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

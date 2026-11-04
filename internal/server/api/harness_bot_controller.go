package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"kode-stream/internal/harness"
	"kode-stream/internal/harness/domain"
)

// harnessBotController serves chat-bot. Its routes carry no Cloud
// session: each request authenticates with the bot's bearer token, compared
// in constant time, and every decision is recorded with channel webex.
type harnessBotController struct {
	harness *harness.Service
}

func (a *harnessBotController) route(handler http.HandlerFunc) gin.HandlerFunc {
	return guardedHTTPHandler(a.harness != nil, "the harness is unavailable", func(w http.ResponseWriter, r *http.Request) {
		token := a.harness.Config().BotToken
		if token == "" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "the harness bot token is not configured", "code": "unavailable", "recoveryHint": "Set " + harness.EnvBotToken + " and restart Kode Stream."})
			return
		}
		presented, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(presented)), []byte(token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "a valid bot token is required", "code": "unauthorized"})
			return
		}
		if !a.harness.Supported() {
			writeHarnessError(w, domain.ErrUnsupported)
			return
		}
		handler(w, r)
	})
}

func (a *harnessBotController) status(w http.ResponseWriter, r *http.Request) {
	status, err := a.harness.Status(r.Context())
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *harnessBotController) queue(w http.ResponseWriter, r *http.Request) {
	tasks, err := a.harness.Queue(r.Context(), "", queryLimit(r, 10, 50))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (a *harnessBotController) approvals(w http.ResponseWriter, r *http.Request) {
	approvals, err := a.harness.Approvals(r.Context(), domain.ApprovalFilter{States: []domain.ApprovalState{domain.ApprovalRequested}, Limit: 50})
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"approvals": approvals})
}

func (a *harnessBotController) run(w http.ResponseWriter, r *http.Request) {
	run, err := a.harness.Run(r.Context(), r.PathValue("id"))
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

type harnessBotActor struct {
	PersonID    string `json:"personId"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

type harnessBotDecision struct {
	Decision         string          `json:"decision"`
	Reason           string          `json:"reason"`
	RequestID        string          `json:"requestId"`
	IdempotencyKey   string          `json:"idempotencyKey"`
	ExpectedRevision *int64          `json:"expectedRevision"`
	Actor            harnessBotActor `json:"actor"`
}

func (a *harnessBotController) decodeDecision(w http.ResponseWriter, r *http.Request) (harness.DecideInput, bool) {
	var request harnessBotDecision
	if !decodeLimitedJSON(w, r, &request, maxHarnessBodyBytes, true) {
		return harness.DecideInput{}, false
	}
	if strings.TrimSpace(request.Actor.PersonID) == "" {
		writeHarnessInvalid(w, "actor.personId is required")
		return harness.DecideInput{}, false
	}
	name := strings.TrimSpace(request.Actor.DisplayName)
	if name == "" {
		name = request.Actor.Email
	}
	actor := domain.Actor{ActorID: "webex:" + strings.TrimSpace(request.Actor.PersonID), DisplayName: boundedHeader(name), Email: boundedHeader(request.Actor.Email)}
	return harness.DecideInput{Decision: harness.Decision(request.Decision), Reason: request.Reason, IdempotencyKey: request.IdempotencyKey, ExpectedRevision: request.ExpectedRevision, RequestID: request.RequestID, Actor: actor, Channel: domain.ChannelWebex}, true
}

func boundedHeader(value string) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 200 {
		return string([]rune(value)[:200])
	}
	return value
}

func (a *harnessBotController) decideApproval(w http.ResponseWriter, r *http.Request) {
	input, ok := a.decodeDecision(w, r)
	if !ok {
		return
	}
	approval, err := a.harness.Decide(r.Context(), r.PathValue("id"), input)
	writeHarnessDecision(w, approval, err)
}

func (a *harnessBotController) decideRun(w http.ResponseWriter, r *http.Request) {
	input, ok := a.decodeDecision(w, r)
	if !ok {
		return
	}
	approval, err := a.harness.DecideRun(r.Context(), r.PathValue("id"), input)
	writeHarnessDecision(w, approval, err)
}

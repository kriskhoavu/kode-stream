package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"kode-stream/internal/harness"
	"kode-stream/internal/storage"
)

func serveBot(handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	handler.ServeHTTP(recorder, request)
	return recorder
}

func newBotHandler(t *testing.T, token string) (http.Handler, *harness.Service) {
	t.Helper()
	repository, closeStore, err := storage.OpenSQLiteHarness(filepath.Join(t.TempDir(), "harness.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closeStore() })
	seedWaitingPlanApproval(t, repository)
	service := harness.New(harness.Dependencies{Repository: repository, Workspaces: harnessTestWorkspaces{"ws": {ID: "ws"}}, Config: harness.Config{LocalMode: true, BotToken: token}})
	return New(Dependencies{Harness: service}).Routes(), service
}

const botDecision = `{"decision":"approve","reason":"ok","requestId":"msg-1","idempotencyKey":"bot-1","actor":{"personId":"p-1","email":"anna@example.com","displayName":"Anna"}}`

func TestHarnessBotRoutesAuthenticateWithTheBotToken(t *testing.T) {
	unconfigured, _ := newBotHandler(t, "")
	if response := serveBot(unconfigured, http.MethodGet, "/api/harness/bot/status", "anything", ""); response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), harness.EnvBotToken) {
		t.Fatalf("no token = %d %s", response.Code, response.Body.String())
	}
	handler, _ := newBotHandler(t, "s3cret")
	for _, token := range []string{"", "wrong", "s3cret-not"} {
		if response := serveBot(handler, http.MethodGet, "/api/harness/bot/status", token, ""); response.Code != http.StatusUnauthorized {
			t.Fatalf("token %q = %d %s", token, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{"/api/harness/bot/status", "/api/harness/bot/queue?limit=5", "/api/harness/bot/approvals", "/api/harness/bot/runs/run-9"} {
		if response := serveBot(handler, http.MethodGet, path, "s3cret", ""); response.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", path, response.Code, response.Body.String())
		}
	}
	unsupported := New(Dependencies{Harness: harness.New(harness.Dependencies{Config: harness.Config{LocalMode: true, BotToken: "s3cret"}})}).Routes()
	if response := serveBot(unsupported, http.MethodGet, "/api/harness/bot/status", "s3cret", ""); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unsupported = %d %s", response.Code, response.Body.String())
	}
}

func TestHarnessBotDecisionsReplayConflictAndRecordWebex(t *testing.T) {
	handler, _ := newBotHandler(t, "s3cret")
	stale := serveBot(handler, http.MethodPost, "/api/harness/bot/approvals/apr-9/decide", "s3cret", strings.Replace(botDecision, `"idempotencyKey":"bot-1"`, `"idempotencyKey":"bot-0","expectedRevision":4`, 1))
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), `"code":"conflict"`) {
		t.Fatalf("stale = %d %s", stale.Code, stale.Body.String())
	}
	noActor := serveBot(handler, http.MethodPost, "/api/harness/bot/runs/run-9/decision", "s3cret", `{"decision":"approve","idempotencyKey":"bot-x"}`)
	if noActor.Code != http.StatusBadRequest {
		t.Fatalf("no actor = %d %s", noActor.Code, noActor.Body.String())
	}
	first := serveBot(handler, http.MethodPost, "/api/harness/bot/runs/run-9/decision", "s3cret", botDecision)
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"channel":"webex"`) || !strings.Contains(first.Body.String(), `"actorId":"webex:p-1"`) || !strings.Contains(first.Body.String(), `"displayName":"Anna"`) {
		t.Fatalf("first = %d %s", first.Code, first.Body.String())
	}
	replay := serveBot(handler, http.MethodPost, "/api/harness/bot/runs/run-9/decision", "s3cret", botDecision)
	if replay.Code != http.StatusOK || replay.Body.String() != first.Body.String() {
		t.Fatalf("replay = %d %s", replay.Code, replay.Body.String())
	}
	replayByApproval := serveBot(handler, http.MethodPost, "/api/harness/bot/approvals/apr-9/decide", "s3cret", botDecision)
	if replayByApproval.Code != http.StatusOK {
		t.Fatalf("replay by approval = %d %s", replayByApproval.Code, replayByApproval.Body.String())
	}
	other := serveBot(handler, http.MethodPost, "/api/harness/bot/approvals/apr-9/decide", "s3cret", strings.Replace(botDecision, "bot-1", "bot-2", 1))
	if other.Code != http.StatusConflict || !strings.Contains(other.Body.String(), `"code":"already_decided"`) {
		t.Fatalf("other key = %d %s", other.Code, other.Body.String())
	}
	nothing := serveBot(handler, http.MethodPost, "/api/harness/bot/runs/run-9/decision", "s3cret", strings.Replace(botDecision, "bot-1", "bot-3", 1))
	if nothing.Code != http.StatusNotFound {
		t.Fatalf("no waiting approval = %d %s", nothing.Code, nothing.Body.String())
	}
	missing := serveBot(handler, http.MethodPost, "/api/harness/bot/runs/run-404/decision", "s3cret", strings.Replace(botDecision, "bot-1", "bot-4", 1))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing run = %d %s", missing.Code, missing.Body.String())
	}
}

package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/storage"
)

type harnessTestWorkspaces map[string]models.WorkspaceConfig

func (f harnessTestWorkspaces) Get(id string) (models.WorkspaceConfig, bool, error) {
	workspace, ok := f[id]
	return workspace, ok, nil
}

func (f harnessTestWorkspaces) List() ([]models.WorkspaceConfig, error) { return nil, nil }

func newHarnessTestService(t *testing.T, local bool) (*harness.Service, domain.Repository) {
	t.Helper()
	repository, closeStore, err := storage.OpenSQLiteHarness(filepath.Join(t.TempDir(), "harness.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closeStore() })
	service := harness.New(harness.Dependencies{Repository: repository, Workspaces: harnessTestWorkspaces{"ws": {ID: "ws"}}, Config: harness.Config{LocalMode: local}})
	return service, repository
}

func serveHarness(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	var request *http.Request
	if body == "" {
		request = httptest.NewRequest(method, path, nil)
	} else {
		request = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestHarnessRoutesWithoutServiceAnswerUnavailable(t *testing.T) {
	handler := (&API{}).Routes()
	response := serveHarness(handler, http.MethodGet, "/api/harness/status", "")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestHarnessUnsupportedStorageReadsEmptyAndRefusesWrites(t *testing.T) {
	handler := New(Dependencies{Harness: harness.New(harness.Dependencies{Config: harness.Config{LocalMode: true}})}).Routes()
	status := serveHarness(handler, http.MethodGet, "/api/harness/status", "")
	var payload harness.Status
	if status.Code != http.StatusOK || json.Unmarshal(status.Body.Bytes(), &payload) != nil || payload.StorageSupported {
		t.Fatalf("status = %d body = %s", status.Code, status.Body.String())
	}
	tasks := serveHarness(handler, http.MethodGet, "/api/harness/tasks", "")
	if tasks.Code != http.StatusOK || !strings.Contains(tasks.Body.String(), `"tasks":[]`) {
		t.Fatalf("tasks = %d body = %s", tasks.Code, tasks.Body.String())
	}
	put := serveHarness(handler, http.MethodPut, "/api/harness/settings", `{"enabled":true}`)
	if put.Code != http.StatusServiceUnavailable || !strings.Contains(put.Body.String(), `"code":"unavailable"`) || !strings.Contains(put.Body.String(), "SQLite") {
		t.Fatalf("put = %d body = %s", put.Code, put.Body.String())
	}
}

func TestHarnessSettingsAndReadRoutes(t *testing.T) {
	service, repository := newHarnessTestService(t, true)
	handler := New(Dependencies{Harness: service}).Routes()

	get := serveHarness(handler, http.MethodGet, "/api/harness/settings", "")
	var settings harness.DashboardSettings
	if get.Code != http.StatusOK || json.Unmarshal(get.Body.Bytes(), &settings) != nil || settings.Revision != 0 || settings.Enabled || settings.Slots["qa"] != 1 || settings.ModelPolicy[domain.RoleCoder].HardModel != "opus" {
		t.Fatalf("get = %d body = %s", get.Code, get.Body.String())
	}
	settings.Enabled = true
	settings.Paused = map[string]bool{"qa": true}
	settings.Sources = []harness.DashboardSource{{WorkspaceID: "ws", Kind: "jira", Enabled: true, Query: "project = PM"}}
	policy := settings.ModelPolicy[domain.RoleCoder]
	policy.HardModel = "opus-next"
	settings.ModelPolicy[domain.RoleCoder] = policy
	body, _ := json.Marshal(settings)
	put := serveHarness(handler, http.MethodPut, "/api/harness/settings", string(body))
	if put.Code != http.StatusOK || !strings.Contains(put.Body.String(), `"revision":1`) || !strings.Contains(put.Body.String(), `"query":"project = PM"`) || !strings.Contains(put.Body.String(), `"paused":{"qa":true}`) || !strings.Contains(put.Body.String(), `"hardModel":"opus-next"`) {
		t.Fatalf("put = %d body = %s", put.Code, put.Body.String())
	}
	stored, _ := service.Settings(context.Background())
	if stored.Workspaces[0].Jira == nil || stored.Workspaces[0].Jira.JQL != "project = PM" || !stored.Paused[domain.RoleKeyQAFixer] || stored.ModelPolicy.Roles[domain.RoleCoder].AllowedTools == nil {
		t.Fatalf("stored = %#v", stored)
	}
	stale := serveHarness(handler, http.MethodPut, "/api/harness/settings", string(body))
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), `"code":"conflict"`) {
		t.Fatalf("stale = %d body = %s", stale.Code, stale.Body.String())
	}
	invalid := serveHarness(handler, http.MethodPut, "/api/harness/settings", `{"expectedRevision":1,"slots":{"coder":99}}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid = %d body = %s", invalid.Code, invalid.Body.String())
	}
	invalid = serveHarness(handler, http.MethodPut, "/api/harness/settings", `{"expectedRevision":1,"sources":[{"workspaceId":"ws","kind":"gitlab","query":"x"}]}`)
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("invalid = %d body = %s", invalid.Code, invalid.Body.String())
	}

	ctx := context.Background()
	now := time.Now().UTC()
	if err := repository.InsertTask(ctx, domain.Task{ID: "tsk-1", WorkspaceID: "ws", Source: domain.Source{Kind: domain.SourceJira, Key: "PM-1"}, Title: "One", Body: "secret-ish body", State: domain.TaskRanked, DiscoveredAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	queue := serveHarness(handler, http.MethodGet, "/api/harness/queue", "")
	if queue.Code != http.StatusOK || !strings.Contains(queue.Body.String(), `"key":"PM-1"`) || !strings.Contains(queue.Body.String(), `"createdAt":`) || strings.Contains(queue.Body.String(), "secret-ish") {
		t.Fatalf("queue = %d body = %s", queue.Code, queue.Body.String())
	}
	detail := serveHarness(handler, http.MethodGet, "/api/harness/tasks/tsk-1", "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), "secret-ish") {
		t.Fatalf("detail = %d body = %s", detail.Code, detail.Body.String())
	}
	filtered := serveHarness(handler, http.MethodGet, "/api/harness/tasks?state=done", "")
	if filtered.Code != http.StatusOK || !strings.Contains(filtered.Body.String(), `"tasks":[]`) {
		t.Fatalf("filtered = %d body = %s", filtered.Code, filtered.Body.String())
	}
	for _, path := range []string{"/api/harness/tasks/missing", "/api/harness/runs/missing", "/api/harness/agent-runs/missing"} {
		if response := serveHarness(handler, http.MethodGet, path, ""); response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
			t.Fatalf("%s = %d body = %s", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{"/api/harness/runs", "/api/harness/approvals", "/api/harness/status"} {
		if response := serveHarness(handler, http.MethodGet, path, ""); response.Code != http.StatusOK {
			t.Fatalf("%s = %d body = %s", path, response.Code, response.Body.String())
		}
	}
}

func TestHarnessRoutesNeedTheHarnessCapabilityToChange(t *testing.T) {
	apiHandler := &API{}
	apiHandler.initializeControllers()
	for _, route := range routeManifest(apiHandler) {
		if route.owner != "harness" {
			continue
		}
		policy, ok := policyForRoute(route)
		if !ok {
			t.Fatalf("%s %s has no policy", route.method, route.path)
		}
		if isMutatingMethod(route.method) {
			if policy.capability != models.CapabilityHarness || !policy.csrf {
				t.Fatalf("%s %s policy = %#v", route.method, route.path, policy)
			}
			if roleCanAccess(models.CloudRoleViewer, policy) || !roleCanAccess(models.CloudRoleEditor, policy) {
				t.Fatalf("%s %s role matrix is wrong", route.method, route.path)
			}
		} else if policy.capability != models.CapabilityRead {
			t.Fatalf("%s %s read policy = %#v", route.method, route.path, policy)
		}
	}
}

func TestHarnessEventStreamCatchesUpThenStreamsLive(t *testing.T) {
	service, repository := newHarnessTestService(t, true)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := repository.InsertTask(ctx, domain.Task{ID: "tsk-1", WorkspaceID: "ws", Source: domain.Source{Kind: domain.SourceJira, Key: "PM-1"}, Title: "One", State: domain.TaskRanked, DiscoveredAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	first, err := repository.AppendEvent(ctx, domain.Event{Kind: domain.EventTaskDiscovered, At: time.Now(), TaskID: "tsk-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.AppendEvent(ctx, domain.Event{Kind: domain.EventTaskRanked, At: time.Now(), TaskID: "tsk-1"}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(Dependencies{Harness: service}).Routes())
	defer server.Close()
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/api/harness/events", nil)
	request.Header.Set("Last-Event-ID", "1")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content type = %q", response.Header.Get("Content-Type"))
	}
	reader := bufio.NewReader(response.Body)
	readUntil := func(want string) string {
		t.Helper()
		deadline := time.After(3 * time.Second)
		lines := make(chan string)
		go func() {
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					close(lines)
					return
				}
				lines <- line
				if strings.Contains(line, want) {
					return
				}
			}
		}()
		var seen strings.Builder
		for {
			select {
			case line, ok := <-lines:
				if !ok {
					t.Fatalf("stream ended before %q; saw %q", want, seen.String())
				}
				seen.WriteString(line)
				if strings.Contains(line, want) {
					return seen.String()
				}
			case <-deadline:
				t.Fatalf("timed out waiting for %q; saw %q", want, seen.String())
			}
		}
	}
	caughtUp := readUntil(": connected")
	if strings.Contains(caughtUp, `"seq":`+itoa(first.Seq)+`,`) || !strings.Contains(caughtUp, `"type":"task.ranked"`) || !strings.Contains(caughtUp, `"task":{"id":"tsk-1"`) || !strings.Contains(caughtUp, "id: 2") {
		t.Fatalf("catch-up = %q", caughtUp)
	}
	if _, err := service.SaveSettings(ctx, domain.DefaultSettings(), 0, domain.Actor{ActorID: "local"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetRolePaused(ctx, domain.RoleKeyQAFixer, true, domain.Actor{ActorID: "local"}); err != nil {
		t.Fatal(err)
	}
	live := readUntil("role.paused")
	if !strings.Contains(live, "id: 4") || !strings.Contains(live, `"role":{"role":"qa","paused":true`) || strings.Contains(live, "settings.updated") {
		t.Fatalf("live = %q", live)
	}
}

func itoa(value int64) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func TestHarnessQueueOperationRoutes(t *testing.T) {
	service, repository := newHarnessTestService(t, true)
	handler := New(Dependencies{Harness: service}).Routes()
	ctx := context.Background()
	now := time.Now().UTC()
	if err := repository.InsertTask(ctx, domain.Task{ID: "tsk-1", WorkspaceID: "ws", Source: domain.Source{Kind: domain.SourceJira, Key: "PM-1"}, Title: "One", State: domain.TaskRanked, DiscoveredAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	pin := serveHarness(handler, http.MethodPost, "/api/harness/tasks/tsk-1/pin", `{"pinned":true,"expectedRevision":0}`)
	if pin.Code != http.StatusOK || !strings.Contains(pin.Body.String(), `"pinned":true`) {
		t.Fatalf("pin = %d %s", pin.Code, pin.Body.String())
	}
	stale := serveHarness(handler, http.MethodPost, "/api/harness/tasks/tsk-1/pin", `{"pinned":false,"expectedRevision":0}`)
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale pin = %d %s", stale.Code, stale.Body.String())
	}
	unknown := serveHarness(handler, http.MethodPost, "/api/harness/tasks/tsk-1/pin", `{"pinned":false,"expectedRevision":1,"extra":1}`)
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown field = %d %s", unknown.Code, unknown.Body.String())
	}
	priority := serveHarness(handler, http.MethodPost, "/api/harness/tasks/tsk-1/priority", `{"priorityOverride":5,"expectedRevision":1}`)
	if priority.Code != http.StatusOK || !strings.Contains(priority.Body.String(), `"priorityOverride":5`) {
		t.Fatalf("priority = %d %s", priority.Code, priority.Body.String())
	}
	if err := repository.InsertTask(ctx, domain.Task{ID: "tsk-2", WorkspaceID: "ws", Source: domain.Source{Kind: domain.SourceJira, Key: "PM-2"}, Title: "Two", State: domain.TaskRanked, Score: 60, DiscoveredAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	moved := serveHarness(handler, http.MethodPost, "/api/harness/tasks/tsk-1/priority", `{"before":"tsk-2","expectedRevision":2}`)
	if moved.Code != http.StatusOK || !strings.Contains(moved.Body.String(), `"priorityOverride":-500`) {
		t.Fatalf("moved = %d %s", moved.Code, moved.Body.String())
	}
	if empty := serveHarness(handler, http.MethodPost, "/api/harness/tasks/tsk-1/priority", `{"expectedRevision":3}`); empty.Code != http.StatusBadRequest {
		t.Fatalf("empty priority = %d %s", empty.Code, empty.Body.String())
	}
	start := serveHarness(handler, http.MethodPost, "/api/harness/tasks/tsk-1/start", `{"expectedRevision":3,"idempotencyKey":"k1"}`)
	if start.Code != http.StatusOK || !strings.Contains(start.Body.String(), `"startRequested":true`) {
		t.Fatalf("start = %d %s", start.Code, start.Body.String())
	}
	dismiss := serveHarness(handler, http.MethodPost, "/api/harness/tasks/tsk-1/dismiss", `{"reason":"no","expectedRevision":4}`)
	if dismiss.Code != http.StatusOK || !strings.Contains(dismiss.Body.String(), `"state":"dismissed"`) {
		t.Fatalf("dismiss = %d %s", dismiss.Code, dismiss.Body.String())
	}
	poll := serveHarness(handler, http.MethodPost, "/api/harness/scout/poll", "")
	if poll.Code != http.StatusOK || !strings.Contains(poll.Body.String(), `"started":true`) {
		t.Fatalf("poll = %d %s", poll.Code, poll.Body.String())
	}
	if noTranscript := serveHarness(handler, http.MethodGet, "/api/harness/agent-runs/missing/transcript", ""); noTranscript.Code != http.StatusNotFound {
		t.Fatalf("missing transcript = %d %s", noTranscript.Code, noTranscript.Body.String())
	}
	cleared := serveHarness(handler, http.MethodPost, "/api/harness/queue/clear", "")
	if cleared.Code != http.StatusOK || !strings.Contains(cleared.Body.String(), `"removed":2`) {
		t.Fatalf("clear = %d %s", cleared.Code, cleared.Body.String())
	}
	missing := serveHarness(handler, http.MethodPost, "/api/harness/tasks/missing/start", `{"expectedRevision":0}`)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing = %d %s", missing.Code, missing.Body.String())
	}

	cloud, _ := newHarnessTestService(t, false)
	cloudHandler := New(Dependencies{Harness: cloud}).Routes()
	refused := serveHarness(cloudHandler, http.MethodPost, "/api/harness/scout/poll", "")
	if refused.Code != http.StatusServiceUnavailable || !strings.Contains(refused.Body.String(), "Local mode") {
		t.Fatalf("cloud poll = %d %s", refused.Code, refused.Body.String())
	}
}

func seedWaitingPlanApproval(t *testing.T, repository domain.Repository) domain.Approval {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	task := domain.Task{ID: "tsk-9", WorkspaceID: "ws", Source: domain.Source{Kind: domain.SourceJira, Key: "PM-9"}, Title: "Nine", State: domain.TaskRunning, RunID: "run-9", NeedsYou: true, DiscoveredAt: now, UpdatedAt: now}
	stage := domain.NewStage("run-9", domain.StagePlanApproval, 1)
	stage.State = domain.StageAwaitingApproval
	run := domain.Run{ID: "run-9", TaskID: task.ID, WorkspaceID: "ws", SourceKey: "PM-9", State: domain.RunRunning, CurrentStageID: stage.ID, CurrentStageKind: stage.Kind, StartedAt: now}
	approval := domain.Approval{ID: "apr-9", RunID: run.ID, TaskID: task.ID, StageID: stage.ID, SourceKey: "PM-9", Gate: domain.GatePlan, State: domain.ApprovalRequested, RequestedAt: now}
	for _, err := range []error{repository.InsertTask(ctx, task), repository.InsertRun(ctx, run), repository.InsertStage(ctx, stage), repository.InsertApproval(ctx, approval)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	return approval
}

func TestHarnessDecideCancelCostAndRoleRoutes(t *testing.T) {
	service, repository := newHarnessTestService(t, true)
	handler := New(Dependencies{Harness: service}).Routes()
	seedWaitingPlanApproval(t, repository)

	missingKey := serveHarness(handler, http.MethodPost, "/api/harness/approvals/apr-9/decide", `{"decision":"approve"}`)
	if missingKey.Code != http.StatusBadRequest {
		t.Fatalf("missing key = %d %s", missingKey.Code, missingKey.Body.String())
	}
	stale := serveHarness(handler, http.MethodPost, "/api/harness/approvals/apr-9/decide", `{"decision":"approve","idempotencyKey":"a","expectedRevision":7}`)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), `"code":"conflict"`) {
		t.Fatalf("stale = %d %s", stale.Code, stale.Body.String())
	}
	approve := serveHarness(handler, http.MethodPost, "/api/harness/approvals/apr-9/decide", `{"decision":"approve","idempotencyKey":"a","expectedRevision":0}`)
	if approve.Code != http.StatusOK || !strings.Contains(approve.Body.String(), `"state":"approved"`) || !strings.Contains(approve.Body.String(), `"channel":"dashboard"`) {
		t.Fatalf("approve = %d %s", approve.Code, approve.Body.String())
	}
	replay := serveHarness(handler, http.MethodPost, "/api/harness/approvals/apr-9/decide", `{"decision":"approve","idempotencyKey":"a","expectedRevision":0}`)
	if replay.Code != http.StatusOK || replay.Body.String() != approve.Body.String() {
		t.Fatalf("replay = %d %s", replay.Code, replay.Body.String())
	}
	again := serveHarness(handler, http.MethodPost, "/api/harness/approvals/apr-9/decide", `{"decision":"reject","reason":"no","idempotencyKey":"b"}`)
	if again.Code != http.StatusConflict || !strings.Contains(again.Body.String(), `"code":"already_decided"`) || !strings.Contains(again.Body.String(), `"approval":{`) {
		t.Fatalf("again = %d %s", again.Code, again.Body.String())
	}
	run := serveHarness(handler, http.MethodGet, "/api/harness/runs/run-9", "")
	var detail harness.DashboardRunDetail
	if run.Code != http.StatusOK || json.Unmarshal(run.Body.Bytes(), &detail) != nil || detail.Run.CurrentStageKind != domain.StageImplement || detail.Run.CurrentStage == nil || detail.Run.ActiveAgentRun == nil || detail.Task.ID != "tsk-9" || len(detail.Approvals) != 1 || len(detail.AgentRuns) != 1 {
		t.Fatalf("run = %d %s", run.Code, run.Body.String())
	}

	pause := serveHarness(handler, http.MethodPost, "/api/harness/roles/qa/pause", "")
	if pause.Code != http.StatusOK || !strings.Contains(pause.Body.String(), `"paused":true`) {
		t.Fatalf("pause = %d %s", pause.Code, pause.Body.String())
	}
	if !strings.Contains(pause.Body.String(), `"role":"qa"`) {
		t.Fatalf("pause role = %s", pause.Body.String())
	}
	if resume := serveHarness(handler, http.MethodPost, "/api/harness/roles/qa/resume", ""); resume.Code != http.StatusOK || !strings.Contains(resume.Body.String(), `"paused":false`) {
		t.Fatalf("resume = %d %s", resume.Code, resume.Body.String())
	}
	if unknown := serveHarness(handler, http.MethodPost, "/api/harness/roles/janitor/pause", ""); unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown role = %d %s", unknown.Code, unknown.Body.String())
	}
	cost := serveHarness(handler, http.MethodGet, "/api/harness/cost?groupBy=role&from=2026-01-01", "")
	if cost.Code != http.StatusOK || !strings.Contains(cost.Body.String(), `"groupBy":"role"`) || !strings.Contains(cost.Body.String(), `"groups":[{"key":"coder"`) || !strings.Contains(cost.Body.String(), `"agentRuns":1`) {
		t.Fatalf("cost = %d %s", cost.Code, cost.Body.String())
	}
	if bad := serveHarness(handler, http.MethodGet, "/api/harness/cost?groupBy=week", ""); bad.Code != http.StatusBadRequest {
		t.Fatalf("bad cost = %d %s", bad.Code, bad.Body.String())
	}
	if badDate := serveHarness(handler, http.MethodGet, "/api/harness/cost?from=yesterday", ""); badDate.Code != http.StatusBadRequest {
		t.Fatalf("bad date = %d %s", badDate.Code, badDate.Body.String())
	}
	changes := serveHarness(handler, http.MethodGet, "/api/harness/runs/run-9/changes", "")
	if changes.Code != http.StatusOK || !strings.Contains(changes.Body.String(), `"availability":"unavailable"`) {
		t.Fatalf("changes = %d %s", changes.Code, changes.Body.String())
	}
	cancel := serveHarness(handler, http.MethodPost, "/api/harness/runs/run-9/cancel", `{"reason":"enough"}`)
	if cancel.Code != http.StatusOK || !strings.Contains(cancel.Body.String(), `"state":"cancelled"`) {
		t.Fatalf("cancel = %d %s", cancel.Code, cancel.Body.String())
	}
	if twice := serveHarness(handler, http.MethodPost, "/api/harness/runs/run-9/cancel", ""); twice.Code != http.StatusConflict {
		t.Fatalf("cancel twice = %d %s", twice.Code, twice.Body.String())
	}
}

func TestHarnessMutationsAreHostedExecutionInCloudMode(t *testing.T) {
	if !cloudHostedExecutionRoute(http.MethodPost, "/api/harness/approvals/:id/decide") || cloudHostedExecutionRoute(http.MethodGet, "/api/harness/status") {
		t.Fatal("harness mutations must be refused as hosted execution in Cloud mode")
	}
}

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
	var settings domain.Settings
	if get.Code != http.StatusOK || json.Unmarshal(get.Body.Bytes(), &settings) != nil || settings.Revision != 0 || settings.Enabled {
		t.Fatalf("get = %d body = %s", get.Code, get.Body.String())
	}
	settings.Enabled = true
	settings.Workspaces = []domain.WorkspaceSettings{{WorkspaceID: "ws", Jira: &domain.JiraSourceSettings{Enabled: true, JQL: "project = PM"}}}
	body, _ := json.Marshal(settings)
	put := serveHarness(handler, http.MethodPut, "/api/harness/settings", string(body))
	if put.Code != http.StatusOK || !strings.Contains(put.Body.String(), `"revision":1`) {
		t.Fatalf("put = %d body = %s", put.Code, put.Body.String())
	}
	stale := serveHarness(handler, http.MethodPut, "/api/harness/settings", string(body))
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), `"code":"conflict"`) {
		t.Fatalf("stale = %d body = %s", stale.Code, stale.Body.String())
	}
	invalid := serveHarness(handler, http.MethodPut, "/api/harness/settings", `{"expectedRevision":1,"slots":{"coder":99}}`)
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("invalid = %d body = %s", invalid.Code, invalid.Body.String())
	}

	ctx := context.Background()
	now := time.Now().UTC()
	if err := repository.InsertTask(ctx, domain.Task{ID: "tsk-1", WorkspaceID: "ws", Source: domain.Source{Kind: domain.SourceJira, Key: "PM-1"}, Title: "One", Body: "secret-ish body", State: domain.TaskRanked, DiscoveredAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	queue := serveHarness(handler, http.MethodGet, "/api/harness/queue", "")
	if queue.Code != http.StatusOK || !strings.Contains(queue.Body.String(), `"key":"PM-1"`) || strings.Contains(queue.Body.String(), "secret-ish") {
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
	if strings.Contains(caughtUp, `"seq":`+itoa(first.Seq)+`,`) || !strings.Contains(caughtUp, `"type":"task.ranked"`) || !strings.Contains(caughtUp, "id: 2") {
		t.Fatalf("catch-up = %q", caughtUp)
	}
	settings := domain.DefaultSettings()
	if _, err := service.SaveSettings(ctx, settings, 0, domain.Actor{ActorID: "local"}); err != nil {
		t.Fatal(err)
	}
	live := readUntil("settings.updated")
	if !strings.Contains(live, "id: 3") {
		t.Fatalf("live = %q", live)
	}
}

func itoa(value int64) string {
	data, _ := json.Marshal(value)
	return string(data)
}

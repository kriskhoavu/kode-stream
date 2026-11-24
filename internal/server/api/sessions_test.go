package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appaisession "kode-stream/internal/ai"
	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/system"
	"kode-stream/internal/workspace/registry"
)

type sessionsTestFixture struct {
	controller *sessionsController
	records    *appaisession.FileSessionRecordRepository
	workspace  models.WorkspaceConfig
	cloud      *cloudWorkspaceStore
}

func newSessionsTestFixture(t *testing.T, mode models.RuntimeMode) sessionsTestFixture {
	t.Helper()
	root := t.TempDir()
	gitCmd := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	gitCmd("init", "-b", "main")
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("seed"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd("add", ".")
	gitCmd("commit", "-m", "seed")

	dataDir := t.TempDir()
	reg := registry.New(filepath.Join(dataDir, "workspaces.yaml"), gitadapter.New())
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Test", Path: root, BaselineBranch: "main", Sources: models.SourceList("plans")})
	if err != nil {
		t.Fatal(err)
	}
	launchCommit := func() string {
		adapter := gitadapter.New()
		branch, err := adapter.CurrentBranch(root)
		if err != nil {
			t.Fatal(err)
		}
		_, commit, err := adapter.ResolveBranch(root, branch)
		if err != nil {
			t.Fatal(err)
		}
		return commit
	}()

	if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd("add", "seed.txt")
	gitCmd("commit", "-m", "modify")

	records := appaisession.NewFileSessionRecordRepository(filepath.Join(dataDir, "records.yaml"))
	now := time.Now().UTC()
	if _, err := records.Upsert(appaisession.SessionRecord{
		ID: "session-1", WorkspaceID: workspace.ID, Provider: "claude", Intent: "workspace_only",
		RequestedBranch: "main", ObservedCommit: launchCommit, State: appaisession.StateExited,
		StartedAt: now, LastKnownAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	service := appaisession.New(appaisession.NewSettingsRepository(filepath.Join(dataDir, "ai-settings.yaml")))
	service.ConfigureLaunch(reg, itemindex.New(filepath.Join(dataDir, "items.yaml")), nil, t.TempDir())
	service.ConfigureSessionRecords(records, gitadapter.New())

	cloud := newCloudWorkspaceStore()
	runtimeConfig := system.RuntimeConfig{Mode: mode}
	aiCtrl := &aiController{aiSessions: service, runtimeConfig: runtimeConfig, cloudWorkspaces: cloud}
	return sessionsTestFixture{
		controller: &sessionsController{aiController: aiCtrl},
		records:    records,
		workspace:  workspace,
		cloud:      cloud,
	}
}

func withCloudSession(r *http.Request, userID string) *http.Request {
	session := cloudSession{User: models.CloudUser{ID: userID}, ExpiresAt: time.Now().Add(time.Hour)}
	return r.WithContext(context.WithValue(r.Context(), cloudSessionContextKey{}, session))
}

func TestSessionChangesReturnsAvailableChanges(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeLocal)
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/session-1/changes", nil)
	req.SetPathValue("id", "session-1")
	recorder := httptest.NewRecorder()
	fixture.controller.sessionChanges(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var changes appaisession.SessionChanges
	if err := json.Unmarshal(recorder.Body.Bytes(), &changes); err != nil {
		t.Fatal(err)
	}
	if changes.Availability != appaisession.SessionChangesAvailable || len(changes.Files) == 0 {
		t.Fatalf("changes=%#v", changes)
	}
}

func TestSessionChangesReturns404ForUnknownSession(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeLocal)
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/does-not-exist/changes", nil)
	req.SetPathValue("id", "does-not-exist")
	recorder := httptest.NewRecorder()
	fixture.controller.sessionChanges(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

// A non-owner must see the 404 a missing session gives, never a 403.
func TestSessionChangesCloudOwnership(t *testing.T) {
	for _, tc := range []struct {
		caller string
		want   int
	}{
		{"owner", http.StatusOK},
		{"intruder", http.StatusNotFound},
	} {
		t.Run(tc.caller, func(t *testing.T) {
			fixture := newSessionsTestFixture(t, models.RuntimeModeCloud)
			if _, err := fixture.cloud.Upsert(context.Background(), models.WorkspaceConfig{ID: fixture.workspace.ID, OwnerUserID: "owner", Name: fixture.workspace.Name, Path: fixture.workspace.Path}); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodGet, "/api/sessions/session-1/changes", nil)
			req.SetPathValue("id", "session-1")
			req = withCloudSession(req, tc.caller)
			recorder := httptest.NewRecorder()
			fixture.controller.sessionChanges(recorder, req)
			if recorder.Code != tc.want {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestSessionDiffValidatesPath(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeLocal)
	for _, path := range []string{"", ":stage", "-oops"} {
		req := httptest.NewRequest(http.MethodGet, "/api/sessions/session-1/diff?path="+path, nil)
		req.SetPathValue("id", "session-1")
		recorder := httptest.NewRecorder()
		fixture.controller.sessionDiff(recorder, req)
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"session_request_invalid"`) {
			t.Fatalf("path=%q status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestSessionDiffReturnsDiffForAvailableChanges(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeLocal)
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/session-1/diff?path=seed.txt", nil)
	req.SetPathValue("id", "session-1")
	recorder := httptest.NewRecorder()
	fixture.controller.sessionDiff(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var diff appaisession.SessionFileDiffResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &diff); err != nil {
		t.Fatal(err)
	}
	if diff.Path != "seed.txt" || !strings.Contains(diff.Diff, "seed.txt") {
		t.Fatalf("diff=%#v", diff)
	}
}

func TestSessionDiffReturnsConflictWhenChangesUnavailable(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeLocal)
	record, found, err := fixture.records.Get("session-1")
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	record.ObservedCommit = ""
	if _, err := fixture.records.Upsert(record); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/session-1/diff?path=seed.txt", nil)
	req.SetPathValue("id", "session-1")
	recorder := httptest.NewRecorder()
	fixture.controller.sessionDiff(recorder, req)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"code":"launch_commit_unknown"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestGlobalSessionsRejectsInvalidScope(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeLocal)
	req := httptest.NewRequest(http.MethodGet, "/api/sessions?scope=bogus", nil)
	recorder := httptest.NewRecorder()
	fixture.controller.globalSessions(recorder, req)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"session_request_invalid"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestGlobalSessionsDefaultsToActiveScope(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeLocal)
	payload := fetchGlobalSessions(t, fixture, "/api/sessions", "")
	if len(payload.Rows) != 0 {
		t.Fatalf("rows=%#v", payload.Rows)
	}
}

func TestGlobalSessionsScopeAllListsExitedSession(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeLocal)
	payload := fetchGlobalSessions(t, fixture, "/api/sessions?scope=all", "")
	if len(payload.Rows) != 1 || payload.Rows[0].Record.ID != "session-1" || payload.Rows[0].WorkspaceName != "Test" {
		t.Fatalf("rows=%#v", payload.Rows)
	}
}

func TestDeleteSessionRemovesFinishedRecord(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeLocal)
	req := httptest.NewRequest(http.MethodDelete, "/api/sessions/session-1", nil)
	req.SetPathValue("id", "session-1")
	recorder := httptest.NewRecorder()
	fixture.controller.deleteSession(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if _, found, err := fixture.records.Get("session-1"); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}

func TestGlobalSessionsHidesWorkspacesCloudCallerMayNotRead(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeCloud)
	payload := fetchGlobalSessions(t, fixture, "/api/sessions?scope=all", "stranger")
	if len(payload.Rows) != 0 {
		t.Fatalf("rows=%#v, want the workspace hidden rather than a 403", payload.Rows)
	}
}

type globalSessionsPayload struct {
	Rows      []appaisession.GlobalSessionRow `json:"rows"`
	Truncated bool                            `json:"truncated"`
}

func fetchGlobalSessions(t *testing.T, fixture sessionsTestFixture, path, caller string) globalSessionsPayload {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if caller != "" {
		req = withCloudSession(req, caller)
	}
	recorder := httptest.NewRecorder()
	fixture.controller.globalSessions(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload globalSessionsPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

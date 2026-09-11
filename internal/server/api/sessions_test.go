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

	"gopkg.in/yaml.v3"

	appaisession "kode-stream/internal/ai"
	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/system"
	"kode-stream/internal/workspace/registry"
)

// sessionsTestFixture is a real (not faked) Git checkout, workspace registry,
// item index, and AI session service, wired the way server.go wires them,
// so sessions_test.go exercises the sessions controller against the same
// collaborators the ai package tests already validate in isolation.
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
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Test", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
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

	// A change made after the launch commit so an available SessionChanges has
	// something to report.
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

func TestSessionChangesReturns404WhenCloudCallerMayNotReadWorkspace(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeCloud)
	// The workspace is registered under "owner", not "intruder": a Cloud caller
	// who is not the owner must see the same 404 a missing session would give,
	// never a 403 that would confirm the session exists.
	if _, err := fixture.cloud.Upsert(context.Background(), models.WorkspaceConfig{ID: fixture.workspace.ID, OwnerUserID: "owner", Name: fixture.workspace.Name, Path: fixture.workspace.Path}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/session-1/changes", nil)
	req.SetPathValue("id", "session-1")
	req = withCloudSession(req, "intruder")
	recorder := httptest.NewRecorder()
	fixture.controller.sessionChanges(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSessionChangesAllowsCloudOwner(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeCloud)
	if _, err := fixture.cloud.Upsert(context.Background(), models.WorkspaceConfig{ID: fixture.workspace.ID, OwnerUserID: "owner", Name: fixture.workspace.Name, Path: fixture.workspace.Path}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/session-1/changes", nil)
	req.SetPathValue("id", "session-1")
	req = withCloudSession(req, "owner")
	recorder := httptest.NewRecorder()
	fixture.controller.sessionChanges(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

// TestSessionChangesReportsContentProviderUnsupported confirms a session
// whose workspace's content provider is not a local checkout (here, a Remote
// Snapshot workspace) comes back as a 200 with
// availability=unsupported/reason=content_provider_unsupported, rather than
// an error -- the controller simply forwards the guard result inline. The
// workspace is written directly into the registry's YAML file (rather than
// through Registry.Create, which does not persist AccessMode at all) so the
// guard sees a non-local-checkout workspace without needing a real Remote
// Snapshot checkout on disk.
func TestSessionChangesReportsContentProviderUnsupported(t *testing.T) {
	dataDir := t.TempDir()
	workspacesPath := filepath.Join(dataDir, "workspaces.yaml")
	workspace := models.WorkspaceConfig{
		ID:             "ws-snap",
		Name:           "Snapshot",
		Path:           t.TempDir(),
		AccessMode:     models.WorkspaceAccessModeRemoteSnapshot,
		BaselineBranch: "main",
		Sources:        []string{"plans"},
		CreatedAt:      time.Now().UTC(),
	}
	yamlBytes, err := yaml.Marshal([]models.WorkspaceConfig{workspace})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workspacesPath, yamlBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.New(workspacesPath, gitadapter.New())

	records := appaisession.NewFileSessionRecordRepository(filepath.Join(dataDir, "records.yaml"))
	now := time.Now().UTC()
	if _, err := records.Upsert(appaisession.SessionRecord{
		ID: "session-1", WorkspaceID: workspace.ID, Provider: "claude", Intent: "workspace_only",
		RequestedBranch: "main", ObservedCommit: "deadbeef", State: appaisession.StateExited,
		StartedAt: now, LastKnownAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	service := appaisession.New(appaisession.NewSettingsRepository(filepath.Join(dataDir, "ai-settings.yaml")))
	service.ConfigureLaunch(reg, itemindex.New(filepath.Join(dataDir, "items.yaml")), nil, t.TempDir())
	service.ConfigureSessionRecords(records, gitadapter.New())
	aiCtrl := &aiController{aiSessions: service, runtimeConfig: system.RuntimeConfig{Mode: models.RuntimeModeLocal}, cloudWorkspaces: newCloudWorkspaceStore()}
	controller := &sessionsController{aiController: aiCtrl}

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/session-1/changes", nil)
	req.SetPathValue("id", "session-1")
	recorder := httptest.NewRecorder()
	controller.sessionChanges(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var changes appaisession.SessionChanges
	if err := json.Unmarshal(recorder.Body.Bytes(), &changes); err != nil {
		t.Fatal(err)
	}
	if changes.Availability != appaisession.SessionChangesUnsupported || changes.Reason != appaisession.ReasonContentProviderUnsupported {
		t.Fatalf("changes=%#v", changes)
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
	// Overwrite the seeded record with an empty ObservedCommit so the guard
	// chain reports launch_commit_unknown.
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
	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	recorder := httptest.NewRecorder()
	fixture.controller.globalSessions(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Rows      []appaisession.GlobalSessionRow `json:"rows"`
		Truncated bool                            `json:"truncated"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	// The seeded record is exited (not live), so the default active scope
	// excludes it.
	if len(payload.Rows) != 0 {
		t.Fatalf("rows=%#v", payload.Rows)
	}
}

func TestGlobalSessionsScopeAllListsExitedSession(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeLocal)
	req := httptest.NewRequest(http.MethodGet, "/api/sessions?scope=all", nil)
	recorder := httptest.NewRecorder()
	fixture.controller.globalSessions(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Rows      []appaisession.GlobalSessionRow `json:"rows"`
		Truncated bool                            `json:"truncated"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Rows) != 1 || payload.Rows[0].Record.ID != "session-1" || payload.Rows[0].WorkspaceName != "Test" {
		t.Fatalf("rows=%#v", payload.Rows)
	}
}

func TestGlobalSessionsHidesWorkspacesCloudCallerMayNotRead(t *testing.T) {
	fixture := newSessionsTestFixture(t, models.RuntimeModeCloud)
	req := httptest.NewRequest(http.MethodGet, "/api/sessions?scope=all", nil)
	req = withCloudSession(req, "stranger")
	recorder := httptest.NewRecorder()
	fixture.controller.globalSessions(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Rows      []appaisession.GlobalSessionRow `json:"rows"`
		Truncated bool                            `json:"truncated"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Rows) != 0 {
		t.Fatalf("rows=%#v, want the workspace hidden rather than a 403", payload.Rows)
	}
}

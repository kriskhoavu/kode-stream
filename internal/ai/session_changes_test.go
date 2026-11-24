package ai

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/workspace/registry"
)

type fakeWorkspaceRegistry struct {
	workspaces map[string]models.WorkspaceConfig
	getErr     error
	listErr    error
}

func (f *fakeWorkspaceRegistry) List() ([]models.WorkspaceConfig, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	result := make([]models.WorkspaceConfig, 0, len(f.workspaces))
	for _, workspace := range f.workspaces {
		result = append(result, workspace)
	}
	return result, nil
}

func (f *fakeWorkspaceRegistry) Get(id string) (models.WorkspaceConfig, bool, error) {
	if f.getErr != nil {
		return models.WorkspaceConfig{}, false, f.getErr
	}
	workspace, ok := f.workspaces[id]
	return workspace, ok, nil
}

func (f *fakeWorkspaceRegistry) Create(models.WorkspaceInput) (models.WorkspaceConfig, error) {
	return models.WorkspaceConfig{}, errors.New("not implemented")
}
func (f *fakeWorkspaceRegistry) Validate(models.WorkspaceInput) (models.WorkspaceConfig, error) {
	return models.WorkspaceConfig{}, errors.New("not implemented")
}
func (f *fakeWorkspaceRegistry) Path() string { return "" }
func (f *fakeWorkspaceRegistry) BatchCreate([]models.WorkspaceInput) ([]registry.BatchCreateResult, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeWorkspaceRegistry) Update(string, models.WorkspaceInput) (models.WorkspaceConfig, error) {
	return models.WorkspaceConfig{}, errors.New("not implemented")
}
func (f *fakeWorkspaceRegistry) Delete(string) error { return errors.New("not implemented") }
func (f *fakeWorkspaceRegistry) TouchScanned(string, time.Time) error {
	return errors.New("not implemented")
}
func (f *fakeWorkspaceRegistry) SetLastSelectedBranch(string, string) error {
	return errors.New("not implemented")
}
func (f *fakeWorkspaceRegistry) SetRuntime(string, *models.WorkspaceRuntimeConfig) (models.WorkspaceConfig, error) {
	return models.WorkspaceConfig{}, errors.New("not implemented")
}

type fakeItemIndex struct {
	items map[string]models.ItemDetail
}

func (f *fakeItemIndex) Get(id string) (models.ItemDetail, bool, error) {
	item, ok := f.items[id]
	return item, ok, nil
}
func (f *fakeItemIndex) ReplaceWorkspace(string, []models.ItemDetail, []models.ScanWarning, time.Time) error {
	return errors.New("not implemented")
}
func (f *fakeItemIndex) ReplaceWorkspaceBranch(string, string, []models.ItemDetail, models.BranchScanMetadata) error {
	return errors.New("not implemented")
}
func (f *fakeItemIndex) DeleteWorkspace(string) error { return errors.New("not implemented") }
func (f *fakeItemIndex) Query(itemindex.Query) ([]models.ItemSummary, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeItemIndex) BranchItems(string, string) ([]models.ItemSummary, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeItemIndex) VisitContext(context.Context, itemindex.Query, func(models.ItemSummary) bool) error {
	return errors.New("not implemented")
}
func (f *fakeItemIndex) BranchScan(string, string) (models.BranchScanMetadata, bool, error) {
	return models.BranchScanMetadata{}, false, errors.New("not implemented")
}

type fakeChangeGit struct {
	currentBranch      string
	currentBranchErr   error
	currentBranchCalls int
	resolvedCommit     string
	resolveErr         error
	commitExistsErr    error
	nameStatus         []gitadapter.NameStatusEntry
	nameStatusTrunc    bool
	nameStatusErr      error
	additions          int
	deletions          int
	shortStatErr       error
	revListCount       int
	revListErr         error
	diff               string
	diffTruncated      bool
	diffErr            error
}

func (f *fakeChangeGit) CurrentBranch(string) (string, error) {
	f.currentBranchCalls++
	return f.currentBranch, f.currentBranchErr
}
func (f *fakeChangeGit) ResolveBranch(string, string) (string, string, error) {
	return f.currentBranch, f.resolvedCommit, f.resolveErr
}
func (f *fakeChangeGit) Status(string, string) (models.GitStatus, error) {
	return models.GitStatus{}, nil
}
func (f *fakeChangeGit) NameStatusSinceContext(context.Context, string, string) ([]gitadapter.NameStatusEntry, bool, error) {
	return f.nameStatus, f.nameStatusTrunc, f.nameStatusErr
}
func (f *fakeChangeGit) ShortStatSinceContext(context.Context, string, string) (int, int, int, error) {
	return f.additions, f.deletions, len(f.nameStatus), f.shortStatErr
}
func (f *fakeChangeGit) DiffFileSince(context.Context, string, string, string, int64) (string, bool, error) {
	return f.diff, f.diffTruncated, f.diffErr
}
func (f *fakeChangeGit) RevListCountContext(context.Context, string, string, string) (int, error) {
	return f.revListCount, f.revListErr
}
func (f *fakeChangeGit) CommitExistsContext(context.Context, string, string) error {
	return f.commitExistsErr
}

func changesTestService(t *testing.T, workspace models.WorkspaceConfig, record SessionRecord, git *fakeChangeGit) (*Service, *FileSessionRecordRepository) {
	t.Helper()
	dataDir := t.TempDir()
	records := NewFileSessionRecordRepository(filepath.Join(dataDir, "records.yaml"))
	if _, err := records.Upsert(record); err != nil {
		t.Fatal(err)
	}
	reg := &fakeWorkspaceRegistry{workspaces: map[string]models.WorkspaceConfig{workspace.ID: workspace}}
	index := &fakeItemIndex{items: map[string]models.ItemDetail{}}
	service := New(NewSettingsRepository(filepath.Join(dataDir, "ai-settings.yaml")))
	service.ConfigureLaunch(reg, index, nil, t.TempDir())
	service.ConfigureSessionRecords(records, git)
	return service, records
}

func baseSessionRecord(workspaceID string, observedCommit string) SessionRecord {
	now := time.Now().UTC()
	return SessionRecord{
		ID: "session-1", WorkspaceID: workspaceID, Provider: "claude", Intent: "workspace_only",
		RequestedBranch: "main", ObservedCommit: observedCommit, State: StateExited,
		StartedAt: now, LastKnownAt: now,
	}
}

func TestSessionChangesUnsupportedForNonLocalContentProvider(t *testing.T) {
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "Snapshot", Path: t.TempDir(), AccessMode: models.WorkspaceAccessModeRemoteSnapshot}
	service, _ := changesTestService(t, workspace, baseSessionRecord(workspace.ID, "deadbeef"), &fakeChangeGit{})
	result, err := service.SessionChanges(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if result.Availability != SessionChangesUnavailable && result.Availability != SessionChangesUnsupported {
		t.Fatalf("availability=%q", result.Availability)
	}
	if result.Availability != SessionChangesUnsupported || result.Reason != ReasonContentProviderUnsupported {
		t.Fatalf("availability=%q reason=%q", result.Availability, result.Reason)
	}
}

func TestSessionChangesUnavailableReasons(t *testing.T) {
	for _, tc := range []struct {
		name         string
		missingPath  bool
		launchCommit string
		git          *fakeChangeGit
		want         string
	}{
		{"workspace path missing", true, "deadbeef", &fakeChangeGit{}, ReasonWorkspaceMissing},
		{"launch commit unknown", false, "", &fakeChangeGit{}, ReasonLaunchCommitUnknown},
		{"launch commit missing", false, "deadbeef", &fakeChangeGit{commitExistsErr: &gitadapter.ErrUnknownCommit{Ref: "deadbeef"}}, ReasonLaunchCommitMissing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir()
			if tc.missingPath {
				path = filepath.Join(path, "does-not-exist")
			}
			workspace := models.WorkspaceConfig{ID: "ws-1", Name: "Local", Path: path}
			service, _ := changesTestService(t, workspace, baseSessionRecord(workspace.ID, tc.launchCommit), tc.git)
			result, err := service.SessionChanges(context.Background(), "session-1")
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if result.Availability != SessionChangesUnavailable || result.Reason != tc.want {
				t.Fatalf("availability=%q reason=%q, want %q", result.Availability, result.Reason, tc.want)
			}
		})
	}
}

func TestSessionChangesUnavailableWhenWorkspaceRecordMissing(t *testing.T) {
	dataDir := t.TempDir()
	records := NewFileSessionRecordRepository(filepath.Join(dataDir, "records.yaml"))
	if _, err := records.Upsert(baseSessionRecord("ws-ghost", "deadbeef")); err != nil {
		t.Fatal(err)
	}
	reg := &fakeWorkspaceRegistry{workspaces: map[string]models.WorkspaceConfig{}}
	service := New(NewSettingsRepository(filepath.Join(dataDir, "ai-settings.yaml")))
	service.ConfigureLaunch(reg, &fakeItemIndex{items: map[string]models.ItemDetail{}}, nil, t.TempDir())
	service.ConfigureSessionRecords(records, &fakeChangeGit{})
	result, err := service.SessionChanges(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if result.Availability != SessionChangesUnavailable || result.Reason != ReasonWorkspaceMissing {
		t.Fatalf("availability=%q reason=%q", result.Availability, result.Reason)
	}
}

func TestSessionChangesPropagatesNonUnknownCommitFailure(t *testing.T) {
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "Local", Path: t.TempDir()}
	infraErr := errors.New("git cat-file -e: exec: \"git\": executable file not found in $PATH")
	git := &fakeChangeGit{commitExistsErr: infraErr}
	service, _ := changesTestService(t, workspace, baseSessionRecord(workspace.ID, "deadbeef"), git)
	_, err := service.SessionChanges(context.Background(), "session-1")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, infraErr) {
		t.Fatalf("err = %v, want it to wrap infraErr", err)
	}
}

func TestSessionChangesAvailableComputesFullValue(t *testing.T) {
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "Local", Path: t.TempDir()}
	git := &fakeChangeGit{
		currentBranch:  "main",
		resolvedCommit: "head-sha",
		nameStatus: []gitadapter.NameStatusEntry{
			{Path: "a.go", Status: "M"},
			{Path: "b.go", Status: "A"},
		},
		additions:    5,
		deletions:    2,
		revListCount: 3,
	}
	service, _ := changesTestService(t, workspace, baseSessionRecord(workspace.ID, "launch-sha"), git)
	result, err := service.SessionChanges(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if result.Availability != SessionChangesAvailable || result.Reason != "" {
		t.Fatalf("availability=%q reason=%q", result.Availability, result.Reason)
	}
	if result.LaunchCommit != "launch-sha" || result.HeadCommit != "head-sha" || result.HeadMoved != 3 {
		t.Fatalf("result=%#v", result)
	}
	if len(result.Files) != 2 || result.Additions != 5 || result.Deletions != 2 || result.Truncated {
		t.Fatalf("result=%#v", result)
	}
}

func TestSessionChangesAvailableEmptyResultMarshalsFieldsPresent(t *testing.T) {
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "Local", Path: t.TempDir()}
	git := &fakeChangeGit{
		currentBranch:  "main",
		resolvedCommit: "launch-sha",
		nameStatus:     nil,
		additions:      0,
		deletions:      0,
		revListCount:   0,
	}
	service, _ := changesTestService(t, workspace, baseSessionRecord(workspace.ID, "launch-sha"), git)
	result, err := service.SessionChanges(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if result.Availability != SessionChangesAvailable {
		t.Fatalf("availability=%q reason=%q", result.Availability, result.Reason)
	}
	if result.Files == nil {
		t.Fatalf("files should be an empty slice, not nil")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal err=%v", err)
	}
	body := string(encoded)
	for _, field := range []string{`"files":[]`, `"additions":0`, `"deletions":0`, `"headMoved":0`} {
		if !strings.Contains(body, field) {
			t.Fatalf("expected %s in %s", field, body)
		}
	}
}

func TestSessionChangesReturnsErrNotFoundForMissingSession(t *testing.T) {
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "Local", Path: t.TempDir()}
	service, _ := changesTestService(t, workspace, baseSessionRecord(workspace.ID, "launch-sha"), &fakeChangeGit{})
	if _, err := service.SessionChanges(context.Background(), "missing-session"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestSessionFileDiffReturnsChangesErrorWhenUnavailable(t *testing.T) {
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "Local", Path: t.TempDir()}
	service, _ := changesTestService(t, workspace, baseSessionRecord(workspace.ID, ""), &fakeChangeGit{})
	_, err := service.SessionFileDiff(context.Background(), "session-1", "a.go", 1<<16)
	var changesErr *SessionChangesError
	if !errors.As(err, &changesErr) || changesErr.Availability != SessionChangesUnavailable || changesErr.Reason != ReasonLaunchCommitUnknown {
		t.Fatalf("err=%v", err)
	}
}

func TestSessionFileDiffReturnsDiffWhenAvailable(t *testing.T) {
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "Local", Path: t.TempDir()}
	git := &fakeChangeGit{currentBranch: "main", resolvedCommit: "head-sha", diff: "--- a\n+++ b\n", diffTruncated: true}
	service, _ := changesTestService(t, workspace, baseSessionRecord(workspace.ID, "launch-sha"), git)
	result, err := service.SessionFileDiff(context.Background(), "session-1", "a.go", 1<<16)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if result.Path != "a.go" || result.Diff != "--- a\n+++ b\n" || !result.Truncated {
		t.Fatalf("result=%#v", result)
	}
}

func TestSessionRecordWorkspaceResolvesOrReportsNotFound(t *testing.T) {
	workspace := models.WorkspaceConfig{ID: "ws-1", Name: "Local", Path: t.TempDir()}
	service, _ := changesTestService(t, workspace, baseSessionRecord(workspace.ID, "launch-sha"), &fakeChangeGit{})
	workspaceID, err := service.SessionRecordWorkspace("session-1")
	if err != nil || workspaceID != workspace.ID {
		t.Fatalf("workspaceID=%q err=%v", workspaceID, err)
	}
	if _, err := service.SessionRecordWorkspace("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestSessionChangesUsesRealGitAdapter(t *testing.T) {
	root := t.TempDir()
	gitCommand(t, root, "init", "-b", "main")
	gitCommand(t, root, "config", "user.email", "test@example.com")
	gitCommand(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("seed"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", ".")
	gitCommand(t, root, "commit", "-m", "seed")
	launchCommit := currentGitCommit(t, root)
	if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "seed.txt")
	gitCommand(t, root, "commit", "-m", "modify")

	workspace := models.WorkspaceConfig{ID: "ws-real", Name: "Real", Path: root}
	service, _ := changesTestServiceWithRealGit(t, workspace, baseSessionRecord(workspace.ID, launchCommit))
	result, err := service.SessionChanges(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if result.Availability != SessionChangesAvailable {
		t.Fatalf("availability=%q reason=%q", result.Availability, result.Reason)
	}
	if result.LaunchCommit != launchCommit || result.HeadCommit == "" || result.HeadMoved != 1 {
		t.Fatalf("result=%#v", result)
	}
	found := map[string]bool{}
	for _, file := range result.Files {
		found[file.Path] = true
	}
	if !found["seed.txt"] || !found["new.txt"] {
		t.Fatalf("files=%#v", result.Files)
	}
}

func changesTestServiceWithRealGit(t *testing.T, workspace models.WorkspaceConfig, record SessionRecord) (*Service, *FileSessionRecordRepository) {
	t.Helper()
	dataDir := t.TempDir()
	records := NewFileSessionRecordRepository(filepath.Join(dataDir, "records.yaml"))
	if _, err := records.Upsert(record); err != nil {
		t.Fatal(err)
	}
	reg := &fakeWorkspaceRegistry{workspaces: map[string]models.WorkspaceConfig{workspace.ID: workspace}}
	service := New(NewSettingsRepository(filepath.Join(dataDir, "ai-settings.yaml")))
	service.ConfigureLaunch(reg, &fakeItemIndex{items: map[string]models.ItemDetail{}}, nil, t.TempDir())
	service.ConfigureSessionRecords(records, gitadapter.New())
	return service, records
}

func currentGitCommit(t *testing.T, root string) string {
	t.Helper()
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
}

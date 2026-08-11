package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	appgit "kode-stream/internal/git"
	"kode-stream/internal/provider"
	"kode-stream/internal/storage"
	"kode-stream/internal/system"
)

type cloudPersistenceDouble struct {
	mu          sync.Mutex
	errByOwner  map[string]error
	workspaces  map[string]map[string]models.WorkspaceConfig
	agents      map[string]map[string]models.CloudAgent
	instances   map[string]provider.Instance
	connections map[string]string
}

func TestCloudPostgresPersistenceAcrossAPIReconstruction(t *testing.T) {
	databaseURL := os.Getenv(storage.EnvDatabaseURL)
	if databaseURL == "" {
		t.Skip("set KODE_STREAM_DATABASE_URL to run Cloud Postgres reconstruction contract")
	}
	admin, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("kode_stream_api_contract_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA "` + schema + `"`); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`)
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	dir := t.TempDir()
	paths := system.Paths{Dir: dir, RegistryFile: filepath.Join(dir, "workspaces.yaml"), PlanIndexFile: filepath.Join(dir, "item-index.yaml"), SQLiteDatabaseFile: filepath.Join(dir, "state.db"), KnowledgeIndexFile: filepath.Join(dir, "knowledge.yaml"), AuditLogFile: filepath.Join(dir, "audit.jsonl"), SavedFiltersFile: filepath.Join(dir, "filters.yaml"), RecentItemsFile: filepath.Join(dir, "recents.yaml"), AISettingsFile: filepath.Join(dir, "ai.yaml"), CanvasFile: filepath.Join(dir, "canvas.yaml"), AISessionRecordsFile: filepath.Join(dir, "sessions.yaml")}
	runtime := system.RuntimeConfig{Mode: models.RuntimeModeCloud, AuthMode: "app_oidc", CookieSecret: "shared-secret"}
	state, err := storage.OpenAppOwnedState(paths, runtime, appgit.New(), func(key string) string {
		switch key {
		case storage.EnvStorageDriver:
			return storage.StorageDriverPostgres
		case storage.EnvDatabaseURL:
			return parsed.String()
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer state.SQLStore.Close()
	first := New(Dependencies{RuntimeConfig: runtime, CloudPersistence: state.Cloud})
	owner := stableCloudUserID("postgres-owner")
	workspace := models.WorkspaceConfig{ID: "postgres-workspace", Name: "Durable", OwnerUserID: owner, Sources: []string{}}
	if _, err := first.cloud.workspaces.Upsert(context.Background(), workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := first.cloud.agents.Upsert(context.Background(), models.CloudAgent{ID: "agent", UserID: owner, Status: "connected"}); err != nil {
		t.Fatal(err)
	}
	if err := first.cloud.providers.registry.Upsert(provider.Instance{ID: "github", Name: "GitHub", Kind: "github", BaseURL: "https://example.invalid"}); err != nil {
		t.Fatal(err)
	}
	if err := first.cloud.providers.connections.Save(owner, "github", "postgres-secret"); err != nil {
		t.Fatal(err)
	}

	second := New(Dependencies{RuntimeConfig: runtime, CloudPersistence: state.Cloud})
	if workspaces, err := second.cloud.workspaces.List(context.Background(), owner); err != nil || len(workspaces) != 1 {
		t.Fatalf("workspaces=%#v err=%v", workspaces, err)
	}
	if agents, err := second.cloud.agents.List(context.Background(), owner); err != nil || len(agents) != 1 || agents[0].Status != "offline" {
		t.Fatalf("agents=%#v err=%v", agents, err)
	}
	if token, ok, err := second.cloud.providers.connections.Token(context.Background(), owner, "github"); err != nil || !ok || token != "postgres-secret" {
		t.Fatalf("token=%q ok=%v err=%v", token, ok, err)
	}
}

func newCloudPersistenceDouble() *cloudPersistenceDouble {
	return &cloudPersistenceDouble{errByOwner: map[string]error{}, workspaces: map[string]map[string]models.WorkspaceConfig{}, agents: map[string]map[string]models.CloudAgent{}, instances: map[string]provider.Instance{}, connections: map[string]string{}}
}
func (f *cloudPersistenceDouble) ownerError(owner string) error { return f.errByOwner[owner] }
func (f *cloudPersistenceDouble) ListWorkspaces(_ context.Context, owner string) ([]models.WorkspaceConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ownerError(owner); err != nil {
		return nil, err
	}
	result := []models.WorkspaceConfig{}
	for _, value := range f.workspaces[owner] {
		result = append(result, value)
	}
	return result, nil
}
func (f *cloudPersistenceDouble) GetWorkspace(_ context.Context, owner, id string) (models.WorkspaceConfig, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ownerError(owner); err != nil {
		return models.WorkspaceConfig{}, false, err
	}
	value, ok := f.workspaces[owner][id]
	return value, ok, nil
}
func (f *cloudPersistenceDouble) UpsertWorkspace(_ context.Context, owner string, value models.WorkspaceConfig) (models.WorkspaceConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ownerError(owner); err != nil {
		return models.WorkspaceConfig{}, err
	}
	if f.workspaces[owner] == nil {
		f.workspaces[owner] = map[string]models.WorkspaceConfig{}
	}
	f.workspaces[owner][value.ID] = value
	return value, nil
}
func (f *cloudPersistenceDouble) ListAgents(_ context.Context, owner string) ([]models.CloudAgent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ownerError(owner); err != nil {
		return nil, err
	}
	result := []models.CloudAgent{}
	for _, value := range f.agents[owner] {
		result = append(result, value)
	}
	return result, nil
}
func (f *cloudPersistenceDouble) UpsertAgent(_ context.Context, owner string, value models.CloudAgent) (models.CloudAgent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ownerError(owner); err != nil {
		return models.CloudAgent{}, err
	}
	if f.agents[owner] == nil {
		f.agents[owner] = map[string]models.CloudAgent{}
	}
	f.agents[owner][value.ID] = value
	return value, nil
}
func (f *cloudPersistenceDouble) GetProviderInstance(_ context.Context, _, id string) (provider.Instance, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.instances[id]
	return value, ok, nil
}
func (f *cloudPersistenceDouble) UpsertProviderInstance(_ context.Context, owner string, value provider.Instance) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ownerError(owner); err != nil {
		return err
	}
	f.instances[value.ID] = value
	return nil
}
func (f *cloudPersistenceDouble) GetEncryptedConnection(_ context.Context, owner, id string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ownerError(owner); err != nil {
		return "", false, err
	}
	value, ok := f.connections[owner+"\x00"+id]
	return value, ok, nil
}
func (f *cloudPersistenceDouble) SaveEncryptedConnection(_ context.Context, owner, id, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ownerError(owner); err != nil {
		return err
	}
	f.connections[owner+"\x00"+id] = value
	return nil
}
func (f *cloudPersistenceDouble) RevokeConnection(_ context.Context, owner, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ownerError(owner); err != nil {
		return err
	}
	delete(f.connections, owner+"\x00"+id)
	return nil
}

func TestCloudPersistenceErrorsAreCallScopedAndDoNotPublishConnectedAgent(t *testing.T) {
	persistence := newCloudPersistenceDouble()
	persistence.errByOwner["broken"] = errors.New("database unavailable")
	workspaces := newCloudWorkspaceStore(persistence)
	if _, err := workspaces.List(context.Background(), "broken"); err == nil {
		t.Fatal("workspace error was discarded")
	}
	if values, err := workspaces.List(context.Background(), "healthy"); err != nil || len(values) != 0 {
		t.Fatalf("independent request consumed another request's error: %#v %v", values, err)
	}
	agents := newCloudAgentStore(time.Now, persistence)
	if _, err := agents.Upsert(context.Background(), models.CloudAgent{ID: "agent", UserID: "broken", Status: "connected"}); err == nil {
		t.Fatal("agent durability error was discarded")
	}
	if agents.HasConnected("broken", "agent") {
		t.Fatal("failed durable upsert published a locally connected agent")
	}
	connections := newCloudProviderStore(persistence, "test-secret").connections
	if err := connections.Revoke(context.Background(), "broken", "github"); err == nil {
		t.Fatal("provider revoke error was discarded")
	}
	persistence.errByOwner[cloudProviderSystemOwner] = errors.New("provider database unavailable")
	providers := newCloudProviderStore(persistence, "test-secret")
	if err := providers.registry.Upsert(provider.Instance{ID: "github", Name: "GitHub", Kind: "github", BaseURL: "https://example.invalid"}); err == nil {
		t.Fatal("provider instance persistence error was discarded")
	}
	if _, err := providers.registry.memory.Integration("github", "token"); err == nil {
		t.Fatal("failed durable provider write leaked into process-local registry")
	}
}

func TestCloudPersistenceSurvivesAPIReconstructionAndIsolatesHTTPReads(t *testing.T) {
	persistence := newCloudPersistenceDouble()
	runtime := system.RuntimeConfig{Mode: models.RuntimeModeCloud, AuthMode: "app_oidc", CookieSecret: "shared-secret"}
	first := New(Dependencies{RuntimeConfig: runtime, CloudPersistence: persistence})
	ownerA := stableCloudUserID("owner-a")
	workspace := models.WorkspaceConfig{ID: "workspace", Name: "Durable", OwnerUserID: ownerA, Sources: []string{}}
	if _, err := first.cloud.workspaces.Upsert(context.Background(), workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := first.cloud.agents.Upsert(context.Background(), models.CloudAgent{ID: "agent", UserID: ownerA, Status: "connected"}); err != nil {
		t.Fatal(err)
	}
	if err := first.cloud.providers.registry.Upsert(provider.Instance{ID: "github", Name: "GitHub", Kind: "github", BaseURL: "https://example.invalid"}); err != nil {
		t.Fatal(err)
	}
	if err := first.cloud.providers.connections.Save(ownerA, "github", "plain-token"); err != nil {
		t.Fatal(err)
	}
	if persisted := persistence.connections[ownerA+"\x00github"]; persisted == "" || persisted == "plain-token" {
		t.Fatalf("credential was not encrypted: %q", persisted)
	}

	second := New(Dependencies{RuntimeConfig: runtime, CloudPersistence: persistence})
	for _, subject := range []struct {
		value string
		want  int
	}{{"owner-a", http.StatusOK}, {"owner-b", http.StatusOK}} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/workspaces", nil)
		request.Header.Set("X-Kode-Stream-Subject", subject.value)
		request.Header.Set("X-Kode-Stream-Role", "viewer")
		second.Routes().ServeHTTP(response, request)
		if response.Code != subject.want {
			t.Fatalf("%s status=%d body=%s", subject.value, response.Code, response.Body.String())
		}
		containsWorkspace := response.Body.String() != "[]\n"
		if containsWorkspace != (subject.value == "owner-a") {
			t.Fatalf("%s isolation body=%s", subject.value, response.Body.String())
		}
	}
	listed, err := second.cloud.agents.List(context.Background(), ownerA)
	if err != nil || len(listed) != 1 || listed[0].Status != "offline" {
		t.Fatalf("reconciled agents=%#v err=%v", listed, err)
	}
	token, ok, err := second.cloud.providers.connections.Token(context.Background(), ownerA, "github")
	if err != nil || !ok || token != "plain-token" {
		t.Fatalf("credential restart token=%q ok=%v err=%v", token, ok, err)
	}
}

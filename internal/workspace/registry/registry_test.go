package registry

// Package registry persists registered Workspace definitions.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
)

func TestCreateDefaultsRegistrationModeToLocalPath(t *testing.T) {
	root := newRegistryGitRepo(t)
	registry := New(filepath.Join(t.TempDir(), "workspaces.yaml"), gitadapter.New())

	workspace, err := registry.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	if workspace.RegistrationMode != models.WorkspaceRegistrationModeLocalPath {
		t.Fatalf("registration mode = %q", workspace.RegistrationMode)
	}
	if workspace.Location != models.WorkspaceLocationLocalPath {
		t.Fatalf("location = %q", workspace.Location)
	}
	if workspace.RemoteURL != "" || workspace.ClonePathManaged {
		t.Fatalf("expected local workspace metadata, got %+v", workspace)
	}
}

func TestKnowledgeSettingsRoundTripAndSurviveUnrelatedUpdate(t *testing.T) {
	root := newRegistryGitRepo(t)
	registry := New(filepath.Join(t.TempDir(), "workspaces.yaml"), gitadapter.New())
	disabled := false
	created, err := registry.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}, Knowledge: &models.KnowledgeSettings{Enabled: &disabled, EnrichExecutable: " tool ", EnrichArgs: []string{"--sync"}}})
	if err != nil {
		t.Fatal(err)
	}
	if created.Knowledge == nil || created.Knowledge.EnrichExecutable != "tool" || *created.Knowledge.Enabled {
		t.Fatalf("knowledge = %#v", created.Knowledge)
	}
	updated, err := registry.Update(created.ID, models.WorkspaceInput{Name: "Renamed", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Knowledge == nil || updated.Knowledge.EnrichExecutable != "tool" {
		t.Fatalf("knowledge lost = %#v", updated.Knowledge)
	}
}

func TestValidateJiraConnectionNormalizesCloudAndServer(t *testing.T) {
	cloud, err := ValidateJiraConnection(&models.JiraConnection{DeploymentType: " CLOUD ", BaseURL: "https://jira.example.com/", ProjectKey: "di", AccountEmail: " user@example.com ", TokenEnvVar: "JIRA_TOKEN"})
	if err != nil {
		t.Fatal(err)
	}
	if cloud.DeploymentType != "cloud" || cloud.BaseURL != "https://jira.example.com" || cloud.ProjectKey != "DI" || cloud.AccountEmail != "user@example.com" {
		t.Fatalf("cloud = %#v", cloud)
	}
	server, err := ValidateJiraConnection(&models.JiraConnection{DeploymentType: "server", BaseURL: "http://127.0.0.1:8080", ProjectKey: "OPS", AccountEmail: "ignored", TokenEnvVar: "JIRA_PAT"})
	if err != nil || server.AccountEmail != "" {
		t.Fatalf("server=%#v err=%v", server, err)
	}
}

func TestValidateJiraConnectionRejectsUnsafeOrIncompleteValues(t *testing.T) {
	tests := []*models.JiraConnection{
		{DeploymentType: "other", BaseURL: "https://jira.example.com", ProjectKey: "DI", TokenEnvVar: "JIRA_TOKEN"},
		{DeploymentType: "cloud", BaseURL: "http://jira.example.com", ProjectKey: "DI", AccountEmail: "a@b.com", TokenEnvVar: "JIRA_TOKEN"},
		{DeploymentType: "cloud", BaseURL: "https://user@jira.example.com", ProjectKey: "DI", AccountEmail: "a@b.com", TokenEnvVar: "JIRA_TOKEN"},
		{DeploymentType: "cloud", BaseURL: "https://jira.example.com", ProjectKey: "bad-key", AccountEmail: "a@b.com", TokenEnvVar: "JIRA_TOKEN"},
		{DeploymentType: "cloud", BaseURL: "https://jira.example.com", ProjectKey: "DI", TokenEnvVar: "JIRA_TOKEN"},
		{DeploymentType: "server", BaseURL: "https://jira.example.com", ProjectKey: "DI", TokenEnvVar: "not valid"},
	}
	for _, connection := range tests {
		if _, err := ValidateJiraConnection(connection); err == nil {
			t.Fatalf("expected rejection: %#v", connection)
		}
	}
}

func TestCreateRemoteCloneRequiresRemoteURL(t *testing.T) {
	root := newRegistryGitRepo(t)
	registry := New(filepath.Join(t.TempDir(), "workspaces.yaml"), gitadapter.New())

	_, err := registry.Create(models.WorkspaceInput{
		Name:             "Remote Workspace",
		Path:             root,
		RegistrationMode: models.WorkspaceRegistrationModeRemoteClone,
		BaselineBranch:   "main",
		Sources:          []string{"plans"},
	})
	if err == nil || !strings.Contains(err.Error(), "remote URL") {
		t.Fatalf("err = %v", err)
	}
}

func TestBatchCreateWritesAcceptedWorkspacesAtomicallyWithPrivateMode(t *testing.T) {
	first := newRegistryGitRepo(t)
	second := newRegistryGitRepo(t)
	path := filepath.Join(t.TempDir(), "workspaces.yaml")
	registry := New(path, gitadapter.New())
	results, err := registry.BatchCreate([]models.WorkspaceInput{
		{Name: "First", Path: first, BaselineBranch: "main", Sources: []string{"plans"}, RegistrationMode: models.WorkspaceRegistrationModeExisting},
		{Name: "Duplicate", Path: first, BaselineBranch: "main", Sources: []string{"plans"}, RegistrationMode: models.WorkspaceRegistrationModeExisting},
		{Name: "Second", Path: second, BaselineBranch: "main", Sources: []string{"plans"}, RegistrationMode: models.WorkspaceRegistrationModeExisting},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || results[0].Err != nil || results[1].Err == nil || results[2].Err != nil {
		t.Fatalf("results = %+v", results)
	}
	listed, err := registry.List()
	if err != nil || len(listed) != 2 {
		t.Fatalf("listed=%+v err=%v", listed, err)
	}
	for _, workspace := range listed {
		if workspace.RegistrationMode != models.WorkspaceRegistrationModeExisting || workspace.ClonePathManaged {
			t.Fatalf("workspace = %+v", workspace)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("registry mode = %o", info.Mode().Perm())
	}
}

func TestBatchCreatePersistenceFailureDoesNotMutateLoadedRecords(t *testing.T) {
	root := newRegistryGitRepo(t)
	path := filepath.Join(t.TempDir(), "workspaces.yaml")
	registry := New(path, gitadapter.New())
	if listed, err := registry.List(); err != nil || len(listed) != 0 {
		t.Fatalf("initial list=%+v err=%v", listed, err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.BatchCreate([]models.WorkspaceInput{{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}}}); err == nil {
		t.Fatal("expected atomic replacement failure")
	}
	listed, err := registry.List()
	if err != nil || len(listed) != 0 {
		t.Fatalf("failed batch mutated records: list=%+v err=%v", listed, err)
	}
}

func TestConcurrentBatchCreateRechecksDuplicateUnderLock(t *testing.T) {
	root := newRegistryGitRepo(t)
	registry := New(filepath.Join(t.TempDir(), "workspaces.yaml"), gitadapter.New())
	input := []models.WorkspaceInput{{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}}}
	start := make(chan struct{})
	results := make(chan []BatchCreateResult, 2)
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			outcome, err := registry.BatchCreate(input)
			results <- outcome
			errors <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	created := 0
	conflicted := 0
	for outcome := range results {
		if outcome[0].Err == nil {
			created++
		} else {
			conflicted++
		}
	}
	listed, err := registry.List()
	if err != nil || created != 1 || conflicted != 1 || len(listed) != 1 {
		t.Fatalf("created=%d conflicted=%d listed=%+v err=%v", created, conflicted, listed, err)
	}
}

func newRegistryGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", root, "add", ".").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	commit := exec.Command("git", "-C", root, "commit", "--allow-empty", "-m", "init")
	commit.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	return root
}

package registry

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"plan-manager/internal/gitadapter"
	"plan-manager/internal/models"
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

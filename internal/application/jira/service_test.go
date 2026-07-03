package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"plan-manager/internal/gitadapter"
	"plan-manager/internal/itemindex"
	jiraclient "plan-manager/internal/jira"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
)

func TestIssueMatchesCachesAndRefreshesExactKey(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"key":"DI-170","fields":{"summary":"Feature","description":"Text","status":{"name":"Open"},"issuetype":{"name":"Story"}}}`))
	}))
	defer server.Close()
	service, item := jiraTestService(t, server.URL, "DI-170")
	first, err := service.Issue(context.Background(), item.ID, false)
	if err != nil || first.State != "available" || first.Issue.Summary != "Feature" {
		t.Fatalf("state=%#v err=%v", first, err)
	}
	_, _ = service.Issue(context.Background(), item.ID, false)
	if requests != 1 {
		t.Fatalf("cached requests=%d", requests)
	}
	_, _ = service.Issue(context.Background(), item.ID, true)
	if requests != 2 {
		t.Fatalf("refresh requests=%d", requests)
	}
}

func TestIssueReturnsTypedStatesWithoutUnrelatedLookup(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer server.Close()
	service, item := jiraTestService(t, server.URL, "OTHER-1")
	state, err := service.Issue(context.Background(), item.ID, false)
	if err != nil || state.State != "project_mismatch" || requests != 0 {
		t.Fatalf("state=%#v requests=%d err=%v", state, requests, err)
	}
}

func TestAttachmentRejectsIDOutsideMatchedIssue(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"key":"DI-170","fields":{"summary":"Feature","attachment":[{"id":"1","filename":"a.txt","content":"` + server.URL + `/file"}]}}`))
	}))
	defer server.Close()
	service, item := jiraTestService(t, server.URL, "DI-170")
	_, err := service.Attachment(context.Background(), item.ID, "2")
	if err == nil {
		t.Fatal("expected ownership rejection")
	}
}

func jiraTestService(t *testing.T, baseURL, identifier string) (*Service, models.ItemDetail) {
	t.Helper()
	t.Setenv("JIRA_TEST_TOKEN", "secret")
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	commit := exec.Command("git", "-C", root, "commit", "--allow-empty", "-m", "init")
	commit.Env = append(os.Environ(), "GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@e")
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v %s", err, out)
	}
	dir := t.TempDir()
	reg := registry.New(filepath.Join(dir, "workspaces.yaml"), gitadapter.New())
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Test", Path: root, BaselineBranch: "main", Sources: []string{"plans"}, Jira: &models.JiraConnection{DeploymentType: "server", BaseURL: baseURL, ProjectKey: "DI", TokenEnvVar: "JIRA_TEST_TOKEN"}})
	if err != nil {
		t.Fatal(err)
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "item", WorkspaceID: workspace.ID, Identifier: identifier}}
	index := itemindex.New(filepath.Join(dir, "index.yaml"))
	if err := index.ReplaceWorkspace(workspace.ID, []models.ItemDetail{item}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	return New(reg, index, jiraclient.New()), item
}

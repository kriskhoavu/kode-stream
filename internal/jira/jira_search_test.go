package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kode-stream/internal/common/models"
)

func TestSearchIssuesUsesTheDeploymentsSearchEndpoint(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Query().Get("jql") != "project = PM" || r.URL.Query().Get("maxResults") != "5" || !strings.Contains(r.URL.Query().Get("fields"), "issuelinks") {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"issues":[{"key":"PM-2","fields":{"summary":"Add harness","description":"Plain text","status":{"name":"To Do","statusCategory":{"key":"new"}},"issuetype":{"name":"Story"},"assignee":{"displayName":"Anna"},"priority":{"name":"High"},"labels":["bug"],"updated":"2026-11-01T10:00:00.000+0000","fixVersions":[{"name":"M1"}],"parent":{"key":"PM-1"},"timeoriginalestimate":3600,"issuelinks":[{"type":{"inward":"is blocked by"},"inwardIssue":{"key":"PM-3"}},{"type":{"inward":"relates to"},"inwardIssue":{"key":"PM-9"}}]}}]}`))
	}))
	defer server.Close()
	client := New()
	client.getenv = func(string) string { return "secret" }
	issues, err := client.SearchIssues(context.Background(), models.JiraConnection{DeploymentType: "server", BaseURL: server.URL, TokenEnvVar: "JIRA_TOKEN"}, "project = PM", 5)
	if err != nil || len(issues) != 1 {
		t.Fatalf("issues = %#v err=%v", issues, err)
	}
	issue := issues[0]
	if issue.Key != "PM-2" || issue.Priority != "High" || issue.Assignee != "Anna" || issue.Epic != "PM-1" || issue.OriginalEstimateSeconds != 3600 || len(issue.BlockedBy) != 1 || issue.BlockedBy[0] != "PM-3" || issue.FixVersions[0] != "M1" || issue.Description != "Plain text" {
		t.Fatalf("issue = %#v", issue)
	}
	if _, err := client.SearchIssues(context.Background(), models.JiraConnection{DeploymentType: "cloud", BaseURL: server.URL, AccountEmail: "a@example.com", TokenEnvVar: "JIRA_TOKEN"}, "project = PM", 5); err != nil {
		t.Fatal(err)
	}
	if paths[0] != "/rest/api/2/search" || paths[1] != "/rest/api/3/search/jql" {
		t.Fatalf("paths = %v", paths)
	}
	if _, err := client.SearchIssues(context.Background(), models.JiraConnection{BaseURL: server.URL, TokenEnvVar: "JIRA_TOKEN"}, " ", 5); err == nil {
		t.Fatal("an empty JQL was accepted")
	}
}

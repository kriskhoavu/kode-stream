package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/adapters/credential"
	"kode-stream/internal/harness/domain"
)

func TestGitHubScoutListsOpenIssuesAndSkipsPullRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/app/issues" || r.URL.Query().Get("labels") != "harness" || r.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("request = %s %s auth=%q", r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`[{"number":7,"title":"Fix login","body":"steps","html_url":"https://github.com/acme/app/issues/7","state":"open","updated_at":"2026-11-01T10:00:00Z","labels":[{"name":"harness"},{"name":"priority: high"}],"assignee":{"login":"anna"},"milestone":{"title":"M1"}},{"number":8,"title":"A PR","state":"open","pull_request":{"url":"x"}}]`))
	}))
	defer server.Close()
	scout := &Scout{BaseURL: server.URL, HTTPClient: server.Client(), Tokens: credential.Resolver{Getenv: func(string) string { return "tok" }}}
	settings := domain.WorkspaceSettings{WorkspaceID: "ws", GitHub: &domain.GitHubSourceSettings{Enabled: true, Repo: "acme/app", Labels: []string{"harness"}, TokenEnvVar: "GITHUB_TOKEN"}}
	candidates, err := scout.Poll(context.Background(), models.WorkspaceConfig{ID: "ws"}, settings)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates = %#v err=%v", candidates, err)
	}
	source := candidates[0].Source
	if source.Key != "acme/app#7" || source.Priority != "High" || source.Assignee != "anna" || source.Milestone != "M1" || source.UpdatedAt == nil {
		t.Fatalf("source = %#v", source)
	}
}

func TestGitHubScoutReportsUpstreamFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer server.Close()
	scout := &Scout{BaseURL: server.URL, HTTPClient: server.Client()}
	settings := domain.WorkspaceSettings{GitHub: &domain.GitHubSourceSettings{Enabled: true, Repo: "acme/app"}}
	if _, err := scout.Poll(context.Background(), models.WorkspaceConfig{ID: "ws"}, settings); err == nil {
		t.Fatal("a 401 was accepted")
	}
}

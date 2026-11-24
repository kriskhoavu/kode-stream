package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/adapters/credential"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

func TestGitHubAdapterOpensADraftThroughRESTOrTheCLI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Path != "/repos/acme/app/pulls" || body["draft"] != true || body["head"] != "harness/PM-1-abc" || body["base"] != "main" || r.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("request = %s %#v", r.URL.Path, body)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"html_url":"https://github.com/acme/app/pull/9"}`))
	}))
	defer server.Close()
	workspace := models.WorkspaceConfig{ID: "ws", RemoteURL: "git@github.com:acme/app.git"}
	adapter := &Adapter{BaseURL: server.URL, HTTPClient: server.Client(), Tokens: credential.Resolver{Getenv: func(string) string { return "tok" }}}
	if !adapter.Supports(workspace, domain.WorkspaceSettings{}) || Repo(models.WorkspaceConfig{RemoteURL: "https://github.com/acme/app"}, domain.WorkspaceSettings{}) != "acme/app" {
		t.Fatal("remote parsing is wrong")
	}
	request := ports.PullRequest{Workspace: workspace, Branch: "harness/PM-1-abc", Base: "main", Title: "PM-1: Add", Body: "body"}
	url, err := adapter.CreateDraft(context.Background(), request)
	if err != nil || url != "https://github.com/acme/app/pull/9" {
		t.Fatalf("url = %q err=%v", url, err)
	}
	var args []string
	cli := &Adapter{Tokens: credential.Resolver{Getenv: func(string) string { return "" }}, GH: func(_ context.Context, _ string, given ...string) (string, error) {
		args = given
		return "Creating draft pull request\nhttps://github.com/acme/app/pull/10\n", nil
	}}
	url, err = cli.CreateDraft(context.Background(), request)
	if err != nil || url != "https://github.com/acme/app/pull/10" || args[2] != "--draft" {
		t.Fatalf("cli url = %q args=%v err=%v", url, args, err)
	}
}

func TestGitHubRepoFallsBackToTheCheckoutOrigin(t *testing.T) {
	adapter := &Adapter{RemoteURL: func(path string) (string, error) { return "ssh://git@github.com/acme/app.git", nil }}
	byPath := models.WorkspaceConfig{ID: "ws", Path: "/tmp/app"}
	if !adapter.Supports(byPath, domain.WorkspaceSettings{}) || adapter.repoOrOrigin(byPath, domain.WorkspaceSettings{}) != "acme/app" {
		t.Fatal("the checkout origin must name the repository")
	}
	if (&Adapter{}).Supports(byPath, domain.WorkspaceSettings{}) {
		t.Fatal("without a reader, a path alone names nothing")
	}
	other := &Adapter{RemoteURL: func(string) (string, error) { return "ssh://git@bitbucket.example.com:7999/proj/app.git", nil }}
	if other.Supports(byPath, domain.WorkspaceSettings{}) {
		t.Fatal("a Bitbucket origin is not a GitHub repository")
	}
}

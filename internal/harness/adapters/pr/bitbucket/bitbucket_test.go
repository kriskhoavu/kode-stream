package bitbucket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/adapters/credential"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

func TestBitbucketAdapterOpensCloudDraftsAndServerDraftTitles(t *testing.T) {
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		w.WriteHeader(http.StatusCreated)
		if strings.HasPrefix(r.URL.Path, "/rest/api/1.0/projects/PROJ/repos/app/pull-requests") {
			_, _ = w.Write([]byte(`{"links":{"self":[{"href":"https://bb.example/projects/PROJ/repos/app/pull-requests/3"}]}}`))
			return
		}
		if r.URL.Path != "/repositories/team/app/pullrequests" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"links":{"html":{"href":"https://bitbucket.org/team/app/pull-requests/2"}}}`))
	}))
	defer server.Close()
	adapter := &Adapter{CloudURL: server.URL, HTTPClient: server.Client(), Tokens: credential.Resolver{Getenv: func(string) string { return "tok" }}}
	cloud := domain.WorkspaceSettings{Bitbucket: &domain.BitbucketSourceSettings{Workspace: "team", Repo: "app", Cloud: true, TokenEnvVar: "BB"}}
	request := ports.PullRequest{Workspace: models.WorkspaceConfig{ID: "ws"}, Settings: cloud, Branch: "harness/PM-1-abc", Base: "main", Title: "PM-1: Add"}
	if url, err := adapter.CreateDraft(context.Background(), request); err != nil || url != "https://bitbucket.org/team/app/pull-requests/2" || bodies[0]["draft"] != true {
		t.Fatalf("cloud url = %q err=%v body=%#v", url, err, bodies[0])
	}
	request.Settings = domain.WorkspaceSettings{Bitbucket: &domain.BitbucketSourceSettings{Workspace: "PROJ", Repo: "app", BaseURL: server.URL, TokenEnvVar: "BB"}}
	if !adapter.Supports(request.Workspace, request.Settings) {
		t.Fatal("server settings are not supported")
	}
	if url, err := adapter.CreateDraft(context.Background(), request); err != nil || !strings.HasSuffix(url, "/pull-requests/3") || bodies[1]["title"] != "[DRAFT] PM-1: Add" {
		t.Fatalf("server url = %q err=%v body=%#v", url, err, bodies[1])
	}
}

func TestBitbucketTargetComesFromTheOriginWhenSettingsNameNone(t *testing.T) {
	cases := []struct {
		remote    string
		workspace string
		repo      string
		baseURL   string
		cloud     bool
	}{
		{"ssh://git@bitbucket.example.com:7999/proj/app.git", "proj", "app", "https://bitbucket.example.com", false},
		{"https://user@bitbucket.example.com/scm/proj/app.git", "proj", "app", "https://bitbucket.example.com", false},
		{"git@bitbucket.org:team/app.git", "team", "app", "", true},
		{"https://bitbucket.org/team/app", "team", "app", "", true},
	}
	for _, c := range cases {
		target, ok := ParseRemote(c.remote)
		if !ok || target.Workspace != c.workspace || target.Repo != c.repo || target.BaseURL != c.baseURL || target.Cloud != c.cloud || target.TokenEnvVar != "BITBUCKET_TOKEN" {
			t.Fatalf("%s -> %#v ok=%v", c.remote, target, ok)
		}
	}
	for _, remote := range []string{"", "git@github.com:acme/app.git", "ssh://git@bitbucket.example.com/onlyproject", "https://example.com/proj/app.git"} {
		if _, ok := ParseRemote(remote); ok {
			t.Fatalf("%q must not parse as Bitbucket", remote)
		}
	}

	adapter := &Adapter{RemoteURL: func(path string) (string, error) { return "ssh://git@bitbucket.example.com:7999/proj/app.git", nil }}
	byPath := models.WorkspaceConfig{ID: "ws", Path: "/tmp/app"}
	target, ok := adapter.Target(byPath, domain.WorkspaceSettings{})
	if !ok || target.BaseURL != "https://bitbucket.example.com" || target.Workspace != "proj" || !adapter.Supports(byPath, domain.WorkspaceSettings{}) {
		t.Fatalf("origin read from the checkout: %#v ok=%v", target, ok)
	}
	target, _ = adapter.Target(byPath, domain.WorkspaceSettings{Bitbucket: &domain.BitbucketSourceSettings{TokenEnvVar: "BB_SERVER"}})
	if target.TokenEnvVar != "BB_SERVER" {
		t.Fatalf("a settings token name must win: %#v", target)
	}
	registered := models.WorkspaceConfig{ID: "ws", RemoteURL: "git@bitbucket.org:team/app.git"}
	if target, ok := (&Adapter{}).Target(registered, domain.WorkspaceSettings{}); !ok || !target.Cloud || target.Workspace != "team" {
		t.Fatalf("registered remote: %#v ok=%v", target, ok)
	}
	if (&Adapter{}).Supports(models.WorkspaceConfig{ID: "ws", RemoteURL: "git@github.com:acme/app.git"}, domain.WorkspaceSettings{}) {
		t.Fatal("a GitHub origin is not a Bitbucket target")
	}
}

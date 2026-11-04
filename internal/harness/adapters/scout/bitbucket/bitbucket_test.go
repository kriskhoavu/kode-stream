package bitbucket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/adapters/credential"
	"kode-stream/internal/harness/domain"
)

func TestBitbucketScoutListsOpenCloudIssues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/team/app/issues" || r.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("request = %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"values":[{"id":4,"title":"Crash","state":"open","priority":"major","kind":"bug","content":{"raw":"trace"},"updated_on":"2026-11-01T10:00:00.123456+00:00","links":{"html":{"href":"https://bitbucket.org/team/app/issues/4"}}},{"id":5,"title":"Done","state":"resolved"}]}`))
	}))
	defer server.Close()
	scout := &Scout{BaseURL: server.URL, HTTPClient: server.Client(), Tokens: credential.Resolver{Getenv: func(string) string { return "tok" }}}
	settings := domain.WorkspaceSettings{WorkspaceID: "ws", Bitbucket: &domain.BitbucketSourceSettings{Enabled: true, Cloud: true, Workspace: "team", Repo: "app", TokenEnvVar: "BITBUCKET_TOKEN"}}
	candidates, err := scout.Poll(context.Background(), models.WorkspaceConfig{ID: "ws"}, settings)
	if err != nil || len(candidates) != 1 || candidates[0].Source.Key != "team/app#4" || candidates[0].Source.Priority != "High" || candidates[0].Body != "trace" {
		t.Fatalf("candidates = %#v err=%v", candidates, err)
	}
	settings.Bitbucket.Cloud = false
	if _, err := scout.Poll(context.Background(), models.WorkspaceConfig{ID: "ws"}, settings); err == nil || !strings.Contains(err.Error(), "no issue tracker") {
		t.Fatalf("server err = %v", err)
	}
}

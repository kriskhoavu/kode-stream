package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubReadOnlySnapshotContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer read-token" {
			t.Fatalf("authorization = %q", got)
		}
		switch r.URL.Path {
		case "/user/repos":
			_, _ = w.Write([]byte(`[{"id":1,"name":"repo","full_name":"acme/repo","html_url":"https://github.example/acme/repo"}]`))
		case "/repos/acme/repo/branches":
			_, _ = w.Write([]byte(`[{"name":"main","commit":{"sha":"branch-sha"}}]`))
		case "/repos/acme/repo/tags":
			_, _ = w.Write([]byte(`[{"name":"v1","commit":{"sha":"tag-sha"}}]`))
		case "/repos/acme/repo/git/trees/branch-sha":
			_, _ = w.Write([]byte(`{"tree":[{"path":"plans/PM-034.md","type":"blob","sha":"file-sha","size":12},{"path":"README.md","type":"blob","sha":"readme-sha","size":5}]}`))
		case "/repos/acme/repo/contents/plans/PM-034.md":
			_, _ = w.Write([]byte(`{"path":"plans/PM-034.md","content":"c25hcHNob3Q=","encoding":"base64"}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	github := NewGitHub(server.URL, "read-token")
	repositories, err := github.Repositories(context.Background())
	if err != nil || len(repositories) != 1 || repositories[0].FullName != "acme/repo" {
		t.Fatalf("repositories = %#v, %v", repositories, err)
	}
	ref, err := github.ResolveRef(context.Background(), "acme/repo", "main")
	if err != nil || ref.CommitSHA != "branch-sha" {
		t.Fatalf("ref = %#v, %v", ref, err)
	}
	entries, err := github.Tree(context.Background(), "acme/repo", ref.CommitSHA, "plans")
	if err != nil || len(entries) != 1 || entries[0].Path != "plans/PM-034.md" {
		t.Fatalf("tree = %#v, %v", entries, err)
	}
	file, err := github.ReadFile(context.Background(), "acme/repo", ref.CommitSHA, "plans/PM-034.md")
	if err != nil || file.Content != "snapshot" {
		t.Fatalf("file = %#v, %v", file, err)
	}
}

func TestGitHubReportsForbiddenAndMissingRefs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repos/") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	github := NewGitHub(server.URL, "read-token")
	if _, err := github.ReadFile(context.Background(), "acme/private", "sha", "README.md"); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("forbidden read error = %v", err)
	}
	if _, err := github.ResolveRef(context.Background(), "acme/private", "missing"); err == nil {
		t.Fatal("expected missing ref error")
	}
}

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// encoding/json drops fields that share a tag. Assert values, not counts, to catch it.
func TestBitbucketRefsDecodeBranchAndTagNamesWithCommits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/branches"):
			_, _ = w.Write([]byte(`{"values":[{"id":"refs/heads/main","displayId":"main","latestCommit":"1111111111111111111111111111111111111111"}]}`))
		case strings.Contains(r.URL.Path, "/tags"):
			_, _ = w.Write([]byte(`{"values":[{"id":"refs/tags/v2.0.0","displayId":"v2.0.0","latestCommit":"2222222222222222222222222222222222222222"}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	refs, err := NewBitbucketServer(server.URL, "token").Refs(context.Background(), "ACME/repo")
	if err != nil {
		t.Fatal(err)
	}
	want := []Ref{
		{Name: "main", CommitSHA: "1111111111111111111111111111111111111111"},
		{Name: "v2.0.0", CommitSHA: "2222222222222222222222222222222222222222"},
	}
	if len(refs) != len(want) {
		t.Fatalf("refs=%#v", refs)
	}
	for index, expected := range want {
		if refs[index] != expected {
			t.Errorf("refs[%d]=%#v, want %#v", index, refs[index], expected)
		}
	}
}

func TestBitbucketResolveRefFindsABranchByName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/branches") {
			_, _ = w.Write([]byte(`{"values":[{"displayId":"main","latestCommit":"3333333333333333333333333333333333333333"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"values":[]}`))
	}))
	defer server.Close()

	ref, err := NewBitbucketServer(server.URL, "token").ResolveRef(context.Background(), "ACME/repo", "main")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Name != "main" || ref.CommitSHA != "3333333333333333333333333333333333333333" {
		t.Fatalf("ref=%#v", ref)
	}
}

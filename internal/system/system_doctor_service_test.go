package system

import "testing"

func TestParseGitVersion(t *testing.T) {
	major, minor := parseGitVersion("git version 2.45.1")
	if major != 2 || minor != 45 {
		t.Fatalf("unexpected version: %d.%d", major, minor)
	}
}

func TestLooksLikeRemote(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{value: "git@github.com:org/repo.git", want: true},
		{value: "https://github.com/org/repo.git", want: true},
		{value: "ssh://git@bitbucket.org/org/repo.git", want: true},
		{value: "./local/path", want: false},
	}
	for _, tc := range cases {
		if got := looksLikeRemote(tc.value); got != tc.want {
			t.Fatalf("looksLikeRemote(%q)=%v want %v", tc.value, got, tc.want)
		}
	}
}

func TestProviderFromRemote(t *testing.T) {
	if got := providerFromRemote("git@github.com:org/repo.git"); got != "github" {
		t.Fatalf("unexpected provider: %q", got)
	}
	if got := providerFromRemote("https://bitbucket.org/org/repo.git"); got != "bitbucket" {
		t.Fatalf("unexpected provider: %q", got)
	}
}

func TestExitCode(t *testing.T) {
	if got := (Result{Summary: Summary{Passed: 1}}).ExitCode(false); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
	if got := (Result{Summary: Summary{Warnings: 1}}).ExitCode(false); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
	if got := (Result{Summary: Summary{Warnings: 1}}).ExitCode(true); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if got := (Result{Summary: Summary{Failed: 1}}).ExitCode(false); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}

package system

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseGitVersion(t *testing.T) {
	major, minor := parseGitVersion("git version 2.45.1")
	if major != 2 || minor != 45 {
		t.Fatalf("unexpected version: %d.%d", major, minor)
	}
}

type countingDoctorGit struct {
	calls int
	err   error
}

func (g *countingDoctorGit) Version() (string, error)          { return "git version 2.45.1", nil }
func (g *countingDoctorGit) IsRepository(string) (bool, error) { return true, nil }
func (g *countingDoctorGit) RemoteURL(string, string) (string, error) {
	return "https://github.com/acme/repo.git", nil
}
func (g *countingDoctorGit) LSRemote(string) error { g.calls++; return g.err }

type doctorRuntime struct{ dir, executable string }

func (r doctorRuntime) CurrentExecutable() (string, error) { return r.executable, nil }
func (r doctorRuntime) ResolvePaths() (Paths, error)       { return Paths{Dir: r.dir}, nil }
func (r doctorRuntime) CanWriteDir(string) error           { return nil }

func TestDoctorProbesTheRemoteOnceAndReportsItsFailure(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "kode-stream")
	if err := os.WriteFile(executable, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, probeErr := range []error{nil, errors.New("remote unavailable")} {
		git := &countingDoctorGit{err: probeErr}
		service := &DoctorService{runtime: doctorRuntime{dir: dir, executable: executable}, git: git, getwd: func() (string, error) { return dir, nil }}
		result := service.Run(Options{Repo: "https://github.com/acme/repo.git"})
		failed := probeErr != nil
		if git.calls != 1 || result.OK == failed || (result.ExitCode(false) == 1) != failed || (result.Summary.Failed > 0) != failed {
			t.Fatalf("probe error %v: result=%#v exit=%d LSRemote calls=%d", probeErr, result, result.ExitCode(false), git.calls)
		}
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

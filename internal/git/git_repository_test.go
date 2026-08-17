package git

// Git infrastructure contract tests.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/common/models"
)

func TestSwitchBranchSafelyCarriesOrStashesLocalChanges(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "shared.md", "main\n")
	gitCommit(t, root, "main")
	gitRun(t, root, "branch", "safe")
	gitRun(t, root, "switch", "-c", "overlap")
	writeGitFile(t, root, "shared.md", "overlap\n")
	gitCommit(t, root, "overlap")
	gitRun(t, root, "switch", "main")
	writeGitFile(t, root, "local.md", "local\n")
	adapter := New()
	if _, err := adapter.SwitchBranchSafely(root, "safe", "carry", ""); err != nil {
		t.Fatalf("carry safe changes: %v", err)
	}
	if current, _ := adapter.CurrentBranch(root); current != "safe" {
		t.Fatalf("branch = %q", current)
	}
	if _, err := os.Stat(filepath.Join(root, "local.md")); err != nil {
		t.Fatalf("local file was not carried: %v", err)
	}
	gitRun(t, root, "switch", "main")
	writeGitFile(t, root, "shared.md", "local main\n")
	if _, err := adapter.SwitchBranchSafely(root, "overlap", "carry", ""); !errors.Is(err, ErrCarryChangesUnsafe) {
		t.Fatalf("unsafe carry error = %v", err)
	}
	ref, err := adapter.SwitchBranchSafely(root, "overlap", "stash", "Kode Stream: stash main before switching to overlap")
	if err != nil {
		t.Fatalf("stash switch: %v", err)
	}
	if ref == "" {
		t.Fatal("expected stash ref")
	}
}

func TestParseBranchLine(t *testing.T) {
	status := models.GitStatus{}
	parseBranchLine(&status, "feature/PM-002...origin/feature/PM-002 [ahead 2, behind 1]")

	if status.Branch != "feature/PM-002" {
		t.Fatalf("branch = %q", status.Branch)
	}
	if status.Upstream != "origin/feature/PM-002" {
		t.Fatalf("upstream = %q", status.Upstream)
	}
	if status.Ahead != 2 || status.Behind != 1 {
		t.Fatalf("ahead/behind = %d/%d", status.Ahead, status.Behind)
	}
}

func TestPathStatesNormalizesWorkspaceChanges(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "new.md"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	states, err := New().PathStates("ws", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || states[0].Path != "new.md" || states[0].Status != models.GitChangeUntracked {
		t.Fatalf("states = %#v", states)
	}
}

func TestTreeReadsBranchSnapshotWithoutCheckout(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "plans/main/README.md", "# Main\n")
	gitCommit(t, root, "main item")
	gitRun(t, root, "switch", "-c", "snapshot")
	writeGitFile(t, root, "plans/snapshot/README.md", "# Snapshot\n")
	writeGitFile(t, root, "plans/snapshot/design/backend.md", "# Backend\n")
	gitCommit(t, root, "snapshot item")
	gitRun(t, root, "switch", "main")

	adapter := New()
	ref, commit, err := adapter.ResolveBranch(root, "snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if ref != "refs/heads/snapshot" || commit == "" {
		t.Fatalf("resolved branch = %q %q", ref, commit)
	}

	data, err := adapter.TreeReadFile(root, ref, "plans/snapshot/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Snapshot\n" {
		t.Fatalf("snapshot README = %q", data)
	}
	entries, err := adapter.TreeReadDir(root, ref, "plans/snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name != "README.md" || entries[1].Name != "design" || !entries[1].Type.IsDir() {
		t.Fatalf("entries = %#v", entries)
	}
	walked, err := adapter.TreeWalk(root, ref, "plans/snapshot")
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(walked))
	filePaths := make([]string, 0, len(walked))
	for _, entry := range walked {
		paths = append(paths, entry.Path)
		if !entry.Type.IsDir() {
			filePaths = append(filePaths, entry.Path)
		}
	}
	if strings.Join(paths, ",") != "plans/snapshot/README.md,plans/snapshot/design,plans/snapshot/design/backend.md" {
		t.Fatalf("walked paths = %#v", paths)
	}
	if strings.Join(filePaths, ",") != "plans/snapshot/README.md,plans/snapshot/design/backend.md" {
		t.Fatalf("walked files = %#v", filePaths)
	}
	if author := adapter.LastAuthorAtRef(root, ref, "plans/snapshot/README.md"); author != "Kode Stream" {
		t.Fatalf("author = %q", author)
	}
	if updated := adapter.LastUpdateAtRef(root, ref, "plans/snapshot/README.md"); updated.IsZero() {
		t.Fatal("expected update time at ref")
	}
	current, err := adapter.CurrentBranch(root)
	if err != nil {
		t.Fatal(err)
	}
	if current != "main" {
		t.Fatalf("tree reads changed branch to %q", current)
	}
}

func TestBoundedTreeAndBlobReadsHonorLimitsAndContext(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "plans/review/README.md", strings.Repeat("x", 2048))
	writeGitFile(t, root, "plans/review/deep/a/b/c.md", "deep")
	gitCommit(t, root, "review fixture")
	adapter := New()
	_, commit, err := adapter.ResolveBranch(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.TreeWalkBounded(context.Background(), root, commit, "plans/review", 100, 1); !errors.Is(err, ErrTreeLimit) {
		t.Fatalf("depth limit error = %v", err)
	}
	if _, _, _, err := adapter.TreeReadFileBounded(context.Background(), root, commit, "plans/review/README.md", 32); !errors.Is(err, ErrBlobLimit) {
		t.Fatalf("blob limit error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := adapter.TreeWalkBounded(ctx, root, commit, "plans/review", 100, 64); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled tree error = %v", err)
	}
}

func TestCloneClonesRepositoryIntoDestination(t *testing.T) {
	remote := newGitRepo(t)
	writeGitFile(t, remote, "plans/platform/PM-201/README.md", "# PM-201\n")
	gitCommit(t, remote, "seed")

	cloneRoot := t.TempDir()
	destination := filepath.Join(cloneRoot, "remote-clone")
	if err := New().Clone("file://"+remote, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, ".git")); err != nil {
		t.Fatalf("expected .git folder in cloned repository: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "plans", "platform", "PM-201", "README.md")); err != nil {
		t.Fatalf("expected seeded README in clone: %v", err)
	}
}

func TestParseChangeLine(t *testing.T) {
	change := parseChangeLine(" M plans/platform/PM-002/README.md")
	if change.Path != "plans/platform/PM-002/README.md" {
		t.Fatalf("path = %q", change.Path)
	}
	if change.Status != models.GitChangeModified || change.Staged {
		t.Fatalf("change = %#v", change)
	}

	renamed := parseChangeLine("R  literal -> path.md")
	renamed.OldPath = "old\tname.md"
	if renamed.Status != models.GitChangeRenamed || renamed.OldPath != "old\tname.md" || renamed.Path != "literal -> path.md" || !renamed.Staged {
		t.Fatalf("renamed = %#v", renamed)
	}

	conflicted := parseChangeLine("UU plans/platform/PM-002/README.md")
	if conflicted.Status != models.GitChangeConflicted || !conflicted.Conflict {
		t.Fatalf("conflicted = %#v", conflicted)
	}
}

func TestNULDelimitedGitParsersPreserveAdversarialPathBytes(t *testing.T) {
	path := " leading\tname -> literal\n"
	status := models.GitStatus{Changes: []models.GitChange{parseChangeLine(" M " + path)}}
	if status.Changes[0].Path != path {
		t.Fatalf("status path=%q", status.Changes[0].Path)
	}
	commit := strings.Repeat("a", 40)
	out := "\x00" + commit + "\x002026-01-02T03:04:05Z\x00A U Thor\x00message\x00R100\x00old\tname\x00" + path + "\x00"
	entries, err := parseActivityNUL(out, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || len(entries[0].Paths) != 1 || entries[0].Paths[0].OldPath != "old\tname" || entries[0].Paths[0].Path != path {
		t.Fatalf("entries=%#v", entries)
	}
}

func TestStatusContextPreservesAdversarialGitPaths(t *testing.T) {
	root := newGitRepo(t)
	path := " plans/leading\tname -> literal\n.md "
	writeGitFile(t, root, path, "content")
	status, err := New().StatusContext(context.Background(), "ws", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Changes) != 1 || status.Changes[0].Path != path {
		t.Fatalf("changes=%#v", status.Changes)
	}
}

func TestContextCommandsFailClosedBeforeStarting(t *testing.T) {
	root := newGitRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().StatusContext(ctx, "ws", root); !errors.Is(err, context.Canceled) {
		t.Fatalf("status cancellation error = %v", err)
	}
	if err := New().FetchContext(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("fetch cancellation error = %v", err)
	}
}

func TestWorkspaceMutationLockHonorsCancellation(t *testing.T) {
	root := newGitRepo(t)
	adapter := New()
	entered := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = adapter.WithWorkspaceMutation(root, func() error { close(entered); <-release; return nil })
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := adapter.WithWorkspaceMutationContext(ctx, root, func() error { t.Fatal("cancelled action ran"); return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lock cancellation error = %v", err)
	}
	close(release)
}

func TestLimitedBufferRejectsOversizedOutput(t *testing.T) {
	buffer := &limitedBuffer{limit: 3}
	if _, err := buffer.Write([]byte("abcd")); !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("write error = %v", err)
	}
	if got := buffer.String(); got != "" {
		t.Fatalf("bounded output = %q", got)
	}
}

func TestActivityReturnsRecentCommitsForPath(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "docs/guide.md", "first\n")
	gitCommit(t, root, "add guide")
	writeGitFile(t, root, "docs/guide.md", "second\n")
	gitCommit(t, root, "update guide")
	writeGitFile(t, root, "README.md", "workspace\n")
	gitCommit(t, root, "update readme")

	entries, err := New().Activity(root, "docs/guide.md", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %#v", entries)
	}
	if entries[0].Message != "update guide" {
		t.Fatalf("latest message = %q", entries[0].Message)
	}
	if len(entries[0].Paths) != 1 || entries[0].Paths[0].Path != "docs/guide.md" {
		t.Fatalf("paths = %#v", entries[0].Paths)
	}
}

func TestActivityPreservesRealGitRenamePathBytes(t *testing.T) {
	root := newGitRepo(t)
	oldPath := " plans/old\tname -> literal\n.md "
	newPath := " plans/new\tname -> literal\n.md "
	writeGitFile(t, root, oldPath, "first")
	gitCommit(t, root, "add")
	if err := os.Rename(filepath.Join(root, oldPath), filepath.Join(root, newPath)); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	gitCommit(t, root, "rename")
	entries, err := New().Activity(root, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 || len(entries[0].Paths) != 1 || entries[0].Paths[0].OldPath != oldPath || entries[0].Paths[0].Path != newPath {
		t.Fatalf("activity=%#v", entries)
	}
}

func TestParseActivityNULFailsClosedOnMalformedRecord(t *testing.T) {
	if _, err := parseActivityNUL("\x00"+strings.Repeat("a", 40)+"\x002026-01-02T03:04:05Z\x00author\x00message\x00R100\x00old\x00", 1); err == nil {
		t.Fatal("malformed rename was accepted")
	}
}

func TestActivityReturnsEmptyForMissingPath(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "README.md", "seed\n")
	gitCommit(t, root, "seed")

	entries, err := New().Activity(root, "docs/missing.md", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %#v", entries)
	}
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	gitRun(t, root, "config", "user.name", "Kode Stream")
	gitRun(t, root, "config", "user.email", "kode-stream@example.test")
	return root
}

func writeGitFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitCommit(t *testing.T, root, message string) {
	t.Helper()
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-m", message)
}

func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}

package git

// Tests for the since-commit readers, on temporary repositories built with
// real git.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNameStatusSinceReportsCommittedStagedUnstagedUntrackedAndRenames(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "keep.md", "keep\n")
	writeGitFile(t, root, "modify.md", "before\n")
	writeGitFile(t, root, "delete.md", "gone\n")
	writeGitFile(t, root, "rename-me.md", "renamed\n")
	gitCommit(t, root, "base")
	base := gitRevParse(t, root, "HEAD")

	writeGitFile(t, root, "committed.md", "added and committed\n")
	gitCommit(t, root, "add committed file")

	writeGitFile(t, root, "modify.md", "after\n")
	gitRun(t, root, "add", "modify.md")

	gitRun(t, root, "rm", "delete.md")

	gitRun(t, root, "mv", "rename-me.md", "renamed.md")

	writeGitFile(t, root, "untracked.md", "new\n")

	adapter := New()
	entries, truncated, err := adapter.NameStatusSince(root, base)
	if err != nil {
		t.Fatalf("NameStatusSince: %v", err)
	}
	if truncated {
		t.Fatal("did not expect truncation")
	}
	byPath := make(map[string]NameStatusEntry)
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	if e, ok := byPath["committed.md"]; !ok || e.Status != "A" {
		t.Fatalf("committed.md = %#v, ok=%v", e, ok)
	}
	if e, ok := byPath["modify.md"]; !ok || e.Status != "M" {
		t.Fatalf("modify.md = %#v, ok=%v", e, ok)
	}
	if e, ok := byPath["delete.md"]; !ok || e.Status != "D" {
		t.Fatalf("delete.md = %#v, ok=%v", e, ok)
	}
	if e, ok := byPath["untracked.md"]; !ok || e.Status != "?" {
		t.Fatalf("untracked.md = %#v, ok=%v", e, ok)
	}
	renamed, ok := byPath["renamed.md"]
	if !ok || renamed.Status != "R" || renamed.OldPath != "rename-me.md" {
		t.Fatalf("renamed.md = %#v, ok=%v", renamed, ok)
	}
}

func TestNameStatusSinceTruncatesAt500Entries(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "base.md", "base\n")
	gitCommit(t, root, "base")
	base := gitRevParse(t, root, "HEAD")

	for i := 0; i < 510; i++ {
		writeGitFile(t, root, "file"+strconv.Itoa(i)+".md", "content\n")
	}

	adapter := New()
	entries, truncated, err := adapter.NameStatusSince(root, base)
	if err != nil {
		t.Fatalf("NameStatusSince: %v", err)
	}
	if !truncated {
		t.Fatal("expected truncation")
	}
	if len(entries) != maxNameStatusEntries {
		t.Fatalf("len(entries) = %d, want %d", len(entries), maxNameStatusEntries)
	}
}

func TestNameStatusSinceRejectsUnknownCommit(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "base.md", "base\n")
	gitCommit(t, root, "base")

	adapter := New()
	_, _, err := adapter.NameStatusSince(root, "0000000000000000000000000000000000000000")
	var unknown *ErrUnknownCommit
	if !errors.As(err, &unknown) {
		t.Fatalf("err = %v, want *ErrUnknownCommit", err)
	}
	if unknown.Ref != "0000000000000000000000000000000000000000" {
		t.Fatalf("unknown.Ref = %q", unknown.Ref)
	}
}

func TestNameStatusSinceRejectsBadRef(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "base.md", "base\n")
	gitCommit(t, root, "base")

	adapter := New()
	if _, _, err := adapter.NameStatusSince(root, "--upload-pack=x"); err == nil {
		t.Fatal("expected an error for an option-like ref")
	}
	if _, _, err := adapter.NameStatusSince(root, ""); err == nil {
		t.Fatal("expected an error for an empty ref")
	}
	if _, _, err := adapter.NameStatusSince(root, ":a.md"); err == nil {
		t.Fatal("expected an error for a colon-prefixed ref")
	}
}

func TestNameStatusSinceRoundTripsSpecialPaths(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "base.md", "base\n")
	gitCommit(t, root, "base")
	base := gitRevParse(t, root, "HEAD")

	cases := []string{
		"has space.md",
		"nön-ascii-résumé.md",
	}
	if p := "newline\nfile.md"; canCreateFile(t, root, p) {
		cases = append(cases, p)
	}

	for _, rel := range cases {
		writeGitFile(t, root, rel, "content\n")
		// Stage the addition so it shows up in `git diff <sha>` too --
		// DiffFileSince only diffs against the commit, and an untracked file
		// carries no diff against it until it is at least staged.
		gitRun(t, root, "add", "--", rel)
	}

	adapter := New()
	entries, truncated, err := adapter.NameStatusSince(root, base)
	if err != nil {
		t.Fatalf("NameStatusSince: %v", err)
	}
	if truncated {
		t.Fatal("did not expect truncation")
	}
	byPath := make(map[string]NameStatusEntry)
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	for _, rel := range cases {
		entry, ok := byPath[rel]
		if !ok || entry.Status != "A" {
			t.Fatalf("path %q not round-tripped: %#v (ok=%v); have %#v", rel, entry, ok, byPath)
		}
		ctx := context.Background()
		diff, _, err := adapter.DiffFileSince(ctx, root, base, rel, 1<<20)
		if err != nil {
			t.Fatalf("DiffFileSince(%q): %v", rel, err)
		}
		if diff == "" {
			t.Fatalf("DiffFileSince(%q) returned no diff", rel)
		}
	}
}

// canCreateFile reports whether the test filesystem accepts a file whose
// relative path contains rel (used to probe newline support, which some
// filesystems refuse).
func canCreateFile(t *testing.T, root, rel string) bool {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return false
	}
	if err := os.WriteFile(full, []byte("probe\n"), 0o644); err != nil {
		return false
	}
	if err := os.Remove(full); err != nil {
		t.Fatalf("cleanup probe file %q: %v", rel, err)
	}
	return true
}

func TestNameStatusSinceUntrackedDirectory(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "base.md", "base\n")
	gitCommit(t, root, "base")
	base := gitRevParse(t, root, "HEAD")

	writeGitFile(t, root, "newdir/nested.md", "nested\n")

	adapter := New()
	entries, truncated, err := adapter.NameStatusSince(root, base)
	if err != nil {
		t.Fatalf("NameStatusSince: %v", err)
	}
	if truncated {
		t.Fatal("did not expect truncation")
	}
	byPath := make(map[string]NameStatusEntry)
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	if e, ok := byPath["newdir/nested.md"]; !ok || e.Status != "?" {
		t.Fatalf("newdir/nested.md = %#v, ok=%v", e, ok)
	}
}

func TestShortStatSinceCountsWorkingTreeChanges(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "a.md", "line one\nline two\nline three\n")
	gitCommit(t, root, "base")
	base := gitRevParse(t, root, "HEAD")

	writeGitFile(t, root, "a.md", "line one\nline two changed\nline three\nline four\n")
	writeGitFile(t, root, "b.md", "new file\n")
	gitRun(t, root, "add", "b.md")

	adapter := New()
	additions, deletions, files, err := adapter.ShortStatSince(root, base)
	if err != nil {
		t.Fatalf("ShortStatSince: %v", err)
	}
	if files != 2 {
		t.Fatalf("files = %d, want 2", files)
	}
	if additions == 0 {
		t.Fatal("expected additions > 0")
	}
	if deletions == 0 {
		t.Fatal("expected deletions > 0")
	}
}

func TestShortStatSinceHandlesBinaryOnlyChange(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "keep.md", "keep\n")
	gitCommit(t, root, "base")
	base := gitRevParse(t, root, "HEAD")

	binPath := filepath.Join(root, "asset.bin")
	if err := os.WriteFile(binPath, []byte{0x00, 0x01, 0x02, 0x00, 0xff}, 0o644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}
	gitRun(t, root, "add", "asset.bin")

	adapter := New()
	additions, deletions, files, err := adapter.ShortStatSince(root, base)
	if err != nil {
		t.Fatalf("ShortStatSince: %v", err)
	}
	if files != 1 {
		t.Fatalf("files = %d, want 1", files)
	}
	// git's --shortstat reports no insertions/deletions for a binary-only
	// change; the regexes should simply find nothing rather than error.
	if additions != 0 || deletions != 0 {
		t.Fatalf("additions=%d deletions=%d, want 0,0 for a binary-only change", additions, deletions)
	}
}

func TestDiffFileSinceBoundsOutputAndFlagsTruncation(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "big.md", "one\ntwo\nthree\n")
	gitCommit(t, root, "base")
	base := gitRevParse(t, root, "HEAD")

	writeGitFile(t, root, "big.md", "one changed\ntwo changed\nthree changed\nfour changed\n")

	adapter := New()
	ctx := context.Background()
	full, truncated, err := adapter.DiffFileSince(ctx, root, base, "big.md", 1<<20)
	if err != nil {
		t.Fatalf("DiffFileSince: %v", err)
	}
	if truncated {
		t.Fatal("did not expect truncation at a generous bound")
	}
	if !strings.Contains(full, "big.md") {
		t.Fatalf("diff = %q", full)
	}

	small, truncated, err := adapter.DiffFileSince(ctx, root, base, "big.md", 10)
	if err != nil {
		t.Fatalf("DiffFileSince bounded: %v", err)
	}
	if !truncated {
		t.Fatal("expected truncation at a small bound")
	}
	if len(small) != 10 {
		t.Fatalf("len(small) = %d, want 10", len(small))
	}
}

func TestDiffFileSinceTruncatesUTF8OnRuneBoundary(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "unicode.md", "start\n")
	gitCommit(t, root, "base")
	base := gitRevParse(t, root, "HEAD")

	// "é" is two UTF-8 bytes; repeat it so a small byte bound is very likely
	// to land in the middle of one of the multi-byte runes in the diff.
	writeGitFile(t, root, "unicode.md", strings.Repeat("é", 200)+"\n")

	adapter := New()
	ctx := context.Background()
	for maxBytes := int64(1); maxBytes < 40; maxBytes++ {
		out, truncated, err := adapter.DiffFileSince(ctx, root, base, "unicode.md", maxBytes)
		if err != nil {
			t.Fatalf("DiffFileSince(maxBytes=%d): %v", maxBytes, err)
		}
		if !truncated {
			continue
		}
		if int64(len(out)) > maxBytes {
			t.Fatalf("maxBytes=%d: len(out)=%d exceeds bound", maxBytes, len(out))
		}
		if !utf8.ValidString(out) {
			t.Fatalf("maxBytes=%d: truncated diff is not valid UTF-8: %q", maxBytes, out)
		}
	}
}

func TestNameStatusSinceRejectsTreeRef(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "base.md", "base\n")
	gitCommit(t, root, "base")
	head := gitRevParse(t, root, "HEAD")
	tree := gitRevParse(t, root, head+"^{tree}")

	adapter := New()
	_, _, err := adapter.NameStatusSince(root, tree)
	var unknown *ErrUnknownCommit
	if !errors.As(err, &unknown) {
		t.Fatalf("err = %v, want *ErrUnknownCommit for a tree ref", err)
	}
}

func TestDiffFileSinceRejectsInvalidPath(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "a.md", "a\n")
	gitCommit(t, root, "base")
	base := gitRevParse(t, root, "HEAD")

	adapter := New()
	ctx := context.Background()
	if _, _, err := adapter.DiffFileSince(ctx, root, base, "../outside.md", 1<<10); err == nil {
		t.Fatal("expected an error for a path escaping the tree")
	}
	if _, _, err := adapter.DiffFileSince(ctx, root, base, "-x", 1<<10); err == nil {
		t.Fatal("expected an error for a dash-prefixed path")
	}
	if _, _, err := adapter.DiffFileSince(ctx, root, base, ":a.md", 1<<10); err == nil {
		t.Fatal("expected an error for a colon-prefixed path")
	}
}

func TestRevListCountCountsCommitsBetweenRefs(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "a.md", "1\n")
	gitCommit(t, root, "one")
	base := gitRevParse(t, root, "HEAD")
	writeGitFile(t, root, "a.md", "2\n")
	gitCommit(t, root, "two")
	writeGitFile(t, root, "a.md", "3\n")
	gitCommit(t, root, "three")
	head := gitRevParse(t, root, "HEAD")

	adapter := New()
	count, err := adapter.RevListCount(root, base, head)
	if err != nil {
		t.Fatalf("RevListCount: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

func TestRevListCountRejectsUnknownCommit(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "a.md", "1\n")
	gitCommit(t, root, "one")
	head := gitRevParse(t, root, "HEAD")

	adapter := New()
	_, err := adapter.RevListCount(root, "0000000000000000000000000000000000000000", head)
	var unknown *ErrUnknownCommit
	if !errors.As(err, &unknown) {
		t.Fatalf("err = %v, want *ErrUnknownCommit", err)
	}
}

func TestCommitExists(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "a.md", "1\n")
	gitCommit(t, root, "one")
	head := gitRevParse(t, root, "HEAD")

	adapter := New()
	if err := adapter.CommitExists(root, head); err != nil {
		t.Fatalf("CommitExists(head): %v", err)
	}
	err := adapter.CommitExists(root, "0000000000000000000000000000000000000000")
	var unknown *ErrUnknownCommit
	if !errors.As(err, &unknown) {
		t.Fatalf("err = %v, want *ErrUnknownCommit", err)
	}
	if err := adapter.CommitExists(root, "--not-a-ref"); err == nil {
		t.Fatal("expected an error for an option-like ref")
	}
}

func gitRevParse(t *testing.T, root, ref string) string {
	t.Helper()
	out, err := New().run(root, "rev-parse", ref)
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(out)
}

package git

// Since-commit readers: bounded, read-only queries that describe what a
// checkout has done since a session's launch commit. Every method here
// validates its ref arguments before use and resolves them with
// `git rev-parse --verify --end-of-options <ref>^{commit}` (or, for
// CommitExists, `git cat-file -e`) so a bad or missing commit fails with a
// typed error instead of an ambiguous git exit status.

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// maxNameStatusEntries bounds the file list NameStatusSince returns, mirroring
// the cap TreeWalkBounded applies to review trees: a stable limit and a
// truncated flag beat a partial list with unknown omissions.
const maxNameStatusEntries = 500

// ErrUnknownCommit reports that a ref did not resolve to a real commit in the
// workspace. Err carries the underlying git failure.
type ErrUnknownCommit struct {
	Ref string
	Err error
}

func (e *ErrUnknownCommit) Error() string {
	return fmt.Sprintf("unknown commit %q: %v", e.Ref, e.Err)
}

func (e *ErrUnknownCommit) Unwrap() error { return e.Err }

// NameStatusEntry describes one path changed since a commit, or currently
// untracked. Status is one of A, M, D, R, C, or ? (untracked); OldPath is set
// only for R and C.
type NameStatusEntry struct {
	Path    string
	Status  string
	OldPath string
}

// NameStatusSince lists paths changed between commit and the working tree
// (staged and unstaged), plus untracked files, capped at maxNameStatusEntries.
func (g *GitAdapter) NameStatusSince(workspacePath, commit string) ([]NameStatusEntry, bool, error) {
	return g.NameStatusSinceContext(context.Background(), workspacePath, commit)
}

func (g *GitAdapter) NameStatusSinceContext(ctx context.Context, workspacePath, commit string) ([]NameStatusEntry, bool, error) {
	sha, err := g.resolveCommitContext(ctx, workspacePath, commit)
	if err != nil {
		return nil, false, err
	}

	// Stream both commands' NUL-delimited output rather than buffering it:
	// a workspace with an enormous number of changed or untracked paths must
	// truncate at maxNameStatusEntries, not fail with ErrOutputLimit. Ask
	// for one more than the cap so we can tell "exactly at the cap" apart
	// from "more entries exist beyond it".
	budget := maxNameStatusEntries + 1
	entries, err := g.streamNameStatusSince(ctx, workspacePath, sha, budget)
	if err != nil {
		return nil, false, err
	}
	if len(entries) <= maxNameStatusEntries {
		remaining := budget - len(entries)
		untracked, err := g.streamUntrackedSince(ctx, workspacePath, remaining)
		if err != nil {
			return nil, false, err
		}
		entries = append(entries, untracked...)
	}

	truncated := len(entries) > maxNameStatusEntries
	if truncated {
		entries = entries[:maxNameStatusEntries]
	}
	return entries, truncated, nil
}

// streamNameStatusSince runs `git diff --name-status -M -z` against sha and
// parses its NUL-delimited records (status\0path\0, or Rnnn\0old\0new\0 for a
// rename/copy) as they arrive, stopping as soon as limit entries have been
// collected instead of buffering the whole diff.
func (g *GitAdapter) streamNameStatusSince(ctx context.Context, dir, sha string, limit int) ([]NameStatusEntry, error) {
	return g.streamNULEntries(ctx, dir, limit, func(next func() (string, bool)) ([]NameStatusEntry, bool) {
		status, ok := next()
		if !ok {
			return nil, false
		}
		if status == "" {
			return []NameStatusEntry{}, true
		}
		code := status[:1]
		if code == "R" || code == "C" {
			oldPath, ok := next()
			if !ok {
				return nil, false
			}
			newPath, ok := next()
			if !ok {
				return nil, false
			}
			return []NameStatusEntry{{Status: code, OldPath: oldPath, Path: newPath}}, true
		}
		pathValue, ok := next()
		if !ok {
			return nil, false
		}
		return []NameStatusEntry{{Status: code, Path: pathValue}}, true
	}, "diff", "--name-status", "-M", "-z", sha, "--")
}

// streamUntrackedSince runs `git status --porcelain=v1 -z --untracked-files=all`
// and streams its NUL-delimited records for untracked ("??") entries only,
// stopping once limit entries have been collected. A rename/copy record's old-
// path continuation field is consumed and skipped so it can never be mistaken
// for an unrelated untracked record.
func (g *GitAdapter) streamUntrackedSince(ctx context.Context, dir string, limit int) ([]NameStatusEntry, error) {
	if limit <= 0 {
		return []NameStatusEntry{}, nil
	}
	return g.streamNULEntries(ctx, dir, limit, func(next func() (string, bool)) ([]NameStatusEntry, bool) {
		record, ok := next()
		if !ok {
			return nil, false
		}
		if len(record) < 2 {
			return []NameStatusEntry{}, true
		}
		x, y := record[0], record[1]
		if x == '?' && y == '?' {
			if len(record) < 3 {
				return []NameStatusEntry{}, true
			}
			return []NameStatusEntry{{Status: "?", Path: record[3:]}}, true
		}
		if x == 'R' || x == 'C' || y == 'R' || y == 'C' {
			// Consume (and discard) the rename/copy continuation field so it
			// is never parsed as its own record.
			if _, ok := next(); !ok {
				return nil, false
			}
		}
		return []NameStatusEntry{}, true
	}, "status", "--porcelain=v1", "-z", "--untracked-files=all")
}

// streamNULEntries runs git with args, reading its NUL-delimited stdout
// incrementally rather than buffering it. produce is called repeatedly with a
// next() accessor over the token stream; it returns the (possibly empty) set
// of entries the tokens it consumed represent, and false once the stream is
// exhausted. Reading stops, and the process is killed, as soon as limit
// entries have been produced -- a large listing truncates instead of erroring
// or consuming unbounded memory.
func (g *GitAdapter) streamNULEntries(
	ctx context.Context,
	dir string,
	limit int,
	produce func(next func() (string, bool)) ([]NameStatusEntry, bool),
	args ...string,
) ([]NameStatusEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", args...)
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &limitedBuffer{limit: 1 << 20}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 16<<20)
	scanner.Split(splitNULToken)
	next := func() (string, bool) {
		if scanner.Scan() {
			return scanner.Text(), true
		}
		return "", false
	}

	entries := make([]NameStatusEntry, 0, limit)
	truncatedByLimit := false
	for {
		produced, ok := produce(next)
		if !ok {
			break
		}
		entries = append(entries, produced...)
		if len(entries) >= limit {
			truncatedByLimit = true
			break
		}
	}
	scanErr := scanner.Err()

	if truncatedByLimit {
		// Stop reading immediately: killing the process avoids buffering an
		// arbitrarily large remainder we would only discard.
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return entries, nil
	}

	if waitErr := cmd.Wait(); waitErr != nil {
		if errors.Is(cctx.Err(), context.Canceled) || errors.Is(cctx.Err(), context.DeadlineExceeded) {
			return nil, cctx.Err()
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = waitErr.Error()
		}
		return nil, errors.New(message)
	}
	if scanErr != nil {
		return nil, scanErr
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func splitNULToken(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

var (
	shortStatFilesRe  = regexp.MustCompile(`(\d+) files? changed`)
	shortStatInsertRe = regexp.MustCompile(`(\d+) insertions?\(\+\)`)
	shortStatDeleteRe = regexp.MustCompile(`(\d+) deletions?\(-\)`)
)

// ShortStatSince reports additions, deletions, and the changed-file count
// between commit and the working tree (staged and unstaged changes included).
func (g *GitAdapter) ShortStatSince(workspacePath, commit string) (additions, deletions, files int, err error) {
	return g.ShortStatSinceContext(context.Background(), workspacePath, commit)
}

func (g *GitAdapter) ShortStatSinceContext(ctx context.Context, workspacePath, commit string) (additions, deletions, files int, err error) {
	sha, err := g.resolveCommitContext(ctx, workspacePath, commit)
	if err != nil {
		return 0, 0, 0, err
	}
	out, err := g.runContextLimited(ctx, workspacePath, 1<<16, "diff", "--shortstat", sha, "--")
	if err != nil {
		return 0, 0, 0, err
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return 0, 0, 0, nil
	}
	return firstInt(shortStatInsertRe, text), firstInt(shortStatDeleteRe, text), firstInt(shortStatFilesRe, text), nil
}

func firstInt(re *regexp.Regexp, text string) int {
	match := re.FindStringSubmatch(text)
	if len(match) != 2 {
		return 0
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return value
}

// DiffFileSince returns the diff of one path between commit and the working
// tree, bounded by maxBytes with a truncated flag. relPath must pass the same
// tree-path validation the adapter already applies and must not start with
// ':' or '-'.
func (g *GitAdapter) DiffFileSince(ctx context.Context, workspacePath, commit, relPath string, maxBytes int64) (string, bool, error) {
	if maxBytes <= 0 {
		return "", false, ErrBlobLimit
	}
	sha, err := g.resolveCommitContext(ctx, workspacePath, commit)
	if err != nil {
		return "", false, err
	}
	clean, err := cleanGitTreePath(relPath)
	if err != nil {
		return "", false, err
	}
	if clean == "" || strings.HasPrefix(clean, ":") || strings.HasPrefix(clean, "-") {
		return "", false, fmt.Errorf("invalid diff path %q", relPath)
	}
	return g.boundedDiff(ctx, workspacePath, maxBytes, "diff", "--no-ext-diff", "-M", sha, "--", clean)
}

// boundedDiff runs a git diff and returns at most maxBytes of it, with a
// truncated flag, instead of failing on a large diff.
func (g *GitAdapter) boundedDiff(ctx context.Context, workspacePath string, maxBytes int64, args ...string) (string, bool, error) {
	if maxBytes <= 0 {
		return "", false, ErrBlobLimit
	}
	// Cap the command just above the caller's bound: a diff at or under
	// maxBytes always comes back whole, and one that overflows is reported
	// as ErrOutputLimit rather than silently discarded.
	out, err := g.runContextLimited(ctx, workspacePath, maxBytes+1, args...)
	if err != nil {
		if !errors.Is(err, ErrOutputLimit) {
			return "", false, err
		}
		// runContextLimited discards everything it read on overflow, so
		// re-run the same diff capturing exactly the first maxBytes -- the
		// caller gets a usable truncated diff instead of an error.
		partial, rerunErr := g.runTruncated(ctx, workspacePath, maxBytes, args...)
		if rerunErr != nil {
			return "", false, rerunErr
		}
		return truncateUTF8(string(partial), maxBytes), true, nil
	}
	if int64(len(out)) > maxBytes {
		return truncateUTF8(string(out), maxBytes), true, nil
	}
	return string(out), false, nil
}

// runTruncated runs git with args, capturing at most maxBytes of stdout. It
// never fails on overflow -- unlike runContextLimited/limitedBuffer, excess
// output is silently discarded rather than turning into ErrOutputLimit.
func (g *GitAdapter) runTruncated(ctx context.Context, dir string, maxBytes int64, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	stdout := &truncatingWriter{limit: maxBytes}
	stderr := &limitedBuffer{limit: 1 << 20}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, ctx.Err()
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, errors.New(message)
	}
	return stdout.buf.Bytes(), nil
}

// truncatingWriter keeps only the first limit bytes written to it and never
// returns an error on overflow, unlike limitedBuffer whose job is to abort
// the command instead.
type truncatingWriter struct {
	buf   bytes.Buffer
	limit int64
}

func (w *truncatingWriter) Write(p []byte) (int, error) {
	remaining := w.limit - int64(w.buf.Len())
	if remaining > 0 {
		if int64(len(p)) > remaining {
			w.buf.Write(p[:remaining])
		} else {
			w.buf.Write(p)
		}
	}
	return len(p), nil
}

// truncateUTF8 trims s to at most limit bytes without splitting a multi-byte
// rune, so a diff cut off mid-character never yields invalid UTF-8.
func truncateUTF8(s string, limit int64) string {
	if limit < 0 {
		limit = 0
	}
	if int64(len(s)) <= limit {
		return s
	}
	cut := int(limit)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// RevListCount reports the number of commits reachable from to but not from
// from (`git rev-list --count from..to`).
func (g *GitAdapter) RevListCount(workspacePath, from, to string) (int, error) {
	return g.RevListCountContext(context.Background(), workspacePath, from, to)
}

func (g *GitAdapter) RevListCountContext(ctx context.Context, workspacePath, from, to string) (int, error) {
	fromSHA, err := g.resolveCommitContext(ctx, workspacePath, from)
	if err != nil {
		return 0, err
	}
	toSHA, err := g.resolveCommitContext(ctx, workspacePath, to)
	if err != nil {
		return 0, err
	}
	out, err := g.runContextLimited(ctx, workspacePath, 1<<16, "rev-list", "--count", fromSHA+".."+toSHA)
	if err != nil {
		return 0, err
	}
	count, convErr := strconv.Atoi(strings.TrimSpace(string(out)))
	if convErr != nil {
		return 0, convErr
	}
	return count, nil
}

// CommitExists reports whether commit resolves to a real commit object. It
// returns nil when it does, and a typed *ErrUnknownCommit when it does not.
func (g *GitAdapter) CommitExists(workspacePath, commit string) error {
	return g.CommitExistsContext(context.Background(), workspacePath, commit)
}

// CommitExistsContext checks that commit both exists and names a commit
// object, using the bare (unpeeled) forms of `git cat-file -e <commit>` and
// `git cat-file -t <commit>` -- deliberately not the `<commit>^{commit}`
// peel expression the rest of this file uses, because peeling makes git fail
// ref resolution itself on a missing object (`fatal: Not a valid object
// name`, exit 128), which is indistinguishable by exit code from a corrupt
// repository or any other resolution failure. The bare `-e` form instead
// exits 1 -- cleanly, with no stderr -- exactly when the object is simply
// absent, so that is the only case (together with the object existing but
// not being a commit) that becomes a typed *ErrUnknownCommit; every other
// outcome (a corrupt repository, a missing git binary, a timeout, a process
// that could not even start) is returned unchanged so the caller can tell it
// apart from a merely-absent commit.
func (g *GitAdapter) CommitExistsContext(ctx context.Context, workspacePath, commit string) error {
	clean, err := validateRef(commit)
	if err != nil {
		return err
	}
	exitCode, runErr := g.runExitCode(ctx, workspacePath, "cat-file", "-e", clean)
	if runErr != nil {
		return runErr
	}
	switch exitCode {
	case 0:
		// falls through to the type check below
	case 1:
		return &ErrUnknownCommit{Ref: clean, Err: fmt.Errorf("object %q does not exist", clean)}
	default:
		return fmt.Errorf("git cat-file -e %s: exited with status %d", clean, exitCode)
	}
	objType, err := g.runContextLimited(ctx, workspacePath, 1<<10, "cat-file", "-t", clean)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(objType)) != "commit" {
		return &ErrUnknownCommit{Ref: clean, Err: fmt.Errorf("object %q is not a commit", clean)}
	}
	return nil
}

// runExitCode runs `git <args...>` in dir and reports its exit code without
// treating a non-zero exit as an error in itself: err is non-nil only when
// the process could not be run or judged at all (a context cancellation or
// timeout, or git failing to start), never merely because it exited
// non-zero. Callers that need to tell "exited 1" apart from "exited 128"
// apart from "could not run git at all" use this instead of
// runContextLimited, which collapses all three into one opaque error.
func (g *GitAdapter) runExitCode(ctx context.Context, dir string, args ...string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	cctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	stderr := &limitedBuffer{limit: 1 << 20}
	cmd.Stderr = stderr
	runErr := cmd.Run()
	if runErr == nil {
		return 0, nil
	}
	if errors.Is(cctx.Err(), context.Canceled) || errors.Is(cctx.Err(), context.DeadlineExceeded) {
		return 0, cctx.Err()
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	message := strings.TrimSpace(stderr.String())
	if message == "" {
		message = runErr.Error()
	}
	return 0, errors.New(message)
}

// validateRef enforces the shared ref-argument shape: non-empty and never
// starting with '-' or ':', so a ref can never be mistaken for a git option
// or a pathspec magic prefix.
func validateRef(ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", errors.New("git ref must not be empty")
	}
	if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, ":") {
		return "", fmt.Errorf("git ref must not start with '-' or ':': %q", ref)
	}
	return trimmed, nil
}

// resolveCommitContext validates ref and resolves it to a commit SHA with
// `git rev-parse --verify --end-of-options <ref>^{commit}`. A ref that fails
// to resolve returns a typed *ErrUnknownCommit wrapping the underlying git
// failure.
func (g *GitAdapter) resolveCommitContext(ctx context.Context, workspacePath, ref string) (string, error) {
	clean, err := validateRef(ref)
	if err != nil {
		return "", err
	}
	out, err := g.runContextLimited(ctx, workspacePath, 1<<16, "rev-parse", "--verify", "--end-of-options", clean+"^{commit}")
	if err != nil {
		return "", &ErrUnknownCommit{Ref: clean, Err: err}
	}
	return strings.TrimSpace(string(out)), nil
}

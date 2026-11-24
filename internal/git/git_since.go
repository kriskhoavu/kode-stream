package git

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

const maxNameStatusEntries = 500

type ErrUnknownCommit struct {
	Ref string
	Err error
}

func (e *ErrUnknownCommit) Error() string {
	return fmt.Sprintf("unknown commit %q: %v", e.Ref, e.Err)
}

func (e *ErrUnknownCommit) Unwrap() error { return e.Err }

type NameStatusEntry struct {
	Path    string
	Status  string
	OldPath string
}

func (g *GitAdapter) NameStatusSince(workspacePath, commit string) ([]NameStatusEntry, bool, error) {
	return g.NameStatusSinceContext(context.Background(), workspacePath, commit)
}

func (g *GitAdapter) NameStatusSinceContext(ctx context.Context, workspacePath, commit string) ([]NameStatusEntry, bool, error) {
	sha, err := g.resolveCommitContext(ctx, workspacePath, commit)
	if err != nil {
		return nil, false, err
	}

	// Stream, so a huge change list truncates instead of failing. Ask for one more to detect truncation.
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

// A rename record's old-path field is skipped, so it never reads as an untracked record.
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
			if _, ok := next(); !ok {
				return nil, false
			}
		}
		return []NameStatusEntry{}, true
	}, "status", "--porcelain=v1", "-z", "--untracked-files=all")
}

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
		// Kill the process instead of buffering a large remainder this reader would discard.
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

func (g *GitAdapter) boundedDiff(ctx context.Context, workspacePath string, maxBytes int64, args ...string) (string, bool, error) {
	if maxBytes <= 0 {
		return "", false, ErrBlobLimit
	}
	// Cap the command just above the bound: a diff at or under maxBytes comes back whole.
	out, err := g.runContextLimited(ctx, workspacePath, maxBytes+1, args...)
	if err != nil {
		if !errors.Is(err, ErrOutputLimit) {
			return "", false, err
		}
		// runContextLimited discards everything on overflow, so run again and keep the first maxBytes.
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

// Trims on a rune boundary, so a cut diff stays valid UTF-8.
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

func (g *GitAdapter) CommitExists(workspacePath, commit string) error {
	return g.CommitExistsContext(context.Background(), workspacePath, commit)
}

// Bare cat-file -e exits 1 only for a missing object. A peeled ref fails with exit 128, like a corrupt repository.
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

// err is set only when git could not run, so callers can tell exit 1 from exit 128.
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

// A leading '-' or ':' would read as a git option or a pathspec magic prefix.
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

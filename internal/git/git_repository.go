package git

// Package gitadapter implements domain Git repository ports.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"kode-stream/internal/common/models"
)

type GitAdapter struct {
	timeout      time.Duration
	cloneTimeout time.Duration
	mutations    sync.Map
}

var (
	ErrBranchSwitchDecisionRequired = errors.New("branch switch decision required")
	ErrCarryChangesUnsafe           = errors.New("local changes could be overwritten by the target branch")
	ErrTreeLimit                    = errors.New("git tree exceeds the review limit")
	ErrBlobLimit                    = errors.New("git blob exceeds the review limit")
	ErrOutputLimit                  = errors.New("git command output exceeds the safety limit")
)

type TreeEntry struct {
	Name    string
	Path    string
	Type    fs.FileMode
	Mode    string
	Object  string
	Size    int64
	ModTime time.Time
}

func New() *GitAdapter {
	return &GitAdapter{timeout: 5 * time.Second, cloneTimeout: 10 * time.Minute}
}

func (g *GitAdapter) WorkspaceRoot(path string) (string, error) {
	out, err := g.run(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.Clean(strings.TrimSpace(out)), nil
}

func (g *GitAdapter) ValidateBranch(workspacePath, branch string) error {
	_, err := g.run(workspacePath, "show-ref", "--verify", "refs/heads/"+branch)
	return err
}

func (g *GitAdapter) ResolveBranch(workspacePath, branch string) (string, string, error) {
	if err := g.ValidateBranch(workspacePath, branch); err != nil {
		return "", "", err
	}
	ref := "refs/heads/" + branch
	out, err := g.run(workspacePath, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", "", err
	}
	return ref, strings.TrimSpace(out), nil
}

func (g *GitAdapter) CurrentBranch(workspacePath string) (string, error) {
	return g.CurrentBranchContext(context.Background(), workspacePath)
}

func (g *GitAdapter) CurrentBranchContext(ctx context.Context, workspacePath string) (string, error) {
	out, err := g.runContextLimited(ctx, workspacePath, 1<<20, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "HEAD", nil
	}
	return branch, nil
}

func (g *GitAdapter) RemoteURL(workspacePath string) (string, error) {
	out, err := g.run(workspacePath, "config", "--get", "remote.origin.url")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (g *GitAdapter) ListBranches(workspacePath string) ([]string, error) {
	return g.ListBranchesContext(context.Background(), workspacePath)
}

func (g *GitAdapter) ListBranchesContext(ctx context.Context, workspacePath string) ([]string, error) {
	out, err := g.runContextLimited(ctx, workspacePath, 1<<20, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

func (g *GitAdapter) ListStashes(workspacePath string) ([]models.GitStashEntry, error) {
	return g.ListStashesContext(context.Background(), workspacePath)
}

func (g *GitAdapter) ListStashesContext(ctx context.Context, workspacePath string) ([]models.GitStashEntry, error) {
	out, err := g.runContextLimited(ctx, workspacePath, 1<<20, "stash", "list", "--format=%gd%x1f%s")
	if err != nil {
		return nil, err
	}
	entries := make([]models.GitStashEntry, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\x1f", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		entries = append(entries, models.GitStashEntry{Ref: strings.TrimSpace(parts[0]), Message: strings.TrimSpace(parts[1])})
	}
	return entries, nil
}

func (g *GitAdapter) ApplyStash(workspacePath, ref string) error {
	return g.ApplyStashContext(context.Background(), workspacePath, ref)
}

func (g *GitAdapter) ApplyStashContext(ctx context.Context, workspacePath, ref string) error {
	if !regexp.MustCompile(`^stash@\{[0-9]+\}$`).MatchString(strings.TrimSpace(ref)) {
		return fmt.Errorf("invalid stash reference")
	}
	_, err := g.runContextLimited(ctx, workspacePath, 1<<20, "stash", "apply", "--index", ref)
	return err
}

func (g *GitAdapter) LastAuthor(workspacePath, relPath string) string {
	out, err := g.run(workspacePath, "log", "-1", "--format=%an", "--", relPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func (g *GitAdapter) LastUpdate(workspacePath, relPath string) time.Time {
	out, err := g.run(workspacePath, "log", "-1", "--format=%cI", "--", relPath)
	if err == nil {
		if t, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(out)); parseErr == nil {
			return t
		}
	}
	return time.Time{}
}

func (g *GitAdapter) LastAuthorAtRef(workspacePath, ref, relPath string) string {
	clean, err := cleanGitTreePath(relPath)
	if err != nil {
		return ""
	}
	out, err := g.run(workspacePath, "log", "-1", "--format=%an", ref, "--", clean)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func (g *GitAdapter) LastUpdateAtRef(workspacePath, ref, relPath string) time.Time {
	clean, err := cleanGitTreePath(relPath)
	if err != nil {
		return time.Time{}
	}
	out, err := g.run(workspacePath, "log", "-1", "--format=%cI", ref, "--", clean)
	if err == nil {
		if t, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(out)); parseErr == nil {
			return t
		}
	}
	return time.Time{}
}

func (g *GitAdapter) TreeReadDir(workspacePath, ref, relPath string) ([]TreeEntry, error) {
	clean, err := cleanGitTreePath(relPath)
	if err != nil {
		return nil, err
	}
	out, err := g.run(workspacePath, "ls-tree", "-z", "-l", treeish(ref, clean))
	if err != nil {
		return nil, err
	}
	return parseTreeEntries(clean, out)
}

func (g *GitAdapter) TreeReadFile(workspacePath, ref, relPath string) ([]byte, error) {
	clean, err := cleanGitTreePath(relPath)
	if err != nil {
		return nil, err
	}
	out, err := g.run(workspacePath, "show", treeish(ref, clean))
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

func (g *GitAdapter) TreeWalk(workspacePath, ref, root string) ([]TreeEntry, error) {
	clean, err := cleanGitTreePath(root)
	if err != nil {
		return nil, err
	}
	out, err := g.run(workspacePath, "ls-tree", "-r", "-t", "-z", "-l", treeish(ref, clean))
	if err != nil {
		return nil, err
	}
	return parseTreeEntries(clean, out)
}

// TreeWalkBounded lists a review tree without allowing git output or the
// returned model to grow without bound. The caller receives a stable limit
// error instead of a partial tree with unknown omissions.
func (g *GitAdapter) TreeWalkBounded(ctx context.Context, workspacePath, ref, root string, maxEntries, maxDepth int) ([]TreeEntry, error) {
	clean, err := cleanGitTreePath(root)
	if err != nil {
		return nil, err
	}
	if maxEntries <= 0 || maxDepth < 0 {
		return nil, ErrTreeLimit
	}
	// ls-tree records are small, but cap stdout too so a pathological tree is
	// terminated before an unbounded buffer allocation.
	outputLimit := int64(maxEntries*1024 + 4096)
	out, err := g.runContextLimited(ctx, workspacePath, outputLimit, "ls-tree", "-r", "-t", "-z", "-l", treeish(ref, clean))
	if err != nil {
		if errors.Is(err, ErrOutputLimit) {
			return nil, ErrTreeLimit
		}
		return nil, err
	}
	entries, err := parseTreeEntries(clean, string(out))
	if err != nil {
		return nil, err
	}
	if len(entries) > maxEntries {
		return nil, ErrTreeLimit
	}
	for _, entry := range entries {
		rel := strings.TrimPrefix(entry.Path, clean)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || strings.HasPrefix(rel, "../") {
			return nil, ErrTreeLimit
		}
		if depth := strings.Count(rel, "/"); depth > maxDepth {
			return nil, ErrTreeLimit
		}
	}
	return entries, nil
}

// TreeReadFileBounded checks the Git object size before streaming a bounded
// prefix. Images are rejected when oversized; text callers can display the
// bounded prefix with an explicit truncation flag.
func (g *GitAdapter) TreeReadFileBounded(ctx context.Context, workspacePath, ref, relPath string, maxBytes int64) ([]byte, int64, bool, error) {
	clean, err := cleanGitTreePath(relPath)
	if err != nil {
		return nil, 0, false, err
	}
	if maxBytes <= 0 {
		return nil, 0, false, ErrBlobLimit
	}
	sizeOut, err := g.runContextLimited(ctx, workspacePath, 128, "cat-file", "-s", treeish(ref, clean))
	if err != nil {
		return nil, 0, false, err
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(sizeOut)), 10, 64)
	if err != nil {
		return nil, 0, false, err
	}
	if size > maxBytes {
		return nil, size, true, ErrBlobLimit
	}
	out, err := g.runContextLimited(ctx, workspacePath, maxBytes+1, "show", treeish(ref, clean))
	if err != nil {
		return nil, 0, false, err
	}
	return out, size, size > maxBytes, nil
}

func (g *GitAdapter) Diff(workspacePath, relPath string) (string, error) {
	return g.DiffContext(context.Background(), workspacePath, relPath)
}

func (g *GitAdapter) DiffContext(ctx context.Context, workspacePath, relPath string) (string, error) {
	out, err := g.runContextLimited(ctx, workspacePath, 4<<20, "diff", "--no-ext-diff", "--", relPath)
	return string(out), err
}

func (g *GitAdapter) Status(workspaceID, workspacePath string) (models.GitStatus, error) {
	return g.StatusContext(context.Background(), workspaceID, workspacePath)
}

func (g *GitAdapter) StatusContext(ctx context.Context, workspaceID, workspacePath string) (models.GitStatus, error) {
	outBytes, err := g.runContextLimited(ctx, workspacePath, 4<<20, "status", "--porcelain=v1", "-z", "-b", "--untracked-files=all")
	if err != nil {
		return models.GitStatus{}, err
	}
	out := string(outBytes)
	status := models.GitStatus{WorkspaceID: workspaceID, Changes: []models.GitChange{}}
	records := strings.Split(out, "\x00")
	for index := 0; index < len(records); index++ {
		line := records[index]
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			parseBranchLine(&status, strings.TrimPrefix(line, "## "))
			continue
		}
		change := parseChangeLine(line)
		if len(line) >= 2 && (line[0] == 'R' || line[0] == 'C' || line[1] == 'R' || line[1] == 'C') && index+1 < len(records) {
			change.OldPath = records[index+1]
			index++
		}
		if change.Path == "" {
			continue
		}
		status.Changes = append(status.Changes, change)
		if change.Conflict {
			status.Conflicted = true
		}
		if change.Staged || change.Status == models.GitChangeModified || change.Status == models.GitChangeDeleted || change.Status == models.GitChangeUntracked {
			status.Dirty = true
		}
	}
	return status, nil
}

func (g *GitAdapter) Activity(workspacePath, relPath string, limit int) ([]models.GitActivityEntry, error) {
	return g.ActivityContext(context.Background(), workspacePath, relPath, limit)
}

func (g *GitAdapter) ActivityContext(ctx context.Context, workspacePath, relPath string, limit int) ([]models.GitActivityEntry, error) {
	if limit <= 0 {
		limit = 12
	}
	args := []string{"log", "-z", "--date=iso-strict", "--name-status", "--pretty=format:%x00%H%x00%cI%x00%an%x00%s%x00", "-n", strconv.Itoa(limit)}
	if strings.TrimSpace(relPath) != "" {
		clean, err := cleanGitTreePath(relPath)
		if err != nil {
			return nil, err
		}
		args = append(args, "--", clean)
	}
	outBytes, err := g.runContextLimited(ctx, workspacePath, 4<<20, args...)
	if err != nil {
		message := err.Error()
		if strings.Contains(message, "does not exist in") || strings.Contains(message, "unknown revision or path not in the working tree") {
			return []models.GitActivityEntry{}, nil
		}
		return nil, err
	}
	out := string(outBytes)
	if out == "" {
		return []models.GitActivityEntry{}, nil
	}
	return parseActivityNUL(out, limit)
}

func parseActivityNUL(out string, limit int) ([]models.GitActivityEntry, error) {
	records := strings.Split(out, "\x00")
	entries := make([]models.GitActivityEntry, 0, limit)
	var current *models.GitActivityEntry
	for i := 0; i < len(records); {
		if records[i] == "" {
			i++
			continue
		}
		if len(entries) < limit && i+3 < len(records) && len(records[i]) == 40 {
			committedAt, err := time.Parse(time.RFC3339, records[i+1])
			if err != nil {
				return nil, err
			}
			entries = append(entries, models.GitActivityEntry{Commit: records[i], CommittedAt: committedAt, Author: records[i+2], Message: records[i+3], Paths: []models.GitActivityPath{}})
			current = &entries[len(entries)-1]
			i += 4
			continue
		}
		if current == nil {
			return nil, fmt.Errorf("malformed NUL-delimited git activity")
		}
		status := activityStatus(records[i])
		i++
		if i >= len(records) {
			return nil, fmt.Errorf("malformed git activity path")
		}
		pathValue := records[i]
		i++
		if pathValue == "" {
			return nil, fmt.Errorf("malformed git activity path")
		}
		entry := models.GitActivityPath{Path: pathValue, Status: status}
		if status == models.GitChangeRenamed || status == models.GitChangeCopied {
			if i >= len(records) {
				return nil, fmt.Errorf("malformed git activity rename")
			}
			entry.OldPath, entry.Path = pathValue, records[i]
			if entry.Path == "" {
				return nil, fmt.Errorf("malformed git activity rename")
			}
			i++
		}
		current.Paths = append(current.Paths, entry)
	}
	return entries, nil
}

func (g *GitAdapter) PathStates(workspaceID, workspacePath string) ([]models.WorkspacePathGitState, error) {
	status, err := g.Status(workspaceID, workspacePath)
	if err != nil {
		return nil, err
	}
	states := make([]models.WorkspacePathGitState, 0, len(status.Changes))
	for _, change := range status.Changes {
		states = append(states, models.WorkspacePathGitState{
			Path: change.Path, OldPath: change.OldPath, Status: change.Status, Staged: change.Staged, Conflict: change.Conflict,
		})
	}
	return states, nil
}

func (g *GitAdapter) Fetch(workspacePath string) error {
	_, err := g.run(workspacePath, "fetch", "--all", "--prune")
	return err
}
func (g *GitAdapter) FetchContext(ctx context.Context, workspacePath string) error {
	_, err := g.runContextLimited(ctx, workspacePath, 1<<20, "fetch", "--all", "--prune")
	return err
}

func (g *GitAdapter) Pull(workspacePath string) error {
	_, err := g.run(workspacePath, "pull", "--ff-only")
	return err
}
func (g *GitAdapter) PullContext(ctx context.Context, workspacePath string) error {
	_, err := g.runContextLimited(ctx, workspacePath, 1<<20, "pull", "--ff-only")
	return err
}

func (g *GitAdapter) Push(workspacePath string) error {
	_, err := g.run(workspacePath, "push")
	return err
}
func (g *GitAdapter) PushContext(ctx context.Context, workspacePath string) error {
	_, err := g.runContextLimited(ctx, workspacePath, 1<<20, "push")
	return err
}

func (g *GitAdapter) Commit(workspacePath, message string, paths []string) error {
	return g.CommitContext(context.Background(), workspacePath, message, paths)
}

func (g *GitAdapter) CommitContext(ctx context.Context, workspacePath, message string, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}
	args := append([]string{"add", "--"}, paths...)
	if _, err := g.runContextLimited(ctx, workspacePath, 1<<20, args...); err != nil {
		return err
	}
	_, err := g.runContextLimited(ctx, workspacePath, 1<<20, "commit", "-m", message)
	if err != nil {
		return partialOperationError{err: err, recovery: "Selected paths were staged but the commit did not complete; inspect the Git index before retrying."}
	}
	return nil
}

func (g *GitAdapter) RevertPaths(workspacePath string, paths []string) error {
	return g.RevertPathsContext(context.Background(), workspacePath, paths)
}

func (g *GitAdapter) RevertPathsContext(ctx context.Context, workspacePath string, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}
	args := append([]string{"restore", "--source=HEAD", "--staged", "--worktree", "--"}, paths...)
	_, err := g.runContextLimited(ctx, workspacePath, 1<<20, args...)
	return err
}

func (g *GitAdapter) CreateBranch(workspacePath, name, startPoint string, checkout bool) error {
	return g.CreateBranchContext(context.Background(), workspacePath, name, startPoint, checkout)
}

func (g *GitAdapter) CreateBranchContext(ctx context.Context, workspacePath, name, startPoint string, checkout bool) error {
	args := []string{"branch", name}
	if strings.TrimSpace(startPoint) != "" {
		args = append(args, startPoint)
	}
	if _, err := g.runContextLimited(ctx, workspacePath, 1<<20, args...); err != nil {
		return err
	}
	if checkout {
		_, err := g.runContextLimited(ctx, workspacePath, 1<<20, "switch", name)
		if err != nil {
			return partialOperationError{err: err, recovery: "The branch was created but checkout did not complete; switch to it manually when ready."}
		}
	}
	return nil
}

type partialOperationError struct {
	err      error
	recovery string
}

func (e partialOperationError) Error() string { return e.err.Error() }
func (e partialOperationError) Unwrap() error { return e.err }

func (g *GitAdapter) SwitchBranch(workspacePath, name string) error {
	_, err := g.SwitchBranchSafely(workspacePath, name, "carry", "")
	return err
}

// SwitchBranchSafely serializes the decision, optional stash, and checkout so a
// branch can never be switched using a stale dirty-tree assessment.
func (g *GitAdapter) SwitchBranchSafely(workspacePath, name, strategy, stashMessage string) (string, error) {
	return g.SwitchBranchSafelyContext(context.Background(), workspacePath, name, strategy, stashMessage)
}

func (g *GitAdapter) SwitchBranchSafelyContext(ctx context.Context, workspacePath, name, strategy, stashMessage string) (string, error) {
	var stashRef string
	err := func() error {
		status, err := g.StatusContext(ctx, "", workspacePath)
		if err != nil {
			return err
		}
		if status.Conflicted {
			return fmt.Errorf("working tree has conflicts; resolve or abort the current Git operation before switching branches")
		}
		if status.Dirty {
			switch strategy {
			case "stash":
				if strings.TrimSpace(stashMessage) == "" {
					return fmt.Errorf("stash message is required before switching branches")
				}
				if _, err := g.runContextLimited(ctx, workspacePath, 1<<20, "stash", "push", "--include-untracked", "--message", stashMessage); err != nil {
					return err
				}
				ref, err := g.runContextLimited(ctx, workspacePath, 1<<20, "stash", "list", "-1", "--format=%gd")
				if err != nil {
					return err
				}
				stashRef = strings.TrimSpace(string(ref))
			case "carry":
				ok, err := g.CanCarryChangesContext(ctx, workspacePath, name, status)
				if err != nil {
					return err
				}
				if !ok {
					return ErrCarryChangesUnsafe
				}
			default:
				return ErrBranchSwitchDecisionRequired
			}
		}
		_, err = g.runContextLimited(ctx, workspacePath, 1<<20, "switch", name)
		return err
	}()
	return stashRef, err
}

// CanCarryChanges is deliberately conservative.  It permits a normal checkout
// only when the target does not touch a tracked local path and cannot collide
// with an untracked path.
func (g *GitAdapter) CanCarryChanges(workspacePath, target string, status models.GitStatus) (bool, error) {
	return g.CanCarryChangesContext(context.Background(), workspacePath, target, status)
}

func (g *GitAdapter) CanCarryChangesContext(ctx context.Context, workspacePath, target string, status models.GitStatus) (bool, error) {
	changedBytes, err := g.runContextLimited(ctx, workspacePath, 4<<20, "diff", "--name-only", "-z", "HEAD", target, "--")
	if err != nil {
		return false, err
	}
	targetChanged := splitNULPaths(string(changedBytes))
	treeBytes, err := g.runContextLimited(ctx, workspacePath, 4<<20, "ls-tree", "-r", "-z", "--name-only", target)
	if err != nil {
		return false, err
	}
	targetPaths := splitNULPaths(string(treeBytes))
	for _, change := range status.Changes {
		paths := []string{change.Path, change.OldPath}
		for _, local := range paths {
			if local == "" {
				continue
			}
			if change.Status == models.GitChangeUntracked {
				for _, remote := range targetPaths {
					if pathsOverlap(local, remote) {
						return false, nil
					}
				}
				continue
			}
			for _, remote := range targetChanged {
				if pathsOverlap(local, remote) {
					return false, nil
				}
			}
		}
	}
	return true, nil
}

func splitNULPaths(value string) []string {
	records := strings.Split(value, "\x00")
	out := make([]string, 0, len(records))
	for _, record := range records {
		if record != "" {
			out = append(out, record)
		}
	}
	return out
}

func pathsOverlap(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

func (g *GitAdapter) WithWorkspaceMutation(workspacePath string, action func() error) error {
	return g.WithWorkspaceMutationContext(context.Background(), workspacePath, action)
}

func (g *GitAdapter) WithWorkspaceMutationContext(ctx context.Context, workspacePath string, action func() error) error {
	key := filepath.Clean(workspacePath)
	if absolute, err := filepath.Abs(key); err == nil {
		key = absolute
	}
	if resolved, err := filepath.EvalSymlinks(key); err == nil {
		key = resolved
	}
	value, loaded := g.mutations.LoadOrStore(key, make(chan struct{}, 1))
	lock := value.(chan struct{})
	if !loaded {
		lock <- struct{}{}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-lock:
	}
	defer func() { lock <- struct{}{} }()
	return action()
}

func (g *GitAdapter) Clone(remoteURL, destination string) error {
	_, err := g.CloneWithLog(remoteURL, destination)
	return err
}

func (g *GitAdapter) CloneWithLog(remoteURL, destination string) (string, error) {
	var builder strings.Builder
	err := g.CloneWithProgress(remoteURL, destination, func(chunk string) {
		builder.WriteString(chunk)
	})
	return strings.TrimSpace(builder.String()), err
}

func (g *GitAdapter) CloneWithProgress(remoteURL, destination string, onChunk func(string)) error {
	cleanRemote := strings.TrimSpace(remoteURL)
	cleanDestination := strings.TrimSpace(destination)
	if cleanRemote == "" {
		return fmt.Errorf("remote URL is required")
	}
	if cleanDestination == "" {
		return fmt.Errorf("clone destination is required")
	}
	parent := filepath.Dir(cleanDestination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stdout := &limitedBuffer{limit: 4 << 20}
	stderr := &limitedBuffer{limit: 1 << 20}
	stream := &chunkStreamWriter{onChunk: onChunk}
	ctx, cancel := context.WithTimeout(context.Background(), g.cloneTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--progress", "--", cleanRemote, filepath.Base(cleanDestination))
	cmd.Dir = parent
	cmd.Stdout = io.MultiWriter(stdout, stream)
	cmd.Stderr = io.MultiWriter(stderr, stream)
	err := cmd.Run()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("git command timed out after %s", g.cloneTimeout)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func (g *GitAdapter) run(dir string, args ...string) (string, error) {
	return g.runIn(dir, args...)
}

func (g *GitAdapter) runIn(dir string, args ...string) (string, error) {
	return g.runWithTimeout(dir, g.timeout, args...)
}

func (g *GitAdapter) runWithTimeout(dir string, timeout time.Duration, args ...string) (string, error) {
	stdout, _, err := g.runWithTimeoutDetailed(dir, timeout, args...)
	if err != nil {
		return "", err
	}
	return stdout, nil
}

func (g *GitAdapter) runWithTimeoutDetailed(dir string, timeout time.Duration, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	stdout := &limitedBuffer{limit: 4 << 20}
	stderr := &limitedBuffer{limit: 1 << 20}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return stdout.String(), stderr.String(), fmt.Errorf("git command timed out after %s", timeout)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		if errors.Is(err, ErrOutputLimit) {
			return stdout.String(), stderr.String(), ErrOutputLimit
		}
		return stdout.String(), stderr.String(), fmt.Errorf("%s", msg)
	}
	return stdout.String(), stderr.String(), nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit int64
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if int64(b.Len()+len(p)) > b.limit {
		return 0, ErrOutputLimit
	}
	return b.Buffer.Write(p)
}

func (g *GitAdapter) runContextLimited(ctx context.Context, dir string, limit int64, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	stdout := &limitedBuffer{limit: limit}
	stderr := &limitedBuffer{limit: 1 << 20}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, ctx.Err()
		}
		if errors.Is(err, ErrOutputLimit) {
			return nil, ErrOutputLimit
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, errors.New(message)
	}
	return stdout.Bytes(), nil
}

type chunkStreamWriter struct {
	onChunk func(string)
}

func (w *chunkStreamWriter) Write(p []byte) (int, error) {
	if w.onChunk != nil && len(p) > 0 {
		chunk := strings.ReplaceAll(string(p), "\r", "\n")
		w.onChunk(chunk)
	}
	return len(p), nil
}

func cleanGitTreePath(relPath string) (string, error) {
	clean := path.Clean(strings.TrimSpace(filepath.ToSlash(relPath)))
	if clean == "." {
		return "", nil
	}
	if strings.HasPrefix(clean, "../") || clean == ".." || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("invalid git tree path %q", relPath)
	}
	return clean, nil
}

func treeish(ref, relPath string) string {
	if strings.TrimSpace(relPath) == "" {
		return ref + ":"
	}
	return ref + ":" + relPath
}

func parseTreeEntries(root, out string) ([]TreeEntry, error) {
	if strings.Trim(out, "\x00\n\t ") == "" {
		return []TreeEntry{}, nil
	}
	records := strings.Split(out, "\x00")
	entries := make([]TreeEntry, 0, len(records))
	for _, record := range records {
		if record == "" {
			continue
		}
		meta, entryPath, ok := strings.Cut(record, "\t")
		if !ok {
			return nil, fmt.Errorf("invalid ls-tree record %q", record)
		}
		fields := strings.Fields(meta)
		if len(fields) < 4 {
			return nil, fmt.Errorf("invalid ls-tree metadata %q", meta)
		}
		size := int64(0)
		if fields[3] != "-" {
			parsed, err := strconv.ParseInt(fields[3], 10, 64)
			if err != nil {
				return nil, err
			}
			size = parsed
		}
		fullPath := path.Clean(path.Join(root, entryPath))
		if root == "" {
			fullPath = path.Clean(entryPath)
		}
		mode := fs.FileMode(0)
		if fields[1] == "tree" {
			mode = fs.ModeDir
		}
		entries = append(entries, TreeEntry{
			Name:   path.Base(fullPath),
			Path:   fullPath,
			Type:   mode,
			Mode:   fields[0],
			Object: fields[2],
			Size:   size,
		})
	}
	return entries, nil
}

func parseBranchLine(status *models.GitStatus, line string) {
	branchPart := line
	if before, after, ok := strings.Cut(line, "..."); ok {
		branchPart = before
		upstream := after
		if beforeBracket, _, ok := strings.Cut(upstream, " ["); ok {
			upstream = beforeBracket
		}
		status.Upstream = strings.TrimSpace(upstream)
	}
	if branchPart != "" && !strings.HasPrefix(branchPart, "HEAD ") {
		status.Branch = strings.TrimSpace(branchPart)
	}
	if match := regexp.MustCompile(`ahead (\d+)`).FindStringSubmatch(line); len(match) == 2 {
		status.Ahead, _ = strconv.Atoi(match[1])
	}
	if match := regexp.MustCompile(`behind (\d+)`).FindStringSubmatch(line); len(match) == 2 {
		status.Behind, _ = strconv.Atoi(match[1])
	}
}

func parseChangeLine(line string) models.GitChange {
	if len(line) < 4 {
		return models.GitChange{}
	}
	x, y := line[0], line[1]
	path := line[3:]
	change := models.GitChange{
		Path:     path,
		Status:   changeStatus(x, y),
		Staged:   x != ' ' && x != '?',
		Conflict: isConflict(x, y),
	}
	if change.Conflict {
		change.Status = models.GitChangeConflicted
	}
	return change
}

func parseActivityChunk(chunk string) (models.GitActivityEntry, bool) {
	lines := strings.Split(strings.TrimSpace(chunk), "\n")
	if len(lines) == 0 {
		return models.GitActivityEntry{}, false
	}
	meta := strings.SplitN(strings.TrimSpace(lines[0]), "\x1f", 4)
	if len(meta) < 4 {
		return models.GitActivityEntry{}, false
	}
	committedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(meta[1]))
	if err != nil {
		committedAt = time.Time{}
	}
	entry := models.GitActivityEntry{
		Commit:      strings.TrimSpace(meta[0]),
		CommittedAt: committedAt,
		Author:      strings.TrimSpace(meta[2]),
		Message:     strings.TrimSpace(meta[3]),
		Paths:       []models.GitActivityPath{},
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		statusToken, payload, found := strings.Cut(line, "\t")
		if !found {
			continue
		}
		status := activityStatus(statusToken)
		parts := strings.Split(payload, "\t")
		pathValue := strings.TrimSpace(parts[len(parts)-1])
		if pathValue == "" {
			continue
		}
		pathEntry := models.GitActivityPath{Path: pathValue, Status: status}
		if (status == models.GitChangeRenamed || status == models.GitChangeCopied) && len(parts) > 1 {
			pathEntry.OldPath = strings.TrimSpace(parts[0])
		}
		entry.Paths = append(entry.Paths, pathEntry)
	}
	return entry, true
}

func activityStatus(code string) models.GitChangeStatus {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return models.GitChangeModified
	}
	switch trimmed[0] {
	case 'A':
		return models.GitChangeAdded
	case 'D':
		return models.GitChangeDeleted
	case 'R':
		return models.GitChangeRenamed
	case 'C':
		return models.GitChangeCopied
	default:
		return models.GitChangeModified
	}
}

func changeStatus(x, y byte) models.GitChangeStatus {
	if x == '?' && y == '?' {
		return models.GitChangeUntracked
	}
	code := y
	if code == ' ' {
		code = x
	}
	switch code {
	case 'A':
		return models.GitChangeAdded
	case 'D':
		return models.GitChangeDeleted
	case 'R':
		return models.GitChangeRenamed
	case 'C':
		return models.GitChangeCopied
	default:
		return models.GitChangeModified
	}
}

func isConflict(x, y byte) bool {
	return x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D') || (x == 'A' && y == 'D') || (x == 'D' && y == 'A')
}

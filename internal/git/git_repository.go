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
	out, err := g.run(workspacePath, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(out)
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
	out, err := g.run(workspacePath, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

func (g *GitAdapter) ListStashes(workspacePath string) ([]models.GitStashEntry, error) {
	out, err := g.run(workspacePath, "stash", "list", "--format=%gd%x1f%s")
	if err != nil {
		return nil, err
	}
	entries := make([]models.GitStashEntry, 0)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, "\x1f", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		entries = append(entries, models.GitStashEntry{Ref: strings.TrimSpace(parts[0]), Message: strings.TrimSpace(parts[1])})
	}
	return entries, nil
}

func (g *GitAdapter) ApplyStash(workspacePath, ref string) error {
	if !regexp.MustCompile(`^stash@\{[0-9]+\}$`).MatchString(strings.TrimSpace(ref)) {
		return fmt.Errorf("invalid stash reference")
	}
	return g.WithWorkspaceMutation(workspacePath, func() error { _, err := g.run(workspacePath, "stash", "apply", "--index", ref); return err })
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

func (g *GitAdapter) Diff(workspacePath, relPath string) (string, error) {
	return g.run(workspacePath, "diff", "--no-ext-diff", "--", relPath)
}

func (g *GitAdapter) Status(workspaceID, workspacePath string) (models.GitStatus, error) {
	branch, _ := g.CurrentBranch(workspacePath)
	out, err := g.run(workspacePath, "status", "--porcelain=v1", "-b")
	if err != nil {
		return models.GitStatus{}, err
	}
	status := models.GitStatus{WorkspaceID: workspaceID, Branch: branch, Changes: []models.GitChange{}}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			parseBranchLine(&status, strings.TrimPrefix(line, "## "))
			continue
		}
		change := parseChangeLine(line)
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
	if limit <= 0 {
		limit = 12
	}
	args := []string{"log", "--date=iso-strict", "--name-status", "--pretty=format:%x1e%H%x1f%cI%x1f%an%x1f%s", "-n", strconv.Itoa(limit)}
	if strings.TrimSpace(relPath) != "" {
		clean, err := cleanGitTreePath(relPath)
		if err != nil {
			return nil, err
		}
		args = append(args, "--", clean)
	}
	out, err := g.run(workspacePath, args...)
	if err != nil {
		message := err.Error()
		if strings.Contains(message, "does not exist in") || strings.Contains(message, "unknown revision or path not in the working tree") {
			return []models.GitActivityEntry{}, nil
		}
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return []models.GitActivityEntry{}, nil
	}
	entries := make([]models.GitActivityEntry, 0, limit)
	for _, raw := range strings.Split(out, "\x1e") {
		chunk := strings.TrimSpace(raw)
		if chunk == "" {
			continue
		}
		entry, ok := parseActivityChunk(chunk)
		if !ok {
			continue
		}
		entries = append(entries, entry)
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

func (g *GitAdapter) Pull(workspacePath string) error {
	_, err := g.run(workspacePath, "pull", "--ff-only")
	return err
}

func (g *GitAdapter) Push(workspacePath string) error {
	_, err := g.run(workspacePath, "push")
	return err
}

func (g *GitAdapter) Commit(workspacePath, message string, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}
	args := append([]string{"add", "--"}, paths...)
	if _, err := g.run(workspacePath, args...); err != nil {
		return err
	}
	_, err := g.run(workspacePath, "commit", "-m", message)
	return err
}

func (g *GitAdapter) RevertPaths(workspacePath string, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}
	args := append([]string{"restore", "--source=HEAD", "--staged", "--worktree", "--"}, paths...)
	_, err := g.run(workspacePath, args...)
	return err
}

func (g *GitAdapter) CreateBranch(workspacePath, name, startPoint string, checkout bool) error {
	args := []string{"branch", name}
	if strings.TrimSpace(startPoint) != "" {
		args = append(args, startPoint)
	}
	if _, err := g.run(workspacePath, args...); err != nil {
		return err
	}
	if checkout {
		return g.SwitchBranch(workspacePath, name)
	}
	return nil
}

func (g *GitAdapter) SwitchBranch(workspacePath, name string) error {
	_, err := g.SwitchBranchSafely(workspacePath, name, "carry", "")
	return err
}

// SwitchBranchSafely serializes the decision, optional stash, and checkout so a
// branch can never be switched using a stale dirty-tree assessment.
func (g *GitAdapter) SwitchBranchSafely(workspacePath, name, strategy, stashMessage string) (string, error) {
	var stashRef string
	err := g.WithWorkspaceMutation(workspacePath, func() error {
		status, err := g.Status("", workspacePath)
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
				if _, err := g.run(workspacePath, "stash", "push", "--include-untracked", "--message", stashMessage); err != nil {
					return err
				}
				ref, err := g.run(workspacePath, "stash", "list", "-1", "--format=%gd")
				if err != nil {
					return err
				}
				stashRef = strings.TrimSpace(ref)
			case "carry":
				ok, err := g.CanCarryChanges(workspacePath, name, status)
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
		_, err = g.run(workspacePath, "switch", name)
		return err
	})
	return stashRef, err
}

// CanCarryChanges is deliberately conservative.  It permits a normal checkout
// only when the target does not touch a tracked local path and cannot collide
// with an untracked path.
func (g *GitAdapter) CanCarryChanges(workspacePath, target string, status models.GitStatus) (bool, error) {
	changed, err := g.run(workspacePath, "diff", "--name-only", "HEAD", target, "--")
	if err != nil {
		return false, err
	}
	targetChanged := strings.Fields(changed)
	tree, err := g.run(workspacePath, "ls-tree", "-r", "--name-only", target)
	if err != nil {
		return false, err
	}
	targetPaths := strings.Fields(tree)
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

func pathsOverlap(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

func (g *GitAdapter) WithWorkspaceMutation(workspacePath string, action func() error) error {
	key := filepath.Clean(workspacePath)
	if absolute, err := filepath.Abs(key); err == nil {
		key = absolute
	}
	if resolved, err := filepath.EvalSymlinks(key); err == nil {
		key = resolved
	}
	value, _ := g.mutations.LoadOrStore(key, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
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
	var stdout, stderr bytes.Buffer
	stream := &chunkStreamWriter{onChunk: onChunk}
	ctx, cancel := context.WithTimeout(context.Background(), g.cloneTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--progress", "--", cleanRemote, filepath.Base(cleanDestination))
	cmd.Dir = parent
	cmd.Stdout = io.MultiWriter(&stdout, stream)
	cmd.Stderr = io.MultiWriter(&stderr, stream)
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
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return stdout.String(), stderr.String(), fmt.Errorf("git command timed out after %s", timeout)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), stderr.String(), fmt.Errorf("%s", msg)
	}
	return stdout.String(), stderr.String(), nil
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
	path := strings.TrimSpace(line[3:])
	change := models.GitChange{
		Path:     path,
		Status:   changeStatus(x, y),
		Staged:   x != ' ' && x != '?',
		Conflict: isConflict(x, y),
	}
	if strings.Contains(path, " -> ") {
		oldPath, newPath, _ := strings.Cut(path, " -> ")
		change.OldPath = strings.TrimSpace(oldPath)
		change.Path = strings.TrimSpace(newPath)
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

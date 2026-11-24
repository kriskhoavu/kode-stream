package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const worktreeTimeout = 2 * time.Minute

type DiffFile struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

func (g *GitAdapter) runLong(ctx context.Context, dir string, timeout time.Duration, args ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	stdout := &limitedBuffer{limit: 8 << 20}
	stderr := &limitedBuffer{limit: 1 << 20}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", fmt.Errorf("git %s: %w", args[0], ctxErr)
		}
		if errors.Is(err, ErrOutputLimit) {
			return "", ErrOutputLimit
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", errors.New(message)
	}
	return stdout.String(), nil
}

func (g *GitAdapter) RevParseContext(ctx context.Context, repoPath, ref string) (string, error) {
	out, err := g.runLong(ctx, repoPath, worktreeTimeout, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("cannot resolve %s: %w", ref, err)
	}
	return strings.TrimSpace(out), nil
}

func (g *GitAdapter) FetchRemoteContext(ctx context.Context, repoPath, remote string) error {
	_, err := g.runLong(ctx, repoPath, worktreeTimeout, "fetch", "--prune", remote)
	return err
}

func (g *GitAdapter) HasRemoteContext(ctx context.Context, repoPath, remote string) bool {
	out, err := g.runLong(ctx, repoPath, worktreeTimeout, "remote")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == remote {
			return true
		}
	}
	return false
}

func (g *GitAdapter) WorktreeAddContext(ctx context.Context, repoPath, worktreePath, branch, startPoint string) error {
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return err
	}
	_, err := g.runLong(ctx, repoPath, worktreeTimeout, "worktree", "add", "-b", branch, worktreePath, startPoint)
	return err
}

func (g *GitAdapter) WorktreeRemoveContext(ctx context.Context, repoPath, worktreePath string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	_, err := g.runLong(ctx, repoPath, worktreeTimeout, append(args, worktreePath)...)
	return err
}

func (g *GitAdapter) WorktreePruneContext(ctx context.Context, repoPath string) error {
	_, err := g.runLong(ctx, repoPath, worktreeTimeout, "worktree", "prune")
	return err
}

func (g *GitAdapter) DeleteBranchContext(ctx context.Context, repoPath, branch string) error {
	_, err := g.runLong(ctx, repoPath, worktreeTimeout, "branch", "-D", branch)
	return err
}

func (g *GitAdapter) ExcludeContext(ctx context.Context, repoPath, pattern string) error {
	out, err := g.runLong(ctx, repoPath, worktreeTimeout, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	excludePath := filepath.Join(strings.TrimSpace(out), "info", "exclude")
	existing, err := os.ReadFile(excludePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == pattern {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return err
	}
	prefix := ""
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		prefix = "\n"
	}
	file, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(prefix + pattern + "\n")
	return err
}

// git add fails at once on a held index lock, so readers of one worktree take turns.
var indexLocks sync.Map

// Another git process, such as an agent's or a person's, may hold the index lock.
const indexLockRetries = 10

func (g *GitAdapter) intendToAdd(ctx context.Context, worktreePath string) error {
	lock, _ := indexLocks.LoadOrStore(filepath.Clean(worktreePath), &sync.Mutex{})
	mutex := lock.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()
	for attempt := 0; ; attempt++ {
		_, err := g.runLong(ctx, worktreePath, worktreeTimeout, "add", "--intent-to-add", "--all")
		if err == nil || attempt >= indexLockRetries || !strings.Contains(err.Error(), "index.lock") {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (g *GitAdapter) DiffStatContext(ctx context.Context, worktreePath, base string) (string, error) {
	if err := g.intendToAdd(ctx, worktreePath); err != nil {
		return "", err
	}
	return g.runLong(ctx, worktreePath, worktreeTimeout, "diff", "--stat", base)
}

func (g *GitAdapter) ChangedPathsContext(ctx context.Context, worktreePath, base string) ([]string, error) {
	files, err := g.DiffFilesContext(ctx, worktreePath, base)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	return paths, nil
}

func (g *GitAdapter) DiffFilesContext(ctx context.Context, worktreePath, base string) ([]DiffFile, error) {
	if err := g.intendToAdd(ctx, worktreePath); err != nil {
		return nil, err
	}
	statusOut, err := g.runLong(ctx, worktreePath, worktreeTimeout, "diff", "--name-status", "-z", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	numstatOut, err := g.runLong(ctx, worktreePath, worktreeTimeout, "diff", "--numstat", "-z", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	counts := numstatCounts(numstatOut)
	parts := strings.Split(strings.TrimSuffix(statusOut, "\x00"), "\x00")
	files := []DiffFile{}
	for i := 0; i+1 < len(parts); i += 2 {
		status, path := parts[i], parts[i+1]
		if path == "" {
			continue
		}
		count := counts[path]
		files = append(files, DiffFile{Path: path, Status: diffStatus(status), Additions: count[0], Deletions: count[1]})
	}
	return files, nil
}

func diffStatus(code string) string {
	switch {
	case strings.HasPrefix(code, "A"):
		return "added"
	case strings.HasPrefix(code, "D"):
		return "deleted"
	default:
		return "modified"
	}
}

func (g *GitAdapter) RestorePathsContext(ctx context.Context, worktreePath, base string, paths []string) error {
	for _, path := range paths {
		clean := filepath.Clean(path)
		if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return fmt.Errorf("path %q escapes the worktree", path)
		}
		if _, err := g.runLong(ctx, worktreePath, worktreeTimeout, "cat-file", "-e", base+":"+filepath.ToSlash(clean)); err == nil {
			if _, err := g.runLong(ctx, worktreePath, worktreeTimeout, "checkout", base, "--", clean); err != nil {
				return err
			}
			continue
		}
		if _, err := g.runLong(ctx, worktreePath, worktreeTimeout, "rm", "-q", "--cached", "--ignore-unmatch", "--", clean); err != nil {
			return err
		}
		if err := os.RemoveAll(filepath.Join(worktreePath, clean)); err != nil {
			return err
		}
	}
	return nil
}

func (g *GitAdapter) CommitAllContext(ctx context.Context, worktreePath, message string) (string, error) {
	if _, err := g.runLong(ctx, worktreePath, worktreeTimeout, "add", "--all"); err != nil {
		return "", err
	}
	status, err := g.runLong(ctx, worktreePath, worktreeTimeout, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(status) == "" {
		return "", nil
	}
	if _, err := g.runLong(ctx, worktreePath, worktreeTimeout, "commit", "--no-verify", "-m", message); err != nil {
		return "", err
	}
	return g.RevParseContext(ctx, worktreePath, "HEAD")
}

func (g *GitAdapter) PushBranchContext(ctx context.Context, worktreePath, remote, branch, remoteBranch string) error {
	_, err := g.runLong(ctx, worktreePath, worktreeTimeout, "push", "--set-upstream", remote, branch+":"+remoteBranch)
	return err
}

func (g *GitAdapter) RefDiffFilesContext(ctx context.Context, repoPath, base, ref, prefix string) ([]DiffFile, error) {
	args := []string{"diff", "--name-status", "-z", "--no-renames", base, ref}
	if prefix != "" {
		args = append(args, "--", prefix)
	}
	statusOut, err := g.runLong(ctx, repoPath, worktreeTimeout, args...)
	if err != nil {
		return nil, err
	}
	numstatArgs := append([]string{"diff", "--numstat", "-z", "--no-renames", base, ref}, prefixArgs(prefix)...)
	numstatOut, err := g.runLong(ctx, repoPath, worktreeTimeout, numstatArgs...)
	if err != nil {
		return nil, err
	}
	counts := numstatCounts(numstatOut)
	parts := strings.Split(strings.TrimSuffix(statusOut, "\x00"), "\x00")
	files := []DiffFile{}
	for i := 0; i+1 < len(parts); i += 2 {
		if parts[i+1] != "" {
			count := counts[parts[i+1]]
			files = append(files, DiffFile{Path: parts[i+1], Status: diffStatus(parts[i]), Additions: count[0], Deletions: count[1]})
		}
	}
	return files, nil
}

func prefixArgs(prefix string) []string {
	if prefix == "" {
		return nil
	}
	return []string{"--", prefix}
}

func numstatCounts(out string) map[string][2]int {
	counts := map[string][2]int{}
	for _, record := range strings.Split(out, "\x00") {
		fields := strings.SplitN(record, "\t", 3)
		if len(fields) != 3 {
			continue
		}
		additions, _ := strconv.Atoi(fields[0])
		deletions, _ := strconv.Atoi(fields[1])
		counts[fields[2]] = [2]int{additions, deletions}
	}
	return counts
}

func (g *GitAdapter) RefFileDiffContext(ctx context.Context, repoPath, base, ref, relPath string, maxBytes int64) (string, bool, error) {
	clean, err := cleanGitTreePath(relPath)
	if err != nil {
		return "", false, err
	}
	if clean == "" || strings.HasPrefix(clean, ":") || strings.HasPrefix(clean, "-") {
		return "", false, fmt.Errorf("invalid diff path %q", relPath)
	}
	for _, rev := range []string{base, ref} {
		if rev == "" || strings.HasPrefix(rev, "-") {
			return "", false, fmt.Errorf("invalid revision %q", rev)
		}
	}
	return g.boundedDiff(ctx, repoPath, maxBytes, "diff", "--no-ext-diff", "--no-renames", base, ref, "--", clean)
}

func (g *GitAdapter) ShowFileContext(ctx context.Context, repoPath, ref, path string, limit int64) ([]byte, bool, error) {
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return nil, false, fmt.Errorf("path %q escapes the repository", path)
	}
	object := ref + ":" + clean
	size, err := g.runLong(ctx, repoPath, worktreeTimeout, "cat-file", "-s", object)
	if err != nil {
		return nil, false, err
	}
	if bytes, convErr := strconv.ParseInt(strings.TrimSpace(size), 10, 64); convErr == nil && bytes > limit {
		return nil, true, nil
	}
	data, err := g.runContextLimited(ctx, repoPath, limit+1, "cat-file", "blob", object)
	if err != nil {
		return nil, false, err
	}
	return data, false, nil
}

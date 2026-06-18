package gitadapter

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"plan-manager/internal/models"
)

type GitAdapter struct {
	timeout time.Duration
}

func New() *GitAdapter {
	return &GitAdapter{timeout: 5 * time.Second}
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

func (g *GitAdapter) BranchForIdentifier(workspacePath, ticket string) string {
	ticket = strings.ToLower(strings.TrimSpace(ticket))
	if ticket == "" {
		return ""
	}
	branches, err := g.ListBranches(workspacePath)
	if err != nil {
		return ""
	}
	for _, branch := range branches {
		if strings.Contains(strings.ToLower(branch), ticket) {
			return branch
		}
	}
	return ""
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

func (g *GitAdapter) Diff(workspacePath, relPath string) (string, error) {
	return g.run(workspacePath, "diff", "--", relPath)
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
	_, err := g.run(workspacePath, "switch", name)
	return err
}

func (g *GitAdapter) run(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), g.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return stdout.String(), nil
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

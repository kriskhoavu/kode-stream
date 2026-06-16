package gitadapter

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type GitAdapter struct {
	timeout time.Duration
}

func New() *GitAdapter {
	return &GitAdapter{timeout: 5 * time.Second}
}

func (g *GitAdapter) RepositoryRoot(path string) (string, error) {
	out, err := g.run(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.Clean(strings.TrimSpace(out)), nil
}

func (g *GitAdapter) ValidateBranch(repoPath, branch string) error {
	_, err := g.run(repoPath, "show-ref", "--verify", "refs/heads/"+branch)
	return err
}

func (g *GitAdapter) CurrentBranch(repoPath string) (string, error) {
	out, err := g.run(repoPath, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(out)
	if branch == "" {
		return "HEAD", nil
	}
	return branch, nil
}

func (g *GitAdapter) ListBranches(repoPath string) ([]string, error) {
	out, err := g.run(repoPath, "for-each-ref", "--format=%(refname:short)", "refs/heads")
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

func (g *GitAdapter) BranchForTicket(repoPath, ticket string) string {
	ticket = strings.ToLower(strings.TrimSpace(ticket))
	if ticket == "" {
		return ""
	}
	branches, err := g.ListBranches(repoPath)
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

func (g *GitAdapter) LastAuthor(repoPath, relPath string) string {
	out, err := g.run(repoPath, "log", "-1", "--format=%an", "--", relPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func (g *GitAdapter) LastUpdate(repoPath, relPath string) time.Time {
	out, err := g.run(repoPath, "log", "-1", "--format=%cI", "--", relPath)
	if err == nil {
		if t, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(out)); parseErr == nil {
			return t
		}
	}
	return time.Time{}
}

func (g *GitAdapter) Diff(repoPath, relPath string) (string, error) {
	return g.run(repoPath, "diff", "--", relPath)
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

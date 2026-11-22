package git

// Package gitadapter implements domain Git repository ports.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
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
	return g.ResolveBranchContext(context.Background(), workspacePath, branch)
}

func (g *GitAdapter) ResolveBranchContext(ctx context.Context, workspacePath, branch string) (string, string, error) {
	ref := "refs/heads/" + branch
	out, err := g.runContextLimited(ctx, workspacePath, 1<<20, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", "", err
	}
	return ref, strings.TrimSpace(string(out)), nil
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

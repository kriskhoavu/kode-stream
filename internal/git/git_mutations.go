package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"kode-stream/internal/common/models"
)

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

// Decide, stash and check out under one lock, so a stale dirty-tree assessment never switches a branch.
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

// Conservative: allow only a target that touches no tracked local path and no untracked path.
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

// The token goes in before the channel is published, so a second caller never waits on an empty lock.
func newMutationLock() chan struct{} {
	lock := make(chan struct{}, 1)
	lock <- struct{}{}
	return lock
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
	// Load first: LoadOrStore evaluates its argument even when it does not store.
	value, ok := g.mutations.Load(key)
	if !ok {
		value, _ = g.mutations.LoadOrStore(key, newMutationLock())
	}
	lock := value.(chan struct{})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-lock:
	}
	defer func() { lock <- struct{}{} }()
	return action()
}

package git

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kode-stream/internal/common/models"
)

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
	if status.Upstream != "" {
		status.FetchedAt = g.lastFetchedAtContext(ctx, workspacePath)
	}
	return status, nil
}

// lastFetchedAtContext dates the most recent fetch by FETCH_HEAD's modification
// time. A repository that has never fetched since being cloned has no such file,
// which reports as the zero time rather than an error: an unknown fetch age is a
// fact about the remote-tracking ref, not a failure to read the status.
func (g *GitAdapter) lastFetchedAtContext(ctx context.Context, workspacePath string) time.Time {
	if ctx.Err() != nil {
		return time.Time{}
	}
	// A linked worktree keeps a private git dir that holds no FETCH_HEAD; the
	// fetch is recorded once in the common dir every worktree shares. Ask for it
	// without --path-format, which needs Git 2.31, and resolve whatever comes back
	// against the workspace so a relative answer works too.
	out, err := g.runContextLimited(ctx, workspacePath, 1<<16, "rev-parse", "--git-common-dir")
	if err != nil {
		return time.Time{}
	}
	fetchHead := g.resolveFetchHead(workspacePath, strings.TrimSpace(string(out)))
	if fetchHead == "" {
		return time.Time{}
	}
	info, err := os.Stat(fetchHead)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime().UTC()
}

// resolveFetchHead turns a git-common-dir answer, absolute or relative, into the
// FETCH_HEAD path it implies. It returns "" when there is nothing to resolve.
func (g *GitAdapter) resolveFetchHead(workspacePath, commonDir string) string {
	if commonDir == "" {
		return ""
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(workspacePath, commonDir)
	}
	return filepath.Join(commonDir, "FETCH_HEAD")
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

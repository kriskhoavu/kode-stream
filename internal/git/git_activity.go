package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"kode-stream/internal/common/models"
)

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

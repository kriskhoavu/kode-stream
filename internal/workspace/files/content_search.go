package workspacefiles

// Package workspacefiles provides Workspace-owned file operations.

// Package workspacefiles provides bounded workspace file operations.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/content"
)

const (
	MinContentSearchQueryLength         = 2
	MaxContentSearchQueryLength         = 200
	MaxContentSearchResults             = 100
	MaxContentSearchFiles               = 10_000
	MaxContentSearchBytes         int64 = 64 << 20
	MaxContentSearchFileSize      int64 = 2 << 20
	MaxContentSearchSnippetLength       = 240
)

func DefaultContentSearchBudget() models.WorkspaceContentSearchBudget {
	return models.WorkspaceContentSearchBudget{
		MaxResults: MaxContentSearchResults, MaxFiles: MaxContentSearchFiles,
		MaxBytes: MaxContentSearchBytes, MaxFileSize: MaxContentSearchFileSize,
		MaxQueryLength: MaxContentSearchQueryLength, MaxSnippetLength: MaxContentSearchSnippetLength,
	}
}

func ValidateContentSearchQuery(query string, maxLength int) error {
	length := utf8.RuneCountInString(strings.TrimSpace(query))
	if length < MinContentSearchQueryLength {
		return fmt.Errorf("content search query must contain at least %d characters", MinContentSearchQueryLength)
	}
	if maxLength <= 0 {
		maxLength = MaxContentSearchQueryLength
	}
	if length > maxLength {
		return fmt.Errorf("content search query must contain at most %d characters", maxLength)
	}
	return nil
}

func (a *Access) ContentSearch(ctx context.Context, workspace models.WorkspaceConfig, roots []models.WorkspaceContentSearchRoot, request models.WorkspaceContentSearchRequest, budget *models.WorkspaceContentSearchBudget) (models.WorkspaceContentSearchResponse, error) {
	response := models.WorkspaceContentSearchResponse{Results: []models.WorkspaceContentSearchResult{}}
	if budget == nil {
		value := DefaultContentSearchBudget()
		budget = &value
	}
	applyContentSearchBudgetDefaults(budget)
	request.Query = strings.TrimSpace(request.Query)
	if err := ValidateContentSearchQuery(request.Query, budget.MaxQueryLength); err != nil {
		return response, err
	}
	for _, root := range roots {
		if err := a.searchContentRoot(ctx, workspace, root.Path, request, budget, &response); err != nil {
			return response, err
		}
		if response.Truncated {
			break
		}
	}
	response.FilesVisited = budget.FilesVisited
	response.BytesRead = budget.BytesRead
	return response, nil
}

func applyContentSearchBudgetDefaults(budget *models.WorkspaceContentSearchBudget) {
	defaults := DefaultContentSearchBudget()
	if budget.MaxResults <= 0 {
		budget.MaxResults = defaults.MaxResults
	}
	if budget.MaxFiles <= 0 {
		budget.MaxFiles = defaults.MaxFiles
	}
	if budget.MaxBytes <= 0 {
		budget.MaxBytes = defaults.MaxBytes
	}
	if budget.MaxFileSize <= 0 {
		budget.MaxFileSize = defaults.MaxFileSize
	}
	if budget.MaxQueryLength <= 0 {
		budget.MaxQueryLength = defaults.MaxQueryLength
	}
	if budget.MaxSnippetLength <= 0 {
		budget.MaxSnippetLength = defaults.MaxSnippetLength
	}
}

func (a *Access) searchContentRoot(ctx context.Context, workspace models.WorkspaceConfig, rootPath string, request models.WorkspaceContentSearchRequest, budget *models.WorkspaceContentSearchBudget, response *models.WorkspaceContentSearchResponse) error {
	cleanRoot, fullRoot, err := resolve(workspace.Path, rootPath, true)
	if err != nil {
		return err
	}
	type directory struct{ path, full string }
	queue := []directory{{cleanRoot, fullRoot}}
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		current := queue[0]
		queue = queue[1:]
		entries, err := os.ReadDir(current.full)
		if err != nil {
			response.SkippedFiles++
			continue
		}
		sort.SliceStable(entries, func(i, j int) bool { return naturalLess(entries[i].Name(), entries[j].Name()) })
		paths := make([]string, len(entries))
		for i, entry := range entries {
			paths[i] = joinRelative(current.path, entry.Name())
		}
		ignored := map[string]bool{}
		if a.ignore != nil && len(paths) > 0 {
			ignored, err = a.ignore.Ignored(workspace.Path, paths)
			if err != nil {
				return err
			}
		}
		for i, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			path := paths[i]
			if protected(path) || (ignored[path] && !request.IncludeIgnored) {
				continue
			}
			_, full, err := resolve(workspace.Path, path, entry.IsDir())
			if err != nil {
				continue
			}
			info, err := os.Stat(full)
			if err != nil {
				response.SkippedFiles++
				continue
			}
			if info.IsDir() {
				queue = append(queue, directory{path, full})
				continue
			}
			if !info.Mode().IsRegular() {
				response.SkippedFiles++
				continue
			}
			if budget.FilesVisited >= budget.MaxFiles {
				response.Truncated = true
				return nil
			}
			budget.FilesVisited++
			if info.Size() > budget.MaxFileSize {
				response.SkippedFiles++
				continue
			}
			if budget.BytesRead+info.Size() > budget.MaxBytes {
				response.Truncated = true
				return nil
			}
			data, changed, err := readStableContentFile(full, info)
			if err != nil || changed || fileaccess.IsBinary(data) {
				response.SkippedFiles++
				continue
			}
			budget.BytesRead += int64(len(data))
			classification := fileaccess.ClassifyPath(path)
			matches := contentLineMatches(string(data), request.Query, request.CaseSensitive, budget.MaxSnippetLength)
			for _, match := range matches {
				if budget.Results >= budget.MaxResults {
					response.Truncated = true
					return nil
				}
				match.ID = fmt.Sprintf("%s:%s:%d:%d", workspace.ID, workspaceFileID(path), match.LineNumber, match.ColumnStart)
				match.WorkspaceID, match.WorkspaceName = workspace.ID, workspace.Name
				match.Path, match.Name, match.Kind, match.Language, match.Ignored = path, filepath.Base(path), classification.Kind, classification.Language, ignored[path]
				response.Results = append(response.Results, match)
				budget.Results++
			}
		}
	}
	return nil
}

func readStableContentFile(path string, before os.FileInfo) ([]byte, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, before.Size()+1))
	if err != nil {
		return nil, false, err
	}
	after, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	changed := int64(len(data)) != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime())
	return data, changed, nil
}

func contentLineMatches(content, query string, caseSensitive bool, maxSnippet int) []models.WorkspaceContentSearchResult {
	var results []models.WorkspaceContentSearchResult
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	queryRunes := []rune(query)
	for lineIndex, line := range lines {
		lineRunes := []rune(strings.TrimSuffix(line, "\r"))
		for start := 0; start+len(queryRunes) <= len(lineRunes); start++ {
			candidate := string(lineRunes[start : start+len(queryRunes)])
			matched := candidate == query
			if !caseSensitive {
				matched = strings.EqualFold(candidate, query)
			}
			if !matched {
				continue
			}
			results = append(results, models.WorkspaceContentSearchResult{
				LineNumber: lineIndex + 1, ColumnStart: start + 1, ColumnEnd: start + len(queryRunes) + 1,
				Snippet: boundedSnippet(lineRunes, start, start+len(queryRunes), maxSnippet),
			})
			start += len(queryRunes) - 1
		}
	}
	return results
}

func boundedSnippet(line []rune, matchStart, matchEnd, limit int) string {
	if limit <= 0 || len(line) <= limit {
		return string(line)
	}
	start := matchStart - (limit-(matchEnd-matchStart))/2
	if start < 0 {
		start = 0
	}
	if start+limit > len(line) {
		start = len(line) - limit
	}
	end := start + limit
	prefix, suffix := "", ""
	if start > 0 {
		prefix = "…"
		start++
	}
	if end < len(line) {
		suffix = "…"
		end--
	}
	return prefix + string(line[start:end]) + suffix
}

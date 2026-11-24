package workspacefiles

import (
	"bytes"
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
	"kode-stream/internal/filesystem/fileid"
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
	search := &contentRootSearch{ctx: ctx, access: a, workspace: workspace, request: request, budget: budget, response: response}
	search.queue = []searchDirectory{{cleanRoot, fullRoot}}
	return search.run()
}

type searchDirectory struct{ path, full string }

type contentRootSearch struct {
	ctx       context.Context
	access    *Access
	workspace models.WorkspaceConfig
	request   models.WorkspaceContentSearchRequest
	budget    *models.WorkspaceContentSearchBudget
	response  *models.WorkspaceContentSearchResponse
	queue     []searchDirectory
}

func (s *contentRootSearch) run() error {
	for len(s.queue) > 0 {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		current := s.queue[0]
		s.queue = s.queue[1:]
		stop, err := s.visitDirectory(current)
		if stop || err != nil {
			return err
		}
	}
	return nil
}

func (s *contentRootSearch) visitDirectory(current searchDirectory) (bool, error) {
	entries, err := os.ReadDir(current.full)
	if err != nil {
		s.response.SkippedFiles++
		return false, nil
	}
	sort.SliceStable(entries, func(i, j int) bool { return naturalLess(entries[i].Name(), entries[j].Name()) })
	paths := make([]string, len(entries))
	for i, entry := range entries {
		paths[i] = joinRelative(current.path, entry.Name())
	}
	ignored := map[string]bool{}
	if s.access.ignore != nil && len(paths) > 0 {
		ignored, err = s.access.ignore.Ignored(s.workspace.Path, paths)
		if err != nil {
			return false, err
		}
	}
	for i, entry := range entries {
		if err := s.ctx.Err(); err != nil {
			return false, err
		}
		path := paths[i]
		if protected(path) || (ignored[path] && !s.request.IncludeIgnored) {
			continue
		}
		stop, err := s.visitEntry(path, entry.IsDir(), ignored[path])
		if stop || err != nil {
			return stop, err
		}
	}
	return false, nil
}

func (s *contentRootSearch) visitEntry(path string, isDir, ignored bool) (bool, error) {
	_, full, err := resolve(s.workspace.Path, path, isDir)
	if err != nil {
		return false, nil
	}
	info, err := os.Stat(full)
	if err != nil {
		s.response.SkippedFiles++
		return false, nil
	}
	if info.IsDir() {
		s.queue = append(s.queue, searchDirectory{path, full})
		return false, nil
	}
	if !info.Mode().IsRegular() {
		s.response.SkippedFiles++
		return false, nil
	}
	return s.searchFile(path, full, info, ignored)
}

func (s *contentRootSearch) searchFile(path, full string, info os.FileInfo, ignored bool) (bool, error) {
	budget, response := s.budget, s.response
	if budget.FilesVisited >= budget.MaxFiles {
		response.Truncated = true
		return true, nil
	}
	budget.FilesVisited++
	if info.Size() > budget.MaxFileSize {
		response.SkippedFiles++
		return false, nil
	}
	if budget.BytesRead+info.Size() > budget.MaxBytes {
		response.Truncated = true
		return true, nil
	}
	data, changed, err := readStableContentFile(full, info)
	if err != nil || changed || fileaccess.IsBinary(data) {
		response.SkippedFiles++
		return false, nil
	}
	budget.BytesRead += int64(len(data))
	classification := fileaccess.ClassifyPath(path)
	matches, exhausted, err := contentLineMatches(s.ctx, data, s.request.Query, s.request.CaseSensitive, budget.MaxSnippetLength, budget.MaxResults-budget.Results)
	if err != nil {
		return false, err
	}
	workspace := s.workspace
	for _, match := range matches {
		match.ID = fmt.Sprintf("%s:%s:%d:%d", workspace.ID, fileid.Encode(path), match.LineNumber, match.ColumnStart)
		match.WorkspaceID, match.WorkspaceName = workspace.ID, workspace.Name
		match.Path, match.Name, match.Kind, match.Language, match.Ignored = path, filepath.Base(path), classification.Kind, classification.Language, ignored
		response.Results = append(response.Results, match)
		budget.Results++
	}
	if exhausted {
		response.Truncated = true
		return true, nil
	}
	return false, nil
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

// Scans the byte copy a line at a time, so no second full-file copy is made.
func contentLineMatches(ctx context.Context, content []byte, query string, caseSensitive bool, maxSnippet, remaining int) ([]models.WorkspaceContentSearchResult, bool, error) {
	if remaining <= 0 {
		return nil, true, nil
	}
	results := make([]models.WorkspaceContentSearchResult, 0, remaining)
	queryRunes := []rune(query)
	queryBytes := []byte(query)
	asciiQuery := asciiOnly(queryBytes)
	lineNumber := 1
	for startByte := 0; startByte <= len(content); lineNumber++ {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		endByte := startByte
		for endByte < len(content) && content[endByte] != '\n' {
			endByte++
		}
		line := content[startByte:endByte]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if asciiQuery && asciiOnly(line) {
			for start := 0; start+len(queryBytes) <= len(line); {
				if err := ctx.Err(); err != nil {
					return nil, false, err
				}
				match := findASCIIMatch(ctx, line, queryBytes, start, caseSensitive)
				if match < 0 {
					if err := ctx.Err(); err != nil {
						return nil, false, err
					}
					break
				}
				results = append(results, models.WorkspaceContentSearchResult{
					LineNumber: lineNumber, ColumnStart: match + 1, ColumnEnd: match + len(queryBytes) + 1,
					Snippet: boundedASCIISnippet(line, match, match+len(queryBytes), maxSnippet),
				})
				if len(results) > remaining {
					return results[:remaining], true, nil
				}
				start = match + len(queryBytes)
			}
			if endByte == len(content) {
				break
			}
			startByte = endByte + 1
			continue
		}
		lineRunes := []rune(string(line))
		for start := 0; start+len(queryRunes) <= len(lineRunes); start++ {
			if err := ctx.Err(); err != nil {
				return nil, false, err
			}
			candidate := string(lineRunes[start : start+len(queryRunes)])
			matched := candidate == query
			if !caseSensitive {
				matched = strings.EqualFold(candidate, query)
			}
			if !matched {
				continue
			}
			results = append(results, models.WorkspaceContentSearchResult{
				LineNumber: lineNumber, ColumnStart: start + 1, ColumnEnd: start + len(queryRunes) + 1,
				Snippet: boundedSnippet(lineRunes, start, start+len(queryRunes), maxSnippet),
			})
			if len(results) > remaining {
				return results[:remaining], true, nil
			}
			start += len(queryRunes) - 1
		}
		if endByte == len(content) {
			break
		}
		startByte = endByte + 1
	}
	return results, false, nil
}

func asciiOnly(value []byte) bool {
	for _, character := range value {
		if character >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func findASCIIMatch(ctx context.Context, line, query []byte, start int, caseSensitive bool) int {
	if caseSensitive {
		if offset := bytes.Index(line[start:], query); offset >= 0 {
			return start + offset
		}
		return -1
	}
	for index := start; index+len(query) <= len(line); index++ {
		if index&4095 == 0 && ctx.Err() != nil {
			return -1
		}
		if asciiFoldEqual(line[index], query[0]) && bytes.EqualFold(line[index:index+len(query)], query) {
			return index
		}
	}
	return -1
}

func asciiFoldEqual(left, right byte) bool {
	if left == right {
		return true
	}
	if left >= 'A' && left <= 'Z' {
		left += 'a' - 'A'
	}
	if right >= 'A' && right <= 'Z' {
		right += 'a' - 'A'
	}
	return left == right
}

func boundedASCIISnippet(line []byte, matchStart, matchEnd, limit int) string {
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

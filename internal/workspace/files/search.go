package workspacefiles

// Package workspacefiles provides Workspace-owned file operations.

// Package workspacefiles provides bounded workspace file operations.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"plan-manager/internal/common/models"
)

const (
	MaxSearchResults = 100
	MaxSearchEntries = 20_000
)

func (a *Access) Search(workspace models.WorkspaceConfig, query string, includeIgnored bool) (models.WorkspacePathSearchResponse, error) {
	query = strings.TrimSpace(query)
	response := models.WorkspacePathSearchResponse{Results: []models.WorkspacePathSearchResult{}}
	if query == "" {
		return response, nil
	}
	realRoot, err := filepath.EvalSymlinks(workspace.Path)
	if err != nil {
		return response, err
	}
	type directory struct{ path, full string }
	queue := []directory{{path: "", full: realRoot}}
	visited := 0
	resultLimit, entryLimit := a.searchResultLimit, a.searchEntryLimit
	if resultLimit <= 0 {
		resultLimit = MaxSearchResults
	}
	if entryLimit <= 0 {
		entryLimit = MaxSearchEntries
	}
	lowerQuery := strings.ToLower(query)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		entries, err := os.ReadDir(current.full)
		if err != nil {
			return response, err
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
				return response, err
			}
		}
		for i, entry := range entries {
			visited++
			if visited > entryLimit {
				response.Truncated = true
				return sortSearchResponse(response, query), nil
			}
			path := paths[i]
			if protected(path) {
				continue
			}
			isIgnored := ignored[path]
			if isIgnored && !includeIgnored {
				continue
			}
			node, ok := treeEntry(workspace.Path, path, entry, isIgnored)
			if !ok {
				continue
			}
			if strings.Contains(strings.ToLower(entry.Name()), lowerQuery) || strings.Contains(strings.ToLower(path), lowerQuery) {
				response.Results = append(response.Results, models.WorkspacePathSearchResult{
					ID: workspace.ID + ":" + workspaceFileID(path), WorkspaceID: workspace.ID, WorkspaceName: workspace.Name,
					Name: entry.Name(), Path: path, Type: node.Type, Ignored: isIgnored, Context: parentRelative(path),
				})
				if len(response.Results) == resultLimit {
					response.Truncated = true
					return sortSearchResponse(response, query), nil
				}
			}
			if node.Type == "directory" {
				_, full, err := resolve(workspace.Path, path, true)
				if err == nil {
					queue = append(queue, directory{path: path, full: full})
				}
			}
		}
	}
	return sortSearchResponse(response, query), nil
}

func sortSearchResponse(response models.WorkspacePathSearchResponse, query string) models.WorkspacePathSearchResponse {
	sort.SliceStable(response.Results, func(i, j int) bool {
		left, right := response.Results[i], response.Results[j]
		leftExact, rightExact := strings.EqualFold(left.Name, query), strings.EqualFold(right.Name, query)
		if leftExact != rightExact {
			return leftExact
		}
		leftDepth, rightDepth := strings.Count(left.Path, "/"), strings.Count(right.Path, "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return naturalLess(left.Path, right.Path)
	})
	return response
}

func ValidateSearchQuery(query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("search query is required")
	}
	return nil
}

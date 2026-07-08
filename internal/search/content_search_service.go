package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	workspaceaccess "kode-stream/internal/workspace/files"
)

const (
	ModeSources = "sources"
	ModeAll     = "all"
)

type Registry interface {
	Get(id string) (models.WorkspaceConfig, bool, error)
	List() ([]models.WorkspaceConfig, error)
}

type ItemIndex interface {
	Get(id string) (models.ItemDetail, bool, error)
}

type Access interface {
	ContentSearch(context.Context, models.WorkspaceConfig, []models.WorkspaceContentSearchRoot, models.WorkspaceContentSearchRequest, *models.WorkspaceContentSearchBudget) (models.WorkspaceContentSearchResponse, error)
}

type ContentSearchService struct {
	registry Registry
	index    ItemIndex
	files    Access
}

type Service = ContentSearchService

func NewContentSearchService(registry Registry, index ItemIndex, files Access) *ContentSearchService {
	return &ContentSearchService{registry: registry, index: index, files: files}
}

func (s *Service) SearchItem(ctx context.Context, itemID string, request models.WorkspaceContentSearchRequest) (models.WorkspaceContentSearchResponse, error) {
	item, ok, err := s.index.Get(itemID)
	if err != nil {
		return emptyResponse(), err
	}
	if !ok {
		return emptyResponse(), apperrors.ErrItemNotFound
	}
	workspace, ok, err := s.registry.Get(item.WorkspaceID)
	if err != nil {
		return emptyResponse(), err
	}
	if !ok {
		return emptyResponse(), apperrors.ErrWorkspaceNotFound
	}
	roots, err := canonicalRoots(workspace, []string{item.ItemPath}, false)
	if err != nil {
		return emptyResponse(), err
	}
	request.IncludeIgnored = false
	budget := workspaceaccess.DefaultContentSearchBudget()
	response, err := s.files.ContentSearch(ctx, workspace, roots, request, &budget)
	if err != nil {
		return emptyResponse(), err
	}
	itemPrefix := strings.Trim(filepath.ToSlash(item.ItemPath), "/")
	for i := range response.Results {
		response.Results[i].ItemID = item.ID
		itemPath := strings.TrimPrefix(response.Results[i].Path, itemPrefix+"/")
		response.Results[i].FileID = itemFileID(itemPath)
	}
	return response, nil
}

func (s *Service) SearchExplorer(ctx context.Context, mode, workspaceID string, request models.WorkspaceContentSearchRequest) (models.WorkspaceContentSearchResponse, error) {
	if mode == "" {
		mode = ModeSources
	}
	if mode != ModeSources && mode != ModeAll {
		return emptyResponse(), fmt.Errorf("invalid Explorer tree mode")
	}
	workspaces, err := s.workspaces(workspaceID)
	if err != nil {
		return emptyResponse(), err
	}
	budget := workspaceaccess.DefaultContentSearchBudget()
	response := emptyResponse()
	for _, workspace := range workspaces {
		paths := []string{""}
		if mode == ModeSources {
			paths = workspace.Sources
		}
		roots, err := canonicalRoots(workspace, paths, mode == ModeSources)
		if err != nil {
			return emptyResponse(), err
		}
		if len(roots) == 0 {
			continue
		}
		part, err := s.files.ContentSearch(ctx, workspace, roots, request, &budget)
		if err != nil {
			return emptyResponse(), err
		}
		response.Results = append(response.Results, part.Results...)
		response.SkippedFiles += part.SkippedFiles
		response.Truncated = response.Truncated || part.Truncated
		if response.Truncated {
			break
		}
	}
	response.FilesVisited, response.BytesRead = budget.FilesVisited, budget.BytesRead
	return response, nil
}

func (s *Service) workspaces(id string) ([]models.WorkspaceConfig, error) {
	if id == "" {
		return s.registry.List()
	}
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperrors.ErrWorkspaceNotFound
	}
	return []models.WorkspaceConfig{workspace}, nil
}

func canonicalRoots(workspace models.WorkspaceConfig, paths []string, skipMissing bool) ([]models.WorkspaceContentSearchRoot, error) {
	type root struct{ path, real string }
	var roots []root
	for _, path := range paths {
		clean := strings.Trim(filepath.ToSlash(filepath.Clean(path)), "/")
		if clean == "." {
			clean = ""
		}
		full := workspace.Path
		if clean != "" {
			full = filepath.Join(workspace.Path, filepath.FromSlash(clean))
		}
		real, err := filepath.EvalSymlinks(full)
		if err != nil {
			if skipMissing && os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		info, err := os.Stat(real)
		if err != nil || !info.IsDir() {
			if skipMissing {
				continue
			}
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("content search root is not a directory")
		}
		workspaceReal, err := filepath.EvalSymlinks(workspace.Path)
		if err != nil {
			return nil, err
		}
		if real != workspaceReal && !strings.HasPrefix(real, workspaceReal+string(filepath.Separator)) {
			return nil, workspaceaccess.ErrOutsideRoot
		}
		candidate := root{clean, real}
		covered := false
		kept := roots[:0]
		for _, existing := range roots {
			if candidate.real == existing.real || strings.HasPrefix(candidate.real, existing.real+string(filepath.Separator)) {
				covered = true
			}
			if existing.real != candidate.real && strings.HasPrefix(existing.real, candidate.real+string(filepath.Separator)) {
				continue
			}
			kept = append(kept, existing)
		}
		roots = kept
		if !covered {
			roots = append(roots, candidate)
		}
	}
	result := make([]models.WorkspaceContentSearchRoot, len(roots))
	for i, root := range roots {
		result[i] = models.WorkspaceContentSearchRoot{Path: root.path}
	}
	return result, nil
}

func itemFileID(path string) string { return strings.NewReplacer("/", "__", ".", "_").Replace(path) }

func emptyResponse() models.WorkspaceContentSearchResponse {
	return models.WorkspaceContentSearchResponse{Results: []models.WorkspaceContentSearchResult{}}
}

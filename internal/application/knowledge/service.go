package knowledge

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"plan-manager/internal/fileaccess"
	knowledgeindex "plan-manager/internal/knowledge"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
)

var (
	ErrWorkspaceNotFound = errors.New("knowledge workspace not found")
	ErrWikiNotFound      = errors.New("knowledge wiki not found")
	ErrPageNotFound      = errors.New("knowledge page not found")
	ErrUnsafePath        = errors.New("unsafe knowledge path")
)

const (
	maxGraphNodes = 2_000
	maxGraphEdges = 10_000
)

type Service struct {
	registry *registry.Registry
	store    *knowledgeindex.Store
}

func New(registry *registry.Registry, store *knowledgeindex.Store) *Service {
	return &Service{registry: registry, store: store}
}

func (s *Service) Wikis(workspaceID string) ([]knowledgeindex.KnowledgeWiki, error) {
	if _, err := s.workspace(workspaceID); err != nil {
		return nil, err
	}
	wikis, err := s.store.List(workspaceID)
	if err != nil {
		return nil, err
	}
	if wikis == nil {
		wikis = []knowledgeindex.KnowledgeWiki{}
	}
	return wikis, nil
}

func (s *Service) Pages(workspaceID, root string) ([]knowledgeindex.KnowledgePage, []knowledgeindex.KnowledgeWarning, error) {
	wiki, err := s.wiki(workspaceID, root)
	if err != nil {
		return nil, nil, err
	}
	pages := append([]knowledgeindex.KnowledgePage(nil), wiki.Pages...)
	sort.Slice(pages, func(i, j int) bool { return pages[i].Path < pages[j].Path })
	if pages == nil {
		pages = []knowledgeindex.KnowledgePage{}
	}
	warnings := append([]knowledgeindex.KnowledgeWarning(nil), wiki.Warnings...)
	if warnings == nil {
		warnings = []knowledgeindex.KnowledgeWarning{}
	}
	return pages, warnings, nil
}

func (s *Service) Page(workspaceID, root, slug string) (knowledgeindex.KnowledgePageDetail, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return knowledgeindex.KnowledgePageDetail{}, err
	}
	wiki, err := s.wiki(workspaceID, root)
	if err != nil {
		return knowledgeindex.KnowledgePageDetail{}, err
	}
	var selected *knowledgeindex.KnowledgePage
	for index := range wiki.Pages {
		if wiki.Pages[index].Slug == slug {
			selected = &wiki.Pages[index]
			break
		}
	}
	if selected == nil {
		return knowledgeindex.KnowledgePageDetail{}, ErrPageNotFound
	}
	full, err := guardedPagePath(workspace.Path, wiki.Root, selected.Path)
	if err != nil {
		return knowledgeindex.KnowledgePageDetail{}, err
	}
	data, err := os.ReadFile(full)
	if errors.Is(err, os.ErrNotExist) {
		return knowledgeindex.KnowledgePageDetail{}, ErrPageNotFound
	}
	if err != nil {
		return knowledgeindex.KnowledgePageDetail{}, err
	}
	if int64(len(data)) > fileaccess.MaxTextResponseBytes || fileaccess.IsBinary(data) {
		return knowledgeindex.KnowledgePageDetail{}, fileaccess.ErrUnsupportedContent
	}
	warnings := make([]knowledgeindex.KnowledgeWarning, 0)
	for _, warning := range wiki.Warnings {
		if warning.Slug == selected.Slug || warning.Path == selected.Path {
			warnings = append(warnings, warning)
		}
	}
	if warnings == nil {
		warnings = []knowledgeindex.KnowledgeWarning{}
	}
	content := fileaccess.FileContentFromBytes(selected.Path, data)
	content.Editable = false
	return knowledgeindex.KnowledgePageDetail{KnowledgePage: *selected, Content: content, Warnings: warnings}, nil
}

func (s *Service) Graph(workspaceID, root string) (knowledgeindex.KnowledgeGraph, error) {
	wiki, err := s.wiki(workspaceID, root)
	if err != nil {
		return knowledgeindex.KnowledgeGraph{}, err
	}
	_, _, graph := knowledgeindex.ResolveRelationships(wiki.Pages)
	graph.TotalNodes, graph.TotalEdges = len(graph.Nodes), len(graph.Edges)
	if len(graph.Nodes) <= maxGraphNodes && len(graph.Edges) <= maxGraphEdges {
		return graph, nil
	}
	graph.Truncated = true
	if len(graph.Nodes) > maxGraphNodes {
		graph.Nodes = graph.Nodes[:maxGraphNodes]
	}
	allowed := make(map[string]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		allowed[node.ID] = struct{}{}
	}
	edges := make([]knowledgeindex.KnowledgeGraphEdge, 0, min(len(graph.Edges), maxGraphEdges))
	for _, edge := range graph.Edges {
		_, sourceOK := allowed[edge.Source]
		_, targetOK := allowed[edge.Target]
		if sourceOK && targetOK {
			edges = append(edges, edge)
			if len(edges) == maxGraphEdges {
				break
			}
		}
	}
	graph.Edges = edges
	return graph, nil
}

func (s *Service) workspace(id string) (models.WorkspaceConfig, error) {
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	if !ok {
		return models.WorkspaceConfig{}, ErrWorkspaceNotFound
	}
	return workspace, nil
}

func (s *Service) wiki(workspaceID, root string) (knowledgeindex.KnowledgeWiki, error) {
	if _, err := s.workspace(workspaceID); err != nil {
		return knowledgeindex.KnowledgeWiki{}, err
	}
	if clean := filepath.ToSlash(filepath.Clean(root)); clean != root || clean == "." || filepath.IsAbs(root) || strings.HasPrefix(clean, "../") {
		return knowledgeindex.KnowledgeWiki{}, ErrUnsafePath
	}
	wikis, err := s.store.List(workspaceID)
	if err != nil {
		return knowledgeindex.KnowledgeWiki{}, err
	}
	for _, wiki := range wikis {
		if wiki.Root == root {
			return wiki, nil
		}
	}
	return knowledgeindex.KnowledgeWiki{}, ErrWikiNotFound
}

func guardedPagePath(workspaceRoot, wikiRoot, pagePath string) (string, error) {
	root, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return "", err
	}
	wiki, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(wikiRoot)))
	if err != nil {
		return "", err
	}
	if !within(root, wiki) {
		return "", ErrUnsafePath
	}
	page, err := filepath.EvalSymlinks(filepath.Join(wiki, filepath.FromSlash(pagePath)))
	if err != nil {
		return "", err
	}
	if !within(wiki, page) {
		return "", ErrUnsafePath
	}
	return page, nil
}

func within(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

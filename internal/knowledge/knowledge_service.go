package knowledge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/content"
	"kode-stream/internal/workspace/registry"
)

var (
	ErrWorkspaceNotFound    = errors.New("knowledge workspace not found")
	ErrWikiNotFound         = errors.New("knowledge wiki not found")
	ErrPageNotFound         = errors.New("knowledge page not found")
	ErrUnsafePath           = errors.New("unsafe knowledge path")
	ErrConfirmationRequired = errors.New("confirmation required")
	ErrEnrichNotConfigured  = errors.New("knowledge enrichment is not configured")
	ErrKnowledgeDisabled    = errors.New("knowledge is disabled for this workspace")
)

const (
	maxGraphNodes        = 2_000
	maxGraphEdges        = 10_000
	maxActionLogBytes    = 64 << 10
	defaultEnrichTimeout = 5 * time.Minute
)

type workspaceDetector interface {
	DetectWorkspace(context.Context, models.WorkspaceConfig) ([]KnowledgeWiki, error)
	DetectSource(context.Context, models.WorkspaceConfig, string) (KnowledgeWiki, bool, error)
}
type gitPuller interface {
	Pull(string, models.GitOperationInput) models.GitOperationResult
}
type workspaceMutator interface {
	WithWorkspaceMutationContext(context.Context, string, func(context.Context) error) error
}
type checkoutReader interface {
	CurrentBranch(string) (string, error)
	ResolveBranch(string, string) (string, string, error)
}
type auditAppender interface {
	Append(models.AuditEvent) (models.AuditEvent, error)
}

type KnowledgeService struct {
	registry      registry.Repository
	store         *Store
	detector      workspaceDetector
	git           gitPuller
	mutation      workspaceMutator
	checkout      checkoutReader
	audit         auditAppender
	enrichTimeout time.Duration
}

func NewService(registry registry.Repository, store *Store) *KnowledgeService {
	return &KnowledgeService{registry: registry, store: store, enrichTimeout: defaultEnrichTimeout}
}

func (s *KnowledgeService) ConfigureActions(detector workspaceDetector, git gitPuller, audit auditAppender) *KnowledgeService {
	s.detector, s.git, s.audit = detector, git, audit
	if mutation, ok := git.(workspaceMutator); ok {
		s.mutation = mutation
	}
	return s
}

func (s *KnowledgeService) ConfigureCheckout(reader checkoutReader) *KnowledgeService {
	s.checkout = reader
	return s
}

func (s *KnowledgeService) Wikis(workspaceID string) ([]KnowledgeWiki, error) {
	return s.WikisContext(context.Background(), workspaceID)
}

func (s *KnowledgeService) WikisContext(ctx context.Context, workspaceID string) ([]KnowledgeWiki, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if requireKnowledgeEnabled(workspace) != nil {
		return []KnowledgeWiki{}, nil
	}
	if err := s.ensureCheckoutFresh(ctx, workspace); err != nil {
		return nil, err
	}
	wikis, err := s.store.List(workspaceID)
	if err != nil {
		return nil, err
	}
	wikis = configuredWikis(wikis, workspace.SourcesShownIn(models.SourceSurfaceKnowledge))
	if wikis == nil {
		wikis = []KnowledgeWiki{}
	}
	return wikis, nil
}

func (s *KnowledgeService) Pages(workspaceID, root string) ([]KnowledgePage, []KnowledgeWarning, error) {
	return s.PagesContext(context.Background(), workspaceID, root)
}

func (s *KnowledgeService) PagesContext(ctx context.Context, workspaceID, root string) ([]KnowledgePage, []KnowledgeWarning, error) {
	wiki, err := s.wikiContext(ctx, workspaceID, root)
	if err != nil {
		return nil, nil, err
	}
	pages := append([]KnowledgePage(nil), wiki.Pages...)
	sort.Slice(pages, func(i, j int) bool { return pages[i].Path < pages[j].Path })
	if pages == nil {
		pages = []KnowledgePage{}
	}
	warnings := append([]KnowledgeWarning(nil), wiki.Warnings...)
	if warnings == nil {
		warnings = []KnowledgeWarning{}
	}
	return pages, warnings, nil
}

func (s *KnowledgeService) Page(workspaceID, root, slug string) (KnowledgePageDetail, error) {
	return s.PageContext(context.Background(), workspaceID, root, slug)
}

func (s *KnowledgeService) PageContext(ctx context.Context, workspaceID, root, slug string) (KnowledgePageDetail, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return KnowledgePageDetail{}, err
	}
	wiki, err := s.wikiContext(ctx, workspaceID, root)
	if err != nil {
		return KnowledgePageDetail{}, err
	}
	var selected *KnowledgePage
	for index := range wiki.Pages {
		if wiki.Pages[index].Slug == slug {
			selected = &wiki.Pages[index]
			break
		}
	}
	if selected == nil {
		return KnowledgePageDetail{}, ErrPageNotFound
	}
	full, err := guardedPagePath(workspace.Path, wiki.Root, selected.Path)
	if err != nil {
		return KnowledgePageDetail{}, err
	}
	data, err := os.ReadFile(full)
	if errors.Is(err, os.ErrNotExist) {
		return KnowledgePageDetail{}, ErrPageNotFound
	}
	if err != nil {
		return KnowledgePageDetail{}, err
	}
	if int64(len(data)) > fileaccess.MaxTextResponseBytes || fileaccess.IsBinary(data) {
		return KnowledgePageDetail{}, fileaccess.ErrUnsupportedContent
	}
	warnings := make([]KnowledgeWarning, 0)
	for _, warning := range wiki.Warnings {
		if warning.Slug == selected.Slug || warning.Path == selected.Path {
			warnings = append(warnings, warning)
		}
	}
	if warnings == nil {
		warnings = []KnowledgeWarning{}
	}
	content := fileaccess.FileContentFromBytes(selected.Path, data)
	content.Editable = false
	return KnowledgePageDetail{KnowledgePage: *selected, Content: content, Warnings: warnings}, nil
}

func (s *KnowledgeService) Graph(workspaceID, root string) (KnowledgeGraph, error) {
	return s.GraphContext(context.Background(), workspaceID, root)
}

func (s *KnowledgeService) GraphContext(ctx context.Context, workspaceID, root string) (KnowledgeGraph, error) {
	wiki, err := s.wikiContext(ctx, workspaceID, root)
	if err != nil {
		return KnowledgeGraph{}, err
	}
	_, _, graph := ResolveRelationships(wiki.Pages)
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
	edges := make([]KnowledgeGraphEdge, 0, min(len(graph.Edges), maxGraphEdges))
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

func (s *KnowledgeService) workspace(id string) (models.WorkspaceConfig, error) {
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	if !ok {
		return models.WorkspaceConfig{}, ErrWorkspaceNotFound
	}
	return workspace, nil
}

func (s *KnowledgeService) wiki(workspaceID, root string) (KnowledgeWiki, error) {
	return s.wikiContext(context.Background(), workspaceID, root)
}

func (s *KnowledgeService) wikiContext(ctx context.Context, workspaceID, root string) (KnowledgeWiki, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return KnowledgeWiki{}, err
	}
	if err := requireKnowledgeEnabled(workspace); err != nil {
		return KnowledgeWiki{}, err
	}
	if err := s.ensureCheckoutFresh(ctx, workspace); err != nil {
		return KnowledgeWiki{}, err
	}
	if clean := filepath.ToSlash(filepath.Clean(root)); clean != root || clean == "." || filepath.IsAbs(root) || strings.HasPrefix(clean, "../") {
		return KnowledgeWiki{}, ErrUnsafePath
	}
	if !containsSource(workspace.SourcesShownIn(models.SourceSurfaceKnowledge), root) {
		return KnowledgeWiki{}, ErrWikiNotFound
	}
	wikis, err := s.store.List(workspaceID)
	if err != nil {
		return KnowledgeWiki{}, err
	}
	for _, wiki := range wikis {
		if wiki.Root == root {
			return wiki, nil
		}
	}
	return KnowledgeWiki{}, ErrWikiNotFound
}

func (s *KnowledgeService) ensureCheckoutFresh(ctx context.Context, workspace models.WorkspaceConfig) error {
	if s.checkout == nil || s.detector == nil {
		return nil
	}
	branch, err := s.checkout.CurrentBranch(workspace.Path)
	if err != nil {
		return err
	}
	_, commit, err := s.checkout.ResolveBranch(workspace.Path, branch)
	if err != nil {
		return err
	}
	wikis, err := s.store.List(workspace.ID)
	if err != nil {
		return err
	}
	fresh := len(wikis) > 0
	for _, wiki := range wikis {
		if wiki.CheckoutBranch != branch || wiki.CheckoutCommit != commit {
			fresh = false
			break
		}
	}
	if fresh {
		return nil
	}
	return s.withWorkspaceMutation(ctx, workspace.ID, func(actionContext context.Context) error {
		// A competing explicit action may have published a fresh index while this
		// read waited for the mutation boundary, so verify again before rebuilding.
		currentBranch, branchErr := s.checkout.CurrentBranch(workspace.Path)
		if branchErr != nil {
			return branchErr
		}
		_, currentCommit, commitErr := s.checkout.ResolveBranch(workspace.Path, currentBranch)
		if commitErr != nil {
			return commitErr
		}
		current, readErr := s.store.List(workspace.ID)
		if readErr != nil {
			return readErr
		}
		freshNow := len(current) > 0
		for _, wiki := range current {
			if wiki.CheckoutBranch != currentBranch || wiki.CheckoutCommit != currentCommit {
				freshNow = false
				break
			}
		}
		if freshNow {
			return nil
		}
		updated, detectErr := s.detector.DetectWorkspace(actionContext, workspace)
		if detectErr != nil {
			return detectErr
		}
		if err := actionContext.Err(); err != nil {
			return err
		}
		updated = stampKnowledgeWikis(updated, currentBranch, currentCommit)
		return s.store.ReplaceWorkspace(workspace.ID, updated)
	})
}

func (s *KnowledgeService) stampWiki(workspace models.WorkspaceConfig, wiki KnowledgeWiki) (KnowledgeWiki, error) {
	wikis, err := s.stampWikis(workspace, []KnowledgeWiki{wiki})
	if err != nil {
		return KnowledgeWiki{}, err
	}
	return wikis[0], nil
}

func (s *KnowledgeService) stampWikis(workspace models.WorkspaceConfig, wikis []KnowledgeWiki) ([]KnowledgeWiki, error) {
	if s.checkout == nil {
		return wikis, nil
	}
	branch, err := s.checkout.CurrentBranch(workspace.Path)
	if err != nil {
		return nil, err
	}
	_, commit, err := s.checkout.ResolveBranch(workspace.Path, branch)
	if err != nil {
		return nil, err
	}
	return stampKnowledgeWikis(wikis, branch, commit), nil
}

func stampKnowledgeWikis(wikis []KnowledgeWiki, branch, commit string) []KnowledgeWiki {
	for i := range wikis {
		wikis[i].CheckoutBranch = branch
		wikis[i].CheckoutCommit = commit
	}
	return wikis
}

func requireKnowledgeEnabled(workspace models.WorkspaceConfig) error {
	if workspace.Knowledge != nil && workspace.Knowledge.Enabled != nil && !*workspace.Knowledge.Enabled {
		return ErrKnowledgeDisabled
	}
	return nil
}

func containsSource(sources []string, root string) bool {
	for _, source := range sources {
		if filepath.ToSlash(filepath.Clean(strings.TrimSpace(source))) == root {
			return true
		}
	}
	return false
}

func configuredWikis(wikis []KnowledgeWiki, sources []string) []KnowledgeWiki {
	configured := make([]KnowledgeWiki, 0, len(wikis))
	for _, wiki := range wikis {
		if containsSource(sources, wiki.Root) {
			configured = append(configured, wiki)
		}
	}
	return configured
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

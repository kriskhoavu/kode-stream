package knowledge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
type auditAppender interface {
	Append(models.AuditEvent) (models.AuditEvent, error)
}

type KnowledgeService struct {
	registry      registry.Repository
	store         *Store
	detector      workspaceDetector
	git           gitPuller
	audit         auditAppender
	enrichTimeout time.Duration
}

func NewService(registry registry.Repository, store *Store) *KnowledgeService {
	return &KnowledgeService{registry: registry, store: store, enrichTimeout: defaultEnrichTimeout}
}

func (s *KnowledgeService) ConfigureActions(detector workspaceDetector, git gitPuller, audit auditAppender) *KnowledgeService {
	s.detector, s.git, s.audit = detector, git, audit
	return s
}

func (s *KnowledgeService) Rescan(ctx context.Context, workspaceID, root string) (KnowledgeActionResult, error) {
	started := time.Now()
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return KnowledgeActionResult{}, err
	}
	if s.detector == nil {
		return KnowledgeActionResult{}, errors.New("knowledge detector is unavailable")
	}
	if err := requireKnowledgeEnabled(workspace); err != nil {
		return KnowledgeActionResult{}, err
	}
	if !containsSource(workspace.Sources, root) {
		return KnowledgeActionResult{}, ErrWikiNotFound
	}
	wiki, ok, err := s.detector.DetectSource(ctx, workspace, root)
	if err != nil {
		s.recordAudit(workspaceID, "knowledge_rescan", root, started, err)
		return KnowledgeActionResult{}, err
	}
	if ok {
		if err := s.store.ReplaceWiki(workspaceID, root, wiki); err != nil {
			s.recordAudit(workspaceID, "knowledge_rescan", root, started, err)
			return KnowledgeActionResult{}, err
		}
		result := actionResult("rescan", []KnowledgeWiki{wiki}, "", false)
		s.recordAudit(workspaceID, "knowledge_rescan", root, started, nil)
		return result, nil
	}
	s.recordAudit(workspaceID, "knowledge_rescan", root, started, ErrWikiNotFound)
	return KnowledgeActionResult{}, ErrWikiNotFound
}

func (s *KnowledgeService) Sync(ctx context.Context, workspaceID string, input models.GitOperationInput) (KnowledgeActionResult, error) {
	started := time.Now()
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return KnowledgeActionResult{}, err
	}
	if s.git == nil || s.detector == nil {
		return KnowledgeActionResult{}, errors.New("knowledge sync is unavailable")
	}
	if err := requireKnowledgeEnabled(workspace); err != nil {
		return KnowledgeActionResult{}, err
	}
	gitResult := s.git.Pull(workspaceID, input)
	if !gitResult.OK {
		err := errors.New(gitResult.Message)
		s.recordAudit(workspaceID, "knowledge_sync", "", started, err)
		return KnowledgeActionResult{OK: false, Operation: "sync", Message: gitResult.Message, Wikis: []KnowledgeWiki{}, Warnings: []KnowledgeWarning{}, CompletedAt: time.Now().UTC()}, nil
	}
	wikis, err := s.detector.DetectWorkspace(ctx, workspace)
	if err == nil {
		err = s.store.ReplaceWorkspace(workspaceID, wikis)
	}
	if err != nil {
		s.recordAudit(workspaceID, "knowledge_sync", "", started, err)
		return KnowledgeActionResult{}, err
	}
	result := actionResult("sync", wikis, "", false)
	s.recordAudit(workspaceID, "knowledge_sync", "", started, nil)
	return result, nil
}

func (s *KnowledgeService) Enrich(ctx context.Context, workspaceID string, confirm bool) (KnowledgeActionResult, error) {
	started := time.Now()
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return KnowledgeActionResult{}, err
	}
	if err := requireKnowledgeEnabled(workspace); err != nil {
		return KnowledgeActionResult{}, err
	}
	if !confirm {
		return KnowledgeActionResult{}, ErrConfirmationRequired
	}
	if workspace.Knowledge == nil || strings.TrimSpace(workspace.Knowledge.EnrichExecutable) == "" {
		return KnowledgeActionResult{}, ErrEnrichNotConfigured
	}
	if s.detector == nil {
		return KnowledgeActionResult{}, errors.New("knowledge detector is unavailable")
	}
	timeout := s.enrichTimeout
	if timeout <= 0 {
		timeout = defaultEnrichTimeout
	}
	runContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.Command(workspace.Knowledge.EnrichExecutable, workspace.Knowledge.EnrichArgs...)
	command.Dir = workspace.Path
	configureProcess(command)
	buffer := &limitedBuffer{limit: maxActionLogBytes}
	command.Stdout, command.Stderr = buffer, buffer
	if err = command.Start(); err == nil {
		done := make(chan error, 1)
		go func() { done <- command.Wait() }()
		select {
		case err = <-done:
		case <-runContext.Done():
			killProcess(command)
			<-done
			err = fmt.Errorf("enrichment timed out: %w", runContext.Err())
		}
	}
	if err != nil {
		s.recordAudit(workspaceID, "knowledge_enrich", "", started, err)
		return KnowledgeActionResult{OK: false, Operation: "enrich", Message: sanitizeError(err), Wikis: []KnowledgeWiki{}, Warnings: []KnowledgeWarning{}, Log: buffer.String(), LogTruncated: buffer.truncated, CompletedAt: time.Now().UTC()}, nil
	}
	wikis, err := s.detector.DetectWorkspace(ctx, workspace)
	if err == nil {
		err = s.store.ReplaceWorkspace(workspaceID, wikis)
	}
	if err != nil {
		s.recordAudit(workspaceID, "knowledge_enrich", "", started, err)
		return KnowledgeActionResult{}, err
	}
	result := actionResult("enrich", wikis, buffer.String(), buffer.truncated)
	s.recordAudit(workspaceID, "knowledge_enrich", "", started, nil)
	return result, nil
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (w *limitedBuffer) Write(data []byte) (int, error) {
	original := len(data)
	remaining := w.limit - w.buffer.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = w.buffer.Write(data)
	}
	if original > remaining {
		w.truncated = true
	}
	return original, nil
}
func (w *limitedBuffer) String() string { return w.buffer.String() }

func actionResult(operation string, wikis []KnowledgeWiki, log string, truncated bool) KnowledgeActionResult {
	warnings := make([]KnowledgeWarning, 0)
	for _, wiki := range wikis {
		warnings = append(warnings, wiki.Warnings...)
	}
	if wikis == nil {
		wikis = []KnowledgeWiki{}
	}
	return KnowledgeActionResult{OK: true, Operation: operation, Wikis: wikis, Warnings: warnings, Log: log, LogTruncated: truncated, CompletedAt: time.Now().UTC()}
}

func (s *KnowledgeService) recordAudit(workspaceID, operation, path string, started time.Time, err error) {
	if s.audit == nil {
		return
	}
	status, message := models.AuditStatusSuccess, "Knowledge action completed."
	if err != nil {
		status, message = models.AuditStatusFailed, "Knowledge action failed."
	}
	paths := []string{}
	if path != "" {
		paths = []string{path}
	}
	_, _ = s.audit.Append(models.AuditEvent{WorkspaceID: workspaceID, Operation: operation, Status: status, Message: message, Paths: paths, DurationMS: time.Since(started).Milliseconds()})
}

func sanitizeError(err error) string {
	message := strings.TrimSpace(err.Error())
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

func (s *KnowledgeService) Wikis(workspaceID string) ([]KnowledgeWiki, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if requireKnowledgeEnabled(workspace) != nil {
		return []KnowledgeWiki{}, nil
	}
	wikis, err := s.store.List(workspaceID)
	if err != nil {
		return nil, err
	}
	if wikis == nil {
		wikis = []KnowledgeWiki{}
	}
	return wikis, nil
}

func (s *KnowledgeService) Pages(workspaceID, root string) ([]KnowledgePage, []KnowledgeWarning, error) {
	wiki, err := s.wiki(workspaceID, root)
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
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return KnowledgePageDetail{}, err
	}
	wiki, err := s.wiki(workspaceID, root)
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
	wiki, err := s.wiki(workspaceID, root)
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
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return KnowledgeWiki{}, err
	}
	if err := requireKnowledgeEnabled(workspace); err != nil {
		return KnowledgeWiki{}, err
	}
	if clean := filepath.ToSlash(filepath.Clean(root)); clean != root || clean == "." || filepath.IsAbs(root) || strings.HasPrefix(clean, "../") {
		return KnowledgeWiki{}, ErrUnsafePath
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

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

	"plan-manager/internal/fileaccess"
	knowledgeindex "plan-manager/internal/knowledge"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
)

var (
	ErrWorkspaceNotFound    = errors.New("knowledge workspace not found")
	ErrWikiNotFound         = errors.New("knowledge wiki not found")
	ErrPageNotFound         = errors.New("knowledge page not found")
	ErrUnsafePath           = errors.New("unsafe knowledge path")
	ErrConfirmationRequired = errors.New("confirmation required")
	ErrEnrichNotConfigured  = errors.New("knowledge enrichment is not configured")
)

const (
	maxGraphNodes        = 2_000
	maxGraphEdges        = 10_000
	maxActionLogBytes    = 64 << 10
	defaultEnrichTimeout = 5 * time.Minute
)

type workspaceDetector interface {
	DetectWorkspace(context.Context, models.WorkspaceConfig) ([]knowledgeindex.KnowledgeWiki, error)
}
type gitPuller interface {
	Pull(string, models.GitOperationInput) models.GitOperationResult
}
type auditAppender interface {
	Append(models.AuditEvent) (models.AuditEvent, error)
}

type Service struct {
	registry      *registry.Registry
	store         *knowledgeindex.Store
	detector      workspaceDetector
	git           gitPuller
	audit         auditAppender
	enrichTimeout time.Duration
}

func New(registry *registry.Registry, store *knowledgeindex.Store) *Service {
	return &Service{registry: registry, store: store, enrichTimeout: defaultEnrichTimeout}
}

func (s *Service) ConfigureActions(detector workspaceDetector, git gitPuller, audit auditAppender) *Service {
	s.detector, s.git, s.audit = detector, git, audit
	return s
}

func (s *Service) Rescan(ctx context.Context, workspaceID, root string) (knowledgeindex.KnowledgeActionResult, error) {
	started := time.Now()
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return knowledgeindex.KnowledgeActionResult{}, err
	}
	if s.detector == nil {
		return knowledgeindex.KnowledgeActionResult{}, errors.New("knowledge detector is unavailable")
	}
	wikis, err := s.detector.DetectWorkspace(ctx, workspace)
	if err != nil {
		s.recordAudit(workspaceID, "knowledge_rescan", root, started, err)
		return knowledgeindex.KnowledgeActionResult{}, err
	}
	for _, wiki := range wikis {
		if wiki.Root != root {
			continue
		}
		if err := s.store.ReplaceWiki(workspaceID, root, wiki); err != nil {
			s.recordAudit(workspaceID, "knowledge_rescan", root, started, err)
			return knowledgeindex.KnowledgeActionResult{}, err
		}
		result := actionResult("rescan", []knowledgeindex.KnowledgeWiki{wiki}, "", false)
		s.recordAudit(workspaceID, "knowledge_rescan", root, started, nil)
		return result, nil
	}
	s.recordAudit(workspaceID, "knowledge_rescan", root, started, ErrWikiNotFound)
	return knowledgeindex.KnowledgeActionResult{}, ErrWikiNotFound
}

func (s *Service) Sync(ctx context.Context, workspaceID string, input models.GitOperationInput) (knowledgeindex.KnowledgeActionResult, error) {
	started := time.Now()
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return knowledgeindex.KnowledgeActionResult{}, err
	}
	if s.git == nil || s.detector == nil {
		return knowledgeindex.KnowledgeActionResult{}, errors.New("knowledge sync is unavailable")
	}
	gitResult := s.git.Pull(workspaceID, input)
	if !gitResult.OK {
		err := errors.New(gitResult.Message)
		s.recordAudit(workspaceID, "knowledge_sync", "", started, err)
		return knowledgeindex.KnowledgeActionResult{OK: false, Operation: "sync", Message: gitResult.Message, Wikis: []knowledgeindex.KnowledgeWiki{}, Warnings: []knowledgeindex.KnowledgeWarning{}, CompletedAt: time.Now().UTC()}, nil
	}
	wikis, err := s.detector.DetectWorkspace(ctx, workspace)
	if err == nil {
		err = s.store.ReplaceWorkspace(workspaceID, wikis)
	}
	if err != nil {
		s.recordAudit(workspaceID, "knowledge_sync", "", started, err)
		return knowledgeindex.KnowledgeActionResult{}, err
	}
	result := actionResult("sync", wikis, "", false)
	s.recordAudit(workspaceID, "knowledge_sync", "", started, nil)
	return result, nil
}

func (s *Service) Enrich(ctx context.Context, workspaceID string, confirm bool) (knowledgeindex.KnowledgeActionResult, error) {
	started := time.Now()
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return knowledgeindex.KnowledgeActionResult{}, err
	}
	if !confirm {
		return knowledgeindex.KnowledgeActionResult{}, ErrConfirmationRequired
	}
	if workspace.Knowledge == nil || strings.TrimSpace(workspace.Knowledge.EnrichExecutable) == "" {
		return knowledgeindex.KnowledgeActionResult{}, ErrEnrichNotConfigured
	}
	if s.detector == nil {
		return knowledgeindex.KnowledgeActionResult{}, errors.New("knowledge detector is unavailable")
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
		return knowledgeindex.KnowledgeActionResult{OK: false, Operation: "enrich", Message: sanitizeError(err), Wikis: []knowledgeindex.KnowledgeWiki{}, Warnings: []knowledgeindex.KnowledgeWarning{}, Log: buffer.String(), LogTruncated: buffer.truncated, CompletedAt: time.Now().UTC()}, nil
	}
	wikis, err := s.detector.DetectWorkspace(ctx, workspace)
	if err == nil {
		err = s.store.ReplaceWorkspace(workspaceID, wikis)
	}
	if err != nil {
		s.recordAudit(workspaceID, "knowledge_enrich", "", started, err)
		return knowledgeindex.KnowledgeActionResult{}, err
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

func actionResult(operation string, wikis []knowledgeindex.KnowledgeWiki, log string, truncated bool) knowledgeindex.KnowledgeActionResult {
	warnings := make([]knowledgeindex.KnowledgeWarning, 0)
	for _, wiki := range wikis {
		warnings = append(warnings, wiki.Warnings...)
	}
	if wikis == nil {
		wikis = []knowledgeindex.KnowledgeWiki{}
	}
	return knowledgeindex.KnowledgeActionResult{OK: true, Operation: operation, Wikis: wikis, Warnings: warnings, Log: log, LogTruncated: truncated, CompletedAt: time.Now().UTC()}
}

func (s *Service) recordAudit(workspaceID, operation, path string, started time.Time, err error) {
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

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
	"kode-stream/internal/filesystem/pathguard"
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
	var wiki KnowledgeWiki
	var ok bool
	err = s.withWorkspaceMutation(ctx, workspaceID, func(actionContext context.Context) error {
		wiki, ok, err = s.detector.DetectSource(actionContext, workspace, root)
		if err != nil || !ok {
			return err
		}
		wiki, err = s.stampWiki(workspace, wiki)
		if err != nil {
			return err
		}
		return s.store.ReplaceWiki(workspaceID, root, wiki)
	})
	if err != nil {
		s.recordAudit(workspaceID, "knowledge_rescan", root, started, err)
		return KnowledgeActionResult{}, err
	}
	if ok {
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
	var gitResult models.GitOperationResult
	var wikis []KnowledgeWiki
	committed := false
	err = s.withWorkspaceMutation(ctx, workspaceID, func(actionContext context.Context) error {
		gitResult = s.pullContext(actionContext, workspaceID, input)
		if !gitResult.OK {
			return nil
		}
		committed = true
		wikis, err = s.detector.DetectWorkspace(actionContext, workspace)
		if err == nil {
			wikis, err = s.stampWikis(workspace, wikis)
		}
		if err == nil {
			err = s.store.ReplaceWorkspace(workspaceID, wikis)
		}
		return err
	})
	if err != nil {
		if committed {
			s.recordCommittedRefreshAudit(workspaceID, "knowledge_sync", started)
			return committedRefreshResult("sync", "Pull completed, but knowledge refresh failed. Rescan to recover the index."), nil
		}
		s.recordAudit(workspaceID, "knowledge_sync", "", started, err)
		return KnowledgeActionResult{}, err
	}
	if !gitResult.OK {
		err := errors.New(gitResult.Message)
		s.recordAudit(workspaceID, "knowledge_sync", "", started, err)
		return KnowledgeActionResult{OK: false, Operation: "sync", Message: gitResult.Message, Wikis: []KnowledgeWiki{}, Warnings: []KnowledgeWarning{}, CompletedAt: time.Now().UTC()}, nil
	}
	result := actionResult("sync", wikis, "", false)
	s.recordAudit(workspaceID, "knowledge_sync", "", started, nil)
	return result, nil
}

func (s *KnowledgeService) pullContext(ctx context.Context, workspaceID string, input models.GitOperationInput) models.GitOperationResult {
	if git, ok := s.git.(interface {
		PullContext(context.Context, string, models.GitOperationInput) models.GitOperationResult
	}); ok {
		return git.PullContext(ctx, workspaceID, input)
	}
	if err := ctx.Err(); err != nil {
		return models.GitOperationResult{OK: false, Message: err.Error()}
	}
	return s.git.Pull(workspaceID, input)
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
	buffer := &limitedBuffer{limit: maxActionLogBytes}
	committed := false
	var wikis []KnowledgeWiki
	err = s.withWorkspaceMutation(ctx, workspaceID, func(actionContext context.Context) error {
		runContext, cancel := context.WithTimeout(actionContext, timeout)
		defer cancel()
		command := exec.Command(workspace.Knowledge.EnrichExecutable, workspace.Knowledge.EnrichArgs...)
		command.Dir = workspace.Path
		configureProcess(command)
		command.Stdout, command.Stderr = buffer, buffer
		if commandErr := command.Start(); commandErr != nil {
			return commandErr
		}
		done := make(chan error, 1)
		go func() { done <- command.Wait() }()
		select {
		case commandErr := <-done:
			if commandErr != nil {
				return commandErr
			}
		case <-runContext.Done():
			killProcess(command)
			<-done
			return fmt.Errorf("enrichment timed out: %w", runContext.Err())
		}
		committed = true
		wikis, detectErr := s.detector.DetectWorkspace(actionContext, workspace)
		if detectErr != nil {
			return detectErr
		}
		wikis, detectErr = s.stampWikis(workspace, wikis)
		if detectErr != nil {
			return detectErr
		}
		return s.store.ReplaceWorkspace(workspaceID, wikis)
	})
	if err != nil {
		if committed {
			s.recordCommittedRefreshAudit(workspaceID, "knowledge_enrich", started)
			return committedRefreshResult("enrich", "Enrichment completed, but knowledge refresh failed. Rescan to recover the index."), nil
		}
		s.recordAudit(workspaceID, "knowledge_enrich", "", started, err)
		return KnowledgeActionResult{OK: false, Operation: "enrich", Message: sanitizeError(err), Wikis: []KnowledgeWiki{}, Warnings: []KnowledgeWarning{}, Log: buffer.String(), LogTruncated: buffer.truncated, CompletedAt: time.Now().UTC()}, nil
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

func committedRefreshResult(operation, message string) KnowledgeActionResult {
	return KnowledgeActionResult{OK: false, Operation: operation, Message: message, Committed: true, RefreshRequired: true, Wikis: []KnowledgeWiki{}, Warnings: []KnowledgeWarning{}, CompletedAt: time.Now().UTC()}
}

func (s *KnowledgeService) withWorkspaceMutation(ctx context.Context, workspaceID string, action func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.mutation == nil {
		// Partial fixtures retain safe behavior; production wiring always supplies
		// the shared Git mutation capability.
		return action(ctx)
	}
	return s.mutation.WithWorkspaceMutationContext(ctx, workspaceID, action)
}

func (s *KnowledgeService) recordAudit(workspaceID, operation, path string, started time.Time, err error) {
	status, message := models.AuditStatusSuccess, "Knowledge action completed."
	if err != nil {
		status, message = models.AuditStatusFailed, "Knowledge action failed."
	}
	s.appendAudit(workspaceID, operation, path, started, status, message)
}

func (s *KnowledgeService) recordCommittedRefreshAudit(workspaceID, operation string, started time.Time) {
	// The workspace mutation is authoritative here. The index refresh failed,
	// but command output and the underlying error can contain sensitive details.
	s.appendAudit(workspaceID, operation, "", started, models.AuditStatusSuccess, "Knowledge action completed; knowledge index refresh failed. Rescan to recover.")
}

func (s *KnowledgeService) appendAudit(workspaceID, operation, path string, started time.Time, status models.AuditStatus, message string) {
	if s.audit == nil {
		return
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
	wikis = configuredWikis(wikis, workspace.Sources)
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

func (s *KnowledgeService) E2ERunbooksForSources(workspaceID string, sourceRefs []string) (models.E2ERunbookList, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.E2ERunbookList{}, err
	}
	if err := requireKnowledgeEnabled(workspace); err != nil {
		return models.E2ERunbookList{}, err
	}
	wikis, err := s.store.List(workspaceID)
	if err != nil {
		return models.E2ERunbookList{}, err
	}
	matched := make([]models.E2ERunbook, 0)
	for _, wiki := range wikis {
		for _, page := range wiki.Pages {
			if !strings.HasPrefix(page.Domain, "e2e-testing") || !matchesE2ESource(page.SourceRefs, sourceRefs) {
				continue
			}
			matched = append(matched, s.e2ERunbook(workspace, wiki.Root, page))
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].Title < matched[j].Title })
	if len(matched) == 0 {
		return models.E2ERunbookList{Runbooks: matched, Diagnostic: "No canonical E2E journey is linked to this plan."}, nil
	}
	return models.E2ERunbookList{Runbooks: matched}, nil
}

func (s *KnowledgeService) E2ERunbook(workspaceID, root, slug string) (models.E2ERunbookList, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.E2ERunbookList{}, err
	}
	wiki, err := s.wiki(workspaceID, root)
	if err != nil {
		return models.E2ERunbookList{}, err
	}
	for _, page := range wiki.Pages {
		if page.Slug == slug && strings.HasPrefix(page.Domain, "e2e-testing") {
			return models.E2ERunbookList{Runbooks: []models.E2ERunbook{s.e2ERunbook(workspace, wiki.Root, page)}}, nil
		}
	}
	return models.E2ERunbookList{Runbooks: []models.E2ERunbook{}, Diagnostic: "The selected Knowledge page is not an E2E journey."}, nil
}

func matchesE2ESource(pageSources, requested []string) bool {
	for _, source := range pageSources {
		for _, candidate := range requested {
			if sourceReferencePath(source) == sourceReferencePath(candidate) {
				return true
			}
		}
	}
	return false
}

func (s *KnowledgeService) e2ERunbook(workspace models.WorkspaceConfig, wikiRoot string, page KnowledgePage) models.E2ERunbook {
	resultRelative := e2EResultPath(workspace.Path, wikiRoot, page)
	runbook := models.E2ERunbook{
		Title:      page.Title,
		Path:       filepath.ToSlash(filepath.Join(wikiRoot, page.Path)),
		Source:     "wiki",
		ResultPath: resultRelative,
	}
	result, err := pathguard.ValidateMarkdownFile(workspace.Path, resultRelative)
	if err == nil {
		data, readErr := os.ReadFile(result)
		if readErr != nil {
			runbook.Diagnostic = "Latest E2E result could not be read."
			return runbook
		}
		runbook.LatestResult = parseE2EResult(string(data))
	} else if os.IsNotExist(err) {
		runbook.Diagnostic = "Not run"
	} else {
		runbook.Diagnostic = "Latest E2E result path is unsafe or unsupported."
	}
	return runbook
}

func e2EResultPath(workspaceRoot, wikiRoot string, page KnowledgePage) string {
	for index := len(page.SourceRefs) - 1; index >= 0; index-- {
		source := sourceReferencePath(page.SourceRefs[index])
		automationIndex := strings.Index(source, "/automation/")
		if automationIndex < 0 {
			continue
		}
		automationRoot := source[:automationIndex+len("/automation")]
		candidate := filepath.ToSlash(filepath.Join(automationRoot, "results", "latest.md"))
		if _, err := pathguard.ValidateMarkdownTarget(workspaceRoot, candidate); err == nil {
			return candidate
		}
	}
	fallback := filepath.ToSlash(filepath.Join(wikiRoot, filepath.Dir(page.Path), "automation", "results", "latest.md"))
	if _, err := pathguard.ValidateMarkdownTarget(workspaceRoot, fallback); err == nil {
		return fallback
	}
	return ""
}

func sourceReferencePath(value string) string {
	value = strings.TrimSpace(value)
	if path, _, found := strings.Cut(value, " | "); found {
		value = strings.TrimSpace(path)
	}
	return filepath.ToSlash(filepath.Clean(value))
}

func parseE2EResult(content string) *models.E2ELatestResult {
	result := &models.E2ELatestResult{Status: "not run", Evidence: []string{}}
	for _, line := range strings.Split(content, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "status":
			result.Status = strings.ToLower(strings.TrimSpace(value))
		case "provider":
			result.Provider = strings.TrimSpace(value)
		case "environment":
			result.Environment = strings.TrimSpace(value)
		case "failed step":
			result.FailedStep = strings.TrimSpace(value)
		case "evidence":
			if evidence := strings.TrimSpace(value); evidence != "" {
				result.Evidence = append(result.Evidence, evidence)
			}
		}
	}
	switch result.Status {
	case "passed", "failed", "blocked", "not run":
	default:
		result.Status = "not run"
	}
	return result
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
	if !containsSource(workspace.Sources, root) {
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

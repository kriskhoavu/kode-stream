package knowledge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"kode-stream/internal/common/models"
)

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
	if !containsSource(workspace.SourcesShownIn(models.SourceSurfaceKnowledge), root) {
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
		detected, detectErr := s.detector.DetectWorkspace(actionContext, workspace)
		if detectErr != nil {
			return detectErr
		}
		if wikis, detectErr = s.stampWikis(workspace, detected); detectErr != nil {
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

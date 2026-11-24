package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
	appworkspace "kode-stream/internal/workspace"
)

const (
	SessionChangesAvailable   = "available"
	SessionChangesUnavailable = "unavailable"
	SessionChangesUnsupported = "unsupported"
)

const (
	ReasonContentProviderUnsupported = "content_provider_unsupported"
	ReasonWorkspaceMissing           = "workspace_missing"
	ReasonLaunchCommitUnknown        = "launch_commit_unknown"
	ReasonLaunchCommitMissing        = "launch_commit_missing"
)

type SessionFile struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	OldPath string `json:"oldPath,omitempty"`
}

type SessionChanges struct {
	SessionID    string        `json:"sessionId"`
	LaunchCommit string        `json:"launchCommit,omitempty"`
	HeadCommit   string        `json:"headCommit,omitempty"`
	HeadMoved    int           `json:"headMoved"`
	Files        []SessionFile `json:"files"`
	Additions    int           `json:"additions"`
	Deletions    int           `json:"deletions"`
	Truncated    bool          `json:"truncated,omitempty"`
	Availability string        `json:"availability"`
	Reason       string        `json:"reason,omitempty"`
}

type SessionFileDiffResult struct {
	Path      string `json:"path"`
	Diff      string `json:"diff"`
	Truncated bool   `json:"truncated"`
}

type SessionChangesError struct {
	Availability string
	Reason       string
}

func (e *SessionChangesError) Error() string {
	return fmt.Sprintf("session changes %s: %s", e.Availability, e.Reason)
}

type sessionChangeReader interface {
	NameStatusSinceContext(ctx context.Context, workspacePath, commit string) ([]gitadapter.NameStatusEntry, bool, error)
	ShortStatSinceContext(ctx context.Context, workspacePath, commit string) (additions, deletions, files int, err error)
	DiffFileSince(ctx context.Context, workspacePath, commit, relPath string, maxBytes int64) (string, bool, error)
	RevListCountContext(ctx context.Context, workspacePath, from, to string) (int, error)
	CommitExistsContext(ctx context.Context, workspacePath, commit string) error
}

type sessionChangeContext struct {
	record    SessionRecord
	workspace models.WorkspaceConfig
	reader    sessionChangeReader
}

// err is set only for an infrastructure failure. A guard answers through availability and reason.
func (s *Service) resolveSessionChanges(ctx context.Context, sessionID string) (sessionChangeContext, string, string, error) {
	if s.records == nil {
		return sessionChangeContext{}, "", "", ErrNotFound
	}
	record, found, err := s.records.Get(strings.TrimSpace(sessionID))
	if err != nil {
		return sessionChangeContext{}, "", "", err
	}
	if !found {
		return sessionChangeContext{}, "", "", ErrNotFound
	}
	if s.launch == nil || s.launch.registry == nil {
		return sessionChangeContext{record: record}, SessionChangesUnavailable, ReasonWorkspaceMissing, nil
	}
	workspace, found, err := s.launch.registry.Get(record.WorkspaceID)
	if err != nil {
		return sessionChangeContext{}, "", "", err
	}
	if !found {
		return sessionChangeContext{record: record}, SessionChangesUnavailable, ReasonWorkspaceMissing, nil
	}
	// Assuming Cloud can only refuse a workspace that Local allows, never the reverse.
	axes := appworkspace.ProviderAxes(s.runtimeMode, "", workspace)
	if axes.ContentProvider != models.WorkspaceContentProviderLocalCheckout {
		return sessionChangeContext{record: record, workspace: workspace}, SessionChangesUnsupported, ReasonContentProviderUnsupported, nil
	}
	if _, statErr := s.stat(workspace.Path); statErr != nil {
		return sessionChangeContext{record: record, workspace: workspace}, SessionChangesUnavailable, ReasonWorkspaceMissing, nil
	}
	if strings.TrimSpace(record.ObservedCommit) == "" {
		return sessionChangeContext{record: record, workspace: workspace}, SessionChangesUnavailable, ReasonLaunchCommitUnknown, nil
	}
	reader, ok := s.branches.(sessionChangeReader)
	if s.branches == nil || !ok {
		return sessionChangeContext{}, "", "", errors.New("session changes: no Git reader is configured")
	}
	if err := reader.CommitExistsContext(ctx, workspace.Path, record.ObservedCommit); err != nil {
		var unknown *gitadapter.ErrUnknownCommit
		if !errors.As(err, &unknown) {
			// Any other failure is infrastructure, not a missing commit, so it answers 500.
			return sessionChangeContext{}, "", "", err
		}
		return sessionChangeContext{record: record, workspace: workspace}, SessionChangesUnavailable, ReasonLaunchCommitMissing, nil
	}
	return sessionChangeContext{record: record, workspace: workspace, reader: reader}, SessionChangesAvailable, "", nil
}

func (s *Service) SessionChanges(ctx context.Context, sessionID string) (SessionChanges, error) {
	if err := ctx.Err(); err != nil {
		return SessionChanges{}, err
	}
	changeCtx, availability, reason, err := s.resolveSessionChanges(ctx, sessionID)
	if err != nil {
		return SessionChanges{}, err
	}
	result := SessionChanges{
		SessionID:    strings.TrimSpace(sessionID),
		LaunchCommit: changeCtx.record.ObservedCommit,
		Files:        []SessionFile{},
		Availability: availability,
		Reason:       reason,
	}
	if availability != SessionChangesAvailable {
		return result, nil
	}
	branch, err := s.branches.CurrentBranch(changeCtx.workspace.Path)
	if err != nil {
		return SessionChanges{}, err
	}
	_, headCommit, err := s.branches.ResolveBranch(changeCtx.workspace.Path, branch)
	if err != nil {
		return SessionChanges{}, err
	}
	headMoved, err := changeCtx.reader.RevListCountContext(ctx, changeCtx.workspace.Path, changeCtx.record.ObservedCommit, headCommit)
	if err != nil {
		return SessionChanges{}, err
	}
	entries, truncated, err := changeCtx.reader.NameStatusSinceContext(ctx, changeCtx.workspace.Path, changeCtx.record.ObservedCommit)
	if err != nil {
		return SessionChanges{}, err
	}
	additions, deletions, _, err := changeCtx.reader.ShortStatSinceContext(ctx, changeCtx.workspace.Path, changeCtx.record.ObservedCommit)
	if err != nil {
		return SessionChanges{}, err
	}
	files := make([]SessionFile, 0, len(entries))
	for _, entry := range entries {
		files = append(files, SessionFile{Path: entry.Path, Status: entry.Status, OldPath: entry.OldPath})
	}
	result.HeadCommit = headCommit
	result.HeadMoved = headMoved
	result.Files = files
	result.Additions = additions
	result.Deletions = deletions
	result.Truncated = truncated
	return result, nil
}

func (s *Service) SessionFileDiff(ctx context.Context, sessionID, path string, maxBytes int64) (SessionFileDiffResult, error) {
	if err := ctx.Err(); err != nil {
		return SessionFileDiffResult{}, err
	}
	changeCtx, availability, reason, err := s.resolveSessionChanges(ctx, sessionID)
	if err != nil {
		return SessionFileDiffResult{}, err
	}
	if availability != SessionChangesAvailable {
		return SessionFileDiffResult{}, &SessionChangesError{Availability: availability, Reason: reason}
	}
	diff, truncated, err := changeCtx.reader.DiffFileSince(ctx, changeCtx.workspace.Path, changeCtx.record.ObservedCommit, path, maxBytes)
	if err != nil {
		return SessionFileDiffResult{}, err
	}
	return SessionFileDiffResult{Path: path, Diff: diff, Truncated: truncated}, nil
}

func (s *Service) SessionRecordWorkspace(sessionID string) (string, error) {
	if s.records == nil {
		return "", ErrNotFound
	}
	record, found, err := s.records.Get(strings.TrimSpace(sessionID))
	if err != nil {
		return "", err
	}
	if !found {
		return "", ErrNotFound
	}
	return record.WorkspaceID, nil
}

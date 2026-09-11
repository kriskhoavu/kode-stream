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

// Availability values for SessionChanges. "available" means every field below
// was computed from Git; the other two mean the fields were not computed and
// Reason explains why.
const (
	SessionChangesAvailable   = "available"
	SessionChangesUnavailable = "unavailable"
	SessionChangesUnsupported = "unsupported"
)

// Reason codes paired with the unavailable/unsupported availability values.
const (
	ReasonContentProviderUnsupported = "content_provider_unsupported"
	ReasonWorkspaceMissing           = "workspace_missing"
	ReasonLaunchCommitUnknown        = "launch_commit_unknown"
	ReasonLaunchCommitMissing        = "launch_commit_missing"
)

// SessionFile describes one path changed since a session's launch commit.
type SessionFile struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	OldPath string `json:"oldPath,omitempty"`
}

// SessionChanges is the computed, unstored view of what a checkout has done
// since a session's launch commit. LaunchCommit is backed by the session
// record's ObservedCommit; the API calls it launchCommit.
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

// SessionFileDiffResult is the diff of one path since a session's launch
// commit.
type SessionFileDiffResult struct {
	Path      string `json:"path"`
	Diff      string `json:"diff"`
	Truncated bool   `json:"truncated"`
}

// SessionChangesError reports that SessionFileDiff could not run because the
// session's changes are unavailable or unsupported, mirroring the
// availability/reason pair SessionChanges returns inline.
type SessionChangesError struct {
	Availability string
	Reason       string
}

func (e *SessionChangesError) Error() string {
	return fmt.Sprintf("session changes %s: %s", e.Availability, e.Reason)
}

// sessionChangeReader is the subset of the Git adapter's since-commit readers
// SessionChanges and SessionFileDiff need. It is satisfied by *git.GitAdapter;
// tests supply a fake.
type sessionChangeReader interface {
	NameStatusSinceContext(ctx context.Context, workspacePath, commit string) ([]gitadapter.NameStatusEntry, bool, error)
	ShortStatSinceContext(ctx context.Context, workspacePath, commit string) (additions, deletions, files int, err error)
	DiffFileSince(ctx context.Context, workspacePath, commit, relPath string, maxBytes int64) (string, bool, error)
	RevListCountContext(ctx context.Context, workspacePath, from, to string) (int, error)
	CommitExistsContext(ctx context.Context, workspacePath, commit string) error
}

// sessionChangeContext is what the guard chain resolves before either
// SessionChanges or SessionFileDiff can compute anything.
type sessionChangeContext struct {
	record    SessionRecord
	workspace models.WorkspaceConfig
	reader    sessionChangeReader
}

// resolveSessionChanges runs the shared guard chain: session record lookup,
// workspace lookup, content-provider support, workspace path presence, launch
// commit presence, and launch commit existence in the checkout. err is
// non-nil only for infrastructure failures (including ErrNotFound for a
// missing session); a guard that simply makes changes unavailable or
// unsupported is reported through availability/reason instead.
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
	// s.runtimeMode is set once by server.go via SetRuntimeMode, from the
	// same runtime config it resolves at startup; a Service nobody calls
	// that on defaults to Cloud (see New), the conservative choice -- it
	// only changes the axis computation for a workspace with
	// AccessModeAgentBacked, and assuming Cloud can only turn a workspace
	// unsupported that Local would have allowed through, never the reverse.
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
			// A corrupt repository, a missing git binary, or any other
			// infrastructure failure is not "the commit does not exist" --
			// report it as a 500 instead of misreporting it as
			// launch_commit_missing.
			return sessionChangeContext{}, "", "", err
		}
		return sessionChangeContext{record: record, workspace: workspace}, SessionChangesUnavailable, ReasonLaunchCommitMissing, nil
	}
	return sessionChangeContext{record: record, workspace: workspace, reader: reader}, SessionChangesAvailable, "", nil
}

// SessionChanges reports what the session's workspace checkout has changed
// since the session's launch commit. It always returns a value with
// Availability set; err is reserved for a missing session (ErrNotFound) or an
// infrastructure failure.
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

// SessionFileDiff returns the diff of one path since the session's launch
// commit, bounded by maxBytes. It runs the same guard chain as SessionChanges
// and reports a guard failure as *SessionChangesError rather than inline
// fields, since there is no partial diff to return alongside it.
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

// SessionRecordWorkspace resolves the workspace ID owning a session record,
// for callers (the sessions controller) that must authorize a workspace
// before computing or exposing anything about the session.
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

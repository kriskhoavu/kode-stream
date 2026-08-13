package ai

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
)

type contextSessionRecords interface {
	ListContext(context.Context, string, string) ([]SessionRecord, error)
}

type sessionBranchResolver interface {
	CurrentBranch(string) (string, error)
	ResolveBranch(string, string) (string, string, error)
	Status(string, string) (models.GitStatus, error)
}

type SessionRecordView struct {
	SessionRecord
	Live bool `json:"live"`
}

func (s *Service) ConfigureSessionRecords(records SessionRecordRepository, branches sessionBranchResolver) *Service {
	s.records = records
	s.branches = branches
	s.configureSessionObserver()
	_ = s.ReconcileSessionRecords()
	return s
}

func (s *Service) SessionRecords(workspaceID, branch string) ([]SessionRecordView, error) {
	return s.SessionRecordsContext(context.Background(), workspaceID, branch)
}
func (s *Service) SessionRecordsContext(ctx context.Context, workspaceID, branch string) ([]SessionRecordView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.records == nil {
		return []SessionRecordView{}, nil
	}
	var records []SessionRecord
	var err error
	if repository, ok := s.records.(contextSessionRecords); ok {
		records, err = repository.ListContext(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(branch))
	} else {
		records, err = s.records.List(strings.TrimSpace(workspaceID), strings.TrimSpace(branch))
	}
	if err != nil {
		return nil, err
	}
	views := make([]SessionRecordView, 0, len(records))
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		live := false
		if s.embedded != nil {
			_, liveErr := s.embedded.Get(record.ID)
			live = liveErr == nil && (record.State == StateStarting || record.State == StateRunning)
		}
		views = append(views, SessionRecordView{SessionRecord: record, Live: live})
	}
	return views, nil
}

func (s *Service) ReconcileSessionRecords() error {
	if s.records == nil {
		return nil
	}
	live := map[string]Session{}
	if s.embedded != nil {
		for _, session := range s.embedded.List() {
			live[session.ID] = session
		}
	}
	records, err := s.records.Snapshot()
	if err != nil {
		return err
	}
	for _, record := range records {
		if session, ok := live[record.ID]; ok {
			s.syncSessionRecord(session)
			continue
		}
		if record.State == StateStarting || record.State == StateRunning {
			record.State = StateInterrupted
			record.EndedAt = s.sessionNow()
			record.LastKnownAt = record.EndedAt
			if _, err := s.records.Upsert(record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) configureSessionObserver() {
	if s.embedded == nil {
		return
	}
	if s.records == nil {
		s.embedded.SetObserver(nil)
		s.embedded.SetStartObserver(nil)
		return
	}
	s.embedded.SetObserver(s.syncSessionRecord)
	s.embedded.SetStartObserver(s.persistRunningSession)
}

func (s *Service) persistRunningSession(session Session) error {
	if s.records == nil {
		return nil
	}
	record, ok, err := s.records.Get(session.ID)
	if err != nil || !ok {
		if err != nil {
			return err
		}
		return errors.New("session record reservation is missing")
	}
	record.State, record.LastKnownAt = session.State, s.sessionNow()
	_, err = s.records.Upsert(record)
	return err
}

func (s *Service) syncSessionRecord(session Session) {
	if s.records == nil {
		return
	}
	s.sessionRecordMu.Lock()
	defer s.sessionRecordMu.Unlock()
	record, ok, err := s.records.Get(session.ID)
	if err != nil || !ok {
		return
	}
	// A launch compensation deliberately clears the idempotency key and records a
	// terminal failure. A late process observer must never resurrect its stale
	// starting/running copy and poison a legitimate retry.
	if record.State == StateFailed && record.IdempotencyKey == "" && session.State != StateFailed {
		return
	}
	record.State = session.State
	record.ExitCode = session.ExitCode
	record.LastKnownAt = s.sessionNow()
	if session.State == StateExited || session.State == StateCancelled || session.State == StateFailed || session.State == StateInterrupted {
		record.EndedAt = record.LastKnownAt
	}
	_, _ = s.records.Upsert(record)
}

func (s *Service) sessionNow() time.Time {
	if s.launch != nil && s.launch.now != nil {
		return s.launch.now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) idempotentEmbeddedResult(record SessionRecord) (EmbeddedResult, error) {
	result := EmbeddedResult{Record: &record}
	if s.embedded == nil {
		return result, nil
	}
	session, err := s.embedded.Get(record.ID)
	if err != nil {
		return result, nil
	}
	result.Session = session
	if session.State == StateStarting || session.State == StateRunning {
		grant, grantErr := s.embedded.IssueGrant(record.ID)
		if grantErr != nil {
			return EmbeddedResult{}, grantErr
		}
		result.Grant = grant
	}
	return result, nil
}

func (s *Service) validateSessionBranch(workspace models.WorkspaceConfig, expectedBranch, observedCommit string) (string, string, error) {
	resolver := s.branches
	if resolver == nil {
		resolver = gitadapter.New()
	}
	currentBranch, err := resolver.CurrentBranch(workspace.Path)
	if err != nil {
		return "", "", launchErrorWith("launch_failed", err)
	}
	expectedBranch = strings.TrimSpace(expectedBranch)
	if expectedBranch == "" {
		expectedBranch = currentBranch
	}
	if currentBranch != expectedBranch {
		details := map[string]string{"expectedBranch": expectedBranch, "currentBranch": currentBranch}
		if status, statusErr := resolver.Status(workspace.ID, workspace.Path); statusErr == nil {
			details["dirty"] = strconv.FormatBool(status.Dirty)
			details["conflicted"] = strconv.FormatBool(status.Conflicted)
		}
		return "", "", launchErrorWithDetails("terminal_branch_mismatch", "the workspace checkout no longer matches the requested branch", details)
	}
	_, currentCommit, err := resolver.ResolveBranch(workspace.Path, currentBranch)
	if err != nil {
		return "", "", launchErrorWith("launch_failed", err)
	}
	observedCommit = strings.TrimSpace(observedCommit)
	if observedCommit != "" && currentCommit != observedCommit {
		return "", "", launchErrorWithDetails("terminal_revision_mismatch", "the workspace branch changed after the plan reference was loaded", map[string]string{
			"branch":         currentBranch,
			"expectedCommit": observedCommit,
			"currentCommit":  currentCommit,
		})
	}
	return currentBranch, currentCommit, nil
}

func (s *Service) existingSession(workspaceID, idempotencyKey string) (EmbeddedResult, bool, error) {
	if s.records == nil || strings.TrimSpace(idempotencyKey) == "" {
		return EmbeddedResult{}, false, nil
	}
	record, found, err := s.records.FindByIdempotency(workspaceID, idempotencyKey)
	if err != nil || !found {
		return EmbeddedResult{}, found, err
	}
	result, err := s.idempotentEmbeddedResult(record)
	return result, true, err
}

func (s *Service) persistStartingSession(record SessionRecord) (SessionRecord, error) {
	if s.records == nil {
		return record, nil
	}
	return s.records.Upsert(record)
}

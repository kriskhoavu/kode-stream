package audit

import (
	"context"
	"log"
	"sync"

	"kode-stream/internal/common/models"
)

const (
	LocalActor  = "local"
	LegacyActor = "unknown"
)

// Recorder is the single append boundary used by mutation producers. It keeps
// cache invalidation and non-fatal persistence diagnostics consistent without
// coupling domain services to HTTP transport.
type Recorder struct {
	source       Repository
	mu           sync.RWMutex
	invalidators []func()
	report       func(AppendFailure)
}

type AppendFailure struct {
	Operation   string
	WorkspaceID string
	OwnerUserID string
	Err         error
}

type FailureReporter interface {
	ReportAuditAppendFailure(AppendFailure)
}

type FailureReporterFunc func(AppendFailure)

func (f FailureReporterFunc) ReportAuditAppendFailure(failure AppendFailure) { f(failure) }

func NewRecorder(source Repository) *Recorder {
	return NewRecorderWithReporter(source, FailureReporterFunc(func(failure AppendFailure) {
		log.Printf("audit_append_failure operation=%q workspace_id=%q owner_user_id=%q error=%q", failure.Operation, failure.WorkspaceID, failure.OwnerUserID, failure.Err)
	}))
}

func NewRecorderWithReporter(source Repository, reporter FailureReporter) *Recorder {
	if reporter == nil {
		reporter = FailureReporterFunc(func(AppendFailure) {})
	}
	return &Recorder{source: source, report: reporter.ReportAuditAppendFailure}
}

func (r *Recorder) AddInvalidator(invalidate func()) {
	if r == nil || invalidate == nil {
		return
	}
	r.mu.Lock()
	r.invalidators = append(r.invalidators, invalidate)
	r.mu.Unlock()
}

func (r *Recorder) Append(event models.AuditEvent) (models.AuditEvent, error) {
	if event.ActorUserID == "" {
		event.ActorUserID = LocalActor
	}
	if event.OwnerUserID == "" {
		event.OwnerUserID = LocalActor
	}
	stored, err := r.source.Append(event)
	if err != nil {
		r.report(AppendFailure{Operation: event.Operation, WorkspaceID: event.WorkspaceID, OwnerUserID: event.OwnerUserID, Err: err})
		return models.AuditEvent{}, err
	}
	r.mu.RLock()
	callbacks := append([]func(){}, r.invalidators...)
	r.mu.RUnlock()
	for _, invalidate := range callbacks {
		invalidate()
	}
	return stored, nil
}

func (r *Recorder) Recent(limit int) ([]models.AuditEvent, error) { return r.source.Recent(limit) }
func (r *Recorder) RecentContext(ctx context.Context, limit int) ([]models.AuditEvent, error) {
	return r.source.RecentContext(ctx, limit)
}
func (r *Recorder) QueryContext(ctx context.Context, query Query) ([]models.AuditEvent, error) {
	return r.source.QueryContext(ctx, query)
}

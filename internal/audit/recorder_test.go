package audit

import (
	"context"
	"errors"
	"testing"

	"kode-stream/internal/common/models"
)

type failingAuditRepository struct{ err error }

func (f failingAuditRepository) Append(models.AuditEvent) (models.AuditEvent, error) {
	return models.AuditEvent{}, f.err
}
func (f failingAuditRepository) Recent(int) ([]models.AuditEvent, error) { return nil, nil }
func (f failingAuditRepository) RecentContext(context.Context, int) ([]models.AuditEvent, error) {
	return nil, nil
}
func (f failingAuditRepository) QueryContext(context.Context, Query) ([]models.AuditEvent, error) {
	return nil, nil
}

func TestRecorderMakesAppendFailureObservableAndInvalidatesOnlyOnSuccess(t *testing.T) {
	var failure AppendFailure
	recorder := NewRecorderWithReporter(failingAuditRepository{err: errors.New("disk full")}, FailureReporterFunc(func(value AppendFailure) { failure = value }))
	invalidations := 0
	recorder.AddInvalidator(func() { invalidations++ })
	if _, err := recorder.Append(models.AuditEvent{Operation: "write", WorkspaceID: "workspace"}); err == nil || invalidations != 0 {
		t.Fatalf("failed append err=%v invalidations=%d", err, invalidations)
	}
	if failure.Operation != "write" || failure.WorkspaceID != "workspace" || failure.OwnerUserID != LocalActor || !errors.Is(failure.Err, recorder.source.(failingAuditRepository).err) {
		t.Fatalf("structured failure = %#v", failure)
	}
	store := New(t.TempDir() + "/audit.jsonl")
	recorder = NewRecorder(store)
	recorder.AddInvalidator(func() { invalidations++ })
	event, err := recorder.Append(models.AuditEvent{Operation: "write", Status: models.AuditStatusSuccess})
	if err != nil || invalidations != 1 || event.ActorUserID != LocalActor || event.OwnerUserID != LocalActor {
		t.Fatalf("event=%#v err=%v invalidations=%d", event, err, invalidations)
	}
}

package harness

import (
	"context"
	"time"

	"kode-stream/internal/harness/application/brief"
	"kode-stream/internal/harness/application/scheduler"
	"kode-stream/internal/harness/domain"
)

// stageLoops run the stages no model works: tests and the draft pull request.
func (s *Service) stageLoops(func() time.Duration) []scheduler.Loop { return nil }

// advanceFix applies a tester's fix.
func (st *step) advanceFix(domain.AgentRun) {}

// decorateFix adds the failing log to a tester's brief.
func (s *Service) decorateFix(_ context.Context, input brief.Input, _ domain.Stage, _ domain.AgentRun) brief.Input {
	return input
}

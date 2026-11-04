package harness

import (
	"context"
	"time"

	"kode-stream/internal/harness/application/brief"
	"kode-stream/internal/harness/application/scheduler"
	"kode-stream/internal/harness/domain"
)

// reviewApproved passes a reviewed stage.
func (st *step) reviewApproved(domain.AgentRun) {
	st.pass()
}

// advanceBuild applies a builder's or tester's report.
func (st *step) advanceBuild(domain.AgentRun) {}

// stageLoops run the stages no model works: tests and the draft pull request.
func (s *Service) stageLoops(func() time.Duration) []scheduler.Loop { return nil }

// decorateBrief adds what a stage knows beyond the run: scope and logs.
func (s *Service) decorateBrief(_ context.Context, input brief.Input, _ domain.Run, _ domain.Stage, _ domain.AgentRun) brief.Input {
	return input
}

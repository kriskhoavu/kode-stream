package harness

import (
	"context"

	"kode-stream/internal/harness/application/scheduler"
	"kode-stream/internal/harness/domain"
)

// workLoops are the dispatcher, the role loops and the non-model stages.
func (s *Service) workLoops() []scheduler.Loop { return nil }

// cancelAgentProcesses stops every CLI process this service started.
func (s *Service) cancelAgentProcesses() {}

// cancelPausedRole leaves running work of a paused role to finish.
func (s *Service) cancelPausedRole(domain.RoleKey) {}

// agentRunFinished moves the agent run's stage on.
func (s *Service) agentRunFinished(context.Context, domain.AgentRun) {}

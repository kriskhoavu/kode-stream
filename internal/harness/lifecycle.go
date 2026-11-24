package harness

import (
	"context"
	"errors"
	"time"

	"kode-stream/internal/harness/application/scheduler"
	"kode-stream/internal/harness/domain"
)

// A restart means every CLI process is gone, so agent runs left running are interrupted first.
func (s *Service) Start(ctx context.Context) error {
	if !s.Supported() || !s.config.LocalMode {
		return nil
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	interrupted, err := s.repository.InterruptRunning(ctx, s.now())
	if err != nil {
		return err
	}
	for _, agentRun := range interrupted {
		s.requeueInterrupted(ctx, agentRun)
	}
	orchestrator := scheduler.New(s.now, s.loops()...)
	s.mu.Lock()
	s.orchestrator = orchestrator
	s.running = true
	s.mu.Unlock()
	orchestrator.Start(context.WithoutCancel(ctx))
	return nil
}

func (s *Service) Stop(timeout time.Duration) {
	if s == nil {
		return
	}
	s.mu.Lock()
	orchestrator := s.orchestrator
	s.orchestrator = nil
	s.running = false
	s.mu.Unlock()
	s.cancelAgentProcesses()
	if orchestrator != nil {
		orchestrator.Stop(timeout)
	}
}

// A read failure falls back to the defaults, which keep the harness off.
func (s *Service) settingsNow(ctx context.Context) domain.Settings {
	settings, err := s.repository.LoadSettings(ctx)
	if err != nil {
		return domain.DefaultSettings()
	}
	return settings
}

func (s *Service) interval(pick func(domain.Intervals) int) func() time.Duration {
	return func() time.Duration {
		seconds := pick(s.settingsNow(context.Background()).Intervals)
		if seconds <= 0 {
			seconds = 60
		}
		return time.Duration(seconds) * time.Second
	}
}

func active(settings domain.Settings, key domain.RoleKey) bool {
	return settings.Enabled && !settings.Paused[key]
}

func (s *Service) loops() []scheduler.Loop {
	loops := []scheduler.Loop{
		{Name: string(domain.RoleKeyScout), Interval: s.interval(func(i domain.Intervals) int { return i.ScoutSeconds }), Wake: s.pollWake, Run: s.scoutCycle},
		{Name: string(domain.RoleKeySorter), Interval: s.interval(func(i domain.Intervals) int { return i.SorterSeconds }), Wake: s.sortWake, Run: s.sortCycle},
		{Name: "recovery", Interval: s.interval(func(i domain.Intervals) int { return i.WorkerSeconds }), Run: s.recoveryCycle},
	}
	loops = append(loops, s.notifierLoop())
	return append(loops, s.workLoops()...)
}

func (s *Service) scoutCycle(ctx context.Context) error {
	settings := s.settingsNow(ctx)
	if !active(settings, domain.RoleKeyScout) || s.poller == nil {
		return nil
	}
	polled, err := s.poller.Poll(ctx, settings, "")
	s.answerFromSource(ctx)
	for _, health := range polled {
		if health.Added > 0 {
			wake(s.sortWake)
			break
		}
	}
	var failures []error
	for _, health := range polled {
		if health.LastError != "" {
			failures = append(failures, errors.New(health.WorkspaceID+" "+string(health.Kind)+": "+health.LastError))
		}
	}
	return errors.Join(append(failures, err)...)
}

func (s *Service) sortCycle(ctx context.Context) error {
	settings := s.settingsNow(ctx)
	if !active(settings, domain.RoleKeySorter) {
		return nil
	}
	return s.Sort(ctx)
}

func (s *Service) recoveryCycle(ctx context.Context) error {
	agentRuns, _, err := s.repository.RecoverExpiredLeases(ctx, s.now())
	if err != nil {
		return err
	}
	for _, agentRun := range agentRuns {
		s.requeueInterrupted(ctx, agentRun)
	}
	return nil
}

func (s *Service) requeueInterrupted(ctx context.Context, agentRun domain.AgentRun) {
	expect := domain.Expect{State: string(agentRun.State), Revision: agentRun.Revision}
	if agentRun.Attempt+1 <= domain.MaxStageAttempts {
		agentRun.State = domain.AgentQueued
		agentRun.Attempt++
		agentRun.QueuedAt = s.now()
		agentRun.StartedAt = nil
		if _, err := s.repository.SaveAgentRun(ctx, agentRun, expect); err == nil {
			s.wakeWork()
		}
		return
	}
	now := s.now()
	agentRun.State = domain.AgentFailed
	agentRun.FinishedAt = &now
	if agentRun.FailureReason == "" {
		agentRun.FailureReason = "interrupted too often"
	}
	if saved, err := s.repository.SaveAgentRun(ctx, agentRun, expect); err == nil {
		s.agentRunFinished(ctx, saved)
	}
}

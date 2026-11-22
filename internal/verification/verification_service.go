package verification

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"kode-stream/internal/common/models"
	appruntime "kode-stream/internal/runtime"
	"kode-stream/internal/workspace/registry"
)

type Service struct {
	registry        registry.Repository
	runtime         *appruntime.Service
	mu              sync.RWMutex
	jobs            map[string]*Job
	seq             atomic.Int64
	slots           chan struct{}
	timeout         time.Duration
	maxTerminal     int
	retention       time.Duration
	removeArtifact  func(string) error
	cleanupWarnings []string
	ctx             context.Context
	cancel          context.CancelFunc
	fingerprint     Fingerprinter
}

func NewService(reg registry.Repository, runtimeService *appruntime.Service) *Service {
	return NewServiceWithPolicy(reg, runtimeService, 2, 10*time.Minute)
}

func NewServiceWithPolicy(reg registry.Repository, runtimeService *appruntime.Service, maxRunning int, timeout time.Duration) *Service {
	if maxRunning <= 0 {
		maxRunning = 1
	}
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithCancel(context.Background())
	service := &Service{registry: reg, runtime: runtimeService, jobs: map[string]*Job{}, slots: make(chan struct{}, maxRunning), timeout: timeout, maxTerminal: defaultMaxTerminalJobs, retention: defaultTerminalJobRetention, removeArtifact: os.RemoveAll, ctx: ctx, cancel: cancel, fingerprint: GitFingerprinter{}}
	if reg != nil {
		service.cleanupOrphanArtifacts()
	}
	return service
}

func (s *Service) ConfigureFingerprinter(fingerprinter Fingerprinter) *Service {
	if fingerprinter != nil {
		s.fingerprint = fingerprinter
	}
	return s
}

func (s *Service) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Service) Limits() (int, time.Duration) {
	if s == nil {
		return 0, 0
	}
	return cap(s.slots), s.timeout
}

// JobCount is a diagnostic snapshot of in-memory retained jobs.
func (s *Service) JobCount() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.jobs)
}

func (s *Service) Start(workspaceID string, input CreateInput) (Job, error) {
	if err := s.ctx.Err(); err != nil {
		return Job{}, err
	}
	workspace, ok, err := s.registry.Get(workspaceID)
	if err != nil {
		return Job{}, err
	}
	if !ok {
		return Job{}, errors.New("workspace not found")
	}
	if workspace.Runtime == nil {
		return Job{}, errors.New("workspace runtime is not configured")
	}
	mode := normalizeJobMode(input.Mode)
	profile := input.Profile
	if profile == "" {
		profile = appruntime.VerifyProfileSmoke
	}
	automation, err := prepareAutomationJob(workspace.Runtime, input)
	if err != nil {
		return Job{}, err
	}
	job := &Job{
		ID:                 s.nextID(),
		WorkspaceID:        workspaceID,
		Mode:               mode,
		Profile:            profile,
		Status:             JobStatusQueued,
		Trigger:            strings.TrimSpace(input.Trigger),
		Provider:           strings.TrimSpace(input.Provider),
		SessionID:          strings.TrimSpace(input.SessionID),
		TerminalMode:       strings.TrimSpace(input.TerminalMode),
		Runtime:            workspace.Runtime,
		Environment:        automation.environment,
		DisplayMode:        automation.displayMode,
		SelectedSpecs:      automation.selectedSpecs,
		AutomationRepoPath: automation.repositoryPath,
		RenderedCommand:    automation.renderedCommand,
		Steps:              []StepResult{},
		Artifacts:          []Artifact{},
		Freshness:          FreshnessInconclusive,
	}
	if fingerprint, fingerprintErr := s.fingerprint.Fingerprint(workspace, *job); fingerprintErr == nil {
		job.StartFingerprint = &fingerprint
	}
	select {
	case s.slots <- struct{}{}:
	default:
		return Job{}, errors.New("verification queue is full")
	}
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()
	initial := cloneJob(*job)

	go s.run(job.ID, workspace)
	return initial, nil
}

func (s *Service) Get(workspaceID, jobID string) (Job, bool) {
	return s.GetContext(context.Background(), workspaceID, jobID)
}

func (s *Service) GetContext(ctx context.Context, workspaceID, jobID string) (Job, bool) {
	copy, ok := s.snapshot(workspaceID, jobID)
	if !ok {
		return Job{}, false
	}
	if workspace, found, err := s.registry.Get(workspaceID); err == nil && found {
		if current, fingerprintErr := s.fingerprintContext(ctx, workspace, copy); fingerprintErr == nil {
			copy.CurrentFingerprint = &current
		}
	}
	copy.Freshness = projectFreshness(copy)
	return copy, true
}

func (s *Service) fingerprintContext(ctx context.Context, workspace models.WorkspaceConfig, job Job) (RepositoryFingerprint, error) {
	if contextual, ok := s.fingerprint.(ContextFingerprinter); ok {
		return contextual.FingerprintContext(ctx, workspace, job)
	}
	return s.fingerprint.Fingerprint(workspace, job)
}

func (s *Service) Latest(workspaceID string) (Job, bool) {
	s.mu.RLock()
	var latest Job
	found := false
	for _, job := range s.jobs {
		if job.WorkspaceID != workspaceID || (found && !job.StartedAt.After(latest.StartedAt)) {
			continue
		}
		latest = cloneJob(*job)
		found = true
	}
	s.mu.RUnlock()
	if !found {
		return Job{}, false
	}
	return s.Get(workspaceID, latest.ID)
}

func (s *Service) Rerun(workspaceID, jobID string, profile appruntime.VerifyProfile) (Job, error) {
	previous, ok := s.Get(workspaceID, jobID)
	if !ok {
		return Job{}, errors.New("verification job not found")
	}
	return s.Start(workspaceID, CreateInput{
		Profile:       profile,
		Mode:          previous.Mode,
		Environment:   previous.Environment,
		DisplayMode:   previous.DisplayMode,
		SelectedSpecs: previous.SelectedSpecs,
		Trigger:       "rerun",
		Provider:      previous.Provider,
		SessionID:     previous.SessionID,
		TerminalMode:  previous.TerminalMode,
	})
}

func (s *Service) IngestCheckpoint(workspaceID string, event CheckpointEvent) (Job, error) {
	eventType := strings.TrimSpace(event.EventType)
	if eventType == "" {
		eventType = "manual_checkpoint"
	}
	profile := event.Profile
	if profile == "" {
		profile = appruntime.VerifyProfileSmoke
	}
	return s.Start(workspaceID, CreateInput{
		Profile:      profile,
		Trigger:      "checkpoint:" + eventType,
		Provider:     strings.TrimSpace(event.Provider),
		SessionID:    strings.TrimSpace(event.SessionID),
		TerminalMode: strings.TrimSpace(event.TerminalMode),
	})
}

func (s *Service) run(jobID string, workspace models.WorkspaceConfig) {
	defer func() { <-s.slots }()
	job, ok := s.snapshotAny(jobID)
	if !ok {
		return
	}
	s.mutate(jobID, func(state *Job) {
		state.Status = JobStatusRunning
		state.StartedAt = time.Now().UTC()
	})
	job, _ = s.snapshotAny(jobID)
	finalStatus := JobStatusFailed
	defer func() {
		s.captureFinishFingerprint(jobID, workspace)
		s.mutate(jobID, func(state *Job) {
			state.Status = finalStatus
			state.FinishedAt = time.Now().UTC()
		})
		s.pruneTerminalJobs()
	}()
	config := job.Runtime
	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()

	artifactRoot, err := appruntime.EnsureArtifactRoot(workspace.Path, job.ID)
	if err != nil {
		s.failJob(jobID, FailureTypeInfra, 30, "create artifact directory", err)
		return
	}
	runtimeLogPath := filepath.Join(artifactRoot, "runtime.log")
	verifyLogPath := filepath.Join(artifactRoot, verifyLogName(job.Mode))
	runtimeLog, err := os.Create(runtimeLogPath)
	if err != nil {
		s.failJob(jobID, FailureTypeInfra, 30, "open runtime log", err)
		return
	}
	defer runtimeLog.Close()
	verifyLog, err := os.Create(verifyLogPath)
	if err != nil {
		s.failJob(jobID, FailureTypeInfra, 30, "open verify log", err)
		return
	}
	defer verifyLog.Close()

	if err := s.runStep(jobID, "prepare", runtimeLog, func() error {
		return s.runtime.Prepare(ctx, workspace.Path, config, runtimeLog)
	}); err != nil {
		s.failJob(jobID, FailureTypeInfra, 30, "prepare failed", err)
		s.collectArtifacts(jobID, artifactRoot)
		return
	}
	if err := s.runStep(jobID, "up", runtimeLog, func() error {
		return s.runtime.Up(ctx, workspace.Path, config, runtimeLog)
	}); err != nil {
		_ = s.cleanupRuntime(workspace.Path, config, runtimeLog)
		s.failJob(jobID, FailureTypeBoot, 10, "startup failed", err)
		s.collectArtifacts(jobID, artifactRoot)
		return
	}
	if err := s.runStep(jobID, "health", runtimeLog, func() error {
		return s.runtime.Health(ctx, workspace.Path, config, runtimeLog)
	}); err != nil {
		_ = s.cleanupRuntime(workspace.Path, config, runtimeLog)
		s.failJob(jobID, FailureTypeBoot, 10, "health checks failed", err)
		s.collectArtifacts(jobID, artifactRoot)
		return
	}
	if err := s.runStep(jobID, verifyStepName(job.Mode), verifyLog, func() error {
		if job.Mode == JobModeAutomation {
			return s.runtime.RunCommand(ctx, job.AutomationRepoPath, job.RenderedCommand, verifyLog)
		}
		return s.runtime.Verify(ctx, workspace.Path, config, job.Profile, verifyLog)
	}); err != nil {
		_ = s.cleanupRuntime(workspace.Path, config, runtimeLog)
		s.failJob(jobID, FailureTypeTest, 20, "verification failed", err)
		s.collectArtifacts(jobID, artifactRoot)
		s.collectAutomationArtifacts(jobID)
		return
	}
	_ = s.runStep(jobID, "down", runtimeLog, func() error {
		return s.cleanupRuntime(workspace.Path, config, runtimeLog)
	})

	s.collectArtifacts(jobID, artifactRoot)
	s.collectAutomationArtifacts(jobID)
	finalStatus = JobStatusPassed
	s.mutate(jobID, func(state *Job) { state.ExitCode = 0 })
}

func (s *Service) cleanupRuntime(workspacePath string, config *models.WorkspaceRuntimeConfig, out *os.File) error {
	timeout := min(s.timeout, maxRuntimeCleanupDuration)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.runtime.Down(ctx, workspacePath, config, out)
}

func (s *Service) captureFinishFingerprint(jobID string, workspace models.WorkspaceConfig) {
	if current, found, err := s.registry.Get(workspace.ID); err == nil && found {
		workspace = current
	}
	job, ok := s.snapshotAny(jobID)
	if !ok {
		return
	}
	if fingerprint, err := s.fingerprint.Fingerprint(workspace, job); err == nil {
		s.mutate(jobID, func(state *Job) { state.FinishFingerprint = &fingerprint })
	}
}

func verifyLogName(mode JobMode) string {
	if mode == JobModeAutomation {
		return "automation.log"
	}
	return "verify.log"
}

func verifyStepName(mode JobMode) string {
	if mode == JobModeAutomation {
		return "automation"
	}
	return "verify"
}

func (s *Service) runStep(jobID, name string, _ *os.File, run func() error) error {
	start := time.Now()
	err := run()
	step := StepResult{Step: name, At: time.Now().UTC(), DurationMS: time.Since(start).Milliseconds()}
	if err != nil {
		step.Status = "failed"
		step.Message = err.Error()
	} else {
		step.Status = "ok"
	}
	s.mutate(jobID, func(job *Job) { job.Steps = append(job.Steps, step) })
	return err
}

package verification

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"kode-stream/internal/common/models"
	appruntime "kode-stream/internal/runtime"
	"kode-stream/internal/workspace/registry"
)

type JobStatus string

const (
	JobStatusQueued  JobStatus = "queued"
	JobStatusRunning JobStatus = "running"
	JobStatusPassed  JobStatus = "passed"
	JobStatusFailed  JobStatus = "failed"
)

type FailureType string

const (
	FailureTypeBoot  FailureType = "boot_failure"
	FailureTypeTest  FailureType = "test_failure"
	FailureTypeInfra FailureType = "infra_failure"
)

type StepResult struct {
	Step       string    `json:"step"`
	Status     string    `json:"status"`
	Message    string    `json:"message,omitempty"`
	DurationMS int64     `json:"durationMs"`
	At         time.Time `json:"at"`
}

type Artifact struct {
	Kind      string    `json:"kind"`
	Path      string    `json:"path"`
	SizeBytes int64     `json:"sizeBytes"`
	CreatedAt time.Time `json:"createdAt"`
}

type Job struct {
	ID           string                         `json:"id"`
	WorkspaceID  string                         `json:"workspaceId"`
	Profile      appruntime.VerifyProfile       `json:"profile"`
	Status       JobStatus                      `json:"status"`
	FailureType  FailureType                    `json:"failureType,omitempty"`
	ExitCode     int                            `json:"exitCode"`
	Trigger      string                         `json:"trigger,omitempty"`
	Provider     string                         `json:"provider,omitempty"`
	SessionID    string                         `json:"sessionId,omitempty"`
	TerminalMode string                         `json:"terminalMode,omitempty"`
	StartedAt    time.Time                      `json:"startedAt"`
	FinishedAt   time.Time                      `json:"finishedAt,omitempty"`
	Steps        []StepResult                   `json:"steps"`
	Artifacts    []Artifact                     `json:"artifacts"`
	Runtime      *models.WorkspaceRuntimeConfig `json:"runtime,omitempty"`
}

type CreateInput struct {
	Profile      appruntime.VerifyProfile `json:"profile"`
	Trigger      string                   `json:"trigger,omitempty"`
	Provider     string                   `json:"provider,omitempty"`
	SessionID    string                   `json:"sessionId,omitempty"`
	TerminalMode string                   `json:"terminalMode,omitempty"`
}

type CheckpointEvent struct {
	EventType    string                   `json:"eventType"`
	Profile      appruntime.VerifyProfile `json:"profile,omitempty"`
	Provider     string                   `json:"provider,omitempty"`
	SessionID    string                   `json:"sessionId,omitempty"`
	TerminalMode string                   `json:"terminalMode,omitempty"`
}

type Service struct {
	registry *registry.Registry
	runtime  *appruntime.Service
	mu       sync.RWMutex
	jobs     map[string]*Job
	seq      atomic.Int64
}

func NewService(reg *registry.Registry, runtimeService *appruntime.Service) *Service {
	return &Service{registry: reg, runtime: runtimeService, jobs: map[string]*Job{}}
}

func (s *Service) Start(workspaceID string, input CreateInput) (Job, error) {
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
	profile := input.Profile
	if profile == "" {
		profile = appruntime.VerifyProfileSmoke
	}
	job := &Job{
		ID:           s.nextID(),
		WorkspaceID:  workspaceID,
		Profile:      profile,
		Status:       JobStatusQueued,
		Trigger:      strings.TrimSpace(input.Trigger),
		Provider:     strings.TrimSpace(input.Provider),
		SessionID:    strings.TrimSpace(input.SessionID),
		TerminalMode: strings.TrimSpace(input.TerminalMode),
		Runtime:      workspace.Runtime,
		Steps:        []StepResult{},
		Artifacts:    []Artifact{},
	}
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()

	go s.run(job.ID, workspace)
	return cloneJob(*job), nil
}

func (s *Service) Get(workspaceID, jobID string) (Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobID]
	if !ok || job.WorkspaceID != workspaceID {
		return Job{}, false
	}
	copy := cloneJob(*job)
	copy.Artifacts = append([]Artifact(nil), job.Artifacts...)
	copy.Steps = append([]StepResult(nil), job.Steps...)
	return copy, true
}

func (s *Service) Artifacts(workspaceID, jobID string) ([]Artifact, error) {
	job, ok := s.Get(workspaceID, jobID)
	if !ok {
		return nil, errors.New("verification job not found")
	}
	artifacts := append([]Artifact(nil), job.Artifacts...)
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

func (s *Service) Rerun(workspaceID, jobID string, profile appruntime.VerifyProfile) (Job, error) {
	previous, ok := s.Get(workspaceID, jobID)
	if !ok {
		return Job{}, errors.New("verification job not found")
	}
	return s.Start(workspaceID, CreateInput{
		Profile:      profile,
		Trigger:      "rerun",
		Provider:     previous.Provider,
		SessionID:    previous.SessionID,
		TerminalMode: previous.TerminalMode,
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
	job, ok := s.getInternal(jobID)
	if !ok {
		return
	}
	job.Status = JobStatusRunning
	job.StartedAt = time.Now().UTC()
	config := job.Runtime

	artifactRoot, err := appruntime.EnsureArtifactRoot(workspace.Path, job.ID)
	if err != nil {
		s.failJob(job, FailureTypeInfra, 30, "create artifact directory", err)
		return
	}
	runtimeLogPath := filepath.Join(artifactRoot, "runtime.log")
	verifyLogPath := filepath.Join(artifactRoot, "verify.log")
	runtimeLog, err := os.Create(runtimeLogPath)
	if err != nil {
		s.failJob(job, FailureTypeInfra, 30, "open runtime log", err)
		return
	}
	defer runtimeLog.Close()
	verifyLog, err := os.Create(verifyLogPath)
	if err != nil {
		s.failJob(job, FailureTypeInfra, 30, "open verify log", err)
		return
	}
	defer verifyLog.Close()

	ctx := context.Background()
	if err := s.runStep(job, "prepare", runtimeLog, func() error {
		return s.runtime.Prepare(ctx, workspace.Path, config, runtimeLog)
	}); err != nil {
		s.failJob(job, FailureTypeInfra, 30, "prepare failed", err)
		s.collectArtifacts(job, artifactRoot)
		return
	}
	if err := s.runStep(job, "up", runtimeLog, func() error {
		return s.runtime.Up(ctx, workspace.Path, config, runtimeLog)
	}); err != nil {
		s.failJob(job, FailureTypeBoot, 10, "startup failed", err)
		s.collectArtifacts(job, artifactRoot)
		_ = s.runtime.Down(context.Background(), workspace.Path, config, runtimeLog)
		return
	}
	if err := s.runStep(job, "health", runtimeLog, func() error {
		return s.runtime.Health(ctx, workspace.Path, config, runtimeLog)
	}); err != nil {
		s.failJob(job, FailureTypeBoot, 10, "health checks failed", err)
		s.collectArtifacts(job, artifactRoot)
		_ = s.runtime.Down(context.Background(), workspace.Path, config, runtimeLog)
		return
	}
	if err := s.runStep(job, "verify", verifyLog, func() error {
		return s.runtime.Verify(ctx, workspace.Path, config, job.Profile, verifyLog)
	}); err != nil {
		s.failJob(job, FailureTypeTest, 20, "verification failed", err)
		s.collectArtifacts(job, artifactRoot)
		_ = s.runtime.Down(context.Background(), workspace.Path, config, runtimeLog)
		return
	}
	_ = s.runStep(job, "down", runtimeLog, func() error {
		return s.runtime.Down(context.Background(), workspace.Path, config, runtimeLog)
	})

	s.collectArtifacts(job, artifactRoot)
	job.Status = JobStatusPassed
	job.ExitCode = 0
	job.FinishedAt = time.Now().UTC()
}

func (s *Service) runStep(job *Job, name string, _ *os.File, run func() error) error {
	start := time.Now()
	err := run()
	step := StepResult{Step: name, At: time.Now().UTC(), DurationMS: time.Since(start).Milliseconds()}
	if err != nil {
		step.Status = "failed"
		step.Message = err.Error()
	} else {
		step.Status = "ok"
	}
	job.Steps = append(job.Steps, step)
	return err
}

func (s *Service) collectArtifacts(job *Job, root string) {
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = filepath.Base(path)
		}
		artifact := Artifact{
			Kind:      classifyArtifact(rel),
			Path:      filepath.ToSlash(path),
			SizeBytes: info.Size(),
			CreatedAt: time.Now().UTC(),
		}
		job.Artifacts = append(job.Artifacts, artifact)
		return nil
	})
}

func classifyArtifact(path string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	switch {
	case strings.Contains(lower, "trace"):
		return "playwright_trace"
	case strings.Contains(lower, "video"):
		return "playwright_video"
	case strings.Contains(lower, "screenshot") || strings.HasSuffix(lower, ".png"):
		return "playwright_screenshot"
	case strings.Contains(lower, "report"):
		return "playwright_report"
	case strings.Contains(lower, "verify"):
		return "verify_log"
	default:
		return "runtime_log"
	}
}

func (s *Service) failJob(job *Job, failure FailureType, code int, message string, err error) {
	job.Status = JobStatusFailed
	job.FailureType = failure
	job.ExitCode = code
	job.FinishedAt = time.Now().UTC()
	job.Steps = append(job.Steps, StepResult{
		Step:       "failure",
		Status:     "failed",
		Message:    fmt.Sprintf("%s: %v", message, err),
		DurationMS: 0,
		At:         time.Now().UTC(),
	})
}

func (s *Service) getInternal(jobID string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobID]
	return job, ok
}

func (s *Service) nextID() string {
	n := s.seq.Add(1)
	return "verify-" + strconv.FormatInt(time.Now().UTC().Unix(), 10) + "-" + strconv.FormatInt(n, 10)
}

func cloneJob(job Job) Job {
	copy := job
	copy.Steps = append([]StepResult(nil), job.Steps...)
	copy.Artifacts = append([]Artifact(nil), job.Artifacts...)
	return copy
}

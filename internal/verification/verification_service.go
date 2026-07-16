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

type JobMode string

const (
	JobModeRuntime    JobMode = "runtime"
	JobModeAutomation JobMode = "automation"
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
	ID                 string                         `json:"id"`
	WorkspaceID        string                         `json:"workspaceId"`
	Mode               JobMode                        `json:"mode"`
	Profile            appruntime.VerifyProfile       `json:"profile"`
	Environment        string                         `json:"environment,omitempty"`
	DisplayMode        models.AutomationDisplayMode   `json:"displayMode,omitempty"`
	SelectedSpecs      []string                       `json:"selectedSpecs,omitempty"`
	AutomationRepoPath string                         `json:"automationRepoPath,omitempty"`
	RenderedCommand    string                         `json:"renderedCommand,omitempty"`
	Status             JobStatus                      `json:"status"`
	FailureType        FailureType                    `json:"failureType,omitempty"`
	ExitCode           int                            `json:"exitCode"`
	Trigger            string                         `json:"trigger,omitempty"`
	Provider           string                         `json:"provider,omitempty"`
	SessionID          string                         `json:"sessionId,omitempty"`
	TerminalMode       string                         `json:"terminalMode,omitempty"`
	StartedAt          time.Time                      `json:"startedAt"`
	FinishedAt         time.Time                      `json:"finishedAt,omitempty"`
	Steps              []StepResult                   `json:"steps"`
	Artifacts          []Artifact                     `json:"artifacts"`
	Runtime            *models.WorkspaceRuntimeConfig `json:"runtime,omitempty"`
}

type CreateInput struct {
	Profile       appruntime.VerifyProfile     `json:"profile"`
	Mode          JobMode                      `json:"mode,omitempty"`
	Environment   string                       `json:"environment,omitempty"`
	DisplayMode   models.AutomationDisplayMode `json:"displayMode,omitempty"`
	SelectedSpecs []string                     `json:"selectedSpecs,omitempty"`
	Trigger       string                       `json:"trigger,omitempty"`
	Provider      string                       `json:"provider,omitempty"`
	SessionID     string                       `json:"sessionId,omitempty"`
	TerminalMode  string                       `json:"terminalMode,omitempty"`
}

type CheckpointEvent struct {
	EventType    string                   `json:"eventType"`
	Profile      appruntime.VerifyProfile `json:"profile,omitempty"`
	Provider     string                   `json:"provider,omitempty"`
	SessionID    string                   `json:"sessionId,omitempty"`
	TerminalMode string                   `json:"terminalMode,omitempty"`
}

type Service struct {
	registry registry.Repository
	runtime  *appruntime.Service
	mu       sync.RWMutex
	jobs     map[string]*Job
	seq      atomic.Int64
	slots    chan struct{}
	timeout  time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
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
	return &Service{registry: reg, runtime: runtimeService, jobs: map[string]*Job{}, slots: make(chan struct{}, maxRunning), timeout: timeout, ctx: ctx, cancel: cancel}
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
	}
	select {
	case s.slots <- struct{}{}:
	default:
		return Job{}, errors.New("verification queue is full")
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
	job, ok := s.getInternal(jobID)
	if !ok {
		return
	}
	job.Status = JobStatusRunning
	job.StartedAt = time.Now().UTC()
	config := job.Runtime
	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()

	artifactRoot, err := appruntime.EnsureArtifactRoot(workspace.Path, job.ID)
	if err != nil {
		s.failJob(job, FailureTypeInfra, 30, "create artifact directory", err)
		return
	}
	runtimeLogPath := filepath.Join(artifactRoot, "runtime.log")
	verifyLogPath := filepath.Join(artifactRoot, verifyLogName(job.Mode))
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
		_ = s.runtime.Down(context.Background(), workspace.Path, config, runtimeLog)
		s.failJob(job, FailureTypeBoot, 10, "startup failed", err)
		s.collectArtifacts(job, artifactRoot)
		return
	}
	if err := s.runStep(job, "health", runtimeLog, func() error {
		return s.runtime.Health(ctx, workspace.Path, config, runtimeLog)
	}); err != nil {
		_ = s.runtime.Down(context.Background(), workspace.Path, config, runtimeLog)
		s.failJob(job, FailureTypeBoot, 10, "health checks failed", err)
		s.collectArtifacts(job, artifactRoot)
		return
	}
	if err := s.runStep(job, verifyStepName(job.Mode), verifyLog, func() error {
		if job.Mode == JobModeAutomation {
			return s.runtime.RunCommand(ctx, job.AutomationRepoPath, job.RenderedCommand, verifyLog)
		}
		return s.runtime.Verify(ctx, workspace.Path, config, job.Profile, verifyLog)
	}); err != nil {
		_ = s.runtime.Down(context.Background(), workspace.Path, config, runtimeLog)
		s.failJob(job, FailureTypeTest, 20, "verification failed", err)
		s.collectArtifacts(job, artifactRoot)
		s.collectAutomationArtifacts(job)
		return
	}
	_ = s.runStep(job, "down", runtimeLog, func() error {
		return s.runtime.Down(context.Background(), workspace.Path, config, runtimeLog)
	})

	s.collectArtifacts(job, artifactRoot)
	s.collectAutomationArtifacts(job)
	job.Status = JobStatusPassed
	job.ExitCode = 0
	job.FinishedAt = time.Now().UTC()
}

type automationJobConfig struct {
	environment     string
	displayMode     models.AutomationDisplayMode
	selectedSpecs   []string
	repositoryPath  string
	renderedCommand string
}

func normalizeJobMode(mode JobMode) JobMode {
	if mode == "" {
		return JobModeRuntime
	}
	return mode
}

func prepareAutomationJob(runtimeConfig *models.WorkspaceRuntimeConfig, input CreateInput) (automationJobConfig, error) {
	if normalizeJobMode(input.Mode) != JobModeAutomation {
		if input.Mode != "" && input.Mode != JobModeRuntime {
			return automationJobConfig{}, errors.New("verification mode is invalid")
		}
		return automationJobConfig{}, nil
	}
	if runtimeConfig.Automation == nil || !runtimeConfig.Automation.Enabled {
		return automationJobConfig{}, errors.New("runtime automation is not configured")
	}
	selectedSpecs, err := validateSelectedSpecs(runtimeConfig.Automation.RepositoryPath, input.SelectedSpecs)
	if err != nil {
		return automationJobConfig{}, err
	}
	if len(selectedSpecs) == 0 {
		return automationJobConfig{}, errors.New("selectedSpecs is required for automation verification")
	}
	environment := strings.TrimSpace(input.Environment)
	if environment == "" {
		environment = runtimeConfig.Automation.DefaultEnvironment
	}
	displayMode := normalizeAutomationDisplayMode(input.DisplayMode)
	rendered := renderAutomationCommand(runtimeConfig.Automation.CommandTemplate, runtimeConfig.Automation.Runner, environment, selectedSpecs, displayMode)
	if strings.TrimSpace(rendered) == "" {
		return automationJobConfig{}, errors.New("runtime automation commandTemplate is required")
	}
	return automationJobConfig{
		environment:     environment,
		displayMode:     displayMode,
		selectedSpecs:   selectedSpecs,
		repositoryPath:  runtimeConfig.Automation.RepositoryPath,
		renderedCommand: rendered,
	}, nil
}

func validateSelectedSpecs(repositoryPath string, specs []string) ([]string, error) {
	root, err := filepath.Abs(strings.TrimSpace(repositoryPath))
	if err != nil || root == "" {
		return nil, errors.New("runtime automation repositoryPath is invalid")
	}
	selected := make([]string, 0, len(specs))
	seen := map[string]struct{}{}
	for _, spec := range specs {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(spec)))
		if clean == "" || clean == "." {
			continue
		}
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("selected spec %q must be relative", spec)
		}
		full, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(clean)))
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(root, full)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("selected spec %q must stay inside the automation repository", spec)
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		selected = append(selected, clean)
	}
	return selected, nil
}

func normalizeAutomationDisplayMode(mode models.AutomationDisplayMode) models.AutomationDisplayMode {
	if mode == models.AutomationDisplayModeVisible {
		return models.AutomationDisplayModeVisible
	}
	return models.AutomationDisplayModeSilent
}

func renderAutomationCommand(template string, runner models.AutomationRunner, environment string, selectedSpecs []string, displayMode models.AutomationDisplayMode) string {
	rendered := strings.TrimSpace(template)
	modeArgs := automationModeArgs(runner, displayMode)
	hasModePlaceholder := strings.Contains(rendered, "{modeArgs}") || strings.Contains(rendered, "{headed}") || strings.Contains(rendered, "{browser}")
	rendered = strings.ReplaceAll(rendered, "{env}", environment)
	rendered = strings.ReplaceAll(rendered, "{specs}", strings.Join(selectedSpecs, ","))
	rendered = strings.ReplaceAll(rendered, "{modeArgs}", modeArgs)
	rendered = strings.ReplaceAll(rendered, "{headed}", automationHeadedArg(displayMode))
	rendered = strings.ReplaceAll(rendered, "{browser}", automationBrowserArg(runner, displayMode))
	if modeArgs != "" && !hasModePlaceholder {
		rendered = strings.TrimSpace(rendered + " " + modeArgs)
	}
	return rendered
}

func automationModeArgs(runner models.AutomationRunner, displayMode models.AutomationDisplayMode) string {
	if displayMode != models.AutomationDisplayModeVisible {
		return ""
	}
	if runner == models.AutomationRunnerPlaywright {
		return "--headed --project=chromium"
	}
	return "--headed --browser chrome"
}

func automationHeadedArg(displayMode models.AutomationDisplayMode) string {
	if displayMode == models.AutomationDisplayModeVisible {
		return "--headed"
	}
	return ""
}

func automationBrowserArg(runner models.AutomationRunner, displayMode models.AutomationDisplayMode) string {
	if displayMode != models.AutomationDisplayModeVisible {
		return ""
	}
	if runner == models.AutomationRunnerPlaywright {
		return "--project=chromium"
	}
	return "--browser chrome"
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

func (s *Service) collectAutomationArtifacts(job *Job) {
	if job.Mode != JobModeAutomation || job.Runtime == nil || job.Runtime.Automation == nil {
		return
	}
	root := strings.TrimSpace(job.AutomationRepoPath)
	if root == "" {
		return
	}
	for _, relRoot := range job.Runtime.Automation.ArtifactPaths {
		artifactPath := filepath.Join(root, filepath.FromSlash(relRoot))
		_ = filepath.WalkDir(artifactPath, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return nil
			}
			job.Artifacts = append(job.Artifacts, Artifact{
				Kind:      classifyArtifact(path),
				Path:      filepath.ToSlash(path),
				SizeBytes: info.Size(),
				CreatedAt: time.Now().UTC(),
			})
			return nil
		})
	}
}

func classifyArtifact(path string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	name := filepath.Base(lower)
	switch {
	case name == "automation.log":
		return "automation_log"
	case name == "verify.log":
		return "verify_log"
	case name == "runtime.log":
		return "runtime_log"
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
	copy.SelectedSpecs = append([]string(nil), job.SelectedSpecs...)
	return copy
}

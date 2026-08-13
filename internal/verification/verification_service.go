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

const (
	maxAutomationSpecs                = 64
	maxAutomationSpecBytes            = 512
	maxAutomationEnvironment          = 64
	maxArtifactFiles                  = 256
	maxArtifactBytes            int64 = 128 << 20
	maxArtifactDepth                  = 12
	defaultMaxTerminalJobs            = 100
	defaultTerminalJobRetention       = 24 * time.Hour
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

type Freshness string

const (
	FreshnessFresh        Freshness = "fresh"
	FreshnessStale        Freshness = "stale"
	FreshnessInconclusive Freshness = "inconclusive"
)

type RepositoryFingerprint struct {
	Value  string `json:"value"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}

type StepResult struct {
	Step       string    `json:"step"`
	Status     string    `json:"status"`
	Message    string    `json:"message,omitempty"`
	DurationMS int64     `json:"durationMs"`
	At         time.Time `json:"at"`
}

type Artifact struct {
	Kind      string    `json:"kind"`
	Root      string    `json:"root,omitempty"`
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
	ArtifactWarning    string                         `json:"artifactWarning,omitempty"`
	Runtime            *models.WorkspaceRuntimeConfig `json:"runtime,omitempty"`
	StartFingerprint   *RepositoryFingerprint         `json:"startFingerprint,omitempty"`
	FinishFingerprint  *RepositoryFingerprint         `json:"finishFingerprint,omitempty"`
	CurrentFingerprint *RepositoryFingerprint         `json:"currentFingerprint,omitempty"`
	Freshness          Freshness                      `json:"freshness"`
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

// ConfigureRetention bounds process-local completed job history. Active jobs are
// never removed; callers may reduce these values in resource-constrained hosts.
func (s *Service) ConfigureRetention(maxTerminal int, retention time.Duration) *Service {
	if maxTerminal > 0 {
		s.maxTerminal = maxTerminal
	}
	if retention > 0 {
		s.retention = retention
	}
	return s
}

// RetentionWarnings exposes cleanup failures to host diagnostics without
// resurrecting pruned job records. Returned data is copied for callers.
func (s *Service) RetentionWarnings() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.cleanupWarnings...)
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
		_ = s.runtime.Down(context.Background(), workspace.Path, config, runtimeLog)
		s.failJob(jobID, FailureTypeBoot, 10, "startup failed", err)
		s.collectArtifacts(jobID, artifactRoot)
		return
	}
	if err := s.runStep(jobID, "health", runtimeLog, func() error {
		return s.runtime.Health(ctx, workspace.Path, config, runtimeLog)
	}); err != nil {
		_ = s.runtime.Down(context.Background(), workspace.Path, config, runtimeLog)
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
		_ = s.runtime.Down(context.Background(), workspace.Path, config, runtimeLog)
		s.failJob(jobID, FailureTypeTest, 20, "verification failed", err)
		s.collectArtifacts(jobID, artifactRoot)
		s.collectAutomationArtifacts(jobID)
		return
	}
	_ = s.runStep(jobID, "down", runtimeLog, func() error {
		return s.runtime.Down(context.Background(), workspace.Path, config, runtimeLog)
	})

	s.collectArtifacts(jobID, artifactRoot)
	s.collectAutomationArtifacts(jobID)
	finalStatus = JobStatusPassed
	s.mutate(jobID, func(state *Job) { state.ExitCode = 0 })
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

func projectFreshness(job Job) Freshness {
	if job.Status == JobStatusQueued || job.Status == JobStatusRunning || job.StartFingerprint == nil || job.FinishFingerprint == nil || job.CurrentFingerprint == nil {
		return FreshnessInconclusive
	}
	if job.StartFingerprint.Value != job.FinishFingerprint.Value {
		return FreshnessInconclusive
	}
	if job.FinishFingerprint.Value != job.CurrentFingerprint.Value {
		return FreshnessStale
	}
	return FreshnessFresh
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
	if err := validateAutomationEnvironment(environment); err != nil {
		return automationJobConfig{}, err
	}
	if err := appruntime.ValidateAutomationCommandTemplate(runtimeConfig.Automation.CommandTemplate); err != nil {
		return automationJobConfig{}, err
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
	if len(specs) > maxAutomationSpecs {
		return nil, fmt.Errorf("at most %d selected specs are allowed", maxAutomationSpecs)
	}
	root, err := filepath.Abs(strings.TrimSpace(repositoryPath))
	if err != nil || root == "" {
		return nil, errors.New("runtime automation repositoryPath is invalid")
	}
	selected := make([]string, 0, len(specs))
	seen := map[string]struct{}{}
	for _, spec := range specs {
		if len(spec) > maxAutomationSpecBytes || strings.ContainsAny(spec, "\x00\r\n") {
			return nil, fmt.Errorf("selected spec %q is invalid", spec)
		}
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

func validateAutomationEnvironment(environment string) error {
	if environment == "" || len(environment) > maxAutomationEnvironment {
		return errors.New("automation environment is invalid")
	}
	for _, r := range environment {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-') {
			return errors.New("automation environment is invalid")
		}
	}
	return nil
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
	// The command template remains administrator-authored, but values originating
	// from the request are always a single POSIX shell word. Template validation
	// rejects quoted user placeholders so values cannot escape their token.
	rendered = strings.ReplaceAll(rendered, "{env}", shellQuoteIfNeeded(environment))
	rendered = strings.ReplaceAll(rendered, "{specs}", shellQuoteIfNeeded(strings.Join(selectedSpecs, ",")))
	rendered = strings.ReplaceAll(rendered, "{modeArgs}", modeArgs)
	rendered = strings.ReplaceAll(rendered, "{headed}", automationHeadedArg(displayMode))
	rendered = strings.ReplaceAll(rendered, "{browser}", automationBrowserArg(runner, displayMode))
	if modeArgs != "" && !hasModePlaceholder {
		rendered = strings.TrimSpace(rendered + " " + modeArgs)
	}
	return rendered
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\\"'\\\"'") + "'"
}

func shellQuoteIfNeeded(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\r\n\\\"'$`;&|<>(){}[]*!?") {
		return value
	}
	return shellQuote(value)
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

func (s *Service) collectArtifacts(jobID, root string) {
	artifacts, warning := collectArtifacts(root)
	for index := range artifacts {
		artifacts[index].Root = "workspace"
	}
	s.mutate(jobID, func(job *Job) {
		job.Artifacts = append(job.Artifacts, artifacts...)
		if warning != "" {
			job.ArtifactWarning = warning
		}
	})
}

func collectArtifacts(root string) ([]Artifact, string) {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, "Artifact collection root is unavailable."
	}
	canonicalRoot, err = filepath.Abs(canonicalRoot)
	if err != nil {
		return nil, "Artifact collection root is unavailable."
	}
	walkRoot := canonicalRoot
	if info, statErr := os.Stat(canonicalRoot); statErr != nil {
		return nil, "Artifact collection root is unavailable."
	} else if !info.IsDir() {
		walkRoot = filepath.Dir(canonicalRoot)
	}
	artifacts := []Artifact{}
	var total int64
	warning := ""
	_ = filepath.WalkDir(canonicalRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			warning = "Some artifacts could not be collected."
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(walkRoot, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			warning = "Unsafe artifact path was skipped."
			return nil
		}
		if len(strings.Split(filepath.ToSlash(rel), "/")) > maxArtifactDepth {
			warning = "Artifact collection reached its depth limit."
			return nil
		}
		if len(artifacts) >= maxArtifactFiles || total+info.Size() > maxArtifactBytes {
			warning = "Artifact collection reached its retention limit."
			return filepath.SkipDir
		}
		total += info.Size()
		artifact := Artifact{
			Kind:      classifyArtifact(rel),
			Path:      filepath.ToSlash(rel),
			SizeBytes: info.Size(),
			CreatedAt: time.Now().UTC(),
		}
		artifacts = append(artifacts, artifact)
		return nil
	})
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, warning
}

func (s *Service) collectAutomationArtifacts(jobID string) {
	job, ok := s.snapshotAny(jobID)
	if !ok {
		return
	}
	if job.Mode != JobModeAutomation || job.Runtime == nil || job.Runtime.Automation == nil {
		return
	}
	root := strings.TrimSpace(job.AutomationRepoPath)
	if root == "" {
		return
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		s.mutate(jobID, func(state *Job) { state.ArtifactWarning = "Automation artifact root is unavailable." })
		return
	}
	for _, relRoot := range job.Runtime.Automation.ArtifactPaths {
		if err := appruntime.ValidateArtifactRelativePath(relRoot); err != nil {
			s.mutate(jobID, func(state *Job) { state.ArtifactWarning = "Unsafe automation artifact path was skipped." })
			continue
		}
		artifactPath := filepath.Join(root, filepath.FromSlash(relRoot))
		canonicalPath, resolveErr := filepath.EvalSymlinks(artifactPath)
		if resolveErr == nil && !pathWithin(canonicalRoot, canonicalPath) {
			s.mutate(jobID, func(state *Job) { state.ArtifactWarning = "Unsafe automation artifact path was skipped." })
			continue
		}
		artifacts, warning := collectArtifacts(artifactPath)
		includeRoot := true
		if info, statErr := os.Stat(artifactPath); statErr == nil && !info.IsDir() {
			includeRoot = false
		}
		for index := range artifacts {
			artifacts[index].Root = "automation"
			if includeRoot {
				artifacts[index].Path = filepath.ToSlash(filepath.Join(relRoot, artifacts[index].Path))
			}
		}
		s.mutate(jobID, func(state *Job) {
			state.Artifacts = append(state.Artifacts, artifacts...)
			if warning != "" {
				state.ArtifactWarning = warning
			}
		})
	}
}

func pathWithin(root, path string) bool {
	root, rootErr := filepath.Abs(root)
	path, pathErr := filepath.Abs(path)
	if rootErr != nil || pathErr != nil {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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

func (s *Service) failJob(jobID string, failure FailureType, code int, message string, err error) {
	s.mutate(jobID, func(job *Job) {
		job.FailureType = failure
		job.ExitCode = code
		job.Steps = append(job.Steps, StepResult{
			Step:       "failure",
			Status:     "failed",
			Message:    fmt.Sprintf("%s: %v", message, err),
			DurationMS: 0,
			At:         time.Now().UTC(),
		})
	})
}

func (s *Service) snapshot(workspaceID, jobID string) (Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobID]
	if !ok || job.WorkspaceID != workspaceID {
		return Job{}, false
	}
	return cloneJob(*job), true
}

func (s *Service) snapshotAny(jobID string) (Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return Job{}, false
	}
	return cloneJob(*job), true
}

func (s *Service) mutate(jobID string, mutate func(*Job)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[jobID]; ok {
		mutate(job)
	}
}

func (s *Service) pruneTerminalJobs() {
	now := time.Now().UTC()
	type candidate struct {
		id          string
		workspaceID string
		finished    time.Time
	}
	var remove []candidate
	s.mu.Lock()
	latestByWorkspace := map[string]string{}
	terminals := make([]candidate, 0)
	for id, job := range s.jobs {
		if job.Status != JobStatusPassed && job.Status != JobStatusFailed {
			continue
		}
		candidate := candidate{id: id, workspaceID: job.WorkspaceID, finished: job.FinishedAt}
		terminals = append(terminals, candidate)
		if previous, ok := latestByWorkspace[job.WorkspaceID]; !ok || s.jobs[previous].FinishedAt.Before(job.FinishedAt) {
			latestByWorkspace[job.WorkspaceID] = id
		}
	}
	sort.Slice(terminals, func(i, j int) bool { return terminals[i].finished.Before(terminals[j].finished) })
	remaining := len(terminals)
	for _, candidate := range terminals {
		expired := s.retention > 0 && !candidate.finished.IsZero() && now.Sub(candidate.finished) > s.retention
		overCapacity := remaining > s.maxTerminal
		if (expired || overCapacity) && latestByWorkspace[candidate.workspaceID] != candidate.id {
			delete(s.jobs, candidate.id)
			remove = append(remove, candidate)
			remaining--
		}
	}
	s.mu.Unlock()
	for _, candidate := range remove {
		if err := s.removeArtifactRoot(candidate.workspaceID, candidate.id); err != nil {
			s.mu.Lock()
			s.cleanupWarnings = append(s.cleanupWarnings, fmt.Sprintf("could not remove retained verification artifacts for %s: %v", candidate.id, err))
			s.mu.Unlock()
		}
	}
}

func (s *Service) removeArtifactRoot(workspaceID, jobID string) error {
	workspace, found, err := s.registry.Get(workspaceID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(workspace.Path) == "" || strings.Contains(jobID, string(filepath.Separator)) {
		return errors.New("verification artifact cleanup target is unavailable")
	}
	root := filepath.Join(workspace.Path, ".artifacts", "verification")
	target := filepath.Join(root, jobID)
	if !pathWithin(root, target) {
		return errors.New("verification artifact cleanup target is unsafe")
	}
	return s.removeArtifact(target)
}

// cleanupOrphanArtifacts establishes the restart contract: job state is
// process-local, so an owned verification generation found at startup has no
// runnable owner and is safely removed. Only direct, non-symlinked child
// directories under each registered workspace's owned root are eligible.
func (s *Service) cleanupOrphanArtifacts() {
	workspaces, err := s.registry.List()
	if err != nil {
		s.recordCleanupWarning(fmt.Sprintf("could not inspect orphan verification artifacts: %v", err))
		return
	}
	for _, workspace := range workspaces {
		root := filepath.Join(workspace.Path, ".artifacts", "verification")
		entries, readErr := os.ReadDir(root)
		if os.IsNotExist(readErr) {
			continue
		}
		if readErr != nil {
			s.recordCleanupWarning(fmt.Sprintf("could not inspect orphan verification artifacts: %v", readErr))
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasPrefix(entry.Name(), "verify-") {
				continue
			}
			target := filepath.Join(root, entry.Name())
			if !pathWithin(root, target) {
				continue
			}
			if removeErr := s.removeArtifact(target); removeErr != nil {
				s.recordCleanupWarning(fmt.Sprintf("could not remove orphan verification artifacts for %s: %v", entry.Name(), removeErr))
			}
		}
	}
}

func (s *Service) recordCleanupWarning(message string) {
	s.mu.Lock()
	s.cleanupWarnings = append(s.cleanupWarnings, message)
	s.mu.Unlock()
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

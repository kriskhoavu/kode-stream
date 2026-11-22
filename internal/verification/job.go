package verification

import (
	"time"

	"kode-stream/internal/common/models"
	appruntime "kode-stream/internal/runtime"
)

type JobStatus string

type FailureType string

type JobMode string

type Freshness string

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
	maxRuntimeCleanupDuration         = time.Minute
)

const (
	FailureTypeBoot  FailureType = "boot_failure"
	FailureTypeTest  FailureType = "test_failure"
	FailureTypeInfra FailureType = "infra_failure"
)

const (
	JobModeRuntime    JobMode = "runtime"
	JobModeAutomation JobMode = "automation"
)

const (
	FreshnessFresh        Freshness = "fresh"
	FreshnessStale        Freshness = "stale"
	FreshnessInconclusive Freshness = "inconclusive"
)

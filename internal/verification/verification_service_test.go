package verification

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	"kode-stream/internal/common/models"
	appruntime "kode-stream/internal/runtime"
	"kode-stream/internal/workspace/registry"
)

func TestRuntimeVerificationModeRemainsDefault(t *testing.T) {
	workspacePath := t.TempDir()
	reg := verificationRegistry(t, models.WorkspaceConfig{
		ID:             "workspace-1",
		Name:           "Workspace",
		Path:           workspacePath,
		BaselineBranch: "main",
		Sources:        []string{"plans"},
		Runtime:        verificationRuntime("printf verify > verify.out", nil),
	})
	service := NewService(reg, appruntime.NewService())

	job, err := service.Start("workspace-1", CreateInput{})
	if err != nil {
		t.Fatal(err)
	}
	job = waitVerificationJob(t, service, "workspace-1", job.ID)
	if job.Status != JobStatusPassed || job.Mode != JobModeRuntime || job.RenderedCommand != "" {
		t.Fatalf("job = %#v", job)
	}
	data, err := os.ReadFile(filepath.Join(workspacePath, "verify.out"))
	if err != nil || strings.TrimSpace(string(data)) != "verify" {
		t.Fatalf("verify output = %q err=%v", data, err)
	}
}

func TestAutomationVerificationRunsSelectedSpecsAndCollectsArtifacts(t *testing.T) {
	workspacePath := t.TempDir()
	automationRepo := t.TempDir()
	config := &models.RuntimeAutomationConfig{
		Enabled:            true,
		RepositoryPath:     automationRepo,
		Runner:             models.AutomationRunnerCypress,
		DefaultEnvironment: "local",
		CommandTemplate:    "printf '{env}:{specs}' > run.txt",
		ArtifactPaths:      []string{"run.txt"},
	}
	reg := verificationRegistry(t, models.WorkspaceConfig{
		ID:             "workspace-1",
		Name:           "Workspace",
		Path:           workspacePath,
		BaselineBranch: "main",
		Sources:        []string{"plans"},
		Runtime:        verificationRuntime("printf runtime > verify.out", config),
	})
	service := NewService(reg, appruntime.NewService())

	job, err := service.Start("workspace-1", CreateInput{
		Mode:          JobModeAutomation,
		Environment:   "nightly",
		SelectedSpecs: []string{"cypress/e2e/create.cy.ts"},
	})
	if err != nil {
		t.Fatal(err)
	}
	job = waitVerificationJob(t, service, "workspace-1", job.ID)
	if job.Status != JobStatusPassed || job.Mode != JobModeAutomation {
		t.Fatalf("job = %#v", job)
	}
	if job.RenderedCommand != "printf 'nightly:cypress/e2e/create.cy.ts' > run.txt" {
		t.Fatalf("rendered command = %q", job.RenderedCommand)
	}
	if !hasStep(job, "automation") || !hasStep(job, "down") {
		t.Fatalf("steps = %#v", job.Steps)
	}
	if !hasArtifact(job, filepath.Join(automationRepo, "run.txt")) {
		t.Fatalf("artifacts = %#v", job.Artifacts)
	}
	if !hasArtifactKindSuffix(job, "automation_log", "automation.log") || !hasArtifactKindSuffix(job, "runtime_log", "runtime.log") {
		t.Fatalf("log artifacts = %#v", job.Artifacts)
	}
}

func TestAutomationVisibleModeRendersRunnerFlags(t *testing.T) {
	cypress := renderAutomationCommand("npx cypress run --spec \"{specs}\"", models.AutomationRunnerCypress, "local", []string{"cypress/e2e/create.cy.ts"}, models.AutomationDisplayModeVisible)
	if cypress != "npx cypress run --spec \"cypress/e2e/create.cy.ts\" --headed --browser chrome" {
		t.Fatalf("cypress command = %q", cypress)
	}
	playwright := renderAutomationCommand("npx playwright test {specs} {modeArgs}", models.AutomationRunnerPlaywright, "local", []string{"tests/create.spec.ts"}, models.AutomationDisplayModeVisible)
	if playwright != "npx playwright test tests/create.spec.ts --headed --project=chromium" {
		t.Fatalf("playwright command = %q", playwright)
	}
	silent := renderAutomationCommand("npx playwright test {specs}", models.AutomationRunnerPlaywright, "local", []string{"tests/create.spec.ts"}, models.AutomationDisplayModeSilent)
	if silent != "npx playwright test tests/create.spec.ts" {
		t.Fatalf("silent command = %q", silent)
	}
}

func TestAutomationVerificationFailureRunsTeardown(t *testing.T) {
	workspacePath := t.TempDir()
	automationRepo := t.TempDir()
	config := &models.RuntimeAutomationConfig{
		Enabled:         true,
		RepositoryPath:  automationRepo,
		Runner:          models.AutomationRunnerCypress,
		CommandTemplate: "exit 7",
	}
	reg := verificationRegistry(t, models.WorkspaceConfig{
		ID:             "workspace-1",
		Name:           "Workspace",
		Path:           workspacePath,
		BaselineBranch: "main",
		Sources:        []string{"plans"},
		Runtime:        verificationRuntime("true", config),
	})
	service := NewService(reg, appruntime.NewService())

	job, err := service.Start("workspace-1", CreateInput{Mode: JobModeAutomation, SelectedSpecs: []string{"cypress/e2e/fail.cy.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	job = waitVerificationJob(t, service, "workspace-1", job.ID)
	if job.Status != JobStatusFailed || job.FailureType != FailureTypeTest {
		t.Fatalf("job = %#v", job)
	}
	if _, err := os.Stat(filepath.Join(workspacePath, "down.out")); err != nil {
		t.Fatalf("teardown did not run: %v", err)
	}
}

func TestAutomationVerificationSkipsSpecsWhenBootFails(t *testing.T) {
	workspacePath := t.TempDir()
	automationRepo := t.TempDir()
	config := &models.RuntimeAutomationConfig{
		Enabled:         true,
		RepositoryPath:  automationRepo,
		Runner:          models.AutomationRunnerCypress,
		CommandTemplate: "printf should-not-run > run.txt",
	}
	runtimeConfig := verificationRuntime("true", config)
	runtimeConfig.Commands.Up = "exit 1"
	reg := verificationRegistry(t, models.WorkspaceConfig{
		ID:             "workspace-1",
		Name:           "Workspace",
		Path:           workspacePath,
		BaselineBranch: "main",
		Sources:        []string{"plans"},
		Runtime:        runtimeConfig,
	})
	service := NewService(reg, appruntime.NewService())

	job, err := service.Start("workspace-1", CreateInput{Mode: JobModeAutomation, SelectedSpecs: []string{"cypress/e2e/skip.cy.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	job = waitVerificationJob(t, service, "workspace-1", job.ID)
	if job.Status != JobStatusFailed || job.FailureType != FailureTypeBoot {
		t.Fatalf("job = %#v", job)
	}
	if _, err := os.Stat(filepath.Join(automationRepo, "run.txt")); !os.IsNotExist(err) {
		t.Fatalf("automation command should not run, stat err=%v", err)
	}
}

func TestAutomationVerificationRejectsPathTraversal(t *testing.T) {
	automationRepo := t.TempDir()
	reg := verificationRegistry(t, models.WorkspaceConfig{
		ID:             "workspace-1",
		Name:           "Workspace",
		Path:           t.TempDir(),
		BaselineBranch: "main",
		Sources:        []string{"plans"},
		Runtime: verificationRuntime("true", &models.RuntimeAutomationConfig{
			Enabled:         true,
			RepositoryPath:  automationRepo,
			Runner:          models.AutomationRunnerCypress,
			CommandTemplate: "true",
		}),
	})
	service := NewService(reg, appruntime.NewService())

	_, err := service.Start("workspace-1", CreateInput{Mode: JobModeAutomation, SelectedSpecs: []string{"../secret.cy.ts"}})
	if err == nil || !strings.Contains(err.Error(), "relative") {
		t.Fatalf("err = %v", err)
	}
}

func TestVerificationRejectsWhenConcurrencyLimitIsFull(t *testing.T) {
	workspacePath := t.TempDir()
	reg := verificationRegistry(t, models.WorkspaceConfig{
		ID:             "workspace-1",
		Name:           "Workspace",
		Path:           workspacePath,
		BaselineBranch: "main",
		Sources:        []string{"plans"},
		Runtime:        verificationRuntime("sleep 0.2", nil),
	})
	service := NewServiceWithPolicy(reg, appruntime.NewService(), 1, time.Minute)

	first, err := service.Start("workspace-1", CreateInput{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Start("workspace-1", CreateInput{})
	if err == nil || !strings.Contains(err.Error(), "queue is full") {
		t.Fatalf("err = %v", err)
	}
	_ = waitVerificationJob(t, service, "workspace-1", first.ID)
}

func TestVerificationShutdownCancelsRunningJob(t *testing.T) {
	workspacePath := t.TempDir()
	reg := verificationRegistry(t, models.WorkspaceConfig{
		ID:             "workspace-1",
		Name:           "Workspace",
		Path:           workspacePath,
		BaselineBranch: "main",
		Sources:        []string{"plans"},
		Runtime:        verificationRuntime("sleep 5", nil),
	})
	service := NewServiceWithPolicy(reg, appruntime.NewService(), 1, time.Minute)

	job, err := service.Start("workspace-1", CreateInput{})
	if err != nil {
		t.Fatal(err)
	}
	service.Close()
	job = waitVerificationJob(t, service, "workspace-1", job.ID)
	if job.Status != JobStatusFailed || !jobHasMessage(job, "context canceled") {
		t.Fatalf("job = %#v", job)
	}
	if _, err := service.Start("workspace-1", CreateInput{}); err == nil {
		t.Fatal("Start after Close succeeded")
	}
}

func verificationRuntime(verify string, automation *models.RuntimeAutomationConfig) *models.WorkspaceRuntimeConfig {
	return &models.WorkspaceRuntimeConfig{
		Type: models.RuntimeTypeCustom,
		Commands: models.RuntimeCommandSet{
			Up:   "true",
			Down: "printf down > down.out",
			Verify: models.RuntimeVerifyCommands{
				Smoke: verify,
			},
		},
		Automation: automation,
	}
}

func verificationRegistry(t *testing.T, workspace models.WorkspaceConfig) *registry.Registry {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workspaces.yaml")
	data, err := yaml.Marshal([]models.WorkspaceConfig{workspace})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return registry.New(path, nil)
}

func waitVerificationJob(t *testing.T, service *Service, workspaceID, jobID string) Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := service.Get(workspaceID, jobID)
		if !ok {
			t.Fatalf("job %s not found", jobID)
		}
		if job.Status == JobStatusPassed || job.Status == JobStatusFailed {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	job, _ := service.Get(workspaceID, jobID)
	t.Fatalf("job did not finish: %#v", job)
	return Job{}
}

func hasStep(job Job, step string) bool {
	for _, candidate := range job.Steps {
		if candidate.Step == step {
			return true
		}
	}
	return false
}

func jobHasMessage(job Job, text string) bool {
	for _, step := range job.Steps {
		if strings.Contains(step.Message, text) {
			return true
		}
	}
	return false
}

func hasArtifact(job Job, path string) bool {
	path = filepath.ToSlash(path)
	for _, candidate := range job.Artifacts {
		if candidate.Path == path {
			return true
		}
	}
	return false
}

func hasArtifactKindSuffix(job Job, kind, suffix string) bool {
	suffix = filepath.ToSlash(suffix)
	for _, candidate := range job.Artifacts {
		if candidate.Kind == kind && strings.HasSuffix(candidate.Path, suffix) {
			return true
		}
	}
	return false
}

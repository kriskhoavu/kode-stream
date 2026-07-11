package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"kode-stream/internal/common/models"
)

type VerifyProfile string

const (
	VerifyProfileSmoke    VerifyProfile = "smoke"
	VerifyProfileCritical VerifyProfile = "critical"
	VerifyProfileFull     VerifyProfile = "full"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func NormalizeRuntimeConfig(input *models.WorkspaceRuntimeConfig) (*models.WorkspaceRuntimeConfig, error) {
	if input == nil {
		return nil, nil
	}
	config := *input
	config.Type = models.RuntimeType(strings.TrimSpace(string(config.Type)))
	config.ConfigPath = strings.TrimSpace(config.ConfigPath)
	if strings.TrimSpace(string(config.RebuildPolicy)) == "" {
		config.RebuildPolicy = models.RebuildPolicyChangedOnly
	}
	config.RebuildPolicy = models.RebuildPolicy(strings.TrimSpace(string(config.RebuildPolicy)))
	config.Commands.Up = strings.TrimSpace(config.Commands.Up)
	config.Commands.Down = strings.TrimSpace(config.Commands.Down)
	config.Commands.RebuildChanged = strings.TrimSpace(config.Commands.RebuildChanged)
	config.Commands.Verify.Smoke = strings.TrimSpace(config.Commands.Verify.Smoke)
	config.Commands.Verify.Critical = strings.TrimSpace(config.Commands.Verify.Critical)
	config.Commands.Verify.Full = strings.TrimSpace(config.Commands.Verify.Full)
	applyAdapterDefaults(&config)

	if config.HealthChecks == nil {
		config.HealthChecks = []models.RuntimeHealthCheck{}
	}
	for i := range config.HealthChecks {
		config.HealthChecks[i].Name = strings.TrimSpace(config.HealthChecks[i].Name)
		config.HealthChecks[i].Type = strings.ToLower(strings.TrimSpace(config.HealthChecks[i].Type))
		config.HealthChecks[i].Target = strings.TrimSpace(config.HealthChecks[i].Target)
		if config.HealthChecks[i].TimeoutSeconds <= 0 {
			config.HealthChecks[i].TimeoutSeconds = 30
		}
	}

	if config.Artifacts.Paths == nil {
		config.Artifacts.Paths = []string{}
	}
	paths := make([]string, 0, len(config.Artifacts.Paths))
	for _, p := range config.Artifacts.Paths {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(p)))
		if clean == "" || clean == "." {
			continue
		}
		paths = append(paths, clean)
	}
	config.Artifacts.Paths = paths
	config.Automation = normalizeAutomationConfig(config.Automation)

	if err := ValidateRuntimeConfig(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

func ValidateRuntimeConfig(config *models.WorkspaceRuntimeConfig) error {
	if config == nil {
		return nil
	}
	switch config.Type {
	case models.RuntimeTypeDockerCompose, models.RuntimeTypeProcfile, models.RuntimeTypeMakefile, models.RuntimeTypeCustom:
	default:
		return errors.New("runtime type is invalid")
	}
	if config.Commands.Up == "" {
		return errors.New("runtime commands.up is required")
	}
	if config.Commands.Down == "" {
		return errors.New("runtime commands.down is required")
	}
	if config.Commands.Verify.Smoke == "" {
		return errors.New("runtime commands.verify.smoke is required")
	}
	switch config.RebuildPolicy {
	case models.RebuildPolicyNever, models.RebuildPolicyChangedOnly, models.RebuildPolicyAlways:
	default:
		return errors.New("runtime rebuild policy is invalid")
	}
	for _, check := range config.HealthChecks {
		switch check.Type {
		case "http", "command":
		default:
			return fmt.Errorf("runtime health check type %q is invalid", check.Type)
		}
		if check.Target == "" {
			return errors.New("runtime health check target is required")
		}
	}
	if err := validateAutomationConfig(config.Automation); err != nil {
		return err
	}
	return nil
}

func normalizeAutomationConfig(input *models.RuntimeAutomationConfig) *models.RuntimeAutomationConfig {
	if input == nil {
		return nil
	}
	config := *input
	config.RepositoryPath = strings.TrimSpace(config.RepositoryPath)
	config.Runner = models.AutomationRunner(strings.ToLower(strings.TrimSpace(string(config.Runner))))
	if config.Runner == "" {
		config.Runner = models.AutomationRunnerCypress
	}
	config.DefaultEnvironment = strings.TrimSpace(config.DefaultEnvironment)
	if config.DefaultEnvironment == "" {
		config.DefaultEnvironment = "local"
	}
	config.CommandTemplate = strings.TrimSpace(config.CommandTemplate)
	if config.CommandTemplate == "" {
		config.CommandTemplate = defaultAutomationCommandTemplate(config.Runner)
	}
	artifactPaths := make([]string, 0, len(config.ArtifactPaths))
	for _, p := range config.ArtifactPaths {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(p)))
		if clean == "" || clean == "." {
			continue
		}
		artifactPaths = append(artifactPaths, clean)
	}
	if len(artifactPaths) == 0 {
		artifactPaths = defaultAutomationArtifactPaths(config.Runner)
	}
	config.ArtifactPaths = artifactPaths
	return &config
}

func validateAutomationConfig(config *models.RuntimeAutomationConfig) error {
	if config == nil {
		return nil
	}
	switch config.Runner {
	case models.AutomationRunnerCypress, models.AutomationRunnerPlaywright:
	default:
		return errors.New("runtime automation runner is invalid")
	}
	if !config.Enabled {
		return nil
	}
	if config.RepositoryPath == "" {
		return errors.New("runtime automation repositoryPath is required")
	}
	if stat, err := os.Stat(config.RepositoryPath); err != nil || !stat.IsDir() {
		return errors.New("runtime automation repositoryPath must be an existing directory")
	}
	if strings.TrimSpace(config.CommandTemplate) == "" {
		return errors.New("runtime automation commandTemplate is required")
	}
	return nil
}

func defaultAutomationCommandTemplate(runner models.AutomationRunner) string {
	if runner == models.AutomationRunnerPlaywright {
		return "npx playwright test {specs}"
	}
	return "CYPRESS_EPSAP_ENVIRONMENT={env} npx cypress run --spec \"{specs}\""
}

func defaultAutomationArtifactPaths(runner models.AutomationRunner) []string {
	if runner == models.AutomationRunnerPlaywright {
		return []string{"playwright-report", "test-results"}
	}
	return []string{"cypress/reports", "cypress/screenshots", "cypress/videos"}
}

func applyAdapterDefaults(config *models.WorkspaceRuntimeConfig) {
	if config == nil {
		return
	}
	switch config.Type {
	case models.RuntimeTypeDockerCompose:
		prefix := "docker compose"
		if config.ConfigPath != "" {
			prefix += " -f " + shellQuote(config.ConfigPath)
		}
		if config.Commands.Up == "" {
			config.Commands.Up = prefix + " up -d --no-build"
		}
		if config.Commands.Down == "" {
			config.Commands.Down = prefix + " down"
		}
		if config.Commands.RebuildChanged == "" {
			config.Commands.RebuildChanged = prefix + " build"
		}
	case models.RuntimeTypeMakefile:
		if config.Commands.Up == "" {
			config.Commands.Up = "make up"
		}
		if config.Commands.Down == "" {
			config.Commands.Down = "make down"
		}
		if config.Commands.RebuildChanged == "" {
			config.Commands.RebuildChanged = "make build-changed"
		}
	case models.RuntimeTypeProcfile:
		fileFlag := ""
		if config.ConfigPath != "" {
			fileFlag = " -f " + shellQuote(config.ConfigPath)
		}
		if config.Commands.Up == "" {
			config.Commands.Up = "overmind start" + fileFlag
		}
		if config.Commands.Down == "" {
			config.Commands.Down = "true"
		}
	}
}

func shellQuote(value string) string {
	if value == "" {
		return value
	}
	if strings.ContainsAny(value, " \t\n\"'") {
		return strconv.Quote(value)
	}
	return value
}

func (s *Service) VerifyCommand(config *models.WorkspaceRuntimeConfig, profile VerifyProfile) string {
	if config == nil {
		return ""
	}
	switch profile {
	case VerifyProfileCritical:
		if config.Commands.Verify.Critical != "" {
			return config.Commands.Verify.Critical
		}
		return config.Commands.Verify.Smoke
	case VerifyProfileFull:
		if config.Commands.Verify.Full != "" {
			return config.Commands.Verify.Full
		}
		if config.Commands.Verify.Critical != "" {
			return config.Commands.Verify.Critical
		}
		return config.Commands.Verify.Smoke
	default:
		return config.Commands.Verify.Smoke
	}
}

func (s *Service) Prepare(ctx context.Context, workspacePath string, config *models.WorkspaceRuntimeConfig, out io.Writer) error {
	if config == nil {
		return errors.New("runtime config is required")
	}
	if config.RebuildPolicy == models.RebuildPolicyAlways && config.Commands.RebuildChanged != "" {
		return s.RunCommand(ctx, workspacePath, config.Commands.RebuildChanged, out)
	}
	if config.RebuildPolicy == models.RebuildPolicyChangedOnly && config.Commands.RebuildChanged != "" {
		return s.RunCommand(ctx, workspacePath, config.Commands.RebuildChanged, out)
	}
	return nil
}

func (s *Service) Up(ctx context.Context, workspacePath string, config *models.WorkspaceRuntimeConfig, out io.Writer) error {
	return s.RunCommand(ctx, workspacePath, config.Commands.Up, out)
}

func (s *Service) Down(ctx context.Context, workspacePath string, config *models.WorkspaceRuntimeConfig, out io.Writer) error {
	if config == nil || strings.TrimSpace(config.Commands.Down) == "" {
		return nil
	}
	return s.RunCommand(ctx, workspacePath, config.Commands.Down, out)
}

func (s *Service) Verify(ctx context.Context, workspacePath string, config *models.WorkspaceRuntimeConfig, profile VerifyProfile, out io.Writer) error {
	command := s.VerifyCommand(config, profile)
	if strings.TrimSpace(command) == "" {
		return errors.New("verify command is required")
	}
	return s.RunCommand(ctx, workspacePath, command, out)
}

func (s *Service) Health(ctx context.Context, workspacePath string, config *models.WorkspaceRuntimeConfig, out io.Writer) error {
	for _, check := range config.HealthChecks {
		timeout := time.Duration(check.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		healthCtx, cancel := context.WithTimeout(ctx, timeout)
		var err error
		switch check.Type {
		case "http":
			err = executeHTTPHealth(healthCtx, check.Target)
		case "command":
			err = s.RunCommand(healthCtx, workspacePath, check.Target, out)
		}
		cancel()
		if err != nil {
			name := check.Name
			if name == "" {
				name = check.Target
			}
			return fmt.Errorf("health check failed (%s): %w", name, err)
		}
	}
	return nil
}

func executeHTTPHealth(ctx context.Context, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 500 {
		return fmt.Errorf("status %d", response.StatusCode)
	}
	return nil
}

func (s *Service) RunCommand(ctx context.Context, workdir, command string, out io.Writer) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("command is required")
	}
	execCmd := shellCommand(ctx, command)
	execCmd.Dir = workdir
	execCmd.Stdout = out
	execCmd.Stderr = out
	return execCmd.Run()
}

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "sh", "-lc", command)
}

func EnsureArtifactRoot(workspacePath, jobID string) (string, error) {
	root := filepath.Join(workspacePath, ".artifacts", "verification", jobID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	return root, nil
}

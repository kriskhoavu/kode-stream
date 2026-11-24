package verification

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"kode-stream/internal/common/models"
	appruntime "kode-stream/internal/runtime"
)

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
	// A request value is always one POSIX shell word, so it cannot escape its token.
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

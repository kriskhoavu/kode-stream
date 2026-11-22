package ai

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"kode-stream/internal/audit"
	"kode-stream/internal/filesystem/pathguard"
	"kode-stream/internal/item/index"
	"kode-stream/internal/workspace/registry"
)

type LaunchInput struct {
	Provider       string   `json:"provider"`
	Terminal       string   `json:"terminal"`
	ContextMode    string   `json:"contextMode"`
	Surface        string   `json:"surface,omitempty"`
	PresetID       string   `json:"presetId,omitempty"`
	PromptDraft    string   `json:"promptDraft,omitempty"`
	CustomPrompt   string   `json:"customPrompt,omitempty"`
	SelectedSkills []string `json:"selectedSkills,omitempty"`
	SelectedAgents []string `json:"selectedAgents,omitempty"`
	ContextPath    string   `json:"contextPath,omitempty"`
}

func (s *Service) LaunchWorkspace(workspaceID string, input LaunchInput) (result LaunchResult, err error) {
	if s.launch == nil || s.launch.registry == nil {
		return LaunchResult{}, launchError("launch_failed", "AI session launch is unavailable")
	}
	workspace, found, getErr := s.launch.registry.Get(workspaceID)
	if getErr != nil {
		return LaunchResult{}, launchErrorWith("launch_failed", getErr)
	}
	if !found {
		return LaunchResult{}, launchError("workspace_not_found", "workspace not found")
	}
	contextPath := strings.TrimSpace(input.ContextPath)
	if contextPath != "" {
		if _, pathErr := pathguard.ValidateMarkdownFile(workspace.Path, contextPath); pathErr != nil {
			return LaunchResult{}, launchError("invalid_context_path", "context path must be an existing Markdown file inside the workspace")
		}
	}
	settings, settingsErr := s.Settings()
	if settingsErr != nil {
		return LaunchResult{}, launchErrorWith("launch_failed", settingsErr)
	}
	providerID, terminalID := strings.TrimSpace(input.Provider), strings.TrimSpace(input.Terminal)
	provider, ok := settings.Providers[providerID]
	if !ok || !provider.Enabled {
		return LaunchResult{}, launchError("ai_provider_missing", "selected AI provider is unavailable")
	}
	terminal, ok := settings.Terminals[terminalID]
	if !ok || !terminal.Enabled {
		return LaunchResult{}, launchError("terminal_missing", "selected terminal is unavailable")
	}
	if !s.detect(provider.Executable).Detected {
		return LaunchResult{}, launchError("ai_provider_missing", "selected AI provider executable was not found")
	}
	if !s.detect(terminal.Executable).Detected {
		return LaunchResult{}, launchError("terminal_missing", "selected terminal executable was not found")
	}
	prompt, presetID, promptErr := s.composeWorkspacePrompt(providerID, workspaceID, "workspace_only", input.PresetID, input.PromptDraft, input.CustomPrompt, input.SelectedSkills, input.SelectedAgents)
	if promptErr != nil {
		return LaunchResult{}, promptErr
	}
	contextLabel := contextPath
	if contextLabel == "" {
		contextLabel = "workspace root"
	}
	values := map[string]string{"workspace": workspace.Path, "contextFile": contextLabel, "itemPath": contextLabel, "identifier": contextLabel, "contextMode": "workspace_only", "intent": "workspace_only", "prompt": prompt}
	sessionID := "external-" + randomID()
	if startErr := s.startTerminal(terminalID, terminal, expandAll(terminal.Args, values), workspace.Path, expand(provider.Executable, values), launchProviderArgs("workspace_only", provider.Args, values), verificationCheckpoint{WorkspaceID: workspace.ID, Provider: providerID, SessionID: sessionID, TerminalMode: "external", Profile: "smoke"}); startErr != nil {
		return LaunchResult{}, launchErrorWith("launch_failed", startErr)
	}
	return LaunchResult{Accepted: true, Provider: providerID, Terminal: terminalID, ContextMode: "workspace_only", Surface: "external", PresetID: presetID, SessionID: sessionID, StartedAt: s.launch.now().UTC()}, nil
}

type LaunchResult struct {
	Accepted    bool      `json:"accepted"`
	Provider    string    `json:"provider"`
	Terminal    string    `json:"terminal"`
	ContextMode string    `json:"contextMode"`
	Surface     string    `json:"surface,omitempty"`
	PresetID    string    `json:"presetId,omitempty"`
	SessionID   string    `json:"sessionId,omitempty"`
	StartedAt   time.Time `json:"startedAt"`
}

type verificationCheckpoint struct {
	WorkspaceID  string
	Provider     string
	SessionID    string
	TerminalMode string
	Profile      string
}

type PlanPreset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Prompt      string `json:"prompt"`
	ContextMode string `json:"contextMode"`
	Provider    string `json:"provider,omitempty"`
}

type Eligibility struct {
	Editable             bool     `json:"editable"`
	CardContextAvailable bool     `json:"cardContextAvailable"`
	Missing              []string `json:"missing"`
}

type LaunchError struct {
	Code    string
	Err     error
	Details map[string]string
}

func (e *LaunchError) Error() string { return e.Err.Error() }
func (e *LaunchError) Unwrap() error { return e.Err }

type ProcessRunner interface {
	Start(name string, args []string, dir string) error
}

type execRunner struct{}

func (execRunner) Start(name string, args []string, dir string) error {
	command := exec.Command(name, args...)
	command.Dir = dir
	return command.Start()
}

type launchDependencies struct {
	registry   registry.Repository
	index      itemindex.Repository
	audit      audit.Repository
	wrapperDir string
	runner     ProcessRunner
	now        func() time.Time
}

func (s *Service) ConfigureLaunch(reg registry.Repository, index itemindex.Repository, auditStore audit.Repository, wrapperDir string) *Service {
	s.launch = &launchDependencies{registry: reg, index: index, audit: auditStore, wrapperDir: wrapperDir, runner: execRunner{}, now: time.Now}
	return s
}

func (s *Service) Presets() []PlanPreset {
	return []PlanPreset{
		{ID: "implementation-plan", Name: "Create implementation plan", ContextMode: "card_context", Prompt: "Create or update the implementation plan from the selected item context. Read the Jira context and local plan files first, then produce concrete backend, frontend, verification, and rollout steps."},
		{ID: "technical-design", Name: "Create technical design", ContextMode: "card_context", Prompt: "Draft the technical design for the selected item. Cover affected modules, API contracts, data flow, edge cases, and compatibility decisions."},
		{ID: "test-scenarios", Name: "Create test scenarios", ContextMode: "card_context", Prompt: "Create test scenarios for the selected item. Include happy paths, validation failures, remote integration failures, regression checks, and acceptance criteria."},
	}
}

func (s *Service) Eligibility(itemID string) (Eligibility, error) {
	if s.launch == nil || s.launch.registry == nil || s.launch.index == nil {
		return Eligibility{}, launchError("launch_failed", "AI session launch is unavailable")
	}
	item, found, err := s.launch.index.Get(itemID)
	if err != nil {
		return Eligibility{}, launchErrorWith("launch_failed", err)
	}
	if !found {
		return Eligibility{}, launchError("item_not_found", "item not found")
	}
	workspace, found, err := s.launch.registry.Get(item.WorkspaceID)
	if err != nil {
		return Eligibility{}, launchErrorWith("launch_failed", err)
	}
	if !found {
		return Eligibility{}, launchError("workspace_not_found", "workspace not found")
	}
	result := Eligibility{Editable: item.SourceMode != "snapshot" && item.Editable, Missing: []string{}}
	if !result.Editable {
		result.Missing = append(result.Missing, "editable working-tree item")
		return result, nil
	}
	_, err = pathguard.SafeJoin(workspace.Path, item.ItemPath)
	if err != nil {
		result.Editable = false
		result.Missing = append(result.Missing, "valid item path")
		return result, nil
	}
	result.CardContextAvailable = true
	return result, nil
}

func (s *Service) resolvePrompt(presetID, promptDraft, customPrompt string) (string, string, error) {
	presetID = strings.TrimSpace(presetID)
	promptDraft = strings.TrimSpace(promptDraft)
	customPrompt = strings.TrimSpace(customPrompt)
	if promptDraft != "" && customPrompt != "" {
		return "", "", launchError("invalid_prompt", "choose either promptDraft or customPrompt")
	}
	if presetID != "" && customPrompt != "" && promptDraft == "" {
		return "", "", launchError("invalid_prompt", "choose either an AI preset or a free prompt")
	}
	if promptDraft != "" {
		if presetID == "" {
			return promptDraft, "", nil
		}
		return promptDraft, presetID, nil
	}
	if customPrompt != "" {
		return customPrompt, "", nil
	}
	if presetID == "" {
		return "", "", nil
	}
	for _, preset := range s.Presets() {
		if preset.ID == presetID {
			return preset.Prompt, preset.ID, nil
		}
	}
	return "", "", launchError("invalid_prompt", "selected AI preset is unavailable")
}

// ItemWorkspace is deliberately narrow transport support for ownership checks;
// it does not expose the workspace path or item content.
func (s *Service) ItemWorkspace(itemID string) (string, error) {
	if s.launch == nil || s.launch.index == nil {
		return "", launchError("launch_failed", "AI session launch is unavailable")
	}
	item, found, err := s.launch.index.Get(strings.TrimSpace(itemID))
	if err != nil {
		return "", err
	}
	if !found {
		return "", launchError("item_not_found", "item not found")
	}
	return item.WorkspaceID, nil
}

func (s *Service) startTerminal(id string, terminal LaunchTemplate, terminalArgs []string, workspace, provider string, providerArgs []string, checkpoint verificationCheckpoint) error {
	terminalExecutable := s.detect(terminal.Executable).Executable
	providerExecutable := s.detect(provider).Executable
	wrapper, err := writeWrapper(s.launch.wrapperDir, workspace, providerExecutable, providerArgs, checkpoint)
	if err != nil {
		return err
	}
	if id == "wezterm" {
		args := append(terminalArgs, "start", "--cwd", workspace, "--", wrapper)
		if err := s.launch.runner.Start(terminalExecutable, args, workspace); err != nil {
			_ = os.Remove(wrapper)
			return err
		}
		return nil
	}
	if s.goos != "darwin" || (id != "terminal" && id != "iterm2") {
		_ = os.Remove(wrapper)
		return launchError("terminal_missing", "selected terminal has no launch adapter on this platform")
	}
	if err := s.launch.runner.Start("/usr/bin/open", []string{"-a", terminalExecutable, wrapper}, workspace); err != nil {
		_ = os.Remove(wrapper)
		return err
	}
	return nil
}

func serverPort() string {
	value := strings.TrimSpace(os.Getenv("KODE_STREAM_PORT"))
	if value == "" {
		return "4317"
	}
	if _, err := strconv.Atoi(value); err != nil {
		return "4317"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func randomID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func launchError(code, message string) error {
	return &LaunchError{Code: code, Err: errors.New(message)}
}
func launchErrorWith(code string, err error) error { return &LaunchError{Code: code, Err: err} }

func launchErrorWithDetails(code, message string, details map[string]string) error {
	return &LaunchError{Code: code, Err: errors.New(message), Details: details}
}

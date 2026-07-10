package ai

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kode-stream/internal/audit"
	"kode-stream/internal/common/models"
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

type CapabilityDescriptor struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Kind        string `json:"kind"`
	Provider    string `json:"provider"`
	Scope       string `json:"scope"`
	SourcePath  string `json:"sourcePath"`
}

type ProviderCapabilityCatalog struct {
	Provider                string                 `json:"provider"`
	Skills                  []CapabilityDescriptor `json:"skills"`
	Agents                  []CapabilityDescriptor `json:"agents"`
	SupportsNativeSelection bool                   `json:"supportsNativeSelection"`
	SupportsPromptFallback  bool                   `json:"supportsPromptFallback"`
}

type Eligibility struct {
	Editable             bool     `json:"editable"`
	CardContextAvailable bool     `json:"cardContextAvailable"`
	Missing              []string `json:"missing"`
}

type LaunchError struct {
	Code string
	Err  error
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
	registry   *registry.Registry
	index      *itemindex.Index
	audit      *audit.Store
	wrapperDir string
	runner     ProcessRunner
	now        func() time.Time
}

func (s *Service) ConfigureLaunch(reg *registry.Registry, index *itemindex.Index, auditStore *audit.Store, wrapperDir string) *Service {
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

func (s *Service) Launch(itemID string, input LaunchInput) (result LaunchResult, err error) {
	started := time.Now()
	workspaceID := ""
	defer func() {
		if s.launch == nil || s.launch.audit == nil {
			return
		}
		status := models.AuditStatusSuccess
		message := "External AI session launched"
		errorText := ""
		if err != nil {
			status = models.AuditStatusBlocked
			message = "External AI session launch blocked"
			errorText = err.Error()
			var launchErr *LaunchError
			if errors.As(err, &launchErr) && launchErr.Code == "launch_failed" {
				status = models.AuditStatusFailed
				message = "External AI session launch failed"
			}
		}
		_, _ = s.launch.audit.Append(models.AuditEvent{
			WorkspaceID: workspaceID, ItemID: itemID, Operation: "ai_session_launch",
			Status: status, Message: message, DurationMS: time.Since(started).Milliseconds(), Error: errorText,
		})
	}()
	if s.launch == nil || s.launch.registry == nil || s.launch.index == nil {
		return LaunchResult{}, launchError("launch_failed", "AI session launch is unavailable")
	}
	item, found, getErr := s.launch.index.Get(itemID)
	if getErr != nil {
		return LaunchResult{}, launchErrorWith("launch_failed", getErr)
	}
	if !found {
		return LaunchResult{}, launchError("item_not_found", "item not found")
	}
	workspaceID = item.WorkspaceID
	workspace, found, getErr := s.launch.registry.Get(item.WorkspaceID)
	if getErr != nil {
		return LaunchResult{}, launchErrorWith("launch_failed", getErr)
	}
	if !found {
		return LaunchResult{}, launchError("workspace_not_found", "workspace not found")
	}
	contextMode := strings.TrimSpace(input.ContextMode)
	if contextMode != "workspace_only" && contextMode != "card_context" {
		return LaunchResult{}, launchError("invalid_context_mode", "contextMode must be workspace_only or card_context")
	}
	if contextMode == "card_context" {
		if item.SourceMode == "snapshot" || !item.Editable {
			return LaunchResult{}, launchError("item_not_editable", "context-based AI sessions require an editable working-tree item")
		}
		_, joinErr := pathguard.SafeJoin(workspace.Path, item.ItemPath)
		if joinErr != nil {
			return LaunchResult{}, launchError("item_not_editable", "item path is outside the workspace")
		}
	}
	settings, settingsErr := s.Settings()
	if settingsErr != nil {
		return LaunchResult{}, launchErrorWith("launch_failed", settingsErr)
	}
	providerID := strings.TrimSpace(input.Provider)
	terminalID := strings.TrimSpace(input.Terminal)
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
	prompt, presetID, promptErr := s.composePrompt(input.Provider, itemID, input.ContextMode, input.PresetID, input.PromptDraft, input.CustomPrompt, input.SelectedSkills, input.SelectedAgents)
	if promptErr != nil {
		return LaunchResult{}, promptErr
	}
	values := map[string]string{
		"workspace": workspace.Path, "contextFile": item.ItemPath, "itemPath": item.ItemPath,
		"identifier": item.Identifier, "contextMode": contextMode, "intent": contextMode, "prompt": prompt,
	}
	providerName := expand(provider.Executable, values)
	providerArgs := launchProviderArgs(contextMode, provider.Args, values)
	terminalArgs := expandAll(terminal.Args, values)
	sessionID := "external-" + randomID()
	checkpoint := verificationCheckpoint{
		WorkspaceID:  workspace.ID,
		Provider:     providerID,
		SessionID:    sessionID,
		TerminalMode: "external",
		Profile:      "smoke",
	}
	if startErr := s.startTerminal(terminalID, terminal, terminalArgs, workspace.Path, providerName, providerArgs, checkpoint); startErr != nil {
		return LaunchResult{}, launchErrorWith("launch_failed", startErr)
	}
	return LaunchResult{Accepted: true, Provider: providerID, Terminal: terminalID, ContextMode: contextMode, Surface: "external", PresetID: presetID, SessionID: sessionID, StartedAt: s.launch.now().UTC()}, nil
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

func (s *Service) composePrompt(providerID, itemID, contextMode, presetID, promptDraft, customPrompt string, selectedSkills, selectedAgents []string) (string, string, error) {
	basePrompt, resolvedPresetID, err := s.resolvePrompt(presetID, promptDraft, customPrompt)
	if err != nil {
		return "", "", err
	}
	catalog, catalogErr := s.ProviderCapabilities(providerID, itemID)
	if catalogErr != nil {
		return "", "", catalogErr
	}
	skills := normalizeCapabilitySelection(selectedSkills, catalog.Skills)
	agents := normalizeCapabilitySelection(selectedAgents, catalog.Agents)
	if len(skills) == 0 && len(agents) == 0 {
		return basePrompt, resolvedPresetID, nil
	}
	if !catalog.SupportsPromptFallback {
		return basePrompt, resolvedPresetID, nil
	}
	block := buildCapabilityPromptBlock(contextMode, skills, agents)
	if strings.TrimSpace(basePrompt) == "" {
		return block, resolvedPresetID, nil
	}
	return strings.TrimSpace(basePrompt) + "\n\n" + block, resolvedPresetID, nil
}

func (s *Service) ProviderCapabilities(providerID, itemID string) (ProviderCapabilityCatalog, error) {
	settings, err := s.Settings()
	if err != nil {
		return ProviderCapabilityCatalog{}, err
	}
	id := strings.TrimSpace(providerID)
	if id == "" {
		id = settings.DefaultProvider
	}
	if _, ok := settings.Providers[id]; !ok {
		return ProviderCapabilityCatalog{}, launchError("ai_provider_missing", "selected AI provider is unavailable")
	}
	workspacePath := ""
	if strings.TrimSpace(itemID) != "" {
		if s.launch == nil || s.launch.registry == nil || s.launch.index == nil {
			return ProviderCapabilityCatalog{}, launchError("launch_failed", "AI session launch is unavailable")
		}
		item, found, getErr := s.launch.index.Get(strings.TrimSpace(itemID))
		if getErr != nil {
			return ProviderCapabilityCatalog{}, launchErrorWith("launch_failed", getErr)
		}
		if !found {
			return ProviderCapabilityCatalog{}, launchError("item_not_found", "item not found")
		}
		workspace, found, getErr := s.launch.registry.Get(item.WorkspaceID)
		if getErr != nil {
			return ProviderCapabilityCatalog{}, launchErrorWith("launch_failed", getErr)
		}
		if !found {
			return ProviderCapabilityCatalog{}, launchError("workspace_not_found", "workspace not found")
		}
		workspacePath = workspace.Path
	}
	skills, agents := discoverProviderCapabilities(id, workspacePath)
	return ProviderCapabilityCatalog{
		Provider:                id,
		Skills:                  skills,
		Agents:                  agents,
		SupportsNativeSelection: false,
		SupportsPromptFallback:  true,
	}, nil
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

func writeWrapper(wrapperDir, workspace, executable string, args []string, checkpoint verificationCheckpoint) (string, error) {
	if err := os.MkdirAll(wrapperDir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(wrapperDir, "launch-"+randomID()+".command")
	command := "#!/bin/sh\ncd -- " + shellQuote(workspace) + " || exit 1\nself=$0\nrm -f -- \"$self\"\n" + shellQuote(executable)
	for _, arg := range args {
		command += " " + shellQuote(arg)
	}
	command += "\ncode=$?\n"
	if callback := checkpointCommand(checkpoint); callback != "" {
		command += callback + "\n"
	}
	command += "exit $code\n"
	if err := writeAtomic(path, []byte(command), 0o700); err != nil {
		return "", err
	}
	return path, nil
}

func checkpointCommand(checkpoint verificationCheckpoint) string {
	if strings.TrimSpace(checkpoint.WorkspaceID) == "" {
		return ""
	}
	body, err := json.Marshal(map[string]string{
		"eventType":    "session_completed",
		"profile":      firstNonEmpty(strings.TrimSpace(checkpoint.Profile), "smoke"),
		"provider":     strings.TrimSpace(checkpoint.Provider),
		"sessionId":    strings.TrimSpace(checkpoint.SessionID),
		"terminalMode": firstNonEmpty(strings.TrimSpace(checkpoint.TerminalMode), "external"),
	})
	if err != nil {
		return ""
	}
	urlValue := fmt.Sprintf("http://127.0.0.1:%s/api/workspaces/%s/verification-checkpoints", serverPort(), url.PathEscape(checkpoint.WorkspaceID))
	return "if command -v curl >/dev/null 2>&1; then curl -sS -X POST -H 'Content-Type: application/json' --data " + shellQuote(string(body)) + " " + shellQuote(urlValue) + " >/dev/null 2>&1 || true; fi"
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

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".ai-session-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func expand(value string, values map[string]string) string {
	for key, replacement := range values {
		value = strings.ReplaceAll(value, "{"+key+"}", replacement)
	}
	return value
}

func expandAll(values []string, replacements map[string]string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = expand(value, replacements)
	}
	return result
}

func expandProviderArgs(values []string, replacements map[string]string) []string {
	expanded := expandAll(values, replacements)
	if containsPlaceholder(values, "{prompt}") {
		return expanded
	}
	if prompt := strings.TrimSpace(replacements["prompt"]); prompt != "" {
		return append(expanded, prompt)
	}
	return expanded
}

func launchProviderArgs(contextMode string, values []string, replacements map[string]string) []string {
	if strings.TrimSpace(contextMode) == "workspace_only" {
		prompt := strings.TrimSpace(replacements["prompt"])
		if prompt == "" {
			return nil
		}
		return []string{prompt}
	}
	return expandProviderArgs(values, replacements)
}

func containsPlaceholder(values []string, placeholder string) bool {
	for _, value := range values {
		if strings.Contains(value, placeholder) {
			return true
		}
	}
	return false
}

func normalizeCapabilitySelection(selected []string, allowed []CapabilityDescriptor) []CapabilityDescriptor {
	if len(selected) == 0 || len(allowed) == 0 {
		return nil
	}
	allowedByID := map[string]CapabilityDescriptor{}
	for _, item := range allowed {
		allowedByID[item.ID] = item
	}
	result := make([]CapabilityDescriptor, 0, len(selected))
	seen := map[string]bool{}
	for _, id := range selected {
		id = strings.TrimSpace(id)
		item, ok := allowedByID[id]
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, item)
	}
	return result
}

func buildCapabilityPromptBlock(contextMode string, skills, agents []CapabilityDescriptor) string {
	lines := []string{"Capability directives:"}
	if len(skills) > 0 {
		names := make([]string, 0, len(skills))
		for _, skill := range skills {
			names = append(names, skill.Name)
		}
		lines = append(lines, "- Skills: "+strings.Join(names, ", "))
	}
	if len(agents) > 0 {
		names := make([]string, 0, len(agents))
		for _, agent := range agents {
			names = append(names, agent.Name)
		}
		lines = append(lines, "- Agents: "+strings.Join(names, ", "))
	}
	if strings.TrimSpace(contextMode) != "" {
		lines = append(lines, "- Keep behavior consistent with context mode: "+strings.TrimSpace(contextMode)+".")
	}
	lines = append(lines, "- Treat these directives as user-selected guidance for this session.")
	return strings.Join(lines, "\n")
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

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

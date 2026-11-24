package ai

import (
	"errors"
	"strings"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/pathguard"
)

func (s *Service) Launch(itemID string, input LaunchInput) (result LaunchResult, err error) {
	started := time.Now()
	workspaceID := ""
	defer func() { s.auditLaunch(itemID, workspaceID, started, err) }()
	if s.launch == nil || s.launch.registry == nil || s.launch.index == nil {
		return LaunchResult{}, launchError("launch_failed", "AI session launch is unavailable")
	}
	item, workspace, err := s.launchTarget(itemID)
	workspaceID = item.WorkspaceID // Set before the error check, so a failed lookup is still audited.
	if err != nil {
		return LaunchResult{}, err
	}
	contextMode, err := launchContextMode(input.ContextMode, item, workspace)
	if err != nil {
		return LaunchResult{}, err
	}
	providerID, terminalID := strings.TrimSpace(input.Provider), strings.TrimSpace(input.Terminal)
	provider, terminal, err := s.launchTools(providerID, terminalID)
	if err != nil {
		return LaunchResult{}, err
	}
	prompt, presetID, err := s.composePrompt(input.Provider, itemID, input.ContextMode, input.PresetID, input.PromptDraft, input.CustomPrompt, input.SelectedSkills, input.SelectedAgents)
	if err != nil {
		return LaunchResult{}, err
	}
	values := map[string]string{
		"workspace": workspace.Path, "contextFile": item.ItemPath, "itemPath": item.ItemPath,
		"identifier": item.Identifier, "contextMode": contextMode, "intent": contextMode, "prompt": prompt,
	}
	sessionID := "external-" + randomID()
	checkpoint := verificationCheckpoint{WorkspaceID: workspace.ID, Provider: providerID, SessionID: sessionID, TerminalMode: "external", Profile: "smoke"}
	if startErr := s.startTerminal(terminalID, terminal, expandAll(terminal.Args, values), workspace.Path, expand(provider.Executable, values), launchProviderArgs(contextMode, provider.Args, values), checkpoint); startErr != nil {
		return LaunchResult{}, launchErrorWith("launch_failed", startErr)
	}
	return LaunchResult{Accepted: true, Provider: providerID, Terminal: terminalID, ContextMode: contextMode, Surface: "external", PresetID: presetID, SessionID: sessionID, StartedAt: s.launch.now().UTC()}, nil
}

func (s *Service) auditLaunch(itemID, workspaceID string, started time.Time, err error) {
	if s.launch == nil || s.launch.audit == nil {
		return
	}
	status, message, errorText := models.AuditStatusSuccess, "External AI session launched", ""
	if err != nil {
		status, message, errorText = models.AuditStatusBlocked, "External AI session launch blocked", err.Error()
		var launchErr *LaunchError
		if errors.As(err, &launchErr) && launchErr.Code == "launch_failed" {
			status, message = models.AuditStatusFailed, "External AI session launch failed"
		}
	}
	_, _ = s.launch.audit.Append(models.AuditEvent{
		WorkspaceID: workspaceID, ItemID: itemID, Operation: "ai_session_launch",
		Status: status, Message: message, DurationMS: time.Since(started).Milliseconds(), Error: errorText,
	})
}

func (s *Service) launchTarget(itemID string) (models.ItemDetail, models.WorkspaceConfig, error) {
	item, found, err := s.launch.index.Get(itemID)
	if err != nil {
		return item, models.WorkspaceConfig{}, launchErrorWith("launch_failed", err)
	}
	if !found {
		return item, models.WorkspaceConfig{}, launchError("item_not_found", "item not found")
	}
	workspace, found, err := s.launch.registry.Get(item.WorkspaceID)
	if err != nil {
		return item, workspace, launchErrorWith("launch_failed", err)
	}
	if !found {
		return item, workspace, launchError("workspace_not_found", "workspace not found")
	}
	return item, workspace, nil
}

func launchContextMode(mode string, item models.ItemDetail, workspace models.WorkspaceConfig) (string, error) {
	contextMode := strings.TrimSpace(mode)
	if contextMode != "workspace_only" && contextMode != "card_context" {
		return "", launchError("invalid_context_mode", "contextMode must be workspace_only or card_context")
	}
	if contextMode == "card_context" {
		if item.SourceMode == "snapshot" || !item.Editable {
			return "", launchError("item_not_editable", "context-based AI sessions require an editable working-tree item")
		}
		if _, err := pathguard.SafeJoin(workspace.Path, item.ItemPath); err != nil {
			return "", launchError("item_not_editable", "item path is outside the workspace")
		}
	}
	return contextMode, nil
}

func (s *Service) launchTools(providerID, terminalID string) (LaunchTemplate, LaunchTemplate, error) {
	settings, err := s.Settings()
	if err != nil {
		return LaunchTemplate{}, LaunchTemplate{}, launchErrorWith("launch_failed", err)
	}
	provider, ok := settings.Providers[providerID]
	if !ok || !provider.Enabled {
		return provider, LaunchTemplate{}, launchError("ai_provider_missing", "selected AI provider is unavailable")
	}
	terminal, ok := settings.Terminals[terminalID]
	if !ok || !terminal.Enabled {
		return provider, terminal, launchError("terminal_missing", "selected terminal is unavailable")
	}
	if !s.detect(provider.Executable).Detected {
		return provider, terminal, launchError("ai_provider_missing", "selected AI provider executable was not found")
	}
	if !s.detect(terminal.Executable).Detected {
		return provider, terminal, launchError("terminal_missing", "selected terminal executable was not found")
	}
	return provider, terminal, nil
}

package ai

import (
	"strings"

	"kode-stream/internal/filesystem/pathguard"
)

type EmbeddedInput struct {
	Provider       string   `json:"provider"`
	ContextMode    string   `json:"contextMode"`
	PresetID       string   `json:"presetId,omitempty"`
	PromptDraft    string   `json:"promptDraft,omitempty"`
	CustomPrompt   string   `json:"customPrompt,omitempty"`
	SelectedSkills []string `json:"selectedSkills,omitempty"`
	SelectedAgents []string `json:"selectedAgents,omitempty"`
	Columns        uint16   `json:"columns"`
	Rows           uint16   `json:"rows"`
}

type EmbeddedResult struct {
	Session Session `json:"session"`
	Grant   Grant   `json:"grant"`
}

func (s *Service) StartEmbedded(itemID string, input EmbeddedInput) (EmbeddedResult, error) {
	if s.embedded == nil || s.launch == nil || s.launch.registry == nil || s.launch.index == nil {
		return EmbeddedResult{}, launchError("launch_failed", "embedded AI sessions are unavailable")
	}
	item, found, err := s.launch.index.Get(itemID)
	if err != nil {
		return EmbeddedResult{}, launchErrorWith("launch_failed", err)
	}
	if !found {
		return EmbeddedResult{}, launchError("item_not_found", "item not found")
	}
	workspace, found, err := s.launch.registry.Get(item.WorkspaceID)
	if err != nil {
		return EmbeddedResult{}, launchErrorWith("launch_failed", err)
	}
	if !found {
		return EmbeddedResult{}, launchError("workspace_not_found", "workspace not found")
	}
	mode := strings.TrimSpace(input.ContextMode)
	if mode != "workspace_only" && mode != "card_context" {
		return EmbeddedResult{}, launchError("invalid_context_mode", "contextMode must be workspace_only or card_context")
	}
	if mode == "card_context" {
		if item.SourceMode == "snapshot" || !item.Editable {
			return EmbeddedResult{}, launchError("item_not_editable", "context-based AI sessions require an editable working-tree item")
		}
		if _, err := pathguard.SafeJoin(workspace.Path, item.ItemPath); err != nil {
			return EmbeddedResult{}, launchError("item_not_editable", "item path is outside the workspace")
		}
	}
	settings, err := s.Settings()
	if err != nil {
		return EmbeddedResult{}, launchErrorWith("launch_failed", err)
	}
	providerID := strings.TrimSpace(input.Provider)
	provider, ok := settings.Providers[providerID]
	if !ok || !provider.Enabled {
		return EmbeddedResult{}, launchError("ai_provider_missing", "selected AI provider is unavailable")
	}
	capability := s.detect(provider.Executable)
	if !capability.Detected {
		return EmbeddedResult{}, launchError("ai_provider_missing", "selected AI provider executable was not found")
	}
	prompt, _, promptErr := s.composePrompt(input.Provider, itemID, input.ContextMode, input.PresetID, input.PromptDraft, input.CustomPrompt, input.SelectedSkills, input.SelectedAgents)
	if promptErr != nil {
		return EmbeddedResult{}, promptErr
	}
	values := map[string]string{"workspace": workspace.Path, "contextFile": item.ItemPath, "itemPath": item.ItemPath, "identifier": item.Identifier, "contextMode": mode, "intent": mode, "prompt": prompt}
	args := launchProviderArgs(mode, provider.Args, values)
	session, grant, err := s.embedded.Start(StartRequest{ItemID: itemID, ItemIdentifier: item.Identifier, ItemTitle: item.Title, WorkspaceID: item.WorkspaceID, Provider: providerID, Intent: mode, Executable: capability.Executable, Args: args, Dir: workspace.Path, Columns: input.Columns, Rows: input.Rows})
	if err != nil {
		return EmbeddedResult{}, launchErrorWith("launch_failed", err)
	}
	return EmbeddedResult{Session: session, Grant: grant}, nil
}

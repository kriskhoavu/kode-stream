package ai

import (
	"context"
	"path/filepath"
	"strings"
)

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

func (s *Service) composePrompt(providerID, itemID, contextMode, presetID, promptDraft, customPrompt string, selectedSkills, selectedAgents []string) (string, string, error) {
	catalog, catalogErr := s.ProviderCapabilities(providerID, itemID)
	if catalogErr != nil {
		return "", "", catalogErr
	}
	return s.composePromptWithCatalog(catalog, contextMode, presetID, promptDraft, customPrompt, selectedSkills, selectedAgents)
}

func (s *Service) composeWorkspacePrompt(providerID, workspaceID, contextMode, presetID, promptDraft, customPrompt string, selectedSkills, selectedAgents []string) (string, string, error) {
	catalog, catalogErr := s.ProviderCapabilitiesForWorkspace(providerID, workspaceID)
	if catalogErr != nil {
		return "", "", catalogErr
	}
	return s.composePromptWithCatalog(catalog, contextMode, presetID, promptDraft, customPrompt, selectedSkills, selectedAgents)
}

func (s *Service) composePromptWithCatalog(catalog ProviderCapabilityCatalog, contextMode, presetID, promptDraft, customPrompt string, selectedSkills, selectedAgents []string) (string, string, error) {
	basePrompt, resolvedPresetID, err := s.resolvePrompt(presetID, promptDraft, customPrompt)
	if err != nil {
		return "", "", err
	}
	skills, missingSkills := normalizeCapabilitySelection(selectedSkills, catalog.Skills)
	agents, missingAgents := normalizeCapabilitySelection(selectedAgents, catalog.Agents)
	if len(missingSkills) > 0 || len(missingAgents) > 0 {
		return "", "", launchError("capability_missing", "one or more selected AI capabilities are unavailable")
	}
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
	return s.ProviderCapabilitiesContext(context.Background(), providerID, itemID)
}

func (s *Service) ProviderCapabilitiesContext(ctx context.Context, providerID, itemID string) (ProviderCapabilityCatalog, error) {
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
	skills, agents := discoverProviderCapabilitiesContext(ctx, id, workspacePath)
	if err := ctx.Err(); err != nil {
		return ProviderCapabilityCatalog{}, err
	}
	return ProviderCapabilityCatalog{
		Provider:                id,
		Skills:                  skills,
		Agents:                  agents,
		SupportsNativeSelection: false,
		SupportsPromptFallback:  true,
	}, nil
}

func (s *Service) ProviderCapabilitiesForWorkspace(providerID, workspaceID string) (ProviderCapabilityCatalog, error) {
	return s.ProviderCapabilitiesForWorkspaceContext(context.Background(), providerID, workspaceID)
}

func (s *Service) ProviderCapabilitiesForWorkspaceContext(ctx context.Context, providerID, workspaceID string) (ProviderCapabilityCatalog, error) {
	if s.launch == nil || s.launch.registry == nil {
		return ProviderCapabilityCatalog{}, launchError("launch_failed", "AI session launch is unavailable")
	}
	workspace, found, err := s.launch.registry.Get(strings.TrimSpace(workspaceID))
	if err != nil {
		return ProviderCapabilityCatalog{}, launchErrorWith("launch_failed", err)
	}
	if !found {
		return ProviderCapabilityCatalog{}, launchError("workspace_not_found", "workspace not found")
	}
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
	skills, agents := discoverProviderCapabilitiesContext(ctx, id, workspace.Path)
	if err := ctx.Err(); err != nil {
		return ProviderCapabilityCatalog{}, err
	}
	return ProviderCapabilityCatalog{
		Provider:                id,
		Skills:                  skills,
		Agents:                  agents,
		SupportsNativeSelection: false,
		SupportsPromptFallback:  true,
	}, nil
}

func normalizeCapabilitySelection(selected []string, allowed []CapabilityDescriptor) ([]CapabilityDescriptor, []string) {
	if len(selected) == 0 {
		return nil, nil
	}
	allowedByID := map[string]CapabilityDescriptor{}
	allowedByAlias := map[string][]CapabilityDescriptor{}
	for _, item := range allowed {
		allowedByID[item.ID] = item
		for _, alias := range capabilityAliases(item) {
			allowedByAlias[alias] = append(allowedByAlias[alias], item)
		}
	}
	result := make([]CapabilityDescriptor, 0, len(selected))
	missing := make([]string, 0)
	seen := map[string]bool{}
	for _, id := range selected {
		id = strings.TrimSpace(id)
		item, ok := allowedByID[id]
		if !ok {
			candidates := allowedByAlias[normalizeCapabilityAlias(id)]
			if len(candidates) == 1 {
				item, ok = candidates[0], true
			}
		}
		if !ok {
			missing = append(missing, id)
			continue
		}
		if seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		result = append(result, item)
	}
	return result, missing
}

func capabilityAliases(item CapabilityDescriptor) []string {
	nameAlias := normalizeCapabilityAlias(item.Name)
	base := strings.TrimSuffix(filepath.Base(item.SourcePath), filepath.Ext(item.SourcePath))
	if isGenericCapabilityBasename(base) {
		base = filepath.Base(filepath.Dir(item.SourcePath))
	}
	pathAlias := normalizeCapabilityAlias(base)
	if pathAlias == nameAlias {
		return []string{nameAlias}
	}
	return []string{nameAlias, pathAlias}
}

func normalizeCapabilityAlias(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	return strings.Join(strings.Fields(value), "-")
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

package ai

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type capabilityRoot struct {
	path        string
	scope       string
	displayBase string
	kind        string
	mode        string
}

func discoverProviderCapabilities(providerID, workspacePath string) ([]CapabilityDescriptor, []CapabilityDescriptor) {
	roots := providerCapabilityRoots(providerID, workspacePath)
	skills := []CapabilityDescriptor{}
	agents := []CapabilityDescriptor{}
	seen := map[string]bool{}
	for _, root := range roots {
		files, err := discoverCapabilityFiles(root)
		if err != nil {
			continue
		}
		for _, file := range files {
			descriptor, ok := buildCapabilityDescriptor(providerID, root, file)
			seenKey := providerID + ":" + descriptor.Scope + ":" + descriptor.SourcePath
			if !ok || seen[seenKey] {
				continue
			}
			seen[seenKey] = true
			if descriptor.Kind == "agent" {
				agents = append(agents, descriptor)
			} else {
				skills = append(skills, descriptor)
			}
		}
	}
	sort.Slice(skills, func(i, j int) bool { return capabilitySortKey(skills[i]) < capabilitySortKey(skills[j]) })
	sort.Slice(agents, func(i, j int) bool { return capabilitySortKey(agents[i]) < capabilitySortKey(agents[j]) })
	return skills, agents
}

func providerCapabilityRoots(providerID, workspacePath string) []capabilityRoot {
	roots := []capabilityRoot{}
	add := func(path, scope, displayBase, kind, mode string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		roots = append(roots, capabilityRoot{path: path, scope: scope, displayBase: displayBase, kind: kind, mode: mode})
	}
	if workspacePath != "" {
		for _, root := range providerWorkspaceRoots(providerID) {
			add(filepath.Join(workspacePath, root.path), "workspace", workspacePath, root.kind, root.mode)
		}
	}
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		for _, root := range providerGlobalRoots(providerID) {
			add(filepath.Join(home, root.path), "global", home, root.kind, root.mode)
		}
	}
	return roots
}

func providerWorkspaceRoots(providerID string) []capabilityRoot {
	switch strings.TrimSpace(providerID) {
	case "claude":
		return []capabilityRoot{
			{path: ".claude/agents", kind: "agent", mode: "files"},
			{path: ".claude/commands", kind: "skill", mode: "files"},
			{path: ".claude/skills", kind: "skill", mode: "skill_dirs"},
			{path: ".agents", kind: "agent", mode: "files"},
			{path: ".skills", kind: "skill", mode: "files"},
		}
	case "copilot":
		return []capabilityRoot{
			{path: ".github/chatmodes", kind: "agent", mode: "files"},
			{path: ".github/prompts", kind: "skill", mode: "files"},
			{path: ".github/instructions", kind: "skill", mode: "files"},
			{path: ".agents", kind: "agent", mode: "files"},
			{path: ".skills", kind: "skill", mode: "files"},
		}
	case "codex":
		return []capabilityRoot{
			{path: ".codex/agents", kind: "agent", mode: "files"},
			{path: ".codex/skills", kind: "skill", mode: "skill_dirs"},
			{path: ".agents", kind: "agent", mode: "files"},
			{path: ".skills", kind: "skill", mode: "files"},
		}
	case "opencode":
		return []capabilityRoot{
			{path: ".opencode/agents", kind: "agent", mode: "files"},
			{path: ".opencode/skills", kind: "skill", mode: "skill_dirs"},
			{path: ".agents", kind: "agent", mode: "files"},
			{path: ".skills", kind: "skill", mode: "files"},
		}
	default:
		return []capabilityRoot{
			{path: ".agents", kind: "agent", mode: "files"},
			{path: ".skills", kind: "skill", mode: "files"},
		}
	}
}

func providerGlobalRoots(providerID string) []capabilityRoot {
	switch strings.TrimSpace(providerID) {
	case "claude":
		return []capabilityRoot{
			{path: ".claude/agents", kind: "agent", mode: "files"},
			{path: ".claude/commands", kind: "skill", mode: "files"},
			{path: ".claude/skills", kind: "skill", mode: "skill_dirs"},
			{path: ".agents", kind: "agent", mode: "files"},
			{path: ".skills", kind: "skill", mode: "files"},
		}
	case "copilot":
		return []capabilityRoot{
			{path: ".config/github-copilot/chatmodes", kind: "agent", mode: "files"},
			{path: ".config/github-copilot/prompts", kind: "skill", mode: "files"},
			{path: ".config/github-copilot/instructions", kind: "skill", mode: "files"},
			{path: ".copilot/chatmodes", kind: "agent", mode: "files"},
			{path: ".copilot/prompts", kind: "skill", mode: "files"},
			{path: ".copilot/instructions", kind: "skill", mode: "files"},
			{path: ".agents", kind: "agent", mode: "files"},
			{path: ".skills", kind: "skill", mode: "files"},
		}
	case "codex":
		return []capabilityRoot{
			{path: ".codex/agents", kind: "agent", mode: "files"},
			{path: ".codex/skills", kind: "skill", mode: "skill_dirs"},
			{path: ".agents", kind: "agent", mode: "files"},
			{path: ".skills", kind: "skill", mode: "files"},
		}
	case "opencode":
		return []capabilityRoot{
			{path: ".opencode/agents", kind: "agent", mode: "files"},
			{path: ".opencode/skills", kind: "skill", mode: "skill_dirs"},
			{path: ".agents", kind: "agent", mode: "files"},
			{path: ".skills", kind: "skill", mode: "files"},
		}
	default:
		return []capabilityRoot{
			{path: ".agents", kind: "agent", mode: "files"},
			{path: ".skills", kind: "skill", mode: "files"},
		}
	}
}

func discoverCapabilityFiles(root capabilityRoot) ([]string, error) {
	info, err := os.Stat(root.path)
	if err != nil || !info.IsDir() {
		return nil, err
	}
	switch root.mode {
	case "skill_dirs":
		return discoverDirectoryCapabilities(root.path, root.kind)
	case "files":
		fallthrough
	default:
		return discoverDirectCapabilityFiles(root.path)
	}
}

func discoverDirectCapabilityFiles(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if !isCapabilityFile(path) {
			continue
		}
		files = append(files, path)
	}
	return files, nil
}

func discoverDirectoryCapabilities(root string, kind string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if entry.IsDir() {
			if file, ok := findCapabilityEntrypoint(path, kind); ok {
				files = append(files, file)
			}
			continue
		}
		if isCapabilityFile(path) {
			files = append(files, path)
		}
	}
	return files, nil
}

func findCapabilityEntrypoint(root string, kind string) (string, bool) {
	candidates := []string{"README.md"}
	if kind == "agent" {
		candidates = append([]string{"AGENT.md", "AGENTS.md", "agent.md", "agents.md"}, candidates...)
	} else {
		candidates = append([]string{"SKILL.md", "skill.md"}, candidates...)
	}
	for _, candidate := range candidates {
		path := filepath.Join(root, candidate)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

func isCapabilityFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".markdown", ".txt", ".yaml", ".yml", ".json":
		return true
	}
	return strings.Contains(name, "agent") || strings.Contains(name, "prompt") || strings.Contains(name, "skill") || strings.Contains(name, "command") || strings.Contains(name, "chatmode")
}

func buildCapabilityDescriptor(providerID string, root capabilityRoot, file string) (CapabilityDescriptor, bool) {
	relative, err := filepath.Rel(root.path, file)
	if err != nil {
		return CapabilityDescriptor{}, false
	}
	if strings.HasPrefix(relative, "..") {
		return CapabilityDescriptor{}, false
	}
	kind := root.kind
	if kind == "" {
		kind = classifyCapabilityKind(providerID, file)
	}
	normalizedRelative := filepath.ToSlash(strings.TrimSpace(relative))
	name, description := capabilityMetadata(file, normalizedRelative)
	sourcePath := file
	if root.displayBase != "" {
		if displayRelative, relErr := filepath.Rel(root.displayBase, file); relErr == nil && !strings.HasPrefix(displayRelative, "..") {
			sourcePath = filepath.ToSlash(displayRelative)
		}
	}
	return CapabilityDescriptor{
		ID:          providerID + ":" + root.scope + ":" + sourcePath,
		Name:        name,
		Description: firstNonEmpty(strings.TrimSpace(description), capabilityDescription(kind, providerID, root.scope, normalizedRelative)),
		Kind:        kind,
		Provider:    providerID,
		Scope:       root.scope,
		SourcePath:  sourcePath,
	}, true
}

func classifyCapabilityKind(providerID, path string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	switch {
	case strings.TrimSpace(providerID) == "claude" && strings.Contains(lower, "/.claude/agents/"):
		return "agent"
	case strings.TrimSpace(providerID) == "claude" && (strings.Contains(lower, "/.claude/commands/") || strings.Contains(lower, "/.claude/skills/")):
		return "skill"
	case strings.TrimSpace(providerID) == "copilot" && strings.Contains(lower, "/.github/chatmodes/"):
		return "agent"
	case strings.TrimSpace(providerID) == "copilot" && (strings.Contains(lower, "/.github/prompts/") || strings.Contains(lower, "/.github/instructions/")):
		return "skill"
	case strings.TrimSpace(providerID) == "codex" && strings.Contains(lower, "/.codex/agents/"):
		return "agent"
	case strings.TrimSpace(providerID) == "codex" && strings.Contains(lower, "/.codex/skills/"):
		return "skill"
	case strings.TrimSpace(providerID) == "opencode" && strings.Contains(lower, "/.opencode/agents/"):
		return "agent"
	case strings.TrimSpace(providerID) == "opencode" && strings.Contains(lower, "/.opencode/skills/"):
		return "skill"
	case strings.Contains(lower, "/.agents/"), strings.Contains(lower, "/agents/"), strings.Contains(lower, "agent"), strings.Contains(lower, "/chatmodes/"), strings.Contains(lower, "chatmode"):
		return "agent"
	case strings.Contains(lower, "/.skills/"), strings.Contains(lower, "/skills/"), strings.Contains(lower, "skill"), strings.Contains(lower, "/prompts/"), strings.Contains(lower, "/instructions/"), strings.Contains(lower, "/commands/"), strings.Contains(lower, "prompt"), strings.Contains(lower, "command"):
		return "skill"
	default:
		return "skill"
	}
}

func capabilityNameFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if isGenericCapabilityBasename(base) {
		base = filepath.Base(filepath.Dir(path))
	}
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.TrimSpace(base)
	if base == "" {
		return "Unnamed capability"
	}
	parts := strings.Fields(base)
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func isGenericCapabilityBasename(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "skill", "skills", "agent", "agents", "prompt", "prompts", "command", "commands", "chatmode", "chatmodes", "instruction", "instructions", "readme":
		return true
	default:
		return false
	}
}

func capabilityDescription(kind, providerID, scope, relativePath string) string {
	scopeLabel := "Global"
	if scope == "workspace" {
		scopeLabel = "Workspace"
	}
	kindLabel := "skill"
	if kind == "agent" {
		kindLabel = "agent"
	}
	providerLabel := strings.TrimSpace(providerID)
	if providerLabel == "" {
		providerLabel = "provider"
	}
	return scopeLabel + " " + kindLabel + " for " + providerLabel + " from " + filepath.ToSlash(relativePath)
}

func capabilitySortKey(item CapabilityDescriptor) string {
	scopeRank := "1"
	if item.Scope == "workspace" {
		scopeRank = "0"
	}
	return scopeRank + ":" + strings.ToLower(item.Name) + ":" + strings.ToLower(item.SourcePath)
}

func capabilityMetadata(filePath, fallbackRelativePath string) (string, string) {
	name := capabilityNameFromPath(fallbackRelativePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return name, ""
	}
	if metaName, metaDescription, ok := decodeStructuredCapabilityMetadata(data); ok {
		return firstNonEmpty(metaName, name), metaDescription
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	title := ""
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "---") {
			continue
		}
		if strings.HasPrefix(line, "#") {
			title = strings.TrimSpace(strings.TrimLeft(line, "#"))
			break
		}
	}
	return firstNonEmpty(title, name), ""
}

func decodeStructuredCapabilityMetadata(data []byte) (string, string, bool) {
	var structured struct {
		Name        string `json:"name" yaml:"name"`
		Title       string `json:"title" yaml:"title"`
		Description string `json:"description" yaml:"description"`
		Summary     string `json:"summary" yaml:"summary"`
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "", "", false
	}
	if trimmed[0] == '{' {
		if err := json.Unmarshal(trimmed, &structured); err == nil {
			return firstNonEmpty(structured.Name, structured.Title), firstNonEmpty(structured.Description, structured.Summary), true
		}
	}
	if err := yaml.Unmarshal(trimmed, &structured); err == nil {
		if strings.TrimSpace(structured.Name) != "" || strings.TrimSpace(structured.Title) != "" || strings.TrimSpace(structured.Description) != "" || strings.TrimSpace(structured.Summary) != "" {
			return firstNonEmpty(structured.Name, structured.Title), firstNonEmpty(structured.Description, structured.Summary), true
		}
	}
	return "", "", false
}

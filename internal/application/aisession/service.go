package aisession

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"plan-manager/internal/aisettings"
)

type Capability struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Detected   bool   `json:"detected"`
	Configured bool   `json:"configured"`
	Executable string `json:"executable"`
	Reason     string `json:"reason,omitempty"`
}

type Service struct {
	store    *aisettings.Store
	lookPath func(string) (string, error)
	stat     func(string) (os.FileInfo, error)
	goos     string
	launch   *launchDependencies
}

func New(store *aisettings.Store) *Service {
	return &Service{store: store, lookPath: exec.LookPath, stat: os.Stat, goos: runtime.GOOS}
}

func (s *Service) Settings() (aisettings.Settings, error) {
	saved, err := s.store.Load()
	if err != nil {
		return aisettings.Settings{}, err
	}
	settings := mergeDefaults(saved, s.goos)
	if settings.DefaultProvider == "" {
		settings.DefaultProvider = s.firstDetected(settings.Providers)
	}
	if settings.DefaultTerminal == "" {
		settings.DefaultTerminal = s.firstDetected(settings.Terminals)
	}
	if settings.DefaultProvider == "" {
		settings.DefaultProvider = firstEnabled(settings.Providers)
	}
	if settings.DefaultTerminal == "" {
		settings.DefaultTerminal = firstEnabled(settings.Terminals)
	}
	return settings, aisettings.Validate(settings)
}

func (s *Service) Save(settings aisettings.Settings) (aisettings.Settings, error) {
	settings = mergeDefaults(settings, s.goos)
	return s.store.Save(settings)
}

func (s *Service) Capabilities() ([]Capability, error) {
	settings, err := s.Settings()
	if err != nil {
		return nil, err
	}
	capabilities := make([]Capability, 0, len(settings.Providers)+len(settings.Terminals))
	capabilities = append(capabilities, s.detectAll(aisettings.KindProvider, settings.Providers)...)
	capabilities = append(capabilities, s.detectAll(aisettings.KindTerminal, settings.Terminals)...)
	sort.Slice(capabilities, func(i, j int) bool {
		if capabilities[i].Kind == capabilities[j].Kind {
			return capabilities[i].ID < capabilities[j].ID
		}
		return capabilities[i].Kind < capabilities[j].Kind
	})
	return capabilities, nil
}

func (s *Service) firstDetected(templates map[string]aisettings.LaunchTemplate) string {
	for _, id := range orderedIDs(templates) {
		template := templates[id]
		if template.Enabled && s.detect(template.Executable).Detected {
			return id
		}
	}
	return ""
}

func (s *Service) detectAll(kind string, templates map[string]aisettings.LaunchTemplate) []Capability {
	result := make([]Capability, 0, len(templates))
	for id, template := range templates {
		capability := s.detect(template.Executable)
		capability.ID = id
		capability.Kind = kind
		capability.Configured = template.Enabled
		if !template.Enabled {
			capability.Reason = "disabled in settings"
		}
		result = append(result, capability)
	}
	return result
}

func (s *Service) detect(executable string) Capability {
	value := strings.TrimSpace(executable)
	if value == "" {
		return Capability{Reason: "executable is not configured"}
	}
	if filepath.IsAbs(value) || strings.ContainsRune(value, filepath.Separator) {
		info, err := s.stat(value)
		if err != nil {
			return Capability{Executable: value, Reason: "configured path was not found"}
		}
		if !info.IsDir() && info.Mode().Perm()&0o111 == 0 {
			return Capability{Executable: value, Reason: "configured path is not executable"}
		}
		return Capability{Detected: true, Executable: value}
	}
	path, err := s.lookPath(value)
	if err != nil {
		return Capability{Executable: value, Reason: "executable was not found on PATH"}
	}
	return Capability{Detected: true, Executable: path}
}

func mergeDefaults(saved aisettings.Settings, goos string) aisettings.Settings {
	defaults := defaultSettings(goos)
	for id, template := range saved.Providers {
		if len(template.Args) == 1 && (template.Args[0] == "Read {contextFile} and follow its {intent} instructions for {identifier}." || template.Args[0] == "Read {contextFile}. Use it only as context and wait for the user's request.") {
			template.Args[0] = "The selected card is at {itemPath}. Read that path and its relevant documents as context, then wait for the user's request."
		}
		defaults.Providers[id] = template
	}
	for id, template := range saved.Terminals {
		defaults.Terminals[id] = template
	}
	if saved.DefaultProvider != "" {
		defaults.DefaultProvider = saved.DefaultProvider
	}
	if saved.DefaultTerminal != "" {
		defaults.DefaultTerminal = saved.DefaultTerminal
	}
	return defaults
}

func defaultSettings(goos string) aisettings.Settings {
	prompt := "The selected card is at {itemPath}. Read that path and its relevant documents as context, then wait for the user's request."
	settings := aisettings.Settings{
		Providers: map[string]aisettings.LaunchTemplate{
			"claude":   {Enabled: true, Executable: "claude", Args: []string{prompt}},
			"codex":    {Enabled: true, Executable: "codex", Args: []string{prompt}},
			"copilot":  {Enabled: true, Executable: "copilot", Args: []string{prompt}},
			"opencode": {Enabled: true, Executable: "opencode", Args: []string{"--prompt", prompt}},
		},
		Terminals: map[string]aisettings.LaunchTemplate{},
	}
	if goos == "darwin" {
		settings.Terminals["terminal"] = aisettings.LaunchTemplate{Enabled: true, Executable: "/System/Applications/Utilities/Terminal.app"}
		settings.Terminals["iterm2"] = aisettings.LaunchTemplate{Enabled: true, Executable: "/Applications/iTerm.app"}
		settings.Terminals["wezterm"] = aisettings.LaunchTemplate{Enabled: true, Executable: "wezterm"}
	}
	return settings
}

func orderedIDs(templates map[string]aisettings.LaunchTemplate) []string {
	preferred := []string{"codex", "claude", "copilot", "opencode", "iterm2", "wezterm", "terminal"}
	seen := map[string]bool{}
	ids := make([]string, 0, len(templates))
	for _, id := range preferred {
		if _, ok := templates[id]; ok {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	remaining := make([]string, 0, len(templates))
	for id := range templates {
		if !seen[id] {
			remaining = append(remaining, id)
		}
	}
	sort.Strings(remaining)
	return append(ids, remaining...)
}

func firstEnabled(templates map[string]aisettings.LaunchTemplate) string {
	for _, id := range orderedIDs(templates) {
		if templates[id].Enabled {
			return id
		}
	}
	return ""
}

func (c Capability) String() string {
	return fmt.Sprintf("%s:%s", c.Kind, c.ID)
}

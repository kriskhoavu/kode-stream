package ai

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const (
	KindProvider = "provider"
	KindTerminal = "terminal"
)

var (
	allowedPlaceholders = map[string]bool{
		"workspace": true, "contextFile": true, "itemPath": true,
		"identifier": true, "contextMode": true, "intent": true,
		"prompt": true,
	}
	placeholderPattern = regexp.MustCompile(`\{([^{}]+)\}`)
)

type LaunchTemplate struct {
	Enabled    bool     `json:"enabled" yaml:"enabled"`
	Executable string   `json:"executable" yaml:"executable"`
	Args       []string `json:"args" yaml:"args"`
}

type Settings struct {
	DefaultProvider string                    `json:"defaultProvider" yaml:"defaultProvider"`
	DefaultTerminal string                    `json:"defaultTerminal" yaml:"defaultTerminal"`
	Providers       map[string]LaunchTemplate `json:"providers" yaml:"providers"`
	Terminals       map[string]LaunchTemplate `json:"terminals" yaml:"terminals"`
}

type SettingsStore interface {
	Load() (Settings, error)
	Save(Settings) (Settings, error)
}

type AISettingsRepository struct {
	mu   sync.Mutex
	path string
}

func NewSettingsRepository(path string) *AISettingsRepository {
	return &AISettingsRepository{path: path}
}

func (s *AISettingsRepository) Load() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var settings Settings
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return normalize(settings), nil
	}
	if err != nil {
		return Settings{}, err
	}
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return Settings{}, fmt.Errorf("read AI settings: %w", err)
	}
	return normalize(settings), nil
}

func (s *AISettingsRepository) Save(settings Settings) (Settings, error) {
	settings = normalize(settings)
	if err := Validate(settings); err != nil {
		return Settings{}, err
	}
	data, err := yaml.Marshal(settings)
	if err != nil {
		return Settings{}, fmt.Errorf("encode AI settings: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return Settings{}, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".ai-settings-*")
	if err != nil {
		return Settings{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return Settings{}, err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return Settings{}, err
	}
	if err := temporary.Close(); err != nil {
		return Settings{}, err
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func Validate(settings Settings) error {
	if err := validateTemplates(KindProvider, settings.Providers); err != nil {
		return err
	}
	if err := validateTemplates(KindTerminal, settings.Terminals); err != nil {
		return err
	}
	if settings.DefaultProvider != "" {
		template, ok := settings.Providers[settings.DefaultProvider]
		if !ok || !template.Enabled {
			return errors.New("default AI provider must reference an enabled provider")
		}
	}
	if settings.DefaultTerminal != "" {
		template, ok := settings.Terminals[settings.DefaultTerminal]
		if !ok || !template.Enabled {
			return errors.New("default terminal must reference an enabled terminal")
		}
	}
	return nil
}

func validateTemplates(kind string, templates map[string]LaunchTemplate) error {
	for id, template := range templates {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("%s ID is required", kind)
		}
		if template.Enabled && strings.TrimSpace(template.Executable) == "" {
			return fmt.Errorf("%s %q executable is required", kind, id)
		}
		values := append([]string{template.Executable}, template.Args...)
		for _, value := range values {
			for _, match := range placeholderPattern.FindAllStringSubmatch(value, -1) {
				if !allowedPlaceholders[match[1]] {
					return fmt.Errorf("%s %q uses unsupported placeholder {%s}", kind, id, match[1])
				}
			}
			withoutPlaceholders := placeholderPattern.ReplaceAllString(value, "")
			if strings.ContainsAny(withoutPlaceholders, "\n\r\x00") {
				return fmt.Errorf("%s %q contains invalid command characters", kind, id)
			}
		}
	}
	return nil
}

func normalize(settings Settings) Settings {
	settings.DefaultProvider = strings.TrimSpace(settings.DefaultProvider)
	settings.DefaultTerminal = strings.TrimSpace(settings.DefaultTerminal)
	settings.Providers = normalizeTemplates(settings.Providers)
	settings.Terminals = normalizeTemplates(settings.Terminals)
	return settings
}

func normalizeTemplates(input map[string]LaunchTemplate) map[string]LaunchTemplate {
	output := make(map[string]LaunchTemplate, len(input))
	for id, template := range input {
		template.Executable = strings.TrimSpace(template.Executable)
		if template.Args == nil {
			template.Args = []string{}
		}
		output[strings.TrimSpace(id)] = template
	}
	return output
}

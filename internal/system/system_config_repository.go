package system

// This package owns application path configuration persistence.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type Paths struct {
	Dir                string
	DefaultDir         string
	RegistryFile       string
	PlanIndexFile      string
	SQLiteDatabaseFile string
	KnowledgeIndexFile string
	AuditLogFile       string
	SavedFiltersFile   string
	RecentItemsFile    string
	AISettingsFile     string
	CloneRootDir       string
	FrontendAssets     string
}

func ResolvePaths() (Paths, error) {
	defaultDir, err := DefaultDataDir()
	if err != nil {
		return Paths{}, err
	}
	dir := defaultDir
	if override, err := resolveDataDirOverride(defaultDir); err == nil && strings.TrimSpace(override) != "" {
		dir = override
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Paths{}, err
	}
	cloneRootDir := filepath.Join(dir, "clone-root")
	if err := os.MkdirAll(cloneRootDir, 0o755); err != nil {
		return Paths{}, err
	}
	paths := Paths{
		Dir:                dir,
		DefaultDir:         defaultDir,
		RegistryFile:       filepath.Join(dir, "workspaces.yaml"),
		PlanIndexFile:      filepath.Join(dir, "item-index.yaml"),
		SQLiteDatabaseFile: filepath.Join(dir, "kode-stream.db"),
		KnowledgeIndexFile: filepath.Join(dir, "knowledge-index.yaml"),
		AuditLogFile:       filepath.Join(dir, "audit-log.jsonl"),
		SavedFiltersFile:   filepath.Join(dir, "saved-filters.yaml"),
		RecentItemsFile:    filepath.Join(dir, "recent-items.yaml"),
		AISettingsFile:     filepath.Join(dir, "ai-settings.yaml"),
		CloneRootDir:       cloneRootDir,
	}
	return paths, nil
}

func DefaultDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return defaultDataDirForOS(runtime.GOOS, os.Getenv, home)
}

func defaultDataDirForOS(goos string, getenv func(string) string, home string) (string, error) {
	var base string
	switch goos {
	case "darwin":
		base = filepath.Join(home, "Library", "Application Support")
	case "windows":
		base = strings.TrimSpace(getenv("AppData"))
		if base == "" {
			base = strings.TrimSpace(getenv("APPDATA"))
		}
		if base == "" {
			return "", errors.New("AppData is not defined")
		}
	default:
		base = strings.TrimSpace(getenv("XDG_CONFIG_HOME"))
		if base == "" {
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "kode-stream"), nil
}

func SetDataDir(path string) (Paths, error) {
	defaultDir, err := DefaultDataDir()
	if err != nil {
		return Paths{}, err
	}
	settingsPath := filepath.Join(defaultDir, "bootstrap.yaml")
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		return Paths{}, err
	}
	value := strings.TrimSpace(path)
	if value == "" {
		if err := os.Remove(settingsPath); err != nil && !os.IsNotExist(err) {
			return Paths{}, err
		}
		return ResolvePaths()
	}
	resolved, err := filepath.Abs(expandHome(value))
	if err != nil {
		return Paths{}, err
	}
	if err := os.MkdirAll(resolved, 0o755); err != nil {
		return Paths{}, err
	}
	data, err := yaml.Marshal(bootstrapSettings{DataDir: resolved})
	if err != nil {
		return Paths{}, err
	}
	if err := os.WriteFile(settingsPath, data, 0o600); err != nil {
		return Paths{}, err
	}
	return ResolvePaths()
}

type bootstrapSettings struct {
	DataDir string `yaml:"dataDir,omitempty"`
}

func resolveDataDirOverride(defaultDir string) (string, error) {
	if env := strings.TrimSpace(os.Getenv("KODE_STREAM_DATA_DIR")); env != "" {
		resolved, err := filepath.Abs(expandHome(env))
		if err != nil {
			return "", err
		}
		return resolved, nil
	}
	settingsPath := filepath.Join(defaultDir, "bootstrap.yaml")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var settings bootstrapSettings
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return "", err
	}
	if strings.TrimSpace(settings.DataDir) == "" {
		return "", nil
	}
	resolved, err := filepath.Abs(expandHome(settings.DataDir))
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

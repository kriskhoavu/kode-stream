package system

// This package owns application path configuration persistence.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type Paths struct {
	Dir                  string
	DefaultDir           string
	RegistryFile         string
	PlanIndexFile        string
	SQLiteDatabaseFile   string
	KnowledgeIndexFile   string
	AuditLogFile         string
	SavedFiltersFile     string
	RecentItemsFile      string
	AISettingsFile       string
	CanvasFile           string
	AISessionRecordsFile string
	CloneRootDir         string
	FrontendAssets       string
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
		Dir:                  dir,
		DefaultDir:           defaultDir,
		RegistryFile:         filepath.Join(dir, "workspaces.yaml"),
		PlanIndexFile:        filepath.Join(dir, "item-index.yaml"),
		SQLiteDatabaseFile:   filepath.Join(dir, "kode-stream.db"),
		KnowledgeIndexFile:   filepath.Join(dir, "knowledge-index.yaml"),
		AuditLogFile:         filepath.Join(dir, "audit-log.jsonl"),
		SavedFiltersFile:     filepath.Join(dir, "saved-filters.yaml"),
		RecentItemsFile:      filepath.Join(dir, "recent-items.yaml"),
		AISettingsFile:       filepath.Join(dir, "ai-settings.yaml"),
		CanvasFile:           filepath.Join(dir, "canvases.yaml"),
		AISessionRecordsFile: filepath.Join(dir, "ai-session-records.yaml"),
		CloneRootDir:         cloneRootDir,
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
	if DataDirEnvironmentLocked() {
		return Paths{}, errors.New("data directory is controlled by KODE_STREAM_DATA_DIR")
	}
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
		if err := updateBootstrapSetting(settingsPath, "dataDir", ""); err != nil {
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
	if err := updateBootstrapSetting(settingsPath, "dataDir", resolved); err != nil {
		return Paths{}, err
	}
	return ResolvePaths()
}

type bootstrapSettings struct {
	DataDir       string `yaml:"dataDir,omitempty"`
	StorageOption string `yaml:"storageOption,omitempty"`
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
	settings, err := readBootstrapSettings(settingsPath)
	if err != nil {
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

func ResolveStorageOptionOverride(defaultDir string) (string, error) {
	if env := strings.TrimSpace(os.Getenv("KODE_STREAM_STORAGE_OPTION")); env != "" {
		return env, nil
	}
	settings, err := readBootstrapSettings(filepath.Join(defaultDir, "bootstrap.yaml"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(settings.StorageOption), nil
}

func SetStorageOption(option string) error {
	if strings.TrimSpace(os.Getenv("KODE_STREAM_STORAGE_OPTION")) != "" || strings.TrimSpace(os.Getenv("KODE_STREAM_STORAGE_DRIVER")) != "" {
		return errors.New("storage option is controlled by environment variables")
	}
	defaultDir, err := DefaultDataDir()
	if err != nil {
		return err
	}
	settingsPath := filepath.Join(defaultDir, "bootstrap.yaml")
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		return err
	}
	return updateBootstrapSetting(settingsPath, "storageOption", strings.TrimSpace(option))
}

func DataDirEnvironmentLocked() bool {
	return strings.TrimSpace(os.Getenv("KODE_STREAM_DATA_DIR")) != ""
}

func updateBootstrapSetting(path, key, value string) error {
	var document yaml.Node
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(data) == 0 {
		document = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	} else if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("invalid bootstrap settings: %w", err)
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return errors.New("invalid bootstrap settings: expected a mapping")
	}
	mapping := document.Content[0]
	found := -1
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			found = index
			break
		}
	}
	if value == "" {
		if found >= 0 {
			mapping.Content = append(mapping.Content[:found], mapping.Content[found+2:]...)
		}
	} else if found >= 0 {
		mapping.Content[found+1].Kind = yaml.ScalarNode
		mapping.Content[found+1].Tag = "!!str"
		mapping.Content[found+1].Value = value
	} else {
		mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
	}
	if len(mapping.Content) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	encoded, err := yaml.Marshal(&document)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, encoded, 0o600)
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	return atomicWriteFileWithRename(path, data, mode, os.Rename)
}

func atomicWriteFileWithRename(path string, data []byte, mode os.FileMode, rename func(string, string) error) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".bootstrap-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := rename(temporaryPath, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func readBootstrapSettings(settingsPath string) (bootstrapSettings, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return bootstrapSettings{}, nil
		}
		return bootstrapSettings{}, err
	}
	var settings bootstrapSettings
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return bootstrapSettings{}, err
	}
	return settings, nil
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

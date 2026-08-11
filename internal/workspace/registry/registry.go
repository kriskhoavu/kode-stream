package registry

// Package registry persists registered Workspace definitions.

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/sourceguard"
	gitadapter "kode-stream/internal/git"
	appruntime "kode-stream/internal/runtime"
)

type Registry struct {
	mu      sync.RWMutex
	path    string
	git     *gitadapter.GitAdapter
	records []models.WorkspaceConfig
	loaded  bool
	hooks   persistenceHooks
}

type persistenceHooks struct {
	afterWrite         func() error
	afterSync          func() error
	afterClose         func() error
	afterRename        func() error
	afterDirectorySync func() error
}

type Repository interface {
	List() ([]models.WorkspaceConfig, error)
	Get(string) (models.WorkspaceConfig, bool, error)
	Create(models.WorkspaceInput) (models.WorkspaceConfig, error)
	Validate(models.WorkspaceInput) (models.WorkspaceConfig, error)
	Path() string
	BatchCreate([]models.WorkspaceInput) ([]BatchCreateResult, error)
	Update(string, models.WorkspaceInput) (models.WorkspaceConfig, error)
	Delete(string) error
	TouchScanned(string, time.Time) error
	SetLastSelectedBranch(string, string) error
	SetRuntime(string, *models.WorkspaceRuntimeConfig) (models.WorkspaceConfig, error)
}

type BatchCreateResult struct {
	Workspace models.WorkspaceConfig
	Err       error
}

func New(path string, git *gitadapter.GitAdapter) *Registry {
	return &Registry{path: path, git: git}
}

func (r *Registry) List() ([]models.WorkspaceConfig, error) {
	if err := r.load(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.records) == 0 {
		return []models.WorkspaceConfig{}, nil
	}
	records := append([]models.WorkspaceConfig(nil), r.records...)
	for i := range records {
		records[i] = normalizeWorkspace(records[i])
	}
	return records, nil
}

func (r *Registry) Get(id string) (models.WorkspaceConfig, bool, error) {
	if err := r.load(); err != nil {
		return models.WorkspaceConfig{}, false, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, workspace := range r.records {
		if workspace.ID == id {
			return normalizeWorkspace(workspace), true, nil
		}
	}
	return models.WorkspaceConfig{}, false, nil
}

func (r *Registry) Create(input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	if err := r.load(); err != nil {
		return models.WorkspaceConfig{}, err
	}
	workspace, err := r.validate(input)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.records {
		if samePath(existing.Path, workspace.Path) {
			return models.WorkspaceConfig{}, fmt.Errorf("workspace already registered")
		}
	}
	next := append(append([]models.WorkspaceConfig(nil), r.records...), workspace)
	if err := r.commitLocked(next); err != nil {
		return models.WorkspaceConfig{}, err
	}
	return normalizeWorkspace(workspace), nil
}

// Validate checks and normalizes a workspace without changing the registry.
func (r *Registry) Validate(input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	return r.validate(input)
}

func (r *Registry) Path() string {
	return r.path
}

// BatchCreate validates all inputs, rechecks duplicates while locked, and
// persists every accepted workspace with one atomic registry replacement.
func (r *Registry) BatchCreate(inputs []models.WorkspaceInput) ([]BatchCreateResult, error) {
	if err := r.load(); err != nil {
		return nil, err
	}
	results := make([]BatchCreateResult, len(inputs))
	validated := make([]models.WorkspaceConfig, len(inputs))
	for i, input := range inputs {
		workspace, err := r.validate(input)
		if err != nil {
			results[i].Err = err
			continue
		}
		validated[i] = workspace
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	accepted := append([]models.WorkspaceConfig(nil), r.records...)
	for i, workspace := range validated {
		if results[i].Err != nil {
			continue
		}
		duplicate := false
		for _, existing := range accepted {
			if samePath(existing.Path, workspace.Path) {
				duplicate = true
				break
			}
		}
		if duplicate {
			results[i].Err = errors.New("workspace already registered")
			continue
		}
		accepted = append(accepted, workspace)
		results[i].Workspace = normalizeWorkspace(workspace)
	}
	if len(accepted) == len(r.records) {
		return results, nil
	}
	if err := r.commitLocked(accepted); err != nil {
		return results, err
	}
	return results, nil
}

func (r *Registry) Update(id string, input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	if err := r.load(); err != nil {
		return models.WorkspaceConfig{}, err
	}
	var existing models.WorkspaceConfig
	found := false
	r.mu.RLock()
	for _, record := range r.records {
		if record.ID == id {
			existing = record
			found = true
			break
		}
	}
	r.mu.RUnlock()
	if !found {
		return models.WorkspaceConfig{}, fmt.Errorf("workspace not found")
	}
	if strings.TrimSpace(string(input.RegistrationMode)) == "" {
		input.RegistrationMode = existing.RegistrationMode
	}
	if strings.TrimSpace(input.RemoteURL) == "" {
		input.RemoteURL = existing.RemoteURL
	}
	if input.Knowledge == nil {
		input.Knowledge = existing.Knowledge
	}
	if input.Runtime == nil {
		input.Runtime = existing.Runtime
	}
	workspace, err := r.validate(input)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.records {
		if existing.ID != id && samePath(existing.Path, workspace.Path) {
			return models.WorkspaceConfig{}, fmt.Errorf("workspace already registered")
		}
	}
	for i, existing := range r.records {
		if existing.ID == id {
			workspace.ID = existing.ID
			workspace.CreatedAt = existing.CreatedAt
			workspace.LastScannedAt = existing.LastScannedAt
			workspace.LastSelectedBranch = existing.LastSelectedBranch
			if existing.ClonePathManaged {
				if !samePath(existing.Path, workspace.Path) {
					return models.WorkspaceConfig{}, errors.New("managed clone path is immutable")
				}
				workspace.ClonePathManaged = existing.ClonePathManaged
				workspace.ManagedCloneRoot = existing.ManagedCloneRoot
				workspace.ManagedCloneID = existing.ManagedCloneID
				workspace.ManagedCloneVerified = existing.ManagedCloneVerified
				workspace.ManagedCloneCleanupPending = existing.ManagedCloneCleanupPending
			}
			next := append([]models.WorkspaceConfig(nil), r.records...)
			next[i] = workspace
			if err := r.commitLocked(next); err != nil {
				return models.WorkspaceConfig{}, err
			}
			return normalizeWorkspace(workspace), nil
		}
	}
	return models.WorkspaceConfig{}, fmt.Errorf("workspace not found")
}

func (r *Registry) Delete(id string) error {
	if err := r.load(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.records {
		if r.records[i].ID == id {
			next := append([]models.WorkspaceConfig(nil), r.records[:i]...)
			next = append(next, r.records[i+1:]...)
			return r.commitLocked(next)
		}
	}
	return fmt.Errorf("workspace not found")
}

func (r *Registry) TouchScanned(id string, scannedAt time.Time) error {
	if err := r.load(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.records {
		if r.records[i].ID == id {
			next := append([]models.WorkspaceConfig(nil), r.records...)
			next[i].LastScannedAt = scannedAt
			return r.commitLocked(next)
		}
	}
	return fmt.Errorf("workspace not found")
}

func (r *Registry) SetLastSelectedBranch(id, branch string) error {
	if err := r.load(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.records {
		if r.records[i].ID == id {
			next := append([]models.WorkspaceConfig(nil), r.records...)
			next[i].LastSelectedBranch = strings.TrimSpace(branch)
			return r.commitLocked(next)
		}
	}
	return fmt.Errorf("workspace not found")
}

func (r *Registry) SetRuntime(id string, runtimeConfig *models.WorkspaceRuntimeConfig) (models.WorkspaceConfig, error) {
	if err := r.load(); err != nil {
		return models.WorkspaceConfig{}, err
	}
	normalized, err := appruntime.NormalizeRuntimeConfig(runtimeConfig)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.records {
		if r.records[i].ID == id {
			next := append([]models.WorkspaceConfig(nil), r.records...)
			next[i].Runtime = normalized
			if err := r.commitLocked(next); err != nil {
				return models.WorkspaceConfig{}, err
			}
			return normalizeWorkspace(next[i]), nil
		}
	}
	return models.WorkspaceConfig{}, fmt.Errorf("workspace not found")
}

func (r *Registry) MarkManagedCloneCleanup(id string) error {
	if err := r.load(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.records {
		if r.records[i].ID == id {
			next := append([]models.WorkspaceConfig(nil), r.records...)
			next[i].ManagedCloneCleanupPending = true
			return r.commitLocked(next)
		}
	}
	return fmt.Errorf("workspace not found")
}

func (r *Registry) validate(input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.WorkspaceConfig{}, errors.New("workspace name is required")
	}
	mode := normalizeRegistrationMode(input.RegistrationMode)
	branch := strings.TrimSpace(input.BaselineBranch)
	if branch == "" {
		branch = "main"
	}
	pathValue := strings.TrimSpace(input.Path)
	if mode != models.WorkspaceRegistrationModeRemoteClone {
		if pathValue == "" {
			return models.WorkspaceConfig{}, errors.New("workspace path is required")
		}
	} else {
		if pathValue == "" {
			return models.WorkspaceConfig{}, errors.New("cloned workspace path is required")
		}
		if strings.TrimSpace(input.RemoteURL) == "" {
			return models.WorkspaceConfig{}, errors.New("remote URL is required")
		}
	}
	path, err := filepath.Abs(expandHome(strings.TrimSpace(input.Path)))
	if err != nil || path == "" {
		return models.WorkspaceConfig{}, errors.New("workspace path is invalid")
	}
	root, err := r.git.WorkspaceRoot(path)
	if err != nil {
		return models.WorkspaceConfig{}, fmt.Errorf("not a Git workspace: %w", err)
	}
	if err := r.git.ValidateBranch(root, branch); err != nil {
		return models.WorkspaceConfig{}, fmt.Errorf("baseline branch is invalid: %w", err)
	}
	dirs := input.Sources
	if len(dirs) == 0 {
		return models.WorkspaceConfig{}, errors.New("at least one workspace source is required")
	}
	validatedSources, err := sourceguard.ResolveAll(root, dirs)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	cleanDirs := make([]string, 0, len(validatedSources))
	for _, source := range validatedSources {
		cleanDirs = append(cleanDirs, source.Relative)
	}
	jira, err := ValidateJiraConnection(input.Jira)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	runtimeConfig, err := appruntime.NormalizeRuntimeConfig(input.Runtime)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}

	return models.WorkspaceConfig{
		ID:                   slug(name) + "-" + shortHash(root),
		Name:                 name,
		Path:                 root,
		Location:             models.WorkspaceLocationLocalPath,
		BaselineBranch:       branch,
		RegistrationMode:     mode,
		RemoteURL:            strings.TrimSpace(input.RemoteURL),
		ClonePathManaged:     mode == models.WorkspaceRegistrationModeRemoteClone && input.ManagedCloneRoot != "" && input.ManagedCloneID != "",
		ManagedCloneRoot:     input.ManagedCloneRoot,
		ManagedCloneID:       input.ManagedCloneID,
		ManagedCloneVerified: mode == models.WorkspaceRegistrationModeRemoteClone && input.ManagedCloneRoot != "" && input.ManagedCloneID != "",
		Sources:              cleanDirs,
		CreatedAt:            time.Now().UTC(),
		Jira:                 jira,
		Knowledge:            normalizeKnowledgeSettings(input.Knowledge),
		Runtime:              runtimeConfig,
	}, nil
}

var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var projectKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func ValidateJiraConnection(input *models.JiraConnection) (*models.JiraConnection, error) {
	if input == nil {
		return nil, nil
	}
	connection := *input
	connection.DeploymentType = strings.ToLower(strings.TrimSpace(connection.DeploymentType))
	connection.BaseURL = strings.TrimRight(strings.TrimSpace(connection.BaseURL), "/")
	connection.ProjectKey = strings.ToUpper(strings.TrimSpace(connection.ProjectKey))
	connection.AccountEmail = strings.TrimSpace(connection.AccountEmail)
	connection.TokenEnvVar = strings.TrimSpace(connection.TokenEnvVar)
	if connection.DeploymentType != "cloud" && connection.DeploymentType != "server" {
		return nil, errors.New("Jira deployment type must be cloud or server")
	}
	parsed, err := url.Parse(connection.BaseURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("Jira base URL is invalid")
	}
	loopback := parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "::1"
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopback) {
		return nil, errors.New("Jira base URL must use HTTPS")
	}
	if !projectKeyPattern.MatchString(connection.ProjectKey) {
		return nil, errors.New("Jira project key is invalid")
	}
	if !envNamePattern.MatchString(connection.TokenEnvVar) {
		return nil, errors.New("Jira token environment variable is invalid")
	}
	if connection.DeploymentType == "cloud" && connection.AccountEmail == "" {
		return nil, errors.New("Jira Cloud account email is required")
	}
	if connection.DeploymentType == "server" {
		connection.AccountEmail = ""
	}
	return &connection, nil
}

func normalizeWorkspace(workspace models.WorkspaceConfig) models.WorkspaceConfig {
	if workspace.Sources == nil {
		workspace.Sources = []string{}
	}
	if workspace.Location != models.WorkspaceLocationCloudAgent {
		workspace.Location = models.WorkspaceLocationLocalPath
	}
	workspace.RegistrationMode = normalizeRegistrationMode(workspace.RegistrationMode)
	if workspace.RegistrationMode != models.WorkspaceRegistrationModeRemoteClone {
		workspace.RemoteURL = ""
		workspace.ClonePathManaged = false
		workspace.ManagedCloneRoot = ""
		workspace.ManagedCloneID = ""
		workspace.ManagedCloneVerified = false
	} else if workspace.ManagedCloneRoot == "" || workspace.ManagedCloneID == "" {
		// Legacy managed flags are not proof of ownership.
		workspace.ManagedCloneVerified = false
	}
	workspace.Knowledge = normalizeKnowledgeSettings(workspace.Knowledge)
	if normalized, err := appruntime.NormalizeRuntimeConfig(workspace.Runtime); err == nil {
		workspace.Runtime = normalized
	} else {
		workspace.Runtime = nil
	}
	return workspace
}

func normalizeKnowledgeSettings(settings *models.KnowledgeSettings) *models.KnowledgeSettings {
	if settings == nil {
		return nil
	}
	normalized := *settings
	normalized.EnrichExecutable = strings.TrimSpace(normalized.EnrichExecutable)
	if normalized.EnrichArgs == nil {
		normalized.EnrichArgs = []string{}
	} else {
		normalized.EnrichArgs = append([]string(nil), normalized.EnrichArgs...)
	}
	return &normalized
}

func normalizeRegistrationMode(mode models.WorkspaceRegistrationMode) models.WorkspaceRegistrationMode {
	switch strings.TrimSpace(string(mode)) {
	case string(models.WorkspaceRegistrationModeRemoteClone):
		return models.WorkspaceRegistrationModeRemoteClone
	case string(models.WorkspaceRegistrationModeExisting):
		return models.WorkspaceRegistrationModeExisting
	default:
		return models.WorkspaceRegistrationModeLocalPath
	}
}

func (r *Registry) load() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loaded {
		return nil
	}
	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		r.records = []models.WorkspaceConfig{}
		r.loaded = true
		return nil
	}
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, &r.records); err != nil {
		return err
	}
	r.loaded = true
	return nil
}

func (r *Registry) commitLocked(records []models.WorkspaceConfig) error {
	if err := r.saveRecordsLocked(records); err != nil {
		return err
	}
	r.records = records
	return nil
}

func (r *Registry) saveRecordsLocked(records []models.WorkspaceConfig) error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(records)
	if err != nil {
		return err
	}
	previous, readErr := os.ReadFile(r.path)
	previousMode := os.FileMode(0o600)
	previousExists := readErr == nil
	if previousExists {
		if info, statErr := os.Stat(r.path); statErr == nil {
			previousMode = info.Mode().Perm()
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	temporary, err := os.CreateTemp(filepath.Dir(r.path), ".workspaces-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if r.hooks.afterWrite != nil {
		if err := r.hooks.afterWrite(); err != nil {
			_ = temporary.Close()
			return err
		}
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if r.hooks.afterSync != nil {
		if err := r.hooks.afterSync(); err != nil {
			_ = temporary.Close()
			return err
		}
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if r.hooks.afterClose != nil {
		if err := r.hooks.afterClose(); err != nil {
			return err
		}
	}
	if err := os.Rename(temporaryPath, r.path); err != nil {
		return err
	}
	rollback := func(cause error) error {
		if restoreErr := restoreRegistryGeneration(r.path, previous, previousMode, previousExists); restoreErr != nil {
			return fmt.Errorf("registry persistence failed: %v; previous generation restoration failed: %w", cause, restoreErr)
		}
		return cause
	}
	if r.hooks.afterRename != nil {
		if err := r.hooks.afterRename(); err != nil {
			return rollback(err)
		}
	}
	directory, err := os.Open(filepath.Dir(r.path))
	if err != nil {
		return rollback(err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return rollback(err)
	}
	if r.hooks.afterDirectorySync != nil {
		if err := r.hooks.afterDirectorySync(); err != nil {
			_ = directory.Close()
			return rollback(err)
		}
	}
	if err := directory.Close(); err != nil {
		return rollback(err)
	}
	return nil
}

func restoreRegistryGeneration(path string, data []byte, mode os.FileMode, existed bool) error {
	if !existed {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else {
		temporary, err := os.CreateTemp(filepath.Dir(path), ".workspaces-restore-*.tmp")
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
		if err := os.Rename(temporaryPath, path); err != nil {
			return err
		}
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func samePath(a, b string) bool {
	ar, _ := filepath.EvalSymlinks(a)
	br, _ := filepath.EvalSymlinks(b)
	return ar == br
}

func slug(s string) string {
	re := regexp.MustCompile(`[^a-z0-9]+`)
	out := strings.Trim(re.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if out == "" {
		return "workspace"
	}
	return out
}

func shortHash(s string) string {
	var h uint32 = 2166136261
	for _, b := range []byte(s) {
		h ^= uint32(b)
		h *= 16777619
	}
	return fmt.Sprintf("%08x", h)
}

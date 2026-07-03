package registry

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
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/models"
)

type Registry struct {
	mu      sync.RWMutex
	path    string
	git     *gitadapter.GitAdapter
	records []models.WorkspaceConfig
	loaded  bool
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
	r.records = append(r.records, workspace)
	return normalizeWorkspace(workspace), r.saveLocked()
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
			r.records[i] = workspace
			return normalizeWorkspace(workspace), r.saveLocked()
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
			r.records = append(r.records[:i], r.records[i+1:]...)
			return r.saveLocked()
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
			r.records[i].LastScannedAt = scannedAt
			return r.saveLocked()
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
			r.records[i].LastSelectedBranch = strings.TrimSpace(branch)
			return r.saveLocked()
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
	if mode == models.WorkspaceRegistrationModeLocalPath {
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
	cleanDirs := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		clean := filepath.Clean(strings.TrimSpace(dir))
		if clean == "." || clean == "" || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return models.WorkspaceConfig{}, fmt.Errorf("source %q must be relative", dir)
		}
		full := filepath.Join(root, clean)
		stat, err := os.Stat(full)
		if err != nil || !stat.IsDir() {
			return models.WorkspaceConfig{}, fmt.Errorf("source %q does not exist", clean)
		}
		cleanDirs = append(cleanDirs, filepath.ToSlash(clean))
	}
	jira, err := ValidateJiraConnection(input.Jira)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}

	return models.WorkspaceConfig{
		ID:               slug(name) + "-" + shortHash(root),
		Name:             name,
		Path:             root,
		BaselineBranch:   branch,
		RegistrationMode: mode,
		RemoteURL:        strings.TrimSpace(input.RemoteURL),
		ClonePathManaged: mode == models.WorkspaceRegistrationModeRemoteClone,
		Sources:          cleanDirs,
		CreatedAt:        time.Now().UTC(),
		Jira:             jira,
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
	workspace.RegistrationMode = normalizeRegistrationMode(workspace.RegistrationMode)
	if workspace.RegistrationMode != models.WorkspaceRegistrationModeRemoteClone {
		workspace.RemoteURL = ""
		workspace.ClonePathManaged = false
	}
	return workspace
}

func normalizeRegistrationMode(mode models.WorkspaceRegistrationMode) models.WorkspaceRegistrationMode {
	if strings.TrimSpace(string(mode)) == string(models.WorkspaceRegistrationModeRemoteClone) {
		return models.WorkspaceRegistrationModeRemoteClone
	}
	return models.WorkspaceRegistrationModeLocalPath
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
	var legacy []struct {
		ID              string    `yaml:"id"`
		Name            string    `yaml:"name"`
		Path            string    `yaml:"path"`
		BaselineBranch  string    `yaml:"baselineBranch"`
		PlanDirectories []string  `yaml:"planDirectories"`
		CreatedAt       time.Time `yaml:"createdAt"`
		LastScannedAt   time.Time `yaml:"lastScannedAt,omitempty"`
	}
	if err := yaml.Unmarshal(data, &legacy); err == nil {
		for i := range r.records {
			if len(r.records[i].Sources) == 0 && i < len(legacy) {
				r.records[i].Sources = legacy[i].PlanDirectories
			}
		}
	}
	r.loaded = true
	return nil
}

func (r *Registry) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(r.records)
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o600)
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

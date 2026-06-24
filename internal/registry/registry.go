package registry

import (
	"errors"
	"fmt"
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
		if records[i].Sources == nil {
			records[i].Sources = []string{}
		}
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
			return workspace, true, nil
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
	return workspace, r.saveLocked()
}

func (r *Registry) Update(id string, input models.WorkspaceInput) (models.WorkspaceConfig, error) {
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
			return workspace, r.saveLocked()
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
	branch := strings.TrimSpace(input.BaselineBranch)
	if branch == "" {
		branch = "main"
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

	return models.WorkspaceConfig{
		ID:             slug(name) + "-" + shortHash(root),
		Name:           name,
		Path:           root,
		BaselineBranch: branch,
		Sources:        cleanDirs,
		CreatedAt:      time.Now().UTC(),
	}, nil
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

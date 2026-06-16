package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"plan-manager/internal/gitadapter"
	"plan-manager/internal/models"
)

type Registry struct {
	mu      sync.RWMutex
	path    string
	git     *gitadapter.GitAdapter
	records []models.RepositoryConfig
	loaded  bool
}

func New(path string, git *gitadapter.GitAdapter) *Registry {
	return &Registry{path: path, git: git}
}

func (r *Registry) List() ([]models.RepositoryConfig, error) {
	if err := r.load(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.records) == 0 {
		return []models.RepositoryConfig{}, nil
	}
	records := append([]models.RepositoryConfig(nil), r.records...)
	for i := range records {
		if records[i].PlanDirectories == nil {
			records[i].PlanDirectories = []string{}
		}
	}
	return records, nil
}

func (r *Registry) Get(id string) (models.RepositoryConfig, bool, error) {
	if err := r.load(); err != nil {
		return models.RepositoryConfig{}, false, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, repo := range r.records {
		if repo.ID == id {
			return repo, true, nil
		}
	}
	return models.RepositoryConfig{}, false, nil
}

func (r *Registry) Create(input models.RepositoryInput) (models.RepositoryConfig, error) {
	if err := r.load(); err != nil {
		return models.RepositoryConfig{}, err
	}
	repo, err := r.validate(input)
	if err != nil {
		return models.RepositoryConfig{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.records {
		if samePath(existing.Path, repo.Path) {
			return models.RepositoryConfig{}, fmt.Errorf("repository already registered")
		}
	}
	r.records = append(r.records, repo)
	return repo, r.saveLocked()
}

func (r *Registry) Update(id string, input models.RepositoryInput) (models.RepositoryConfig, error) {
	if err := r.load(); err != nil {
		return models.RepositoryConfig{}, err
	}
	repo, err := r.validate(input)
	if err != nil {
		return models.RepositoryConfig{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.records {
		if existing.ID != id && samePath(existing.Path, repo.Path) {
			return models.RepositoryConfig{}, fmt.Errorf("repository already registered")
		}
	}
	for i, existing := range r.records {
		if existing.ID == id {
			repo.ID = existing.ID
			repo.CreatedAt = existing.CreatedAt
			repo.LastScannedAt = existing.LastScannedAt
			r.records[i] = repo
			return repo, r.saveLocked()
		}
	}
	return models.RepositoryConfig{}, fmt.Errorf("repository not found")
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
	return fmt.Errorf("repository not found")
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
	return fmt.Errorf("repository not found")
}

func (r *Registry) validate(input models.RepositoryInput) (models.RepositoryConfig, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.RepositoryConfig{}, errors.New("repository name is required")
	}
	branch := strings.TrimSpace(input.BaselineBranch)
	if branch == "" {
		branch = "main"
	}
	path, err := filepath.Abs(expandHome(strings.TrimSpace(input.Path)))
	if err != nil || path == "" {
		return models.RepositoryConfig{}, errors.New("repository path is invalid")
	}
	root, err := r.git.RepositoryRoot(path)
	if err != nil {
		return models.RepositoryConfig{}, fmt.Errorf("not a Git repository: %w", err)
	}
	if err := r.git.ValidateBranch(root, branch); err != nil {
		return models.RepositoryConfig{}, fmt.Errorf("baseline branch is invalid: %w", err)
	}
	dirs := input.PlanDirectories
	if len(dirs) == 0 {
		dirs = []string{"plans"}
	}
	cleanDirs := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		clean := filepath.Clean(strings.TrimSpace(dir))
		if clean == "." || clean == "" || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return models.RepositoryConfig{}, fmt.Errorf("plan directory %q must be relative", dir)
		}
		full := filepath.Join(root, clean)
		stat, err := os.Stat(full)
		if err != nil || !stat.IsDir() {
			return models.RepositoryConfig{}, fmt.Errorf("plan directory %q does not exist", clean)
		}
		cleanDirs = append(cleanDirs, filepath.ToSlash(clean))
	}

	return models.RepositoryConfig{
		ID:              slug(name) + "-" + shortHash(root),
		Name:            name,
		Path:            root,
		BaselineBranch:  branch,
		PlanDirectories: cleanDirs,
		CreatedAt:       time.Now().UTC(),
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
		r.records = []models.RepositoryConfig{}
		r.loaded = true
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &r.records); err != nil {
		return err
	}
	r.loaded = true
	return nil
}

func (r *Registry) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r.records, "", "  ")
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
		return "repository"
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

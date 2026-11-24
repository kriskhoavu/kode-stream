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

	"kode-stream/internal/common/models"
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

func (r *Registry) Validate(input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	return r.validate(input)
}

func (r *Registry) Path() string {
	return r.path
}

// Duplicates are checked again under the lock. One atomic registry replacement stores the batch.
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
	preserveRuntime := input.Runtime == nil
	workspace, err := r.validate(input)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	if preserveRuntime {
		workspace.Runtime = existing.Runtime
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

package planindex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"plan-manager/internal/models"
)

type Index struct {
	mu     sync.RWMutex
	path   string
	loaded bool
	state  state
}

type state struct {
	Plans    []models.PlanDetail  `json:"plans"`
	Warnings []models.ScanWarning `json:"warnings"`
	Scans    map[string]time.Time `json:"scans"`
}

type Query struct {
	RepositoryID string
	Branch       string
	Status       string
	Text         string
}

func New(path string) *Index {
	return &Index{path: path}
}

func (i *Index) ReplaceRepository(repositoryID string, plans []models.PlanDetail, warnings []models.ScanWarning, scannedAt time.Time) error {
	if err := i.load(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	next := i.state.Plans[:0]
	for _, plan := range i.state.Plans {
		if plan.RepositoryID != repositoryID {
			next = append(next, plan)
		}
	}
	i.state.Plans = append(next, plans...)
	nextWarnings := i.state.Warnings[:0]
	for _, warning := range i.state.Warnings {
		if !strings.HasPrefix(warning.PlanPath, repositoryID+":") {
			nextWarnings = append(nextWarnings, warning)
		}
	}
	for _, warning := range warnings {
		warning.PlanPath = repositoryID + ":" + warning.PlanPath
		nextWarnings = append(nextWarnings, warning)
	}
	i.state.Warnings = nextWarnings
	if i.state.Scans == nil {
		i.state.Scans = map[string]time.Time{}
	}
	i.state.Scans[repositoryID] = scannedAt
	return i.saveLocked()
}

func (i *Index) DeleteRepository(repositoryID string) error {
	if err := i.load(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	next := i.state.Plans[:0]
	for _, plan := range i.state.Plans {
		if plan.RepositoryID != repositoryID {
			next = append(next, plan)
		}
	}
	i.state.Plans = next
	nextWarnings := i.state.Warnings[:0]
	for _, warning := range i.state.Warnings {
		if !strings.HasPrefix(warning.PlanPath, repositoryID+":") {
			nextWarnings = append(nextWarnings, warning)
		}
	}
	i.state.Warnings = nextWarnings
	delete(i.state.Scans, repositoryID)
	return i.saveLocked()
}

func (i *Index) Query(q Query) ([]models.PlanSummary, error) {
	if err := i.load(); err != nil {
		return nil, err
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	text := strings.ToLower(strings.TrimSpace(q.Text))
	out := make([]models.PlanSummary, 0, len(i.state.Plans))
	for _, detail := range i.state.Plans {
		if q.RepositoryID != "" && detail.RepositoryID != q.RepositoryID {
			continue
		}
		if q.Branch != "" && detail.Branch != q.Branch {
			continue
		}
		if q.Status != "" && string(detail.Status) != q.Status {
			continue
		}
		if text != "" && !matchesText(detail.PlanSummary, text) {
			continue
		}
		if detail.Tags == nil {
			detail.Tags = []string{}
		}
		out = append(out, detail.PlanSummary)
	}
	sort.Slice(out, func(a, b int) bool {
		return out[a].UpdatedAt.After(out[b].UpdatedAt)
	})
	return out, nil
}

func (i *Index) Get(id string) (models.PlanDetail, bool, error) {
	if err := i.load(); err != nil {
		return models.PlanDetail{}, false, err
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	for _, plan := range i.state.Plans {
		if plan.ID == id {
			return plan, true, nil
		}
	}
	return models.PlanDetail{}, false, nil
}

func matchesText(plan models.PlanSummary, text string) bool {
	haystack := strings.ToLower(strings.Join([]string{
		plan.Title, plan.Ticket, plan.Service, plan.Description, plan.Author, strings.Join(plan.Tags, " "),
	}, " "))
	return strings.Contains(haystack, text)
}

func (i *Index) load() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.loaded {
		return nil
	}
	data, err := os.ReadFile(i.path)
	if errors.Is(err, os.ErrNotExist) {
		i.state = state{Plans: []models.PlanDetail{}, Warnings: []models.ScanWarning{}, Scans: map[string]time.Time{}}
		i.loaded = true
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &i.state); err != nil {
		return err
	}
	if i.state.Scans == nil {
		i.state.Scans = map[string]time.Time{}
	}
	i.loaded = true
	return nil
}

func (i *Index) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(i.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(i.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(i.path, data, 0o600)
}

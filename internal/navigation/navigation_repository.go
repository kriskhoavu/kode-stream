package navigation

// This package owns persisted navigation state.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"kode-stream/internal/common/models"
)

type NavigationRepository struct {
	mu          sync.Mutex
	filtersPath string
	recentsPath string
	now         func() time.Time
}

type Store = NavigationRepository

func New(filtersPath, recentsPath string) *NavigationRepository {
	return &NavigationRepository{filtersPath: filtersPath, recentsPath: recentsPath, now: time.Now}
}

func (s *Store) Filters() ([]models.SavedFilter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filters := []models.SavedFilter{}
	if err := readYAML(s.filtersPath, &filters); err != nil {
		return nil, err
	}
	normalizeFilters(filters)
	return filters, nil
}

func (s *Store) SaveFilter(filter models.SavedFilter) (models.SavedFilter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filters := []models.SavedFilter{}
	if err := readYAML(s.filtersPath, &filters); err != nil {
		return models.SavedFilter{}, err
	}
	now := s.now().UTC()
	if filter.ID == "" {
		filter.ID = newID()
		filter.CreatedAt = now
	}
	if filter.CreatedAt.IsZero() {
		filter.CreatedAt = now
	}
	filter.UpdatedAt = now
	if filter.Filters == nil {
		filter.Filters = map[string]any{}
	}
	found := false
	for index := range filters {
		if filters[index].ID == filter.ID {
			filter.CreatedAt = filters[index].CreatedAt
			filters[index] = filter
			found = true
			break
		}
	}
	if !found {
		filters = append(filters, filter)
	}
	normalizeFilters(filters)
	return filter, writeYAML(s.filtersPath, filters)
}

func (s *Store) DeleteFilter(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filters := []models.SavedFilter{}
	if err := readYAML(s.filtersPath, &filters); err != nil {
		return false, err
	}
	next := make([]models.SavedFilter, 0, len(filters))
	found := false
	for _, filter := range filters {
		if filter.ID == id {
			found = true
			continue
		}
		next = append(next, filter)
	}
	if !found {
		return false, nil
	}
	return true, writeYAML(s.filtersPath, next)
}

func (s *Store) Recents(limit int) ([]models.RecentItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	recents := []models.RecentItem{}
	if err := readYAML(s.recentsPath, &recents); err != nil {
		return nil, err
	}
	sortRecents(recents)
	if limit > 0 && len(recents) > limit {
		recents = recents[:limit]
	}
	return recents, nil
}

func (s *Store) RecordRecent(item models.RecentItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	recents := []models.RecentItem{}
	if err := readYAML(s.recentsPath, &recents); err != nil {
		return err
	}
	item.OpenedAt = s.now().UTC()
	next := []models.RecentItem{item}
	for _, recent := range recents {
		if recent.ItemID != item.ItemID {
			next = append(next, recent)
		}
	}
	if len(next) > 50 {
		next = next[:50]
	}
	return writeYAML(s.recentsPath, next)
}

func readYAML(path string, target any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, target)
}

func writeYAML(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func normalizeFilters(filters []models.SavedFilter) {
	for index := range filters {
		if filters[index].Filters == nil {
			filters[index].Filters = map[string]any{}
		}
	}
	sort.SliceStable(filters, func(i, j int) bool { return filters[i].UpdatedAt.After(filters[j].UpdatedAt) })
}

func sortRecents(recents []models.RecentItem) {
	sort.SliceStable(recents, func(i, j int) bool { return recents[i].OpenedAt.After(recents[j].OpenedAt) })
}

func newID() string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return time.Now().UTC().Format("20060102150405.000000000")
}

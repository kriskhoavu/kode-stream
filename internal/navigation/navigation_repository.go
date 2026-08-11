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
	write       func(string, any) error
}

type Repository interface {
	Filters() ([]models.SavedFilter, error)
	SaveFilter(models.SavedFilter) (models.SavedFilter, error)
	DeleteFilter(string) (bool, error)
	Recents(int) ([]models.RecentItem, error)
	RecordRecent(models.RecentItem) error
}

// OwnedRepository is the Cloud-safe navigation contract. The legacy methods remain
// available for local callers and snapshot tooling; HTTP controllers always use this
// owner-scoped seam when it is implemented.
type OwnedRepository interface {
	Repository
	FiltersForOwner(string) ([]models.SavedFilter, error)
	SaveFilterForOwner(string, models.SavedFilter) (models.SavedFilter, error)
	DeleteFilterForOwner(string, string) (bool, error)
	RecentsForOwner(string, int) ([]models.RecentItem, error)
	RecordRecentForOwner(string, models.RecentItem) error
}

// SnapshotRepository restores immutable persisted records without applying
// user-mutation clocks. Storage migration and sync use this seam so IDs,
// ownership, and timestamps round-trip unchanged.
type SnapshotRepository interface {
	OwnedRepository
	RestoreFilter(models.SavedFilter) error
	RestoreRecent(models.RecentItem) error
}

const LocalOwner = "local"

type Store = NavigationRepository

func New(filtersPath, recentsPath string) *NavigationRepository {
	return &NavigationRepository{filtersPath: filtersPath, recentsPath: recentsPath, now: time.Now, write: writeYAML}
}

func (s *Store) Filters() ([]models.SavedFilter, error) {
	return s.filtersForOwner("")
}

func (s *Store) FiltersForOwner(owner string) ([]models.SavedFilter, error) {
	return s.filtersForOwner(normalizeOwner(owner))
}

func (s *Store) filtersForOwner(owner string) ([]models.SavedFilter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filters := []models.SavedFilter{}
	if err := readYAML(s.filtersPath, &filters); err != nil {
		return nil, err
	}
	normalizeFilters(filters)
	if owner != "" {
		filtered := make([]models.SavedFilter, 0, len(filters))
		for _, filter := range filters {
			if normalizeOwner(filter.OwnerUserID) == owner {
				filter.OwnerUserID = owner
				filtered = append(filtered, filter)
			}
		}
		filters = filtered
	}
	return filters, nil
}

func (s *Store) SaveFilter(filter models.SavedFilter) (models.SavedFilter, error) {
	return s.saveFilterForOwner("", filter)
}

func (s *Store) SaveFilterForOwner(owner string, filter models.SavedFilter) (models.SavedFilter, error) {
	return s.saveFilterForOwner(normalizeOwner(owner), filter)
}

func (s *Store) saveFilterForOwner(owner string, filter models.SavedFilter) (models.SavedFilter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filters := []models.SavedFilter{}
	if err := readYAML(s.filtersPath, &filters); err != nil {
		return models.SavedFilter{}, err
	}
	now := s.now().UTC()
	if owner != "" {
		filter.OwnerUserID = owner
	}
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
		if filters[index].ID == filter.ID && (owner == "" || normalizeOwner(filters[index].OwnerUserID) == owner) {
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
	return filter, s.write(s.filtersPath, filters)
}

func (s *Store) DeleteFilter(id string) (bool, error) {
	return s.deleteFilterForOwner("", id)
}

func (s *Store) DeleteFilterForOwner(owner, id string) (bool, error) {
	return s.deleteFilterForOwner(normalizeOwner(owner), id)
}

func (s *Store) deleteFilterForOwner(owner, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filters := []models.SavedFilter{}
	if err := readYAML(s.filtersPath, &filters); err != nil {
		return false, err
	}
	next := make([]models.SavedFilter, 0, len(filters))
	found := false
	for _, filter := range filters {
		if filter.ID == id && (owner == "" || normalizeOwner(filter.OwnerUserID) == owner) {
			found = true
			continue
		}
		next = append(next, filter)
	}
	if !found {
		return false, nil
	}
	return true, s.write(s.filtersPath, next)
}

func (s *Store) Recents(limit int) ([]models.RecentItem, error) {
	return s.recentsForOwner("", limit)
}

func (s *Store) RecentsForOwner(owner string, limit int) ([]models.RecentItem, error) {
	return s.recentsForOwner(normalizeOwner(owner), limit)
}

func (s *Store) recentsForOwner(owner string, limit int) ([]models.RecentItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	recents := []models.RecentItem{}
	if err := readYAML(s.recentsPath, &recents); err != nil {
		return nil, err
	}
	if owner != "" {
		filtered := make([]models.RecentItem, 0, len(recents))
		for _, recent := range recents {
			if normalizeOwner(recent.OwnerUserID) == owner {
				recent.OwnerUserID = owner
				filtered = append(filtered, recent)
			}
		}
		recents = filtered
	}
	sortRecents(recents)
	if limit > 0 && len(recents) > limit {
		recents = recents[:limit]
	}
	return recents, nil
}

func (s *Store) RecordRecent(item models.RecentItem) error {
	return s.recordRecentForOwner("", item)
}

func (s *Store) RecordRecentForOwner(owner string, item models.RecentItem) error {
	return s.recordRecentForOwner(normalizeOwner(owner), item)
}

func (s *Store) recordRecentForOwner(owner string, item models.RecentItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	recents := []models.RecentItem{}
	if err := readYAML(s.recentsPath, &recents); err != nil {
		return err
	}
	item.OpenedAt = s.now().UTC()
	if owner != "" {
		item.OwnerUserID = owner
	}
	next := []models.RecentItem{item}
	for _, recent := range recents {
		if recent.ItemID != item.ItemID || (owner != "" && normalizeOwner(recent.OwnerUserID) != owner) {
			next = append(next, recent)
		}
	}
	if owner != "" {
		ownedCount := 0
		pruned := next[:0]
		for _, recent := range next {
			if normalizeOwner(recent.OwnerUserID) == owner {
				ownedCount++
				if ownedCount > 50 {
					continue
				}
			}
			pruned = append(pruned, recent)
		}
		next = pruned
	} else if len(next) > 50 {
		next = next[:50]
	}
	return s.write(s.recentsPath, next)
}

func (s *Store) RestoreFilter(filter models.SavedFilter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	filters := []models.SavedFilter{}
	if err := readYAML(s.filtersPath, &filters); err != nil {
		return err
	}
	for index := range filters {
		if filters[index].ID == filter.ID && normalizeOwner(filters[index].OwnerUserID) == normalizeOwner(filter.OwnerUserID) {
			filters[index] = filter
			normalizeFilters(filters)
			return s.write(s.filtersPath, filters)
		}
	}
	filters = append(filters, filter)
	normalizeFilters(filters)
	return s.write(s.filtersPath, filters)
}

func (s *Store) RestoreRecent(item models.RecentItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	recents := []models.RecentItem{}
	if err := readYAML(s.recentsPath, &recents); err != nil {
		return err
	}
	next := []models.RecentItem{item}
	for _, recent := range recents {
		if recent.ItemID != item.ItemID || normalizeOwner(recent.OwnerUserID) != normalizeOwner(item.OwnerUserID) {
			next = append(next, recent)
		}
	}
	sortRecents(next)
	return s.write(s.recentsPath, next)
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
	temporary, err := os.CreateTemp(filepath.Dir(path), ".navigation-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
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
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func normalizeOwner(owner string) string {
	if owner == "" {
		return LocalOwner
	}
	return owner
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

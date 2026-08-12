package knowledge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

const indexSchemaVersion = 1

type persistedIndex struct {
	Version int             `yaml:"version"`
	Wikis   []KnowledgeWiki `yaml:"wikis"`
}

type Store struct {
	mu    sync.RWMutex
	path  string
	hooks storeHooks
}

type storeHooks struct {
	afterWrite         func() error
	afterSync          func() error
	afterClose         func() error
	afterRename        func() error
	afterDirectorySync func() error
}

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) List(workspaceID string) ([]KnowledgeWiki, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	index, err := s.load()
	if err != nil {
		return nil, err
	}
	result := make([]KnowledgeWiki, 0)
	for _, wiki := range index.Wikis {
		if workspaceID == "" || wiki.WorkspaceID == workspaceID {
			result = append(result, wiki)
		}
	}
	if result == nil {
		result = []KnowledgeWiki{}
	}
	return result, nil
}

func (s *Store) ReplaceWorkspace(workspaceID string, wikis []KnowledgeWiki) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, err := s.load()
	if err != nil {
		return err
	}
	replacement := make([]KnowledgeWiki, 0, len(index.Wikis)+len(wikis))
	for _, wiki := range index.Wikis {
		if wiki.WorkspaceID != workspaceID {
			replacement = append(replacement, wiki)
		}
	}
	for _, wiki := range wikis {
		wiki.WorkspaceID = workspaceID
		replacement = append(replacement, wiki)
	}
	return s.save(persistedIndex{Version: indexSchemaVersion, Wikis: sortWikis(replacement)})
}

func (s *Store) ReplaceWiki(workspaceID, root string, wiki KnowledgeWiki) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, err := s.load()
	if err != nil {
		return err
	}
	replacement := make([]KnowledgeWiki, 0, len(index.Wikis)+1)
	for _, existing := range index.Wikis {
		if existing.WorkspaceID != workspaceID || existing.Root != root {
			replacement = append(replacement, existing)
		}
	}
	wiki.WorkspaceID, wiki.Root = workspaceID, root
	replacement = append(replacement, wiki)
	return s.save(persistedIndex{Version: indexSchemaVersion, Wikis: sortWikis(replacement)})
}

func (s *Store) load() (persistedIndex, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return persistedIndex{Version: indexSchemaVersion, Wikis: []KnowledgeWiki{}}, nil
	}
	if err != nil {
		return persistedIndex{}, err
	}
	var index persistedIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		return persistedIndex{}, err
	}
	if index.Version != indexSchemaVersion {
		return persistedIndex{}, errors.New("unsupported knowledge index version")
	}
	if index.Wikis == nil {
		index.Wikis = []KnowledgeWiki{}
	}
	return index, nil
}

func (s *Store) save(index persistedIndex) error {
	data, err := yaml.Marshal(index)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	directory := filepath.Dir(s.path)
	mode := os.FileMode(0o600)
	previous, readErr := os.ReadFile(s.path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	hadPrevious := readErr == nil
	if info, statErr := os.Stat(s.path); statErr == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	temporary, err := os.CreateTemp(directory, ".knowledge-index-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if s.hooks.afterWrite != nil {
		if err := s.hooks.afterWrite(); err != nil {
			_ = temporary.Close()
			return err
		}
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if s.hooks.afterSync != nil {
		if err := s.hooks.afterSync(); err != nil {
			_ = temporary.Close()
			return err
		}
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if s.hooks.afterClose != nil {
		if err := s.hooks.afterClose(); err != nil {
			return err
		}
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return err
	}
	rollback := func(cause error) error {
		if !hadPrevious {
			if restoreErr := removeIndex(s.path); restoreErr != nil {
				return fmt.Errorf("knowledge index publication failed: %v; absent-state restoration failed: %w", cause, restoreErr)
			}
			return cause
		}
		if restoreErr := restoreIndex(s.path, previous, mode); restoreErr != nil {
			return fmt.Errorf("knowledge index publication failed: %v; original restoration failed: %w", cause, restoreErr)
		}
		return cause
	}
	if s.hooks.afterRename != nil {
		if err := s.hooks.afterRename(); err != nil {
			return rollback(err)
		}
	}
	parent, err := os.Open(directory)
	if err != nil {
		return rollback(err)
	}
	defer parent.Close()
	if err := parent.Sync(); err != nil {
		return rollback(err)
	}
	if s.hooks.afterDirectorySync != nil {
		if err := s.hooks.afterDirectorySync(); err != nil {
			return rollback(err)
		}
	}
	return nil
}

func removeIndex(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Sync()
}

func restoreIndex(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".knowledge-index-restore-*.tmp")
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
	parent, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Sync()
}

func sortWikis(wikis []KnowledgeWiki) []KnowledgeWiki {
	sort.Slice(wikis, func(i, j int) bool {
		if wikis[i].WorkspaceID == wikis[j].WorkspaceID {
			return wikis[i].Root < wikis[j].Root
		}
		return wikis[i].WorkspaceID < wikis[j].WorkspaceID
	})
	return wikis
}

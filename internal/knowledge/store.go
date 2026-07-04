package knowledge

import (
	"errors"
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
	mu   sync.RWMutex
	path string
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
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".knowledge-index-*.tmp")
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
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, s.path)
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

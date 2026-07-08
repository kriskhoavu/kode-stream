package itemindex

// Package itemindex persists the Item domain read model.

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"kode-stream/internal/common/models"
)

type Index struct {
	mu     sync.RWMutex
	path   string
	loaded bool
	state  state
}

type state struct {
	Items       []models.ItemDetail                             `json:"items" yaml:"items"`
	Warnings    []models.ScanWarning                            `json:"warnings" yaml:"warnings"`
	Scans       map[string]time.Time                            `json:"scans" yaml:"scans"`
	BranchScans map[string]map[string]models.BranchScanMetadata `json:"branchScans" yaml:"branchScans"`
}

type Query struct {
	WorkspaceID string
	Branch      string
	Status      string
	Text        string
}

func New(path string) *Index {
	return &Index{path: path}
}

func (i *Index) ReplaceWorkspace(workspaceID string, items []models.ItemDetail, warnings []models.ScanWarning, scannedAt time.Time) error {
	if err := i.load(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	next := i.state.Items[:0]
	for _, item := range i.state.Items {
		if item.WorkspaceID != workspaceID {
			next = append(next, item)
		}
	}
	i.state.Items = append(next, items...)
	nextWarnings := i.state.Warnings[:0]
	for _, warning := range i.state.Warnings {
		if !strings.HasPrefix(warning.ItemPath, workspaceID+":") {
			nextWarnings = append(nextWarnings, warning)
		}
	}
	for _, warning := range warnings {
		warning.ItemPath = workspaceID + ":" + warning.ItemPath
		nextWarnings = append(nextWarnings, warning)
	}
	i.state.Warnings = nextWarnings
	if i.state.Scans == nil {
		i.state.Scans = map[string]time.Time{}
	}
	i.state.Scans[workspaceID] = scannedAt
	if i.state.BranchScans == nil {
		i.state.BranchScans = map[string]map[string]models.BranchScanMetadata{}
	}
	delete(i.state.BranchScans, workspaceID)
	return i.saveLocked()
}

func (i *Index) ReplaceWorkspaceBranch(workspaceID, branch string, items []models.ItemDetail, metadata models.BranchScanMetadata) error {
	if err := i.load(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	next := i.state.Items[:0]
	for _, item := range i.state.Items {
		if item.WorkspaceID == workspaceID && item.Branch == branch {
			continue
		}
		next = append(next, item)
	}
	i.state.Items = append(next, items...)
	nextWarnings := i.state.Warnings[:0]
	prefix := workspaceID + ":" + branch + ":"
	for _, warning := range i.state.Warnings {
		if !strings.HasPrefix(warning.ItemPath, prefix) {
			nextWarnings = append(nextWarnings, warning)
		}
	}
	for _, warning := range metadata.Warnings {
		warning.ItemPath = prefix + warning.ItemPath
		nextWarnings = append(nextWarnings, warning)
	}
	i.state.Warnings = nextWarnings
	if i.state.Scans == nil {
		i.state.Scans = map[string]time.Time{}
	}
	i.state.Scans[workspaceID] = metadata.ScannedAt
	if i.state.BranchScans == nil {
		i.state.BranchScans = map[string]map[string]models.BranchScanMetadata{}
	}
	if i.state.BranchScans[workspaceID] == nil {
		i.state.BranchScans[workspaceID] = map[string]models.BranchScanMetadata{}
	}
	metadata.WorkspaceID = workspaceID
	metadata.Branch = branch
	i.state.BranchScans[workspaceID][branch] = metadata
	return i.saveLocked()
}

func (i *Index) DeleteWorkspace(workspaceID string) error {
	if err := i.load(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	next := i.state.Items[:0]
	for _, item := range i.state.Items {
		if item.WorkspaceID != workspaceID {
			next = append(next, item)
		}
	}
	i.state.Items = next
	nextWarnings := i.state.Warnings[:0]
	for _, warning := range i.state.Warnings {
		if !strings.HasPrefix(warning.ItemPath, workspaceID+":") {
			nextWarnings = append(nextWarnings, warning)
		}
	}
	i.state.Warnings = nextWarnings
	delete(i.state.Scans, workspaceID)
	delete(i.state.BranchScans, workspaceID)
	return i.saveLocked()
}

func (i *Index) Query(q Query) ([]models.ItemSummary, error) {
	if err := i.load(); err != nil {
		return nil, err
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	text := strings.ToLower(strings.TrimSpace(q.Text))
	out := make([]models.ItemSummary, 0, len(i.state.Items))
	for _, detail := range i.state.Items {
		if q.WorkspaceID != "" && detail.WorkspaceID != q.WorkspaceID {
			continue
		}
		if q.Branch != "" && detail.Branch != q.Branch {
			continue
		}
		if q.Status != "" && string(detail.Status) != q.Status {
			continue
		}
		if text != "" && !matchesText(detail.ItemSummary, text) {
			continue
		}
		if detail.Tags == nil {
			detail.Tags = []string{}
		}
		out = append(out, detail.ItemSummary)
	}
	sort.Slice(out, func(a, b int) bool {
		return out[a].UpdatedAt.After(out[b].UpdatedAt)
	})
	return out, nil
}

func (i *Index) BranchItems(workspaceID, branch string) ([]models.ItemSummary, error) {
	return i.Query(Query{WorkspaceID: workspaceID, Branch: branch})
}

func (i *Index) BranchScan(workspaceID, branch string) (models.BranchScanMetadata, bool, error) {
	if err := i.load(); err != nil {
		return models.BranchScanMetadata{}, false, err
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.state.BranchScans == nil || i.state.BranchScans[workspaceID] == nil {
		return models.BranchScanMetadata{}, false, nil
	}
	metadata, ok := i.state.BranchScans[workspaceID][branch]
	return metadata, ok, nil
}

func (i *Index) Get(id string) (models.ItemDetail, bool, error) {
	if err := i.load(); err != nil {
		return models.ItemDetail{}, false, err
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	for _, item := range i.state.Items {
		if item.ID == id {
			return item, true, nil
		}
	}
	return models.ItemDetail{}, false, nil
}

func matchesText(item models.ItemSummary, text string) bool {
	haystack := strings.ToLower(strings.Join([]string{
		item.Title, item.Identifier, item.Scope, item.Description, item.Author, strings.Join(item.Tags, " "),
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
		i.state = state{Items: []models.ItemDetail{}, Warnings: []models.ScanWarning{}, Scans: map[string]time.Time{}, BranchScans: map[string]map[string]models.BranchScanMetadata{}}
		i.loaded = true
		return nil
	}
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, &i.state); err != nil {
		return err
	}
	if i.state.Scans == nil {
		i.state.Scans = map[string]time.Time{}
	}
	for index := range i.state.Items {
		if i.state.Items[index].SourceMode == "" {
			i.state.Items[index].SourceMode = "working_tree"
			i.state.Items[index].Editable = true
		}
	}
	if i.state.BranchScans == nil {
		i.state.BranchScans = map[string]map[string]models.BranchScanMetadata{}
	}
	i.migrateBranchScanMetadataLocked()
	i.loaded = true
	return nil
}

func (i *Index) migrateBranchScanMetadataLocked() {
	for _, item := range i.state.Items {
		if item.WorkspaceID == "" || item.Branch == "" {
			continue
		}
		if i.state.BranchScans[item.WorkspaceID] == nil {
			i.state.BranchScans[item.WorkspaceID] = map[string]models.BranchScanMetadata{}
		}
		if _, ok := i.state.BranchScans[item.WorkspaceID][item.Branch]; ok {
			continue
		}
		scannedAt := i.state.Scans[item.WorkspaceID]
		i.state.BranchScans[item.WorkspaceID][item.Branch] = models.BranchScanMetadata{
			WorkspaceID: item.WorkspaceID,
			Branch:      item.Branch,
			BranchRef:   item.BranchRef,
			Commit:      item.Commit,
			SourceMode:  firstNonEmpty(item.SourceMode, "working_tree"),
			Editable:    item.Editable || item.SourceMode == "" || item.SourceMode == "working_tree",
			ScannedAt:   scannedAt,
			Warnings:    []models.ScanWarning{},
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (i *Index) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(i.path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(i.state)
	if err != nil {
		return err
	}
	return os.WriteFile(i.path, data, 0o600)
}

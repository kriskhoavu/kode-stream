package itemindex

// Package itemindex persists the Item domain read model.

import (
	"context"
	"errors"
	"io"
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

type Repository interface {
	ReplaceWorkspace(string, []models.ItemDetail, []models.ScanWarning, time.Time) error
	ReplaceWorkspaceBranch(string, string, []models.ItemDetail, models.BranchScanMetadata) error
	DeleteWorkspace(string) error
	Query(Query) ([]models.ItemSummary, error)
	BranchItems(string, string) ([]models.ItemSummary, error)
	BranchScan(string, string) (models.BranchScanMetadata, bool, error)
	Get(string) (models.ItemDetail, bool, error)
}

type Query struct {
	WorkspaceID      string
	Branch           string
	Status           string
	Text             string
	IncludeSnapshots bool
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
	previous := cloneState(i.state)
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
	return i.saveOrRestoreLocked(previous)
}

func (i *Index) ReplaceWorkspaceBranch(workspaceID, branch string, items []models.ItemDetail, metadata models.BranchScanMetadata) error {
	if err := i.load(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	previous := cloneState(i.state)
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
	return i.saveOrRestoreLocked(previous)
}

func (i *Index) DeleteWorkspace(workspaceID string) error {
	if err := i.load(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	previous := cloneState(i.state)
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
	return i.saveOrRestoreLocked(previous)
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
		if !q.IncludeSnapshots && detail.SourceMode == "snapshot" {
			continue
		}
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

// VisitContext streams matching summaries without materializing the index result.
// Returning false from visit stops traversal successfully.
func (i *Index) VisitContext(ctx context.Context, q Query, visit func(models.ItemSummary) bool) error {
	if err := i.loadContext(ctx); err != nil {
		return err
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	text := strings.ToLower(strings.TrimSpace(q.Text))
	for _, detail := range i.state.Items {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !q.IncludeSnapshots && detail.SourceMode == "snapshot" {
			continue
		}
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
		if !visit(detail.ItemSummary) {
			return nil
		}
	}
	return nil
}

func (i *Index) BranchItems(workspaceID, branch string) ([]models.ItemSummary, error) {
	return i.Query(Query{WorkspaceID: workspaceID, Branch: branch, IncludeSnapshots: true})
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
	return i.loadContext(context.Background())
}

func (i *Index) loadContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.loaded {
		return nil
	}
	file, err := os.Open(i.path)
	if errors.Is(err, os.ErrNotExist) {
		i.state = state{Items: []models.ItemDetail{}, Warnings: []models.ScanWarning{}, Scans: map[string]time.Time{}, BranchScans: map[string]map[string]models.BranchScanMetadata{}}
		i.loaded = true
		return nil
	}
	if err != nil {
		return err
	}
	data, err := readContext(ctx, file)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, &i.state); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
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

func readContext(ctx context.Context, reader io.Reader) ([]byte, error) {
	buffer := make([]byte, 32<<10)
	data := make([]byte, 0, len(buffer))
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		count, err := reader.Read(buffer)
		if count > 0 {
			data = append(data, buffer[:count]...)
		}
		if errors.Is(err, io.EOF) {
			return data, nil
		}
		if err != nil {
			return nil, err
		}
	}
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
	dir := filepath.Dir(i.path)
	mode := os.FileMode(0o600)
	if info, err := os.Stat(i.path); err == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".items-index-*")
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
	if err := os.Rename(temporaryPath, i.path); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func (i *Index) saveOrRestoreLocked(previous state) error {
	if err := i.saveLocked(); err != nil {
		i.state = previous
		return err
	}
	return nil
}

func cloneState(source state) state {
	cloned := state{
		Items:       append([]models.ItemDetail(nil), source.Items...),
		Warnings:    append([]models.ScanWarning(nil), source.Warnings...),
		Scans:       make(map[string]time.Time, len(source.Scans)),
		BranchScans: make(map[string]map[string]models.BranchScanMetadata, len(source.BranchScans)),
	}
	for workspaceID, scannedAt := range source.Scans {
		cloned.Scans[workspaceID] = scannedAt
	}
	for workspaceID, branches := range source.BranchScans {
		cloned.BranchScans[workspaceID] = make(map[string]models.BranchScanMetadata, len(branches))
		for branch, metadata := range branches {
			metadata.Warnings = append([]models.ScanWarning(nil), metadata.Warnings...)
			cloned.BranchScans[workspaceID][branch] = metadata
		}
	}
	return cloned
}

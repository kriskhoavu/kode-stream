package itemwriter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"plan-manager/internal/fileaccess"
	"plan-manager/internal/itemindex"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
	"plan-manager/internal/scanner"
	"plan-manager/internal/security/pathguard"
	"plan-manager/internal/writeguard"
)

type Writer struct {
	files    *fileaccess.Access
	scanner  *scanner.Scanner
	index    *itemindex.Index
	registry *registry.Registry
}

func New(files *fileaccess.Access, scan *scanner.Scanner, idx *itemindex.Index, reg *registry.Registry) *Writer {
	return &Writer{files: files, scanner: scan, index: idx, registry: reg}
}

func (w *Writer) SaveMarkdown(workspace models.WorkspaceConfig, item models.ItemDetail, input models.FileSaveInput) (models.WriteResult, error) {
	if strings.TrimSpace(input.FileID) == "" {
		return models.WriteResult{}, fmt.Errorf("file ID is required")
	}
	if _, err := w.files.WriteMarkdown(workspace, item, input); err != nil {
		return models.WriteResult{}, err
	}
	return w.refresh(workspace, item.ItemPath)
}

func (w *Writer) SaveMetadata(workspace models.WorkspaceConfig, item models.ItemDetail, input models.ItemMetadataUpdateInput) (models.WriteResult, error) {
	if isDocsRoot(item) {
		return models.WriteResult{}, fmt.Errorf("freestyle docs roots do not support item metadata")
	}
	if input.Status != "" {
		if err := writeguard.ValidateStatus(input.Status); err != nil {
			return models.WriteResult{}, err
		}
	}
	if input.Scope != "" {
		if err := writeguard.ValidateScopeName(input.Scope); err != nil {
			return models.WriteResult{}, err
		}
	}
	if input.Identifier != "" {
		if err := writeguard.ValidateIdentifierName(input.Identifier); err != nil {
			return models.WriteResult{}, err
		}
	}
	meta, err := readPlanMetadata(workspace, item)
	if err != nil {
		return models.WriteResult{}, err
	}
	applyMetadata(&meta, item, input)
	if err := writePlanMetadata(workspace, item, meta); err != nil {
		return models.WriteResult{}, err
	}
	return w.refresh(workspace, item.ItemPath)
}

func (w *Writer) UpdateStatus(workspace models.WorkspaceConfig, item models.ItemDetail, input models.ItemStatusUpdateInput) (models.WriteResult, error) {
	return w.SaveMetadata(workspace, item, models.ItemMetadataUpdateInput{Status: input.Status})
}

func (w *Writer) CreateItem(workspace models.WorkspaceConfig, input models.NewItemInput) (models.WriteResult, error) {
	source, err := validateSource(workspace, input.Source)
	if err != nil {
		return models.WriteResult{}, err
	}
	if err := writeguard.ValidateScopeName(input.Scope); err != nil {
		return models.WriteResult{}, err
	}
	if err := writeguard.ValidateIdentifierName(input.Identifier); err != nil {
		return models.WriteResult{}, err
	}
	status := input.Status
	if status == "" {
		status = models.StatusDraft
	}
	if err := writeguard.ValidateStatus(status); err != nil {
		return models.WriteResult{}, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = input.Identifier
	}
	itemRoot := filepath.ToSlash(filepath.Join(source, input.Scope, input.Identifier))
	fullRoot, err := safeJoin(workspace.Path, itemRoot)
	if err != nil {
		return models.WriteResult{}, err
	}
	if _, err := os.Stat(fullRoot); err == nil {
		return models.WriteResult{}, fmt.Errorf("item already exists")
	} else if !os.IsNotExist(err) {
		return models.WriteResult{}, err
	}
	if err := os.MkdirAll(filepath.Join(fullRoot, "scenario"), 0o755); err != nil {
		return models.WriteResult{}, err
	}
	if err := os.MkdirAll(filepath.Join(fullRoot, "design"), 0o755); err != nil {
		return models.WriteResult{}, err
	}
	files := map[string]string{
		"README.md":                        "# " + input.Identifier + ": " + title + "\n\n## Overview\n\n",
		"scenario/scenario-00-overview.md": "# Scenario Overview\n\n",
		"design/design-01-backend.md":      "# Backend Design\n\n",
		"design/design-02-frontend.md":     "# Frontend Design\n\n",
		"implementation-item.md":           "# Implementation Item\n\n",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(fullRoot, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			return models.WriteResult{}, err
		}
	}
	meta := planYAML{
		Item: planFields{
			Identifier: input.Identifier,
			Title:      title,
			Scope:      input.Scope,
			Status:     string(status),
			Owner:      strings.TrimSpace(input.Owner),
			Tags:       cleanTags(input.Tags),
		},
		Documents: starterDocuments(),
	}
	if err := writePlanMetadataAt(fullRoot, meta); err != nil {
		return models.WriteResult{}, err
	}
	return w.refresh(workspace, itemRoot)
}

func (w *Writer) RefreshWorkspace(workspace models.WorkspaceConfig) (models.ScanResult, error) {
	result, _, err := w.refreshWorkspaceData(workspace)
	return result, err
}

func (w *Writer) refresh(workspace models.WorkspaceConfig, itemRoot string) (models.WriteResult, error) {
	scanResult, data, err := w.refreshWorkspaceData(workspace)
	if err != nil {
		return models.WriteResult{}, err
	}
	for _, item := range data.Items {
		if item.ItemPath == itemRoot {
			return models.WriteResult{Item: item, ScannedAt: scanResult.ScannedAt}, nil
		}
	}
	return models.WriteResult{ScannedAt: scanResult.ScannedAt}, nil
}

func (w *Writer) refreshWorkspaceData(workspace models.WorkspaceConfig) (models.ScanResult, scanner.ScanData, error) {
	scannedAt := time.Now().UTC()
	if w.scanner == nil || w.index == nil {
		return models.ScanResult{WorkspaceID: workspace.ID, ScannedAt: scannedAt}, scanner.ScanData{}, nil
	}
	data, err := w.scanner.Scan(workspace)
	if err != nil {
		return models.ScanResult{}, scanner.ScanData{}, err
	}
	if err := w.index.ReplaceWorkspace(workspace.ID, data.Items, data.Warnings, scannedAt); err != nil {
		return models.ScanResult{}, scanner.ScanData{}, err
	}
	if w.registry != nil {
		_ = w.registry.TouchScanned(workspace.ID, scannedAt)
	}
	return models.ScanResult{
		WorkspaceID: workspace.ID,
		ScannedAt:   scannedAt,
		ItemCount:   len(data.Items),
		Warnings:    data.Warnings,
	}, data, nil
}

type planYAML struct {
	Item       planFields            `yaml:"item"`
	LegacyItem *planFields           `yaml:"plan,omitempty"`
	Documents  []models.ItemDocument `yaml:"documents,omitempty"`
}

type planFields struct {
	Identifier string   `yaml:"identifier,omitempty"`
	Title      string   `yaml:"title,omitempty"`
	Scope      string   `yaml:"scope,omitempty"`
	Status     string   `yaml:"status,omitempty"`
	Owner      string   `yaml:"owner,omitempty"`
	Tags       []string `yaml:"tags,omitempty"`
}

func readPlanMetadata(workspace models.WorkspaceConfig, item models.ItemDetail) (planYAML, error) {
	root, err := safeItemPath(workspace, item)
	if err != nil {
		return planYAML{}, err
	}
	var meta planYAML
	data, err := os.ReadFile(filepath.Join(root, "item.yaml"))
	if os.IsNotExist(err) {
		data, err = os.ReadFile(filepath.Join(root, "plan.yaml"))
		if os.IsNotExist(err) {
			meta.Documents = item.Documents
			return meta, nil
		}
	}
	if err != nil {
		return meta, err
	}
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return meta, err
	}
	if meta.Item.Identifier == "" && meta.LegacyItem != nil && meta.LegacyItem.Identifier != "" {
		meta.Item = *meta.LegacyItem
	}
	meta.LegacyItem = nil
	return meta, nil
}

func applyMetadata(meta *planYAML, item models.ItemDetail, input models.ItemMetadataUpdateInput) {
	if meta.Item.Identifier == "" {
		meta.Item.Identifier = item.Identifier
	}
	if meta.Item.Title == "" {
		meta.Item.Title = item.Title
	}
	if meta.Item.Scope == "" {
		meta.Item.Scope = item.Scope
	}
	if meta.Item.Status == "" {
		meta.Item.Status = string(item.Status)
	}
	if input.Identifier != "" {
		meta.Item.Identifier = strings.TrimSpace(input.Identifier)
	}
	if input.Title != "" {
		meta.Item.Title = strings.TrimSpace(input.Title)
	}
	if input.Scope != "" {
		meta.Item.Scope = strings.TrimSpace(input.Scope)
	}
	if input.Status != "" {
		meta.Item.Status = string(input.Status)
	}
	if input.Owner != "" {
		meta.Item.Owner = strings.TrimSpace(input.Owner)
	}
	if input.Tags != nil {
		meta.Item.Tags = cleanTags(input.Tags)
	}
	if len(meta.Documents) == 0 {
		meta.Documents = item.Documents
	}
}

func writePlanMetadata(workspace models.WorkspaceConfig, item models.ItemDetail, meta planYAML) error {
	root, err := safeItemPath(workspace, item)
	if err != nil {
		return err
	}
	return writePlanMetadataAt(root, meta)
}

func writePlanMetadataAt(root string, meta planYAML) error {
	data, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "item.yaml"), data, 0o644)
}

func starterDocuments() []models.ItemDocument {
	return []models.ItemDocument{
		{ID: "README_md", Role: "overview", Path: "README.md", Label: "README"},
		{ID: "scenario__scenario-00-overview_md", Role: "scenario", Path: "scenario/scenario-00-overview.md", Label: "Scenario Overview"},
		{ID: "design__design-01-backend_md", Role: "design", Track: "backend", Path: "design/design-01-backend.md", Label: "Backend Design"},
		{ID: "design__design-02-frontend_md", Role: "design", Track: "frontend", Path: "design/design-02-frontend.md", Label: "Frontend Design"},
		{ID: "implementation-plan_md", Role: "implementation", Path: "implementation-item.md", Label: "Implementation Item"},
	}
}

func validateSource(workspace models.WorkspaceConfig, dir string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(dir)))
	for _, allowed := range workspace.Sources {
		if clean == allowed {
			return clean, nil
		}
	}
	return "", fmt.Errorf("source is not registered")
}

func safeItemPath(workspace models.WorkspaceConfig, item models.ItemDetail) (string, error) {
	return safeJoin(workspace.Path, item.ItemPath)
}

func safeJoin(root, rel string) (string, error) {
	return pathguard.SafeJoin(root, rel)
}

func isDocsRoot(item models.ItemDetail) bool {
	return item.MetadataSource == "docs"
}

func cleanTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" && !seen[tag] {
			seen[tag] = true
			out = append(out, tag)
		}
	}
	return out
}

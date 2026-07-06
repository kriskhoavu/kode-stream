package itemwriter

// Package itemwriter persists and refreshes Item domain files.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"plan-manager/internal/common/models"
	"plan-manager/internal/filesystem/content"
	"plan-manager/internal/filesystem/pathguard"
	"plan-manager/internal/filesystem/writeguard"
	gitadapter "plan-manager/internal/git"
	"plan-manager/internal/item/index"
	"plan-manager/internal/workspace/registry"
	"plan-manager/internal/workspace/scanner"
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

func (w *Writer) MaterializeSnapshotItem(workspace models.WorkspaceConfig, item models.ItemDetail, fileID string) error {
	if item.SourceMode != "snapshot" {
		return nil
	}
	if strings.TrimSpace(item.BranchRef) == "" {
		return fmt.Errorf("snapshot branch reference is missing")
	}
	reader := scanner.NewGitTreeSourceReader(workspace.Path, item.BranchRef, gitadapter.New())
	scopeRoot := item.ItemPath
	copyOneFile := item.MetadataSource == "docs"
	if copyOneFile {
		relPath := materializeRelativeFile(item, fileID)
		if relPath == "" {
			return fmt.Errorf("snapshot file is not part of the indexed item")
		}
		scopeRoot = filepath.ToSlash(filepath.Join(item.ItemPath, relPath))
	}
	var files []string
	if copyOneFile {
		if info, err := reader.Stat(scopeRoot); err != nil {
			return err
		} else if info.IsDir() {
			return fmt.Errorf("snapshot materialization expected a file")
		}
		files = append(files, scopeRoot)
	} else {
		if err := reader.WalkDir(scopeRoot, func(path string, d scanner.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			files = append(files, path)
			return nil
		}); err != nil {
			return err
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("snapshot item has no files to materialize")
	}
	for _, rel := range files {
		if !isInsideConfiguredSource(workspace, rel) {
			return fmt.Errorf("materialized path is outside configured sources")
		}
		full, err := safeJoin(workspace.Path, rel)
		if err != nil {
			return err
		}
		if _, err := os.Stat(full); err == nil {
			return fmt.Errorf("This snapshot item cannot be copied because files already exist in the current checkout branch. Resolve the conflict manually or switch branches first.")
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	for _, rel := range files {
		if !isInsideConfiguredSource(workspace, rel) {
			return fmt.Errorf("materialized path is outside configured sources")
		}
		data, err := reader.ReadFile(rel)
		if err != nil {
			return err
		}
		full, err := safeJoin(workspace.Path, rel)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) CreateItem(workspace models.WorkspaceConfig, input models.NewItemInput) (models.WriteResult, error) {
	input.Source = strings.TrimSpace(input.Source)
	input.Scope = strings.TrimSpace(input.Scope)
	input.Identifier = strings.TrimSpace(input.Identifier)
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
	if err := os.MkdirAll(fullRoot, 0o755); err != nil {
		return models.WriteResult{}, err
	}
	if err := os.WriteFile(filepath.Join(fullRoot, "README.md"), nil, 0o644); err != nil {
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
	branch := workspace.BaselineBranch
	if len(data.Items) > 0 && data.Items[0].Branch != "" {
		branch = data.Items[0].Branch
	}
	if err := w.index.ReplaceWorkspaceBranch(workspace.ID, branch, data.Items, models.BranchScanMetadata{
		WorkspaceID: workspace.ID,
		Branch:      branch,
		SourceMode:  "working_tree",
		Editable:    true,
		ScannedAt:   scannedAt,
		Warnings:    data.Warnings,
	}); err != nil {
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
	Plan      planFields            `yaml:"plan"`
	Documents []models.ItemDocument `yaml:"documents,omitempty"`
}

type planFields struct {
	Identifier string   `yaml:"identifier,omitempty"`
	Ticket     string   `yaml:"ticket,omitempty"`
	Title      string   `yaml:"title,omitempty"`
	Scope      string   `yaml:"scope,omitempty"`
	Service    string   `yaml:"service,omitempty"`
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
	data, err := os.ReadFile(filepath.Join(root, "plan.yaml"))
	if os.IsNotExist(err) {
		meta.Documents = item.Documents
		return meta, nil
	}
	if err != nil {
		return meta, err
	}
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return meta, err
	}
	if meta.Plan.Identifier == "" {
		meta.Plan.Identifier = meta.Plan.Ticket
	}
	if meta.Plan.Scope == "" {
		meta.Plan.Scope = meta.Plan.Service
	}
	meta.Plan.Ticket = ""
	meta.Plan.Service = ""
	return meta, nil
}

func applyMetadata(meta *planYAML, item models.ItemDetail, input models.ItemMetadataUpdateInput) {
	if meta.Plan.Identifier == "" {
		meta.Plan.Identifier = item.Identifier
	}
	if meta.Plan.Title == "" {
		meta.Plan.Title = item.Title
	}
	if meta.Plan.Scope == "" {
		meta.Plan.Scope = item.Scope
	}
	if meta.Plan.Status == "" {
		meta.Plan.Status = string(item.Status)
	}
	if input.Identifier != "" {
		meta.Plan.Identifier = strings.TrimSpace(input.Identifier)
	}
	if input.Title != "" {
		meta.Plan.Title = strings.TrimSpace(input.Title)
	}
	if input.Scope != "" {
		meta.Plan.Scope = strings.TrimSpace(input.Scope)
	}
	if input.Status != "" {
		meta.Plan.Status = string(input.Status)
	}
	if input.Owner != "" {
		meta.Plan.Owner = strings.TrimSpace(input.Owner)
	}
	if input.Tags != nil {
		meta.Plan.Tags = cleanTags(input.Tags)
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
	compactPlanMetadata(root, &meta)
	data, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "plan.yaml"), data, 0o644)
}

func compactPlanMetadata(root string, meta *planYAML) {
	identifier := strings.TrimSpace(meta.Plan.Identifier)
	if identifier == "" {
		identifier = filepath.Base(root)
	}
	if strings.EqualFold(identifier, filepath.Base(root)) {
		meta.Plan.Identifier = ""
	}
	if strings.EqualFold(strings.TrimSpace(meta.Plan.Scope), filepath.Base(filepath.Dir(root))) {
		meta.Plan.Scope = ""
	}
	if inferredTitle := scanner.InferPlanTitle(root, identifier); inferredTitle != "" && meta.Plan.Title == inferredTitle {
		meta.Plan.Title = ""
	}
	meta.Plan.Ticket = ""
	meta.Plan.Service = ""
	meta.Documents = compactDocumentOverrides(root, meta.Documents)
}

func compactDocumentOverrides(root string, documents []models.ItemDocument) []models.ItemDocument {
	inferred := scanner.InferDocuments(root)
	byPath := make(map[string]models.ItemDocument, len(inferred))
	for _, doc := range inferred {
		byPath[filepath.ToSlash(doc.Path)] = doc
	}
	overrides := make([]models.ItemDocument, 0, len(documents))
	for _, doc := range documents {
		doc.Path = filepath.ToSlash(strings.TrimSpace(doc.Path))
		base, found := byPath[doc.Path]
		if !found {
			overrides = append(overrides, doc)
			continue
		}
		override := models.ItemDocument{Path: doc.Path}
		if doc.Role != "" && doc.Role != base.Role {
			override.Role = doc.Role
		}
		if doc.Track != "" && doc.Track != base.Track {
			override.Track = doc.Track
		}
		if doc.Label != "" && doc.Label != base.Label {
			override.Label = doc.Label
		}
		if override.Role != "" || override.Track != "" || override.Label != "" {
			overrides = append(overrides, override)
		}
	}
	return overrides
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

func isInsideConfiguredSource(workspace models.WorkspaceConfig, rel string) bool {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(rel)))
	if clean == "." || clean == "" || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
		return false
	}
	for _, source := range workspace.Sources {
		if clean == source || strings.HasPrefix(clean, source+"/") {
			return true
		}
	}
	return false
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

func materializeRelativeFile(item models.ItemDetail, fileID string) string {
	for _, doc := range item.Documents {
		if fileIDForPath(doc.Path) == fileID {
			return doc.Path
		}
	}
	return ""
}

func fileIDForPath(path string) string {
	return strings.NewReplacer("/", "__", ".", "_").Replace(path)
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

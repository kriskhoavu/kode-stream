package itemwriter

// Package itemwriter persists and refreshes Item domain files.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/content"
	"kode-stream/internal/filesystem/guardedwrite"
	"kode-stream/internal/filesystem/pathguard"
	"kode-stream/internal/filesystem/sourceguard"
	"kode-stream/internal/filesystem/writeguard"
	gitadapter "kode-stream/internal/git"
	"kode-stream/internal/item/index"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
)

type Writer struct {
	files          *fileaccess.Access
	scanner        *scanner.Scanner
	index          itemindex.Repository
	registry       registry.Repository
	snapshotReader func(models.WorkspaceConfig, models.ItemDetail) scanner.SourceReader
}

var ErrMetadataRevisionRequired = errors.New("expected metadata revision is required")

func New(files *fileaccess.Access, scan *scanner.Scanner, idx itemindex.Repository, reg registry.Repository) *Writer {
	return &Writer{
		files: files, scanner: scan, index: idx, registry: reg,
		snapshotReader: func(workspace models.WorkspaceConfig, item models.ItemDetail) scanner.SourceReader {
			return scanner.NewGitTreeSourceReader(workspace.Path, item.Commit, gitadapter.New())
		},
	}
}

func (w *Writer) SaveMarkdown(workspace models.WorkspaceConfig, item models.ItemDetail, input models.FileSaveInput) (models.WriteResult, error) {
	if strings.TrimSpace(input.FileID) == "" {
		return models.WriteResult{}, fmt.Errorf("file ID is required")
	}
	if _, err := w.files.WriteMarkdown(workspace, item, input); err != nil {
		return models.WriteResult{}, err
	}
	return w.committedRefresh(workspace, item.ItemPath)
}

func (w *Writer) SaveMetadata(workspace models.WorkspaceConfig, item models.ItemDetail, input models.ItemMetadataUpdateInput) (models.WriteResult, error) {
	if isDocumentationRoot(item) {
		return models.WriteResult{}, fmt.Errorf("freestyle documentation roots do not support item metadata")
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
	if input.ExpectedRevision == "" {
		return models.WriteResult{}, ErrMetadataRevisionRequired
	}
	if input.ExpectedRevision != meta.revision {
		return models.WriteResult{}, guardedwrite.ErrStale
	}
	applyMetadata(&meta, item, input)
	if err := writePlanMetadata(workspace, item, meta); err != nil {
		return models.WriteResult{}, err
	}
	return w.committedRefresh(workspace, item.ItemPath)
}

func (w *Writer) MetadataRevision(workspace models.WorkspaceConfig, item models.ItemDetail) (string, error) {
	meta, err := readPlanMetadata(workspace, item)
	return meta.revision, err
}

func (w *Writer) VerificationTests(workspace models.WorkspaceConfig, item models.ItemDetail) (models.VerificationTestSelection, error) {
	meta, err := readPlanMetadata(workspace, item)
	if err != nil {
		return models.VerificationTestSelection{}, err
	}
	selection := normalizeVerificationTests(meta.VerificationTests)
	selection.Revision = meta.revision
	return selection, nil
}

func (w *Writer) SaveVerificationTests(workspace models.WorkspaceConfig, item models.ItemDetail, input models.VerificationTestSelection) (models.WriteResult, error) {
	if isDocumentationRoot(item) {
		return models.WriteResult{}, fmt.Errorf("freestyle documentation roots do not support item verification tests")
	}
	meta, err := readPlanMetadata(workspace, item)
	if err != nil {
		return models.WriteResult{}, err
	}
	if input.ExpectedRevision == "" {
		return models.WriteResult{}, ErrMetadataRevisionRequired
	}
	if input.ExpectedRevision != meta.revision {
		return models.WriteResult{}, guardedwrite.ErrStale
	}
	selection, err := normalizeAndValidateVerificationTests(input)
	if err != nil {
		return models.WriteResult{}, err
	}
	meta.VerificationTests = selection
	if err := writePlanMetadata(workspace, item, meta); err != nil {
		return models.WriteResult{}, err
	}
	return w.committedRefresh(workspace, item.ItemPath)
}

func (w *Writer) committedRefresh(workspace models.WorkspaceConfig, itemPath string) (models.WriteResult, error) {
	result, err := w.refresh(workspace, itemPath)
	if err != nil {
		return models.WriteResult{Committed: true, RefreshRequired: true, RefreshError: err.Error()}, nil
	}
	result.Committed = true
	if result.Item.ItemPath != "" {
		if revision, revisionErr := w.MetadataRevision(workspace, result.Item); revisionErr == nil {
			result.Item.MetadataRevision = revision
		}
	}
	return result, nil
}

func (w *Writer) UpdateStatus(workspace models.WorkspaceConfig, item models.ItemDetail, input models.ItemStatusUpdateInput) (models.WriteResult, error) {
	return w.SaveMetadata(workspace, item, models.ItemMetadataUpdateInput{Status: input.Status, ExpectedRevision: input.ExpectedRevision})
}

func (w *Writer) importReviewedSnapshot(workspace models.WorkspaceConfig, item models.ItemDetail) error {
	if item.SourceMode != "snapshot" {
		return nil
	}
	if strings.TrimSpace(item.Commit) == "" {
		return fmt.Errorf("snapshot commit is missing")
	}
	reader := w.snapshotReader(workspace, item)
	scopeRoot := item.ItemPath
	copyOneFile := false
	var targetItemRoot string
	if !copyOneFile {
		var err error
		targetItemRoot, err = safeItemPathForRelative(workspace, item.ItemPath)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(targetItemRoot); err == nil {
			return fmt.Errorf("import target already exists in the current checkout branch")
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	var files []string
	if !copyOneFile {
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
	return w.publishStructuredSnapshot(workspace, item, reader, files, targetItemRoot)
}

func (w *Writer) publishStructuredSnapshot(workspace models.WorkspaceConfig, item models.ItemDetail, reader scanner.SourceReader, files []string, targetItemRoot string) error {
	parent := filepath.Dir(targetItemRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stagingRoot, err := os.MkdirTemp(parent, "."+filepath.Base(targetItemRoot)+".import-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stagingRoot)
	if err := os.Chmod(stagingRoot, 0o755); err != nil {
		return err
	}
	for _, rel := range files {
		if !isInsideConfiguredSource(workspace, rel) {
			return fmt.Errorf("materialized path is outside configured sources")
		}
		withinItem, err := filepath.Rel(filepath.FromSlash(item.ItemPath), filepath.FromSlash(rel))
		if err != nil || withinItem == ".." || strings.HasPrefix(withinItem, ".."+string(filepath.Separator)) {
			return fmt.Errorf("snapshot file is outside the reviewed item root")
		}
		data, err := reader.ReadFile(rel)
		if err != nil {
			return err
		}
		stagedPath, err := safeJoin(stagingRoot, withinItem)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(stagedPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(stagedPath, data, 0o644); err != nil {
			return err
		}
	}
	if _, err := os.Lstat(targetItemRoot); err == nil {
		return fmt.Errorf("import target already exists in the current checkout branch")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(stagingRoot, targetItemRoot); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("import target already exists in the current checkout branch")
		}
		return err
	}
	return nil
}

func (w *Writer) ImportSnapshotPlan(workspace models.WorkspaceConfig, item models.ItemDetail) (models.WriteResult, error) {
	if item.SourceMode != "snapshot" {
		return models.WriteResult{}, fmt.Errorf("reviewed plan must come from a snapshot")
	}
	if isDocumentationRoot(item) {
		return models.WriteResult{}, fmt.Errorf("only structured plans can be imported")
	}
	if err := w.importReviewedSnapshot(workspace, item); err != nil {
		return models.WriteResult{}, err
	}
	result, err := w.refresh(workspace, item.ItemPath)
	if err == nil {
		return result, nil
	}
	targetItemRoot, pathErr := safeItemPathForRelative(workspace, item.ItemPath)
	if pathErr != nil {
		return models.WriteResult{}, fmt.Errorf("refresh imported plan: %w; resolve rollback target: %v", err, pathErr)
	}
	if rollbackErr := os.RemoveAll(targetItemRoot); rollbackErr != nil {
		return models.WriteResult{}, fmt.Errorf("refresh imported plan: %w; rollback failed: %v", err, rollbackErr)
	}
	return models.WriteResult{}, err
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
	itemRoot, err := w.createItemRoot(workspace, source, input)
	if err != nil {
		return models.WriteResult{}, err
	}
	fullRoot, err := safeItemPathForRelative(workspace, itemRoot)
	if err != nil {
		return models.WriteResult{}, err
	}
	if info, err := os.Stat(fullRoot); err == nil {
		// A manual deletion of an item's contents can leave the directory behind.
		// An empty directory holds no item, so reuse it instead of reporting a
		// duplicate the workspace cannot show.
		empty, emptyErr := isEmptyDir(fullRoot, info)
		if emptyErr != nil {
			return models.WriteResult{}, emptyErr
		}
		if !empty {
			return models.WriteResult{}, fmt.Errorf("item already exists")
		}
	} else if !os.IsNotExist(err) {
		return models.WriteResult{}, err
	}
	if err := os.MkdirAll(fullRoot, 0o755); err != nil {
		return models.WriteResult{}, err
	}
	if err := os.WriteFile(filepath.Join(fullRoot, "README.md"), []byte(input.InitialReadme), 0o644); err != nil {
		return models.WriteResult{}, err
	}
	if strings.TrimSpace(input.JiraKey) != "" {
		meta := planYAML{Plan: planFields{
			Identifier: input.Identifier,
			Title:      strings.TrimSpace(input.Title),
			Scope:      input.Scope,
			Status:     string(status),
			Owner:      strings.TrimSpace(input.Owner),
			Tags:       cleanTags(input.Tags),
		}}
		if err := writePlanMetadataAt(fullRoot, meta); err != nil {
			return models.WriteResult{}, err
		}
	}
	return w.refresh(workspace, itemRoot)
}

// isEmptyDir reports whether path is a directory holding no entries. A
// non-directory is never empty, so an existing file at the item path still
// counts as occupied.
func isEmptyDir(path string, info os.FileInfo) (bool, error) {
	if !info.IsDir() {
		return false, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

var createItemPatternVariable = regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9_]*)\}`)

func (w *Writer) createItemRoot(workspace models.WorkspaceConfig, source string, input models.NewItemInput) (string, error) {
	sourceRoot := filepath.Join(workspace.Path, filepath.FromSlash(source))
	settings, hasSettings, warnings := scanner.ReadSourceStructureSettings(sourceRoot)
	if hasSettings && len(warnings) == 0 && len(settings.Cards) > 0 {
		rel, err := renderCreateItemPathPattern(settings.Cards[0].PathPattern, source, input)
		if err != nil {
			return "", err
		}
		return filepath.ToSlash(filepath.Join(source, rel)), nil
	}
	if isSourceRootScope(source, input.Scope) {
		return filepath.ToSlash(filepath.Join(source, input.Identifier)), nil
	}
	return filepath.ToSlash(filepath.Join(source, input.Scope, input.Identifier)), nil
}

func isSourceRootScope(source, scope string) bool {
	cleanSource := filepath.ToSlash(filepath.Clean(strings.TrimSpace(source)))
	cleanScope := filepath.ToSlash(filepath.Clean(strings.TrimSpace(scope)))
	return cleanScope == cleanSource || cleanScope == filepath.Base(cleanSource)
}

func renderCreateItemPathPattern(pattern, source string, input models.NewItemInput) (string, error) {
	values := map[string]string{
		"folder":     input.Scope,
		"scope":      input.Scope,
		"item":       input.Identifier,
		"identifier": input.Identifier,
		"source":     filepath.Base(source),
	}
	rendered := createItemPatternVariable.ReplaceAllStringFunc(pattern, func(match string) string {
		parts := createItemPatternVariable.FindStringSubmatch(match)
		if len(parts) != 2 {
			return ""
		}
		return values[parts[1]]
	})
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(rendered)))
	if clean == "." || clean == "" || strings.HasPrefix(clean, "../") || clean == ".." || strings.HasPrefix(clean, "/") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("source item path pattern produced an invalid path")
	}
	return clean, nil
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
	Plan              planFields                       `yaml:"plan"`
	Documents         []models.ItemDocument            `yaml:"documents,omitempty"`
	AutomationTests   []models.AutomationTestPath      `yaml:"automation-test,omitempty"`
	VerificationTests models.VerificationTestSelection `yaml:"verificationTests,omitempty"`
	Extra             map[string]any                   `yaml:",inline"`
	raw               *yaml.Node
	revision          string
}

type planFields struct {
	Identifier string         `yaml:"identifier,omitempty"`
	Ticket     string         `yaml:"ticket,omitempty"`
	Title      string         `yaml:"title,omitempty"`
	Scope      string         `yaml:"scope,omitempty"`
	Service    string         `yaml:"service,omitempty"`
	Status     string         `yaml:"status,omitempty"`
	Owner      string         `yaml:"owner,omitempty"`
	Tags       []string       `yaml:"tags,omitempty"`
	Extra      map[string]any `yaml:",inline"`
}

func readPlanMetadata(workspace models.WorkspaceConfig, item models.ItemDetail) (planYAML, error) {
	var meta planYAML
	var data []byte
	var err error
	if item.SourceMode == "snapshot" {
		commit := strings.TrimSpace(item.Commit)
		if commit == "" {
			return meta, fmt.Errorf("snapshot commit is missing")
		}
		data, err = gitadapter.New().TreeReadFile(workspace.Path, commit, filepath.ToSlash(filepath.Join(item.ItemPath, "plan.yaml")))
		if err != nil && snapshotMetadataPathMissing(err) {
			meta.Documents = item.Documents
			meta.revision = planRevision(nil)
			return meta, nil
		}
	} else {
		root, pathErr := safeItemPath(workspace, item)
		if pathErr != nil {
			return meta, pathErr
		}
		data, err = os.ReadFile(filepath.Join(root, "plan.yaml"))
		if os.IsNotExist(err) {
			meta.Documents = item.Documents
			meta.revision = planRevision(nil)
			return meta, nil
		}
	}
	if err != nil {
		return meta, err
	}
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return meta, err
	}
	meta.revision = planRevision(data)
	var raw yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return meta, err
	}
	meta.raw = &raw
	if meta.Plan.Identifier == "" {
		meta.Plan.Identifier = meta.Plan.Ticket
	}
	if meta.Plan.Scope == "" {
		meta.Plan.Scope = meta.Plan.Service
	}
	return meta, nil
}

func planRevision(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func snapshotMetadataPathMissing(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "does not exist in") || strings.Contains(message, "exists on disk, but not in") || strings.Contains(message, "unknown revision or path")
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
	var data []byte
	var err error
	if meta.raw != nil {
		mergePlanMetadataNode(meta.raw, meta)
		data, err = yaml.Marshal(meta.raw)
	} else {
		compactPlanMetadata(root, &meta)
		data, err = yaml.Marshal(meta)
	}
	if err != nil {
		return err
	}
	return atomicWritePlan(filepath.Join(root, "plan.yaml"), data)
}

func mergePlanMetadataNode(document *yaml.Node, meta planYAML) {
	root := document
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return
	}
	plan := mappingValue(root, "plan")
	if plan == nil || plan.Kind != yaml.MappingNode {
		plan = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		setMappingValue(root, "plan", plan)
	}
	setScalarIfNotEmpty(plan, "identifier", meta.Plan.Identifier)
	setScalarIfNotEmpty(plan, "ticket", meta.Plan.Ticket)
	setScalarIfNotEmpty(plan, "title", meta.Plan.Title)
	setScalarIfNotEmpty(plan, "scope", meta.Plan.Scope)
	setScalarIfNotEmpty(plan, "service", meta.Plan.Service)
	setScalarIfNotEmpty(plan, "status", meta.Plan.Status)
	setScalarIfNotEmpty(plan, "owner", meta.Plan.Owner)
	if meta.Plan.Tags != nil {
		setYAMLValue(plan, "tags", meta.Plan.Tags)
	}
	if len(meta.VerificationTests.SelectedSpecs) > 0 || meta.VerificationTests.Environment != "" || meta.VerificationTests.DisplayMode != "" {
		setYAMLValue(root, "verificationTests", meta.VerificationTests)
	}
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
func setMappingValue(node *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}
func setScalarIfNotEmpty(node *yaml.Node, key, value string) {
	if value != "" {
		setMappingValue(node, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
	}
}
func setYAMLValue(node *yaml.Node, key string, value any) {
	var encoded yaml.Node
	if err := encoded.Encode(value); err == nil {
		setMappingValue(node, key, &encoded)
	}
}

// atomicWritePlan publishes a complete next generation and preserves the
// existing mode. Plan files are user source documents, so a direct WriteFile
// is never an acceptable publication path.
func atomicWritePlan(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".plan.yaml-*")
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
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
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
	// ticket/service are documented legacy aliases. Preserve them so metadata
	// saves never rewrite a valid legacy plan into a lossy representation.
	meta.Documents = compactDocumentOverrides(root, meta.Documents)
	if len(meta.VerificationTests.SelectedSpecs) == 0 && strings.TrimSpace(meta.VerificationTests.Environment) == "" && meta.VerificationTests.DisplayMode == "" {
		meta.VerificationTests = models.VerificationTestSelection{}
	}
}

func normalizeVerificationTests(input models.VerificationTestSelection) models.VerificationTestSelection {
	selection := input
	specs := make([]string, 0, len(selection.SelectedSpecs))
	seen := map[string]struct{}{}
	for _, spec := range selection.SelectedSpecs {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(spec)))
		if clean == "" || clean == "." {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		specs = append(specs, clean)
	}
	selection.SelectedSpecs = specs
	selection.Environment = strings.TrimSpace(selection.Environment)
	selection.DisplayMode = normalizeAutomationDisplayMode(selection.DisplayMode)
	return selection
}

func normalizeAutomationDisplayMode(mode models.AutomationDisplayMode) models.AutomationDisplayMode {
	switch mode {
	case models.AutomationDisplayModeVisible:
		return models.AutomationDisplayModeVisible
	default:
		return models.AutomationDisplayModeSilent
	}
}

func normalizeAndValidateVerificationTests(input models.VerificationTestSelection) (models.VerificationTestSelection, error) {
	selection := normalizeVerificationTests(input)
	for _, spec := range selection.SelectedSpecs {
		if filepath.IsAbs(spec) || strings.HasPrefix(spec, "../") || spec == ".." {
			return models.VerificationTestSelection{}, fmt.Errorf("selected spec %q must be relative", spec)
		}
	}
	selection.UpdatedAt = time.Now().UTC()
	return selection, nil
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
			if _, err := sourceguard.ResolveOne(workspace.Path, clean); err != nil {
				return "", err
			}
			return clean, nil
		}
	}
	return "", fmt.Errorf("source is not registered")
}

func isInsideConfiguredSource(workspace models.WorkspaceConfig, rel string) bool {
	_, err := safeItemPathForRelative(workspace, rel)
	return err == nil
}

func safeItemPath(workspace models.WorkspaceConfig, item models.ItemDetail) (string, error) {
	return safeItemPathForRelative(workspace, item.ItemPath)
}

// safeItemPathForRelative revalidates configured sources at each write
// boundary. A source symlink can change after registration, so lexical joins
// are deliberately not treated as an authority to write.
func safeItemPathForRelative(workspace models.WorkspaceConfig, relative string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(relative)))
	var selected string
	for _, source := range workspace.Sources {
		source = filepath.ToSlash(filepath.Clean(strings.TrimSpace(source)))
		if clean == source || strings.HasPrefix(clean, source+"/") {
			selected = source
			break
		}
	}
	if selected == "" {
		return "", fmt.Errorf("item path is outside configured sources")
	}
	resolved, err := sourceguard.ResolveOne(workspace.Path, selected)
	if err != nil {
		// A configured source may be intentionally empty on a new workspace.
		// Create only its lexical path below the resolved workspace root, then
		// immediately resolve it through sourceguard; an existing symlink is
		// never followed as an authority to create outside the workspace.
		if !strings.Contains(err.Error(), "does not exist") {
			return "", err
		}
		realRoot, rootErr := filepath.EvalSymlinks(workspace.Path)
		if rootErr != nil {
			return "", rootErr
		}
		candidate, joinErr := safeJoin(realRoot, selected)
		if joinErr != nil {
			return "", joinErr
		}
		if mkdirErr := os.MkdirAll(candidate, 0o755); mkdirErr != nil {
			return "", mkdirErr
		}
		resolved, err = sourceguard.ResolveOne(workspace.Path, selected)
		if err != nil {
			return "", err
		}
	}
	full, err := safeJoin(workspace.Path, relative)
	if err != nil {
		return "", err
	}
	probe := full
	for {
		if _, statErr := os.Lstat(probe); statErr == nil {
			break
		} else if !os.IsNotExist(statErr) {
			return "", statErr
		}
		next := filepath.Dir(probe)
		if next == probe {
			return "", fmt.Errorf("item path is outside configured sources")
		}
		probe = next
	}
	realProbe, err := filepath.EvalSymlinks(probe)
	if err != nil {
		return "", err
	}
	if sourceguard.Contains(resolved.RealPath, realProbe) {
		return full, nil
	}
	return "", fmt.Errorf("item path is outside configured sources")
}

func safeJoin(root, rel string) (string, error) {
	return pathguard.SafeJoin(root, rel)
}

func isDocumentationRoot(item models.ItemDetail) bool {
	return item.MetadataSource == "docs" || item.MetadataSource == "wiki"
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

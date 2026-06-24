package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"plan-manager/internal/application/apperrors"
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/itemindex"
	"plan-manager/internal/itemwriter"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
	"plan-manager/internal/scanner"
)

type StateResult struct {
	Version        string    `json:"version"`
	WorkspaceCount int       `json:"workspaceCount"`
	ItemCount      int       `json:"itemCount"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type SourceStructureSaveResult struct {
	models.SourceSettingsResult
	Scan models.ScanResult `json:"scan" yaml:"scan"`
}

type Service struct {
	registry *registry.Registry
	index    *itemindex.Index
	scanner  *scanner.Scanner
	writer   *itemwriter.Writer
	git      *gitadapter.GitAdapter
}

func New(reg *registry.Registry, idx *itemindex.Index, scan *scanner.Scanner, writer *itemwriter.Writer, git ...*gitadapter.GitAdapter) *Service {
	var adapter *gitadapter.GitAdapter
	if len(git) > 0 {
		adapter = git[0]
	}
	return &Service{registry: reg, index: idx, scanner: scan, writer: writer, git: adapter}
}

func (s *Service) State() (StateResult, error) {
	workspaces, err := s.registry.List()
	if err != nil {
		return StateResult{}, err
	}
	items, err := s.index.Query(itemindex.Query{})
	if err != nil {
		return StateResult{}, err
	}
	latest := time.Time{}
	for _, workspace := range workspaces {
		if workspace.CreatedAt.After(latest) {
			latest = workspace.CreatedAt
		}
		if !workspace.LastScannedAt.IsZero() && workspace.LastScannedAt.After(latest) {
			latest = workspace.LastScannedAt
		}
	}
	for _, item := range items {
		if item.UpdatedAt.After(latest) {
			latest = item.UpdatedAt
		}
	}
	payload := struct {
		Workspaces []models.WorkspaceConfig `json:"workspaces"`
		Items      []models.ItemSummary     `json:"items"`
	}{Workspaces: workspaces, Items: items}
	data, err := json.Marshal(payload)
	if err != nil {
		return StateResult{}, err
	}
	sum := sha256.Sum256(data)
	return StateResult{
		Version:        hex.EncodeToString(sum[:]),
		WorkspaceCount: len(workspaces),
		ItemCount:      len(items),
		UpdatedAt:      latest,
	}, nil
}

func (s *Service) List() ([]models.WorkspaceConfig, error) {
	return s.registry.List()
}

func (s *Service) Get(id string) (models.WorkspaceConfig, bool, error) {
	return s.registry.Get(id)
}

func (s *Service) Create(input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	return s.registry.Create(input)
}

func (s *Service) Update(id string, input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	return s.registry.Update(id, input)
}

func (s *Service) Delete(id string) error {
	if err := s.registry.Delete(id); err != nil {
		return err
	}
	return s.index.DeleteWorkspace(id)
}

func (s *Service) Scan(id string) (models.ScanResult, error) {
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return models.ScanResult{}, err
	}
	if !ok {
		return models.ScanResult{}, apperrors.ErrWorkspaceNotFound
	}
	data, err := s.scanner.Scan(workspace)
	if err != nil {
		return models.ScanResult{}, err
	}
	scannedAt := time.Now().UTC()
	if err := s.index.ReplaceWorkspace(workspace.ID, data.Items, data.Warnings, scannedAt); err != nil {
		return models.ScanResult{}, err
	}
	_ = s.registry.TouchScanned(workspace.ID, scannedAt)
	return models.ScanResult{
		WorkspaceID: workspace.ID,
		ScannedAt:   scannedAt,
		ItemCount:   len(data.Items),
		Warnings:    data.Warnings,
	}, nil
}

func (s *Service) LoadBranch(id string, input models.BranchLoadInput) (models.BranchLoadResult, error) {
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return models.BranchLoadResult{}, err
	}
	if !ok {
		return models.BranchLoadResult{}, apperrors.ErrWorkspaceNotFound
	}
	if s.git == nil {
		s.git = gitadapter.New()
	}
	currentCheckoutBranch, err := s.git.CurrentBranch(workspace.Path)
	if err != nil {
		return models.BranchLoadResult{}, err
	}
	selectedBranch := strings.TrimSpace(input.Branch)
	if selectedBranch == "" {
		selectedBranch = firstNonEmpty(workspace.LastSelectedBranch, workspace.BaselineBranch, currentCheckoutBranch)
	}
	ref, commit, err := s.git.ResolveBranch(workspace.Path, selectedBranch)
	if err != nil {
		return models.BranchLoadResult{}, err
	}
	sourceMode := "snapshot"
	editable := false
	reader := scanner.SourceReader(scanner.NewGitTreeSourceReader(workspace.Path, ref, s.git))
	if selectedBranch == currentCheckoutBranch {
		sourceMode = "working_tree"
		editable = true
		reader = scanner.NewFilesystemSourceReader(workspace.Path)
	}
	sourceHash := sourceConfigurationHash(workspace)
	if !input.Force {
		if metadata, ok, err := s.index.BranchScan(workspace.ID, selectedBranch); err != nil {
			return models.BranchLoadResult{}, err
		} else if ok && metadata.Commit == commit && metadata.SourceConfigurationHash == sourceHash {
			items, err := s.index.BranchItems(workspace.ID, selectedBranch)
			if err != nil {
				return models.BranchLoadResult{}, err
			}
			_ = s.registry.SetLastSelectedBranch(workspace.ID, selectedBranch)
			return branchLoadResult(workspace.ID, selectedBranch, ref, commit, currentCheckoutBranch, sourceMode, editable, metadata.ScannedAt, metadata.Warnings, items), nil
		}
	}
	data, err := s.scanner.ScanWithRequest(scanner.ScanRequest{
		Workspace:  workspace,
		Branch:     selectedBranch,
		BranchRef:  ref,
		Commit:     commit,
		SourceMode: sourceMode,
		Editable:   editable,
		Reader:     reader,
	})
	if err != nil {
		return models.BranchLoadResult{}, err
	}
	scannedAt := time.Now().UTC()
	metadata := models.BranchScanMetadata{
		WorkspaceID:             workspace.ID,
		Branch:                  selectedBranch,
		BranchRef:               ref,
		Commit:                  commit,
		SourceMode:              sourceMode,
		Editable:                editable,
		SourceConfigurationHash: sourceHash,
		ScannedAt:               scannedAt,
		Warnings:                data.Warnings,
	}
	if err := s.index.ReplaceWorkspaceBranch(workspace.ID, selectedBranch, data.Items, metadata); err != nil {
		return models.BranchLoadResult{}, err
	}
	_ = s.registry.TouchScanned(workspace.ID, scannedAt)
	_ = s.registry.SetLastSelectedBranch(workspace.ID, selectedBranch)
	items, err := s.index.BranchItems(workspace.ID, selectedBranch)
	if err != nil {
		return models.BranchLoadResult{}, err
	}
	return branchLoadResult(workspace.ID, selectedBranch, ref, commit, currentCheckoutBranch, sourceMode, editable, scannedAt, data.Warnings, items), nil
}

func (s *Service) SourceStructure(id, directory string) (models.SourceSettingsResult, error) {
	root, cleanDirectory, err := s.sourceRoot(id, directory)
	if err != nil {
		return models.SourceSettingsResult{}, err
	}
	settings, exists, warnings := scanner.ReadSourceStructureSettings(root)
	mode := scanner.SourceSettingsMode(root)
	if !exists && mode == "structured" {
		settings = scanner.BuiltInStructuredSettings()
	}
	if warnings == nil {
		warnings = []models.ScanWarning{}
	}
	return models.SourceSettingsResult{
		Directory: cleanDirectory,
		Exists:    exists,
		Mode:      mode,
		Settings:  settings,
		Warnings:  warnings,
	}, nil
}

func (s *Service) SaveSourceStructure(id, directory string, settings models.SourceStructureSettings) (SourceStructureSaveResult, error) {
	root, cleanDirectory, err := s.sourceRoot(id, directory)
	if err != nil {
		return SourceStructureSaveResult{}, err
	}
	if warnings := scanner.ValidateSourceStructureSettings(settings); len(warnings) > 0 {
		return SourceStructureSaveResult{}, fmt.Errorf(warnings[0].Message)
	}
	if err := scanner.WriteSourceStructureSettings(root, settings); err != nil {
		return SourceStructureSaveResult{}, err
	}
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return SourceStructureSaveResult{}, err
	}
	if !ok {
		return SourceStructureSaveResult{}, apperrors.ErrWorkspaceNotFound
	}
	scanResult, err := s.writer.RefreshWorkspace(workspace)
	if err != nil {
		return SourceStructureSaveResult{}, err
	}
	return SourceStructureSaveResult{
		SourceSettingsResult: models.SourceSettingsResult{
			Directory: cleanDirectory,
			Exists:    true,
			Mode:      scanner.SourceSettingsMode(root),
			Settings:  settings,
			Warnings:  NonNilWarnings(scanResult.Warnings),
		},
		Scan: scanResult,
	}, nil
}

func (s *Service) sourceRoot(id, directory string) (string, string, error) {
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", apperrors.ErrWorkspaceNotFound
	}
	cleanDirectory := filepath.ToSlash(filepath.Clean(strings.TrimSpace(directory)))
	if cleanDirectory == "." || cleanDirectory == "" || filepath.IsAbs(cleanDirectory) || strings.HasPrefix(cleanDirectory, "../") || cleanDirectory == ".." {
		return "", "", fmt.Errorf("source directory is invalid")
	}
	allowed := false
	for _, source := range workspace.Sources {
		if cleanDirectory == source {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", "", fmt.Errorf("source directory is not registered")
	}
	root := filepath.Join(workspace.Path, filepath.FromSlash(cleanDirectory))
	info, err := os.Stat(root)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("source directory is not a directory")
	}
	return root, cleanDirectory, nil
}

func NonNilWarnings(warnings []models.ScanWarning) []models.ScanWarning {
	if warnings == nil {
		return []models.ScanWarning{}
	}
	return warnings
}

func sourceConfigurationHash(workspace models.WorkspaceConfig) string {
	payload := struct {
		Sources []string `json:"sources"`
	}{Sources: workspace.Sources}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func branchLoadResult(workspaceID, branch, ref, commit, checkout, sourceMode string, editable bool, scannedAt time.Time, warnings []models.ScanWarning, items []models.ItemSummary) models.BranchLoadResult {
	if warnings == nil {
		warnings = []models.ScanWarning{}
	}
	if items == nil {
		items = []models.ItemSummary{}
	}
	return models.BranchLoadResult{
		WorkspaceID:           workspaceID,
		Branch:                branch,
		SelectedBranch:        branch,
		BranchRef:             ref,
		Commit:                commit,
		CurrentCheckoutBranch: checkout,
		SourceMode:            sourceMode,
		Mode:                  sourceMode,
		Editable:              editable,
		ScannedAt:             scannedAt,
		ItemCount:             len(items),
		Warnings:              warnings,
		Items:                 items,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

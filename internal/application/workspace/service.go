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
}

func New(reg *registry.Registry, idx *itemindex.Index, scan *scanner.Scanner, writer *itemwriter.Writer) *Service {
	return &Service{registry: reg, index: idx, scanner: scan, writer: writer}
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

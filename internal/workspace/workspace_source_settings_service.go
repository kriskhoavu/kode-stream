package workspace

import (
	"fmt"
	"path/filepath"
	"strings"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/sourceguard"
	"kode-stream/internal/workspace/scanner"
)

func (s *Service) SourceStructure(id, directory string) (models.SourceSettingsResult, error) {
	return s.sources.SourceStructure(id, directory)
}

func (s *SourceSettingsService) SourceStructure(id, directory string) (models.SourceSettingsResult, error) {
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
	reader := scanner.NewFilesystemSourceReader(filepath.Dir(root))
	proposals, preview := scanner.SourceStructureProposals(reader, cleanDirectory, settings)
	return models.SourceSettingsResult{
		Directory: cleanDirectory,
		Exists:    exists,
		Mode:      mode,
		Settings:  settings,
		Warnings:  warnings,
		Proposals: proposals,
		Preview:   preview,
	}, nil
}

func (s *Service) SaveSourceStructure(id, directory string, settings models.SourceStructureSettings) (SourceStructureSaveResult, error) {
	return s.sources.SaveSourceStructure(id, directory, settings)
}

func (s *SourceSettingsService) SaveSourceStructure(id, directory string, settings models.SourceStructureSettings) (SourceStructureSaveResult, error) {
	root, cleanDirectory, err := s.sourceRoot(id, directory)
	if err != nil {
		return SourceStructureSaveResult{}, err
	}
	if warnings := scanner.ValidateSourceStructureSettings(settings); len(warnings) > 0 {
		return SourceStructureSaveResult{}, fmt.Errorf("%s", warnings[0].Message)
	}
	previous, _, existed, err := scanner.ReadSourceStructureSettingsFile(root)
	if err != nil {
		return SourceStructureSaveResult{}, err
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
		if restoreErr := scanner.RestoreSourceStructureSettings(root, previous, existed); restoreErr != nil {
			return SourceStructureSaveResult{}, fmt.Errorf("refresh failed: %v; settings rollback failed: %w", err, restoreErr)
		}
		return SourceStructureSaveResult{}, fmt.Errorf("refresh failed; settings were restored: %w", err)
	}
	return SourceStructureSaveResult{
		SourceSettingsResult: models.SourceSettingsResult{
			Directory: cleanDirectory,
			Exists:    true,
			Mode:      scanner.SourceSettingsMode(root),
			Settings:  settings,
			Warnings:  NonNilWarnings(scanResult.Warnings),
			Preview:   sourceStructurePreview(root, cleanDirectory, settings),
		},
		Scan: scanResult,
	}, nil
}

func (s *Service) ResetSourceStructure(id, directory string) (SourceStructureSaveResult, error) {
	return s.sources.ResetSourceStructure(id, directory)
}

func (s *SourceSettingsService) ResetSourceStructure(id, directory string) (SourceStructureSaveResult, error) {
	root, cleanDirectory, err := s.sourceRoot(id, directory)
	if err != nil {
		return SourceStructureSaveResult{}, err
	}
	previous, _, existed, err := scanner.ReadSourceStructureSettingsFile(root)
	if err != nil {
		return SourceStructureSaveResult{}, err
	}
	if err := scanner.RemoveSourceStructureSettings(root); err != nil {
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
		if restoreErr := scanner.RestoreSourceStructureSettings(root, previous, existed); restoreErr != nil {
			return SourceStructureSaveResult{}, fmt.Errorf("refresh failed: %v; settings rollback failed: %w", err, restoreErr)
		}
		return SourceStructureSaveResult{}, fmt.Errorf("refresh failed; settings were restored: %w", err)
	}
	result, err := s.SourceStructure(id, cleanDirectory)
	if err != nil {
		return SourceStructureSaveResult{}, err
	}
	result.Warnings = NonNilWarnings(scanResult.Warnings)
	return SourceStructureSaveResult{
		SourceSettingsResult: result,
		Scan:                 scanResult,
	}, nil
}

func sourceStructurePreview(root, cleanDirectory string, settings models.SourceStructureSettings) []models.SourceStructurePreview {
	if len(settings.Cards) == 0 {
		return []models.SourceStructurePreview{}
	}
	return scanner.PreviewSourceStructureCard(scanner.NewFilesystemSourceReader(filepath.Dir(root)), cleanDirectory, settings.Cards[0])
}

func (s *SourceSettingsService) sourceRoot(id, directory string) (string, string, error) {
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
	resolved, err := sourceguard.ResolveAll(workspace.Path, workspace.Sources)
	if err != nil {
		return "", "", fmt.Errorf("configured source boundary is unsafe: %w", err)
	}
	for _, source := range resolved {
		if source.Relative == cleanDirectory {
			return source.RealPath, cleanDirectory, nil
		}
	}
	return "", "", fmt.Errorf("source directory is not registered")
}

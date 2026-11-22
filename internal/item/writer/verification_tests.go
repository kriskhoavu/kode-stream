package itemwriter

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/guardedwrite"
)

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

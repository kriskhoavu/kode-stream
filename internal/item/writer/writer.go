package itemwriter

// Package itemwriter persists and refreshes Item domain files.

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/content"
	"kode-stream/internal/filesystem/guardedwrite"
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

package workspace

import (
	"context"
	"fmt"
	"sync"
	"time"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	"kode-stream/internal/workspace/scanner"
)

func (s *Service) Scan(id string) (models.ScanResult, error) {
	return s.scans.Scan(id)
}

func (s *Service) ScanContext(ctx context.Context, id string) (models.ScanResult, error) {
	return s.scans.ScanContext(ctx, id)
}

func (s *ScanService) Scan(id string) (models.ScanResult, error) {
	return s.ScanContext(context.Background(), id)
}

func (s *ScanService) ScanContext(ctx context.Context, id string) (models.ScanResult, error) {
	lockValue, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	if err := ctx.Err(); err != nil {
		return models.ScanResult{}, err
	}
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return models.ScanResult{}, err
	}
	if !ok {
		return models.ScanResult{}, apperrors.ErrWorkspaceNotFound
	}
	data, err := s.scanner.ScanContext(ctx, workspace, scanner.DefaultScanBudget())
	if err != nil {
		return models.ScanResult{}, err
	}
	scannedAt := time.Now().UTC()
	if err := s.index.ReplaceWorkspace(workspace.ID, data.Items, data.Warnings, scannedAt); err != nil {
		return models.ScanResult{}, err
	}
	if err := s.registry.TouchScanned(workspace.ID, scannedAt); err != nil {
		return models.ScanResult{}, fmt.Errorf("item index was replaced but scan metadata could not be persisted; retry scan: %w", err)
	}
	return models.ScanResult{
		WorkspaceID: workspace.ID,
		ScannedAt:   scannedAt,
		ItemCount:   len(data.Items),
		Warnings:    data.Warnings,
	}, nil
}

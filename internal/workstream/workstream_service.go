package workstream

// Package workstream owns branch-scoped planning context for a workspace.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/sourceguard"
	gitadapter "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
)

var (
	ErrBranchReviewRequired  = errors.New("non-checkout branches must be opened in Branch Review")
	ErrReviewMatchesCheckout = errors.New("the reviewed branch is already the current checkout")
)

type Service struct {
	registry registry.Repository
	index    itemindex.Repository
	scanner  *scanner.Scanner
	git      *gitadapter.GitAdapter
}

func New(reg registry.Repository, idx itemindex.Repository, scan *scanner.Scanner, git *gitadapter.GitAdapter) *Service {
	return &Service{registry: reg, index: idx, scanner: scan, git: git}
}

func (s *Service) LoadBranch(id string, input models.WorkstreamBranchLoadInput) (models.WorkstreamBranchLoadResult, error) {
	return s.LoadBranchContext(context.Background(), id, input)
}

func (s *Service) LoadBranchContext(ctx context.Context, id string, input models.WorkstreamBranchLoadInput) (models.WorkstreamBranchLoadResult, error) {
	return s.loadCheckout(ctx, id, strings.TrimSpace(input.Branch), input.Force)
}

func (s *Service) LoadCheckout(id string, force bool) (models.WorkstreamBranchLoadResult, error) {
	return s.LoadCheckoutContext(context.Background(), id, force)
}
func (s *Service) LoadCheckoutContext(ctx context.Context, id string, force bool) (models.WorkstreamBranchLoadResult, error) {
	return s.loadCheckout(ctx, id, "", force)
}

func (s *Service) loadCheckout(ctx context.Context, id, requestedBranch string, force bool) (models.WorkstreamBranchLoadResult, error) {
	workspace, err := s.workspace(id)
	if err != nil {
		return models.WorkstreamBranchLoadResult{}, err
	}
	var result models.WorkstreamBranchLoadResult
	err = s.git.WithWorkspaceMutation(workspace.Path, func() error {
		currentCheckoutBranch, err := s.git.CurrentBranch(workspace.Path)
		if err != nil {
			return err
		}
		if requestedBranch != "" && requestedBranch != currentCheckoutBranch {
			return ErrBranchReviewRequired
		}
		result, err = s.load(ctx, workspace, currentCheckoutBranch, currentCheckoutBranch, force, false)
		return err
	})
	return result, err
}

func (s *Service) ReviewBranch(id string, input models.WorkstreamBranchLoadInput) (models.WorkstreamBranchLoadResult, error) {
	return s.ReviewBranchContext(context.Background(), id, input)
}
func (s *Service) ReviewBranchContext(ctx context.Context, id string, input models.WorkstreamBranchLoadInput) (models.WorkstreamBranchLoadResult, error) {
	workspace, err := s.workspace(id)
	if err != nil {
		return models.WorkstreamBranchLoadResult{}, err
	}
	selectedBranch := strings.TrimSpace(input.Branch)
	if selectedBranch == "" {
		return models.WorkstreamBranchLoadResult{}, errors.New("review branch is required")
	}
	var result models.WorkstreamBranchLoadResult
	err = s.git.WithWorkspaceMutation(workspace.Path, func() error {
		currentCheckoutBranch, err := s.git.CurrentBranch(workspace.Path)
		if err != nil {
			return err
		}
		if selectedBranch == currentCheckoutBranch {
			return ErrReviewMatchesCheckout
		}
		result, err = s.load(ctx, workspace, selectedBranch, currentCheckoutBranch, input.Force, true)
		return err
	})
	return result, err
}

func (s *Service) workspace(id string) (models.WorkspaceConfig, error) {
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	if !ok {
		return models.WorkspaceConfig{}, apperrors.ErrWorkspaceNotFound
	}
	if s.git == nil {
		s.git = gitadapter.New()
	}
	return workspace, nil
}

func (s *Service) load(ctx context.Context, workspace models.WorkspaceConfig, selectedBranch, currentCheckoutBranch string, force, snapshot bool) (models.WorkstreamBranchLoadResult, error) {
	if err := ctx.Err(); err != nil {
		return models.WorkstreamBranchLoadResult{}, err
	}
	ref, commit, err := s.git.ResolveBranch(workspace.Path, selectedBranch)
	if err != nil {
		return models.WorkstreamBranchLoadResult{}, err
	}
	sourceMode, editable := "snapshot", false
	reader := scanner.SourceReader(scanner.NewGitTreeSourceReader(workspace.Path, commit, s.git))
	if !snapshot {
		sourceMode = "working_tree"
		editable = true
		reader = scanner.NewFilesystemSourceReader(workspace.Path)
	}
	sourceHash := sourceConfigurationHash(workspace)
	workingTreeHash := ""
	if sourceMode == "working_tree" {
		workingTreeHash, err = workingTreeSourceHash(workspace.Path, workspace.Sources)
		if err != nil {
			return models.WorkstreamBranchLoadResult{}, err
		}
	}
	if !force {
		if metadata, ok, err := s.index.BranchScan(workspace.ID, selectedBranch); err != nil {
			return models.WorkstreamBranchLoadResult{}, err
		} else if ok &&
			metadata.Commit == commit &&
			metadata.SourceMode == sourceMode &&
			metadata.SourceConfigurationHash == sourceHash &&
			(sourceMode != "working_tree" || metadata.WorkingTreeHash == workingTreeHash) {
			items, err := s.index.BranchItems(workspace.ID, selectedBranch)
			if err != nil {
				return models.WorkstreamBranchLoadResult{}, err
			}
			return branchLoadResult(workspace.ID, selectedBranch, ref, commit, currentCheckoutBranch, sourceMode, editable, metadata.ScannedAt, metadata.Warnings, items), nil
		}
	}
	data, err := s.scanner.ScanWithRequestContext(ctx, scanner.ScanRequest{
		Workspace:  workspace,
		Branch:     selectedBranch,
		BranchRef:  ref,
		Commit:     commit,
		SourceMode: sourceMode,
		Editable:   editable,
		Reader:     reader,
	})
	if err != nil {
		return models.WorkstreamBranchLoadResult{}, err
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
		WorkingTreeHash:         workingTreeHash,
		ScannedAt:               scannedAt,
		Warnings:                data.Warnings,
	}
	if err := s.index.ReplaceWorkspaceBranch(workspace.ID, selectedBranch, data.Items, metadata); err != nil {
		return models.WorkstreamBranchLoadResult{}, err
	}
	_ = s.registry.TouchScanned(workspace.ID, scannedAt)
	items, err := s.index.BranchItems(workspace.ID, selectedBranch)
	if err != nil {
		return models.WorkstreamBranchLoadResult{}, err
	}
	return branchLoadResult(workspace.ID, selectedBranch, ref, commit, currentCheckoutBranch, sourceMode, editable, scannedAt, data.Warnings, items), nil
}

func sourceConfigurationHash(workspace models.WorkspaceConfig) string {
	payload := struct {
		Sources []string `json:"sources"`
	}{Sources: workspace.Sources}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func workingTreeSourceHash(root string, sources []string) (string, error) {
	resolved, err := sourceguard.ResolveAll(root, sources)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	entries := 0
	const maxHashEntries = 100000
	for _, source := range resolved {
		err := filepath.WalkDir(source.RealPath, func(path string, entry fs.DirEntry, walkErr error) error {
			if os.IsNotExist(walkErr) {
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			entries++
			if entries > maxHashEntries {
				return fmt.Errorf("working tree change detection exceeds %d files", maxHashEntries)
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			relativePath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(hash, "%s\x00%d\x00%d\x00", filepath.ToSlash(relativePath), info.Size(), info.ModTime().UnixNano())
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func branchLoadResult(workspaceID, branch, ref, commit, checkout, sourceMode string, editable bool, scannedAt time.Time, warnings []models.ScanWarning, items []models.ItemSummary) models.WorkstreamBranchLoadResult {
	if warnings == nil {
		warnings = []models.ScanWarning{}
	}
	if items == nil {
		items = []models.ItemSummary{}
	}
	return models.WorkstreamBranchLoadResult{
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

package workstream

// Package workstream owns branch-scoped planning context for a workspace.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
)

type Service struct {
	registry *registry.Registry
	index    *itemindex.Index
	scanner  *scanner.Scanner
	git      *gitadapter.GitAdapter
}

func New(reg *registry.Registry, idx *itemindex.Index, scan *scanner.Scanner, git *gitadapter.GitAdapter) *Service {
	return &Service{registry: reg, index: idx, scanner: scan, git: git}
}

func (s *Service) LoadBranch(id string, input models.WorkstreamBranchLoadInput) (models.WorkstreamBranchLoadResult, error) {
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return models.WorkstreamBranchLoadResult{}, err
	}
	if !ok {
		return models.WorkstreamBranchLoadResult{}, apperrors.ErrWorkspaceNotFound
	}
	if s.git == nil {
		s.git = gitadapter.New()
	}
	currentCheckoutBranch, err := s.git.CurrentBranch(workspace.Path)
	if err != nil {
		return models.WorkstreamBranchLoadResult{}, err
	}
	selectedBranch := strings.TrimSpace(input.Branch)
	if selectedBranch == "" {
		selectedBranch = firstNonEmpty(workspace.LastSelectedBranch, workspace.BaselineBranch, currentCheckoutBranch)
	}
	ref, commit, err := s.git.ResolveBranch(workspace.Path, selectedBranch)
	if err != nil {
		return models.WorkstreamBranchLoadResult{}, err
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
	// A commit hash only identifies a Git snapshot. It says nothing about
	// uncommitted additions, edits, or deletions in the checked-out working tree.
	// Cache immutable branch snapshots, but always rescan the working tree.
	if !input.Force && sourceMode != "working_tree" {
		if metadata, ok, err := s.index.BranchScan(workspace.ID, selectedBranch); err != nil {
			return models.WorkstreamBranchLoadResult{}, err
		} else if ok && metadata.Commit == commit && metadata.SourceConfigurationHash == sourceHash {
			items, err := s.index.BranchItems(workspace.ID, selectedBranch)
			if err != nil {
				return models.WorkstreamBranchLoadResult{}, err
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
		ScannedAt:               scannedAt,
		Warnings:                data.Warnings,
	}
	if err := s.index.ReplaceWorkspaceBranch(workspace.ID, selectedBranch, data.Items, metadata); err != nil {
		return models.WorkstreamBranchLoadResult{}, err
	}
	_ = s.registry.TouchScanned(workspace.ID, scannedAt)
	_ = s.registry.SetLastSelectedBranch(workspace.ID, selectedBranch)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

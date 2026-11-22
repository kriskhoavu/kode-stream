package item

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/content"
	gitadapter "kode-stream/internal/git"
	"kode-stream/internal/item/index"
	"kode-stream/internal/item/writer"
	"kode-stream/internal/workspace/registry"
)

var (
	ErrSnapshotReadOnly     = errors.New("snapshot item is read-only; import it into the checkout before editing")
	ErrSnapshotReviewOnly   = errors.New("snapshot item is available only in Branch Review")
	ErrReviewCommitMoved    = errors.New("reviewed branch changed since it was loaded")
	ErrReviewCheckoutMoved  = errors.New("checkout changed since branch review was loaded")
	ErrReviewedPlan         = errors.New("reviewed snapshot plan is required")
	ErrSnapshotTreeTooLarge = errors.New("reviewed item tree exceeds the response limit")
	ErrSnapshotFileTooLarge = errors.New("reviewed file exceeds the response limit")
)

const (
	snapshotTreeEntryLimit = 10000
	snapshotTreeDepthLimit = 64
)

type ListInput struct {
	WorkspaceID string
	Branch      string
	Status      string
	Text        string
}

type AuditContext struct {
	WorkspaceID string
	ItemID      string
	ItemPath    string
}

type ItemService struct {
	registry registry.Repository
	index    itemindex.Repository
	files    *fileaccess.Access
	writer   *itemwriter.Writer
	git      *gitadapter.GitAdapter
}

type Service = ItemService

func New(reg registry.Repository, idx itemindex.Repository, files *fileaccess.Access, writer *itemwriter.Writer, git *gitadapter.GitAdapter) *ItemService {
	return &ItemService{registry: reg, index: idx, files: files, writer: writer, git: git}
}

func (s *Service) List(input ListInput) ([]models.ItemSummary, error) {
	items, err := s.index.Query(itemindex.Query{
		WorkspaceID: input.WorkspaceID,
		Branch:      input.Branch,
		Status:      input.Status,
		Text:        input.Text,
	})
	for i := range items {
		items[i] = NormalizeSummary(items[i])
	}
	return items, err
}

func (s *Service) Detail(id string) (models.ItemDetail, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return models.ItemDetail{}, err
	}
	if item.SourceMode == "snapshot" {
		return models.ItemDetail{}, ErrSnapshotReviewOnly
	}
	item.Description = s.fullDescription(workspace, item)
	if s.writer != nil {
		if revision, revisionErr := s.writer.MetadataRevision(workspace, item); revisionErr == nil {
			item.MetadataRevision = revision
		}
	}
	return NormalizeDetail(item), nil
}

func (s *Service) AuditContext(id string) (AuditContext, error) {
	item, ok, err := s.index.Get(id)
	if err != nil {
		return AuditContext{}, err
	}
	if !ok {
		return AuditContext{}, apperrors.ErrItemNotFound
	}
	return AuditContext{WorkspaceID: item.WorkspaceID, ItemID: item.ID, ItemPath: item.ItemPath}, nil
}

func (s *Service) Files(id, expectedCommit string) ([]models.FileNode, error) {
	return s.FilesContext(context.Background(), id, expectedCommit)
}

func (s *Service) FilesContext(ctx context.Context, id, expectedCommit string) ([]models.FileNode, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return nil, err
	}
	if err := requireExpectedSnapshotCommit(item, expectedCommit); err != nil {
		return nil, err
	}
	if item.SourceMode == "snapshot" {
		return s.snapshotFiles(ctx, workspace, item)
	}
	return s.files.Tree(workspace, item)
}

func (s *Service) FileContent(id, fileID, expectedCommit string) (models.FileContent, error) {
	return s.FileContentContext(context.Background(), id, fileID, expectedCommit)
}

func (s *Service) FileContentContext(ctx context.Context, id, fileID, expectedCommit string) (models.FileContent, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return models.FileContent{}, err
	}
	if err := requireExpectedSnapshotCommit(item, expectedCommit); err != nil {
		return models.FileContent{}, err
	}
	if item.SourceMode == "snapshot" {
		return s.snapshotFileContent(ctx, workspace, item, fileID)
	}
	return s.files.Read(workspace, item, fileID)
}

func requireExpectedSnapshotCommit(item models.ItemDetail, expectedCommit string) error {
	if item.SourceMode == "snapshot" && (strings.TrimSpace(expectedCommit) == "" || item.Commit != strings.TrimSpace(expectedCommit)) {
		return ErrReviewCommitMoved
	}
	return nil
}

func (s *Service) Diff(id string) (string, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return "", err
	}
	if item.SourceMode == "snapshot" {
		return "", ErrSnapshotReviewOnly
	}
	diff, err := s.git.Diff(workspace.Path, item.ItemPath)
	if err != nil {
		return "", fmt.Errorf("diff unavailable: %w", err)
	}
	return diff, nil
}

func (s *Service) SaveFile(id, fileID string, input models.FileSaveInput) (models.FileContent, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return models.FileContent{}, err
	}
	var saved models.FileContent
	err = s.withItemMutation(workspace, item, func(current models.ItemDetail) error {
		input.FileID = fileID
		var writeErr error
		saved, writeErr = s.files.WriteMarkdown(workspace, current, input)
		if writeErr != nil {
			return writeErr
		}
		// Content edits affect the derived description/documents projection.
		_, writeErr = s.writer.RefreshWorkspace(workspace)
		if writeErr != nil {
			saved.Committed, saved.RefreshRequired, saved.RefreshError = true, true, writeErr.Error()
			return nil
		}
		saved.Committed = true
		return nil
	})
	return saved, err
}

func (s *Service) ImportReviewedPlan(workspaceID string, input models.ReviewedPlanImportInput) (models.WriteResult, error) {
	itemID := strings.TrimSpace(input.ItemID)
	workspace, item, err := s.workspaceAndItem(itemID)
	if err != nil {
		return models.WriteResult{}, err
	}
	if workspace.ID != strings.TrimSpace(workspaceID) {
		return models.WriteResult{}, apperrors.ErrItemNotFound
	}
	if item.SourceMode != "snapshot" || item.Branch != strings.TrimSpace(input.SourceBranch) || isDocumentationItem(item) {
		return models.WriteResult{}, ErrReviewedPlan
	}
	expectedCommit := strings.TrimSpace(input.ExpectedCommit)
	if expectedCommit == "" || item.Commit != expectedCommit {
		return models.WriteResult{}, ErrReviewCommitMoved
	}
	expectedCheckout := strings.TrimSpace(input.ExpectedCheckoutBranch)
	if expectedCheckout == "" {
		return models.WriteResult{}, ErrReviewCheckoutMoved
	}
	var result models.WriteResult
	err = s.git.WithWorkspaceMutation(workspace.Path, func() error {
		ref, commit, err := s.git.ResolveBranch(workspace.Path, item.Branch)
		if err != nil {
			return err
		}
		if commit != expectedCommit {
			return ErrReviewCommitMoved
		}
		currentCheckout, err := s.git.CurrentBranch(workspace.Path)
		if err != nil {
			return err
		}
		if currentCheckout != expectedCheckout {
			return ErrReviewCheckoutMoved
		}
		item.BranchRef = ref
		result, err = s.writer.ImportSnapshotPlan(workspace, item)
		return err
	})
	return result, err
}

func (s *Service) RevertFile(id, fileID string, validatePaths func(models.WorkspaceConfig, []string) error) (models.ScanResult, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return models.ScanResult{}, err
	}
	var result models.ScanResult
	err = s.withItemMutation(workspace, item, func(current models.ItemDetail) error {
		relPath, err := s.files.RelativePath(workspace, current, fileID)
		if err != nil {
			return err
		}
		gitPath := filepath.ToSlash(filepath.Join(current.ItemPath, relPath))
		if err := validatePaths(workspace, []string{gitPath}); err != nil {
			return err
		}
		if err := s.git.RevertPaths(workspace.Path, []string{gitPath}); err != nil {
			return err
		}
		result, err = s.writer.RefreshWorkspace(workspace)
		return err
	})
	return result, err
}

func (s *Service) SaveMetadata(id string, input models.ItemMetadataUpdateInput) (models.WriteResult, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return models.WriteResult{}, err
	}
	var result models.WriteResult
	err = s.withItemMutation(workspace, item, func(current models.ItemDetail) error {
		var e error
		result, e = s.writer.SaveMetadata(workspace, current, input)
		return e
	})
	return result, err
}

func (s *Service) UpdateStatus(id string, input models.ItemStatusUpdateInput) (models.WriteResult, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return models.WriteResult{}, err
	}
	var result models.WriteResult
	err = s.withItemMutation(workspace, item, func(current models.ItemDetail) error {
		var e error
		result, e = s.writer.UpdateStatus(workspace, current, input)
		return e
	})
	return result, err
}

func (s *Service) Create(input models.NewItemInput) (models.WriteResult, error) {
	workspace, ok, err := s.registry.Get(input.WorkspaceID)
	if err != nil {
		return models.WriteResult{}, err
	}
	if !ok {
		return models.WriteResult{}, apperrors.ErrWorkspaceNotFound
	}
	var result models.WriteResult
	err = s.git.WithWorkspaceMutation(workspace.Path, func() error { var e error; result, e = s.writer.CreateItem(workspace, input); return e })
	return result, err
}

// withItemMutation is the single branch-proof/mutation/index-publication unit
// for checkout items. It prevents a branch switch between validation and the
// final write or refresh.
func (s *Service) withItemMutation(workspace models.WorkspaceConfig, item models.ItemDetail, operation func(models.ItemDetail) error) error {
	if item.SourceMode == "snapshot" {
		return ErrSnapshotReadOnly
	}
	if s.git == nil {
		s.git = gitadapter.New()
	}
	return s.git.WithWorkspaceMutation(workspace.Path, func() error {
		if err := s.requireCurrentCheckoutBranch(workspace, item); err != nil {
			// Lightweight file-backed fixtures without a repository remain
			// supported for Local recovery operations; production workspaces
			// always prove their checkout under this same lock.
			if !strings.Contains(err.Error(), "not a git repository") {
				return err
			}
		}
		return operation(item)
	})
}

func (s *Service) workspaceAndItem(itemID string) (models.WorkspaceConfig, models.ItemDetail, error) {
	item, ok, err := s.index.Get(itemID)
	if err != nil {
		return models.WorkspaceConfig{}, models.ItemDetail{}, err
	}
	if !ok {
		return models.WorkspaceConfig{}, models.ItemDetail{}, apperrors.ErrItemNotFound
	}
	workspace, ok, err := s.registry.Get(item.WorkspaceID)
	if err != nil {
		return models.WorkspaceConfig{}, models.ItemDetail{}, err
	}
	if !ok {
		return models.WorkspaceConfig{}, models.ItemDetail{}, apperrors.ErrWorkspaceNotFound
	}
	if item.ItemPath == "" {
		item.ItemPath = FallbackPath(workspace, item)
	}
	return workspace, item, nil
}

func isDocumentationItem(item models.ItemDetail) bool {
	return item.MetadataSource == "docs" || item.MetadataSource == "wiki"
}

func (s *Service) requireCurrentCheckoutBranch(workspace models.WorkspaceConfig, item models.ItemDetail) error {
	if item.SourceMode == "snapshot" {
		return ErrSnapshotReadOnly
	}
	current, err := s.git.CurrentBranch(workspace.Path)
	if err != nil {
		return err
	}
	if item.Branch != "" && current != item.Branch {
		return fmt.Errorf("item branch %q is not the current checkout branch %q", item.Branch, current)
	}
	return nil
}

func (s *Service) snapshotFiles(ctx context.Context, workspace models.WorkspaceConfig, item models.ItemDetail) ([]models.FileNode, error) {
	commit, err := snapshotCommit(item)
	if err != nil {
		return nil, err
	}
	entries, err := s.git.TreeWalkBounded(ctx, workspace.Path, commit, item.ItemPath, snapshotTreeEntryLimit, snapshotTreeDepthLimit)
	if err != nil {
		if errors.Is(err, gitadapter.ErrTreeLimit) {
			return nil, ErrSnapshotTreeTooLarge
		}
		return nil, err
	}
	nodes := []models.FileNode{}
	for _, entry := range entries {
		if entry.Type.IsDir() {
			continue
		}
		rel, err := filepath.Rel(filepath.FromSlash(item.ItemPath), filepath.FromSlash(entry.Path))
		if err != nil {
			continue
		}
		nodes = insertFileNode(nodes, filepath.ToSlash(rel))
	}
	sortFileNodes(nodes)
	return nodes, nil
}

func (s *Service) snapshotFileContent(ctx context.Context, workspace models.WorkspaceConfig, item models.ItemDetail, fileID string) (models.FileContent, error) {
	commit, err := snapshotCommit(item)
	if err != nil {
		return models.FileContent{}, err
	}
	candidates := []string{}
	if relPath := fileIDToRelativePath(item, fileID); relPath != "" {
		candidates = append(candidates, relPath)
	}
	nodes, err := s.snapshotFiles(ctx, workspace, item)
	if err != nil {
		return models.FileContent{}, err
	}
	for _, node := range flattenFileNodes(nodes) {
		if node.ID == fileID {
			candidates = append(candidates, node.Path)
			break
		}
	}

	uniqueCandidates := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		uniqueCandidates = append(uniqueCandidates, candidate)
	}
	if len(uniqueCandidates) == 0 {
		return models.FileContent{}, fmt.Errorf("file not found")
	}

	var (
		data    []byte
		relPath string
		lastErr error
	)
	for _, candidate := range uniqueCandidates {
		path := filepath.ToSlash(filepath.Join(item.ItemPath, candidate))
		limit := fileaccess.MaxTextResponseBytes
		if fileaccess.ClassifyPath(candidate).Kind == models.FileKindImage {
			limit = fileaccess.MaxImageResponseBytes
		}
		attempt, _, _, readErr := s.git.TreeReadFileBounded(ctx, workspace.Path, commit, path, limit)
		if readErr != nil {
			if errors.Is(readErr, gitadapter.ErrBlobLimit) {
				return models.FileContent{}, ErrSnapshotFileTooLarge
			}
			lastErr = readErr
			if strings.Contains(readErr.Error(), "does not exist in") {
				continue
			}
			return models.FileContent{}, readErr
		}
		data = attempt
		relPath = candidate
		break
	}
	if relPath == "" {
		if lastErr != nil {
			return models.FileContent{}, lastErr
		}
		return models.FileContent{}, fmt.Errorf("file not found")
	}
	return fileaccess.FileContentFromBytes(relPath, data), nil
}

func (s *Service) fullDescription(workspace models.WorkspaceConfig, item models.ItemDetail) string {
	if item.SourceMode == "snapshot" && item.Commit != "" && item.ItemPath != "" {
		data, err := s.git.TreeReadFile(workspace.Path, item.Commit, filepath.ToSlash(filepath.Join(item.ItemPath, "README.md")))
		if err == nil {
			if description := FirstMarkdownParagraph(string(data)); description != "" {
				return description
			}
		}
	}
	return FullReadmeDescription(workspace, item)
}

func snapshotCommit(item models.ItemDetail) (string, error) {
	commit := strings.TrimSpace(item.Commit)
	if commit == "" {
		return "", fmt.Errorf("snapshot commit is missing")
	}
	return commit, nil
}

func FallbackPath(workspace models.WorkspaceConfig, item models.ItemDetail) string {
	sources := workspace.SourcesShownIn(models.SourceSurfaceWorkstream)
	if len(sources) == 0 {
		sources = workspace.SourcePaths()
	}
	if len(sources) == 0 || item.Scope == "" || item.Identifier == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(sources[0], item.Scope, item.Identifier))
}

func FullReadmeDescription(workspace models.WorkspaceConfig, item models.ItemDetail) string {
	if item.ItemPath == "" {
		return item.Description
	}
	readme := filepath.Join(workspace.Path, filepath.FromSlash(item.ItemPath), "README.md")
	data, err := os.ReadFile(readme)
	if err != nil {
		return item.Description
	}
	if description := FirstMarkdownParagraph(string(data)); description != "" {
		return description
	}
	return item.Description
}

func NormalizeSummary(item models.ItemSummary) models.ItemSummary {
	if item.Tags == nil {
		item.Tags = []string{}
	}
	return item
}

func NormalizeDetail(item models.ItemDetail) models.ItemDetail {
	item.ItemSummary = NormalizeSummary(item.ItemSummary)
	if item.Documents == nil {
		item.Documents = []models.ItemDocument{}
	}
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	return item
}

func FirstMarkdownParagraph(markdown string) string {
	for _, block := range strings.Split(markdown, "\n\n") {
		clean := strings.TrimSpace(block)
		if clean == "" || strings.HasPrefix(clean, "#") || strings.HasPrefix(clean, "|") {
			continue
		}
		return regexp.MustCompile(`\s+`).ReplaceAllString(clean, " ")
	}
	return ""
}

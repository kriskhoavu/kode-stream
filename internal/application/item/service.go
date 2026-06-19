package item

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"plan-manager/internal/application/apperrors"
	"plan-manager/internal/fileaccess"
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/itemindex"
	"plan-manager/internal/itemwriter"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
)

type ListInput struct {
	WorkspaceID string
	Branch      string
	Status      string
	Text        string
}

type Service struct {
	registry *registry.Registry
	index    *itemindex.Index
	files    *fileaccess.Access
	writer   *itemwriter.Writer
	git      *gitadapter.GitAdapter
}

func New(reg *registry.Registry, idx *itemindex.Index, files *fileaccess.Access, writer *itemwriter.Writer, git *gitadapter.GitAdapter) *Service {
	return &Service{registry: reg, index: idx, files: files, writer: writer, git: git}
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
	item.Description = FullReadmeDescription(workspace, item)
	return NormalizeDetail(item), nil
}

func (s *Service) Files(id string) ([]models.FileNode, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return nil, err
	}
	return s.files.Tree(workspace, item)
}

func (s *Service) FileContent(id, fileID string) (models.FileContent, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return models.FileContent{}, err
	}
	return s.files.Read(workspace, item, fileID)
}

func (s *Service) Diff(id string) (string, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return "", err
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
	input.FileID = fileID
	return s.files.WriteMarkdown(workspace, item, input)
}

func (s *Service) RevertFile(id, fileID string, validatePaths func(models.WorkspaceConfig, []string) error) (models.ScanResult, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return models.ScanResult{}, err
	}
	relPath, err := s.files.RelativePath(workspace, item, fileID)
	if err != nil {
		return models.ScanResult{}, err
	}
	gitPath := filepath.ToSlash(filepath.Join(item.ItemPath, relPath))
	if err := validatePaths(workspace, []string{gitPath}); err != nil {
		return models.ScanResult{}, err
	}
	if err := s.git.RevertPaths(workspace.Path, []string{gitPath}); err != nil {
		return models.ScanResult{}, err
	}
	return s.writer.RefreshWorkspace(workspace)
}

func (s *Service) SaveMetadata(id string, input models.ItemMetadataUpdateInput) (models.WriteResult, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return models.WriteResult{}, err
	}
	return s.writer.SaveMetadata(workspace, item, input)
}

func (s *Service) UpdateStatus(id string, input models.ItemStatusUpdateInput) (models.WriteResult, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return models.WriteResult{}, err
	}
	return s.writer.UpdateStatus(workspace, item, input)
}

func (s *Service) Create(input models.NewItemInput) (models.WriteResult, error) {
	workspace, ok, err := s.registry.Get(input.WorkspaceID)
	if err != nil {
		return models.WriteResult{}, err
	}
	if !ok {
		return models.WriteResult{}, apperrors.ErrWorkspaceNotFound
	}
	return s.writer.CreateItem(workspace, input)
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

func FallbackPath(workspace models.WorkspaceConfig, item models.ItemDetail) string {
	if len(workspace.Sources) == 0 || item.Scope == "" || item.Identifier == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(workspace.Sources[0], item.Scope, item.Identifier))
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

package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
	"kode-stream/internal/item/index"
	"kode-stream/internal/item/writer"
	"kode-stream/internal/system"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
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

type CreateResult struct {
	Workspace    models.WorkspaceConfig `json:"workspace" yaml:"workspace"`
	OperationLog string                 `json:"operationLog,omitempty" yaml:"operationLog,omitempty"`
}

type WorkspaceService struct {
	registry *registry.Registry
	index    *itemindex.Index
	scanner  *scanner.Scanner
	writer   *itemwriter.Writer
	git      *gitadapter.GitAdapter
	audit    interface {
		Append(models.AuditEvent) (models.AuditEvent, error)
	}
	importScan func(string) (models.ScanResult, error)
}

func (s *WorkspaceService) ConfigureAudit(store interface {
	Append(models.AuditEvent) (models.AuditEvent, error)
}) *WorkspaceService {
	s.audit = store
	return s
}

type Service = WorkspaceService

func New(reg *registry.Registry, idx *itemindex.Index, scan *scanner.Scanner, writer *itemwriter.Writer, git ...*gitadapter.GitAdapter) *WorkspaceService {
	var adapter *gitadapter.GitAdapter
	if len(git) > 0 {
		adapter = git[0]
	}
	return &WorkspaceService{registry: reg, index: idx, scanner: scan, writer: writer, git: adapter}
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

func (s *Service) Runtime(id string) (*models.WorkspaceRuntimeConfig, error) {
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperrors.ErrWorkspaceNotFound
	}
	return workspace.Runtime, nil
}

func (s *Service) SaveRuntime(id string, runtimeConfig *models.WorkspaceRuntimeConfig) (*models.WorkspaceRuntimeConfig, error) {
	workspace, err := s.registry.SetRuntime(id, runtimeConfig)
	if err != nil {
		if strings.Contains(err.Error(), "workspace not found") {
			return nil, apperrors.ErrWorkspaceNotFound
		}
		return nil, err
	}
	return workspace.Runtime, nil
}

func (s *Service) Create(input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	result, err := s.CreateWithResult(input)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	return result.Workspace, nil
}

func (s *Service) CreateWithResult(input models.WorkspaceInput) (CreateResult, error) {
	return s.CreateWithResultStreaming(input, nil)
}

func (s *Service) CreateWithResultStreaming(input models.WorkspaceInput, onLog func(string)) (CreateResult, error) {
	mode := normalizeRegistrationMode(input.RegistrationMode)
	input.RegistrationMode = mode
	operationLog := ""
	if mode == models.WorkspaceRegistrationModeRemoteClone {
		resolved, cloneLog, err := s.prepareRemoteClone(input, onLog)
		operationLog = cloneLog
		if err != nil {
			return CreateResult{OperationLog: operationLog}, err
		}
		input = resolved
	}
	workspace, err := s.registry.Create(input)
	if err != nil {
		return CreateResult{OperationLog: operationLog}, err
	}
	return CreateResult{Workspace: workspace, OperationLog: operationLog}, nil
}

func (s *Service) Update(id string, input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	return s.registry.Update(id, input)
}

func (s *Service) Delete(id string) error {
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("workspace not found")
	}
	if workspace.ClonePathManaged {
		if err := removeManagedCloneWorkspace(workspace.Path); err != nil {
			return err
		}
	}
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
	root, cleanDirectory, err := s.sourceRoot(id, directory)
	if err != nil {
		return SourceStructureSaveResult{}, err
	}
	if warnings := scanner.ValidateSourceStructureSettings(settings); len(warnings) > 0 {
		return SourceStructureSaveResult{}, fmt.Errorf("%s", warnings[0].Message)
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
			Preview:   sourceStructurePreview(root, cleanDirectory, settings),
		},
		Scan: scanResult,
	}, nil
}

func (s *Service) ResetSourceStructure(id, directory string) (SourceStructureSaveResult, error) {
	root, cleanDirectory, err := s.sourceRoot(id, directory)
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
		return SourceStructureSaveResult{}, err
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *Service) prepareRemoteClone(input models.WorkspaceInput, onLog func(string)) (models.WorkspaceInput, string, error) {
	if s.git == nil {
		s.git = gitadapter.New()
	}
	remoteURL := strings.TrimSpace(input.RemoteURL)
	if !validRemoteURL(remoteURL) {
		return models.WorkspaceInput{}, "", fmt.Errorf("remote URL must be a valid HTTPS or SSH Git URL")
	}
	cloneRoot, err := resolveCloneRoot(input.CloneRoot)
	if err != nil {
		return models.WorkspaceInput{}, "", err
	}
	repoName := remoteRepositoryName(remoteURL)
	if repoName == "" {
		return models.WorkspaceInput{}, "", fmt.Errorf("remote URL must include a repository name")
	}
	destination := filepath.Join(cloneRoot, repoName)
	if err := ensureCloneDestination(destination); err != nil {
		return models.WorkspaceInput{}, "", err
	}
	var cloneLogBuilder strings.Builder
	err = s.git.CloneWithProgress(remoteURL, destination, func(chunk string) {
		cloneLogBuilder.WriteString(chunk)
		if onLog != nil {
			onLog(chunk)
		}
	})
	cloneLog := strings.TrimSpace(cloneLogBuilder.String())
	if err != nil {
		return models.WorkspaceInput{}, cloneLog, fmt.Errorf("clone failed: %w", err)
	}
	input.Path = destination
	input.RemoteURL = remoteURL
	input.CloneRoot = cloneRoot
	return input, cloneLog, nil
}

func resolveCloneRoot(root string) (string, error) {
	clean := strings.TrimSpace(root)
	if clean == "" {
		paths, err := system.ResolvePaths()
		if err != nil {
			return "", err
		}
		clean = paths.CloneRootDir
	}
	clean = expandHome(clean)
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("clone root is invalid")
	}
	stat, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("clone root does not exist")
	}
	if !stat.IsDir() {
		return "", fmt.Errorf("clone root must be a directory")
	}
	return abs, nil
}

func ensureCloneDestination(destination string) error {
	info, err := os.Stat(destination)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("clone destination already exists and is not a directory")
	}
	entries, err := os.ReadDir(destination)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("clone destination already exists and is not empty")
	}
	return nil
}

func validRemoteURL(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" || strings.Contains(value, " ") {
		return false
	}
	if parsed, err := url.Parse(value); err == nil {
		scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
		if (scheme == "https" || scheme == "ssh") && parsed.Host != "" && strings.Trim(parsed.Path, "/") != "" {
			return true
		}
		if scheme == "file" && strings.Trim(parsed.Path, "/") != "" {
			return true
		}
	}
	scpPattern := regexp.MustCompile(`^[^\s@]+@[^\s:]+:[^\s]+$`)
	return scpPattern.MatchString(value)
}

func remoteRepositoryName(remoteURL string) string {
	trimmed := strings.TrimSpace(remoteURL)
	if trimmed == "" {
		return ""
	}
	pathPart := ""
	if parsed, err := url.Parse(trimmed); err == nil && (parsed.Host != "" || strings.ToLower(strings.TrimSpace(parsed.Scheme)) == "file") {
		pathPart = parsed.Path
	} else if before, after, ok := strings.Cut(trimmed, ":"); ok && strings.Contains(before, "@") {
		pathPart = after
	}
	pathPart = strings.Trim(pathPart, "/")
	pathPart = strings.TrimSuffix(pathPart, ".git")
	base := filepath.Base(filepath.Clean(filepath.FromSlash(pathPart)))
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	valid := regexp.MustCompile(`[^a-zA-Z0-9._-]+`).ReplaceAllString(base, "-")
	valid = strings.Trim(valid, "-._")
	if valid == "" {
		return ""
	}
	return valid
}

func normalizeRegistrationMode(mode models.WorkspaceRegistrationMode) models.WorkspaceRegistrationMode {
	switch strings.TrimSpace(string(mode)) {
	case string(models.WorkspaceRegistrationModeRemoteClone):
		return models.WorkspaceRegistrationModeRemoteClone
	case string(models.WorkspaceRegistrationModeExisting):
		return models.WorkspaceRegistrationModeExisting
	default:
		return models.WorkspaceRegistrationModeLocalPath
	}
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func removeManagedCloneWorkspace(path string) error {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return fmt.Errorf("managed clone path is invalid")
	}
	clean = filepath.Clean(clean)
	if clean == "." || clean == string(filepath.Separator) {
		return fmt.Errorf("managed clone path is invalid")
	}
	if _, err := os.Stat(clean); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return os.RemoveAll(clean)
}

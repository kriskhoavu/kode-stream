package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	"kode-stream/internal/item/index"
	"kode-stream/internal/item/writer"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
)

type StateResult struct {
	Version        string                     `json:"version"`
	WorkspaceCount int                        `json:"workspaceCount"`
	ItemCount      int                        `json:"itemCount"`
	UpdatedAt      time.Time                  `json:"updatedAt"`
	Mode           models.RuntimeMode         `json:"mode"`
	User           *models.CloudUser          `json:"user,omitempty"`
	Role           models.CloudRole           `json:"role,omitempty"`
	Capabilities   map[models.Capability]bool `json:"capabilities,omitempty"`
	Agent          models.AgentConnection     `json:"agent"`
}

type SourceStructureSaveResult struct {
	models.SourceSettingsResult
	Scan models.ScanResult `json:"scan" yaml:"scan"`
}

type CreateResult struct {
	Workspace    models.WorkspaceConfig `json:"workspace" yaml:"workspace"`
	OperationLog string                 `json:"operationLog,omitempty" yaml:"operationLog,omitempty"`
}

type Service struct {
	registry  registry.Repository
	index     itemindex.Repository
	lifecycle *LifecycleService
	scans     *ScanService
	sources   *SourceSettingsService
	audit     interface {
		Append(models.AuditEvent) (models.AuditEvent, error)
	}
	importScan func(string) (models.ScanResult, error)
}

// LifecycleService owns registration, managed clone ownership, and deletion.
type LifecycleService struct {
	registry  registry.Repository
	index     itemindex.Repository
	cloner    ClonePort
	removeAll func(string) error
}

// ScanService owns bounded indexing and per-workspace refresh serialization.
type ScanService struct {
	registry registry.Repository
	index    itemindex.Repository
	scanner  *scanner.Scanner
	locks    sync.Map
}

// SourceSettingsService owns validated source settings and refresh compensation.
type SourceSettingsService struct {
	registry registry.Repository
	writer   *itemwriter.Writer
}

type AuditAppender interface {
	Append(models.AuditEvent) (models.AuditEvent, error)
}

type ClonePort interface {
	CloneWithProgress(string, string, func(string)) error
}

type ServiceDependencies struct {
	Registry registry.Repository
	Index    itemindex.Repository
	Scanner  *scanner.Scanner
	Writer   *itemwriter.Writer
	Cloner   ClonePort
	Audit    AuditAppender
}

func New(deps ServiceDependencies) *Service {
	return &Service{
		registry:  deps.Registry,
		index:     deps.Index,
		lifecycle: &LifecycleService{registry: deps.Registry, index: deps.Index, cloner: deps.Cloner, removeAll: os.RemoveAll},
		scans:     &ScanService{registry: deps.Registry, index: deps.Index, scanner: deps.Scanner},
		sources:   &SourceSettingsService{registry: deps.Registry, writer: deps.Writer},
		audit:     deps.Audit,
	}
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

func NonNilWarnings(warnings []models.ScanWarning) []models.ScanWarning {
	if warnings == nil {
		return []models.ScanWarning{}
	}
	return warnings
}

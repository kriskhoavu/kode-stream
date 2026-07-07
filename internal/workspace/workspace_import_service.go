package workspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"plan-manager/internal/common/models"
)

const (
	maxWorkspaceImportBytes   = 1 << 20
	maxWorkspaceImportEntries = 500
)

func (s *Service) PreviewImport(sourcePath string) (models.WorkspaceImportPreview, error) {
	source, data, records, err := readWorkspaceImport(sourcePath)
	if err != nil {
		return models.WorkspaceImportPreview{}, err
	}
	existing, err := s.registry.List()
	if err != nil {
		return models.WorkspaceImportPreview{}, fmt.Errorf("read effective workspace registry: %w", err)
	}
	existingPaths := make([]string, 0, len(existing))
	for _, workspace := range existing {
		existingPaths = append(existingPaths, workspace.Path)
	}

	preview := models.WorkspaceImportPreview{
		SourcePath:        source,
		DestinationPath:   s.registry.Path(),
		SourceFingerprint: digest(data),
		Candidates:        make([]models.WorkspaceImportCandidate, 0, len(records)),
	}
	seen := map[string]struct{}{}
	for index, record := range records {
		input := importInput(record)
		candidate := models.WorkspaceImportCandidate{
			CandidateKey: candidateKey(index+1, input),
			Position:     index + 1,
			Workspace:    input,
			Status:       "valid",
			Issues:       []models.WorkspaceImportIssue{},
			Selected:     true,
		}
		normalized, validationErr := s.registry.Validate(input)
		if validationErr != nil {
			candidate.Status = "invalid"
			candidate.Selected = false
			candidate.Issues = append(candidate.Issues, importIssue(validationErr))
			preview.Summary.Invalid++
			preview.Candidates = append(preview.Candidates, candidate)
			continue
		}
		candidate.Workspace = models.WorkspaceInput{
			Name: normalized.Name, Path: normalized.Path, BaselineBranch: normalized.BaselineBranch,
			Sources: normalized.Sources, RegistrationMode: models.WorkspaceRegistrationModeExisting,
			RemoteURL: strings.TrimSpace(record.RemoteURL), Jira: normalized.Jira, Knowledge: normalized.Knowledge,
		}
		candidate.CandidateKey = candidateKey(index+1, candidate.Workspace)
		pathKey := canonicalPathKey(normalized.Path)
		if _, duplicate := seen[pathKey]; duplicate {
			candidate.Status = "duplicate"
			candidate.Selected = false
			candidate.Issues = append(candidate.Issues, models.WorkspaceImportIssue{Field: "path", Code: "duplicate_source", Message: "workspace path is repeated in the import source"})
			preview.Summary.Duplicate++
		} else if containsPath(existingPaths, normalized.Path) {
			candidate.Status = "already_registered"
			candidate.Selected = false
			candidate.Issues = append(candidate.Issues, models.WorkspaceImportIssue{Field: "path", Code: "already_registered", Message: "workspace path is already registered"})
			preview.Summary.AlreadyRegistered++
		} else {
			preview.Summary.Valid++
		}
		seen[pathKey] = struct{}{}
		preview.Candidates = append(preview.Candidates, candidate)
	}
	return preview, nil
}

func (s *Service) Import(request models.WorkspaceImportRequest) ([]models.WorkspaceImportResult, error) {
	preview, err := s.PreviewImport(request.SourcePath)
	if err != nil {
		return nil, err
	}
	selected := make([]string, 0, len(request.CandidateKeys))
	selectedSet := map[string]struct{}{}
	for _, key := range request.CandidateKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := selectedSet[key]; exists {
			continue
		}
		selectedSet[key] = struct{}{}
		selected = append(selected, key)
	}
	results := make([]models.WorkspaceImportResult, len(selected))
	resultIndex := make(map[string]int, len(selected))
	for i, key := range selected {
		resultIndex[key] = i
		results[i] = models.WorkspaceImportResult{CandidateKey: key, Status: "skipped", Message: "candidate no longer exists in the import source"}
	}

	inputs := make([]models.WorkspaceInput, 0, len(selected))
	inputKeys := make([]string, 0, len(selected))
	for _, candidate := range preview.Candidates {
		index, wanted := resultIndex[candidate.CandidateKey]
		if !wanted {
			continue
		}
		if candidate.Status != "valid" {
			results[index].Message = importSkipMessage(candidate.Status)
			continue
		}
		input := candidate.Workspace
		input.RegistrationMode = models.WorkspaceRegistrationModeExisting
		inputs = append(inputs, input)
		inputKeys = append(inputKeys, candidate.CandidateKey)
	}
	if len(inputs) == 0 {
		return results, nil
	}

	created, persistErr := s.registry.BatchCreate(inputs)
	if persistErr != nil {
		for _, key := range inputKeys {
			index := resultIndex[key]
			results[index].Status = "failed"
			results[index].Message = "workspace registry could not be updated"
		}
		s.recordImportAudit("", models.AuditStatusFailed, "Workspace import registry update failed.")
		return results, nil
	}
	for i, outcome := range created {
		key := inputKeys[i]
		index := resultIndex[key]
		if outcome.Err != nil {
			results[index].Status = "skipped"
			results[index].Message = "workspace became invalid or was already registered"
			continue
		}
		workspace := outcome.Workspace
		results[index].Workspace = &workspace
		s.recordImportAudit(workspace.ID, models.AuditStatusSuccess, "Workspace imported.")
		if s.importScan == nil && (s.scanner == nil || s.index == nil) {
			results[index].Status = "scan_failed"
			results[index].Message = "workspace was registered but indexing is unavailable"
			s.recordScanAudit(workspace.ID, models.AuditStatusFailed, "Imported workspace indexing failed.")
			continue
		}
		var scan models.ScanResult
		var scanErr error
		if s.importScan != nil {
			scan, scanErr = s.importScan(workspace.ID)
		} else {
			scan, scanErr = s.Scan(workspace.ID)
		}
		if scanErr != nil {
			results[index].Status = "scan_failed"
			results[index].Message = "workspace was registered but indexing failed"
			s.recordScanAudit(workspace.ID, models.AuditStatusFailed, "Imported workspace indexing failed.")
			continue
		}
		results[index].Status = "indexed"
		results[index].Scan = &scan
		results[index].Message = "workspace imported and indexed"
		s.recordScanAudit(workspace.ID, models.AuditStatusSuccess, "Imported workspace indexed.")
	}
	return results, nil
}

func importSkipMessage(status string) string {
	switch status {
	case "duplicate":
		return "workspace path is duplicated in the import source"
	case "already_registered":
		return "workspace is already registered"
	default:
		return "workspace is no longer valid"
	}
}

func (s *Service) recordImportAudit(workspaceID string, status models.AuditStatus, message string) {
	if s.audit == nil {
		return
	}
	_, _ = s.audit.Append(models.AuditEvent{WorkspaceID: workspaceID, Operation: "workspace_import", Status: status, Message: message, Paths: []string{}})
}

func (s *Service) recordScanAudit(workspaceID string, status models.AuditStatus, message string) {
	if s.audit == nil {
		return
	}
	_, _ = s.audit.Append(models.AuditEvent{WorkspaceID: workspaceID, Operation: "workspace_import_scan", Status: status, Message: message, Paths: []string{}})
}

func readWorkspaceImport(sourcePath string) (string, []byte, []models.WorkspaceConfig, error) {
	rawPath := strings.TrimSpace(sourcePath)
	if rawPath == "" || !filepath.IsAbs(rawPath) {
		return "", nil, nil, errors.New("import source path must be absolute")
	}
	extension := strings.ToLower(filepath.Ext(rawPath))
	if extension != ".yaml" && extension != ".yml" {
		return "", nil, nil, errors.New("import source must be a YAML file")
	}
	info, err := os.Stat(rawPath)
	if err != nil {
		return "", nil, nil, fmt.Errorf("import source is not readable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", nil, nil, errors.New("import source must be a regular file")
	}
	if info.Size() > maxWorkspaceImportBytes {
		return "", nil, nil, errors.New("import source exceeds the 1 MiB limit")
	}
	file, err := os.Open(rawPath)
	if err != nil {
		return "", nil, nil, fmt.Errorf("import source is not readable: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxWorkspaceImportBytes+1))
	if err != nil {
		return "", nil, nil, fmt.Errorf("import source is not readable: %w", err)
	}
	if len(data) > maxWorkspaceImportBytes {
		return "", nil, nil, errors.New("import source exceeds the 1 MiB limit")
	}
	var document yaml.Node
	nodeDecoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := nodeDecoder.Decode(&document); err != nil {
		return "", nil, nil, fmt.Errorf("import source is not valid YAML: %w", err)
	}
	if containsYAMLAlias(&document) {
		return "", nil, nil, errors.New("import source must not contain YAML aliases")
	}
	var extra yaml.Node
	if err := nodeDecoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return "", nil, nil, errors.New("import source must contain one YAML document")
		}
		return "", nil, nil, fmt.Errorf("import source is not valid YAML: %w", err)
	}
	var records []models.WorkspaceConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&records); err != nil {
		return "", nil, nil, fmt.Errorf("import source does not match the current workspace schema: %w", err)
	}
	if len(records) > maxWorkspaceImportEntries {
		return "", nil, nil, errors.New("import source exceeds the 500 workspace limit")
	}
	canonical, err := filepath.EvalSymlinks(rawPath)
	if err != nil {
		canonical, err = filepath.Abs(rawPath)
		if err != nil {
			return "", nil, nil, errors.New("import source path is invalid")
		}
	}
	return canonical, data, records, nil
}

func importInput(record models.WorkspaceConfig) models.WorkspaceInput {
	return models.WorkspaceInput{
		Name: strings.TrimSpace(record.Name), Path: strings.TrimSpace(record.Path),
		BaselineBranch: strings.TrimSpace(record.BaselineBranch), Sources: append([]string(nil), record.Sources...),
		RegistrationMode: models.WorkspaceRegistrationModeExisting, RemoteURL: strings.TrimSpace(record.RemoteURL),
		Jira: record.Jira, Knowledge: record.Knowledge,
	}
}

func candidateKey(position int, input models.WorkspaceInput) string {
	data, _ := json.Marshal(struct {
		Position  int                   `json:"position"`
		Workspace models.WorkspaceInput `json:"workspace"`
	}{Position: position, Workspace: input})
	return digest(data)
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func containsYAMLAlias(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return true
	}
	for _, child := range node.Content {
		if containsYAMLAlias(child) {
			return true
		}
	}
	return false
}

func canonicalPathKey(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = resolved
	}
	return filepath.Clean(path)
}

func containsPath(paths []string, target string) bool {
	target = canonicalPathKey(target)
	for _, path := range paths {
		if canonicalPathKey(path) == target {
			return true
		}
	}
	return false
}

func importIssue(err error) models.WorkspaceImportIssue {
	message := err.Error()
	field := "workspace"
	code := "invalid_workspace"
	for _, candidate := range []struct{ match, field, code string }{
		{"name", "name", "invalid_name"}, {"path", "path", "invalid_path"},
		{"branch", "baselineBranch", "invalid_branch"}, {"source", "sources", "invalid_source"},
		{"Jira", "jira", "invalid_jira"},
	} {
		if strings.Contains(message, candidate.match) {
			field, code = candidate.field, candidate.code
			break
		}
	}
	return models.WorkspaceImportIssue{Field: field, Code: code, Message: message}
}

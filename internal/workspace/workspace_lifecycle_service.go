package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/system"
)

func (s *Service) Create(input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	return s.lifecycle.Create(input)
}

func (s *LifecycleService) Create(input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	result, err := s.CreateWithResult(input)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	return result.Workspace, nil
}

func (s *Service) CreateWithResult(input models.WorkspaceInput) (CreateResult, error) {
	return s.lifecycle.CreateWithResult(input)
}

func (s *LifecycleService) CreateWithResult(input models.WorkspaceInput) (CreateResult, error) {
	return s.CreateWithResultStreaming(input, nil)
}

func (s *Service) CreateWithResultStreaming(input models.WorkspaceInput, onLog func(string)) (CreateResult, error) {
	return s.lifecycle.CreateWithResultStreaming(input, onLog)
}

func (s *LifecycleService) CreateWithResultStreaming(input models.WorkspaceInput, onLog func(string)) (CreateResult, error) {
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
		if input.ManagedCloneID != "" {
			_ = removeManagedCloneWorkspace(models.WorkspaceConfig{Path: input.Path, ClonePathManaged: true, ManagedCloneRoot: input.ManagedCloneRoot, ManagedCloneID: input.ManagedCloneID, ManagedCloneVerified: true}, s.removeAll)
		}
		return CreateResult{OperationLog: operationLog}, err
	}
	return CreateResult{Workspace: workspace, OperationLog: operationLog}, nil
}

func (s *Service) Update(id string, input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	return s.lifecycle.Update(id, input)
}

func (s *LifecycleService) Update(id string, input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	return s.registry.Update(id, input)
}

func (s *Service) Delete(id string) error { return s.lifecycle.Delete(id) }

func (s *LifecycleService) Delete(id string) error {
	workspace, ok, err := s.registry.Get(id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("workspace not found")
	}
	if workspace.ClonePathManaged {
		if !workspace.ManagedCloneCleanupPending {
			marker, supported := s.registry.(interface{ MarkManagedCloneCleanup(string) error })
			if !supported {
				return errors.New("managed clone cleanup persistence is unavailable")
			}
			if err := marker.MarkManagedCloneCleanup(id); err != nil {
				return fmt.Errorf("record managed clone cleanup intent: %w", err)
			}
			workspace.ManagedCloneCleanupPending = true
		}
		if err := removeManagedCloneWorkspace(workspace, s.removeAll); err != nil {
			return fmt.Errorf("managed clone cleanup was not performed; workspace remains registered for retry: %w", err)
		}
	}
	if deleter, ok := s.registry.(interface{ DeleteWorkspaceState(string) error }); ok {
		if err := deleter.DeleteWorkspaceState(id); err != nil {
			return err
		}
	} else {
		// File-backed indexes are derived and recoverable. Delete them first so a
		// registry write failure keeps the workspace registered for a retry.
		if err := s.index.DeleteWorkspace(id); err != nil {
			return err
		}
		if err := s.registry.Delete(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *LifecycleService) prepareRemoteClone(input models.WorkspaceInput, onLog func(string)) (models.WorkspaceInput, string, error) {
	if s.cloner == nil {
		return models.WorkspaceInput{}, "", errors.New("workspace clone service is unavailable")
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
	err = s.cloner.CloneWithProgress(remoteURL, destination, func(chunk string) {
		cloneLogBuilder.WriteString(chunk)
		if onLog != nil {
			onLog(chunk)
		}
	})
	cloneLog := strings.TrimSpace(cloneLogBuilder.String())
	if err != nil {
		_ = os.RemoveAll(destination)
		return models.WorkspaceInput{}, cloneLog, fmt.Errorf("clone failed: %w", err)
	}
	markerID, err := newManagedCloneID()
	if err != nil {
		_ = os.RemoveAll(destination)
		return models.WorkspaceInput{}, cloneLog, err
	}
	if err := writeManagedCloneMarker(destination, markerID); err != nil {
		_ = os.RemoveAll(destination)
		return models.WorkspaceInput{}, cloneLog, fmt.Errorf("record managed clone ownership: %w", err)
	}
	input.Path = destination
	input.RemoteURL = remoteURL
	input.CloneRoot = cloneRoot
	input.ManagedCloneRoot = cloneRoot
	input.ManagedCloneID = markerID
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
	_, err := os.Stat(destination)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("clone destination already exists")
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
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" {
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

const managedCloneMarker = ".git/kode-stream-managed-clone"

func newManagedCloneID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func writeManagedCloneMarker(root, id string) error {
	path := filepath.Join(root, filepath.FromSlash(managedCloneMarker))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(id + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func removeManagedCloneWorkspace(workspace models.WorkspaceConfig, removeAll func(string) error) error {
	if !workspace.ClonePathManaged || !workspace.ManagedCloneVerified || workspace.ManagedCloneID == "" || workspace.ManagedCloneRoot == "" {
		return fmt.Errorf("managed clone ownership cannot be proven")
	}
	clean := strings.TrimSpace(workspace.Path)
	if clean == "" {
		return fmt.Errorf("managed clone path is invalid")
	}
	clean = filepath.Clean(clean)
	if clean == "." || clean == string(filepath.Separator) {
		return fmt.Errorf("managed clone path is invalid")
	}
	root, err := filepath.EvalSymlinks(workspace.ManagedCloneRoot)
	if err != nil {
		return fmt.Errorf("managed clone root is unavailable: %w", err)
	}
	real, err := filepath.EvalSymlinks(clean)
	if os.IsNotExist(err) {
		if workspace.ManagedCloneCleanupPending {
			return nil
		}
		return fmt.Errorf("managed clone path is missing and cleanup was not authorized")
	}
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("managed clone path is outside its trusted root")
	}
	marker, err := os.ReadFile(filepath.Join(real, filepath.FromSlash(managedCloneMarker)))
	if err != nil || strings.TrimSpace(string(marker)) != workspace.ManagedCloneID {
		return fmt.Errorf("managed clone ownership marker is missing or does not match")
	}
	if _, err := os.Stat(real); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if removeAll == nil {
		return errors.New("managed clone cleanup operation is unavailable")
	}
	return removeAll(real)
}

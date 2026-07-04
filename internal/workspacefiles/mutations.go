package workspacefiles

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"plan-manager/internal/models"
)

var (
	ErrInvalidName       = errors.New("invalid workspace entry name")
	ErrDestinationExists = errors.New("workspace destination already exists")
	ErrRootMutation      = errors.New("workspace root cannot be changed")
	ErrSymlinkMutation   = errors.New("workspace symlinks cannot be changed")
)

func (a *Access) CreateFile(workspace models.WorkspaceConfig, input models.WorkspaceFileCreateInput) (models.WorkspacePathMutationResult, error) {
	name, err := validateEntryName(input.Name)
	if err != nil {
		return models.WorkspacePathMutationResult{}, err
	}
	parentPath, parent, err := resolve(workspace.Path, input.ParentPath, true)
	if err != nil {
		return models.WorkspacePathMutationResult{}, err
	}
	path := joinRelative(parentPath, name)
	full := filepath.Join(parent, name)
	file, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return models.WorkspacePathMutationResult{}, ErrDestinationExists
	}
	if err != nil {
		return models.WorkspacePathMutationResult{}, err
	}
	complete := false
	defer func() {
		_ = file.Close()
		if !complete {
			_ = os.Remove(full)
		}
	}()
	if _, err := file.WriteString(input.Content); err != nil {
		return models.WorkspacePathMutationResult{}, err
	}
	if err := file.Sync(); err != nil {
		return models.WorkspacePathMutationResult{}, err
	}
	if err := file.Close(); err != nil {
		return models.WorkspacePathMutationResult{}, err
	}
	complete = true
	return mutationResult(workspace.ID, path, "file", parentPath), nil
}

func (a *Access) CreateDirectory(workspace models.WorkspaceConfig, input models.WorkspaceDirectoryCreateInput) (models.WorkspacePathMutationResult, error) {
	name, err := validateEntryName(input.Name)
	if err != nil {
		return models.WorkspacePathMutationResult{}, err
	}
	parentPath, parent, err := resolve(workspace.Path, input.ParentPath, true)
	if err != nil {
		return models.WorkspacePathMutationResult{}, err
	}
	path := joinRelative(parentPath, name)
	if err := os.Mkdir(filepath.Join(parent, name), 0o755); errors.Is(err, os.ErrExist) {
		return models.WorkspacePathMutationResult{}, ErrDestinationExists
	} else if err != nil {
		return models.WorkspacePathMutationResult{}, err
	}
	return mutationResult(workspace.ID, path, "directory", parentPath), nil
}

func (a *Access) Rename(workspace models.WorkspaceConfig, input models.WorkspacePathRenameInput) (models.WorkspacePathMutationResult, error) {
	sourcePath, source, info, err := resolveMutationSource(workspace.Path, input.Path)
	if err != nil {
		return models.WorkspacePathMutationResult{}, err
	}
	destinationPath, destination, err := resolveMutationDestination(workspace.Path, input.DestinationPath)
	if err != nil {
		return models.WorkspacePathMutationResult{}, err
	}
	if sourcePath == destinationPath {
		return models.WorkspacePathMutationResult{}, ErrDestinationExists
	}
	if err := os.Rename(source, destination); err != nil {
		return models.WorkspacePathMutationResult{}, err
	}
	typeName := "file"
	if info.IsDir() {
		typeName = "directory"
	}
	return mutationResult(workspace.ID, destinationPath, typeName, parentRelative(sourcePath), parentRelative(destinationPath)), nil
}

func validateEntryName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) || strings.EqualFold(name, ".git") {
		return "", fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	return name, nil
}

func resolveMutationSource(root, path string) (string, string, os.FileInfo, error) {
	clean, err := cleanMutationPath(path)
	if err != nil {
		return "", "", nil, err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", nil, err
	}
	candidate := filepath.Join(realRoot, filepath.FromSlash(clean))
	lstat, err := os.Lstat(candidate)
	if err != nil {
		return "", "", nil, err
	}
	if lstat.Mode()&os.ModeSymlink != 0 {
		return "", "", nil, ErrSymlinkMutation
	}
	realPath, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", "", nil, err
	}
	if !withinRoot(realRoot, realPath) {
		return "", "", nil, ErrOutsideRoot
	}
	info, err := os.Stat(realPath)
	return clean, candidate, info, err
}

func resolveMutationDestination(root, path string) (string, string, error) {
	clean, err := cleanMutationPath(path)
	if err != nil {
		return "", "", err
	}
	name, err := validateEntryName(filepath.Base(filepath.FromSlash(clean)))
	if err != nil {
		return "", "", err
	}
	parentPath := parentRelative(clean)
	_, parent, err := resolve(root, parentPath, true)
	if err != nil {
		return "", "", err
	}
	destination := filepath.Join(parent, name)
	if _, err := os.Lstat(destination); err == nil {
		return "", "", ErrDestinationExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", err
	}
	return clean, destination, nil
}

func cleanMutationPath(path string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if clean == "." || clean == "" {
		return "", ErrRootMutation
	}
	if filepath.IsAbs(path) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%w: %q", ErrInvalidPath, path)
	}
	if protected(clean) {
		return "", fmt.Errorf("%w: %q", ErrProtectedPath, path)
	}
	return clean, nil
}

func parentRelative(path string) string {
	parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(path)))
	if parent == "." {
		return ""
	}
	return parent
}

func mutationResult(workspaceID, path, typeName string, invalidated ...string) models.WorkspacePathMutationResult {
	seen := map[string]bool{}
	paths := make([]string, 0, len(invalidated))
	for _, path := range invalidated {
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	return models.WorkspacePathMutationResult{WorkspaceID: workspaceID, Path: path, Type: typeName, InvalidatedPaths: paths}
}

func withinRoot(root, path string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

package pathguard

// Package pathguard validates workspace-relative paths.

import (
	"fmt"
	"path/filepath"
	"strings"
)

func SafeJoin(root, rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("invalid path")
	}
	full := filepath.Join(root, clean)
	absRoot, _ := filepath.Abs(root)
	absFull, _ := filepath.Abs(full)
	if absFull != absRoot && !strings.HasPrefix(absFull, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root")
	}
	return absFull, nil
}

func CleanRelative(path string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", fmt.Errorf("path %q is invalid", path)
	}
	return clean, nil
}

func ValidateSourcePath(sources []string, path string) (string, error) {
	clean, err := CleanRelative(path)
	if err != nil {
		return "", err
	}
	for _, source := range sources {
		if clean == source || strings.HasPrefix(clean, source+"/") {
			return clean, nil
		}
	}
	return "", fmt.Errorf("path %q is outside configured sources", path)
}

func ValidateSourcePaths(sources []string, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}
	for _, path := range paths {
		if _, err := ValidateSourcePath(sources, path); err != nil {
			return err
		}
	}
	return nil
}

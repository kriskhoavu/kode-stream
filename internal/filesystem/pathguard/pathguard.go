package pathguard

// Package pathguard validates workspace-relative paths.

import (
	"fmt"
	"os"
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

// ResolveExisting returns an existing workspace-relative path after resolving
// symlinks and proving the result remains below root.
func ResolveExisting(root, rel string) (string, error) {
	full, err := SafeJoin(root, rel)
	if err != nil {
		return "", err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	realPath, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", err
	}
	if !within(realRoot, realPath) {
		return "", fmt.Errorf("path escapes root")
	}
	return realPath, nil
}

// ValidateMarkdownFile resolves an existing regular Markdown file below root.
func ValidateMarkdownFile(root, rel string) (string, error) {
	resolved, err := ResolveExisting(root, rel)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	extension := strings.ToLower(filepath.Ext(resolved))
	if !info.Mode().IsRegular() || (extension != ".md" && extension != ".markdown") {
		return "", fmt.Errorf("path is not a Markdown file")
	}
	return resolved, nil
}

// ValidateMarkdownTarget validates a workspace-relative Markdown write target.
// The file may not exist yet, but its nearest existing parent must resolve
// below root so a symlink cannot redirect the eventual write.
func ValidateMarkdownTarget(root, rel string) (string, error) {
	full, err := SafeJoin(root, rel)
	if err != nil {
		return "", err
	}
	extension := strings.ToLower(filepath.Ext(full))
	if extension != ".md" && extension != ".markdown" {
		return "", fmt.Errorf("path is not a Markdown file")
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(full)
	for {
		if _, statErr := os.Stat(parent); statErr == nil {
			break
		} else if !os.IsNotExist(statErr) {
			return "", statErr
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", fmt.Errorf("no existing parent for path")
		}
		parent = next
	}
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if !within(realRoot, realParent) {
		return "", fmt.Errorf("path escapes root")
	}
	return full, nil
}

func within(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
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

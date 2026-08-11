// Package sourceguard validates configured source roots against a real workspace root.
package sourceguard

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Source struct {
	Relative string
	RealPath string
}

func ResolveAll(workspaceRoot string, values []string) ([]Source, error) {
	realRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	realRoot, err = filepath.Abs(realRoot)
	if err != nil {
		return nil, err
	}
	resolved := make([]Source, 0, len(values))
	for _, value := range values {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
		if clean == "" || clean == "." || filepath.IsAbs(value) || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("source %q must be relative", value)
		}
		candidate := filepath.Join(realRoot, filepath.FromSlash(clean))
		real, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			return nil, fmt.Errorf("source %q does not exist", clean)
		}
		info, err := os.Stat(real)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("source %q does not exist", clean)
		}
		if !contained(realRoot, real) {
			return nil, fmt.Errorf("source %q escapes the workspace root", clean)
		}
		resolved = append(resolved, Source{Relative: clean, RealPath: real})
	}
	sort.Slice(resolved, func(i, j int) bool { return key(resolved[i].RealPath) < key(resolved[j].RealPath) })
	for i := range resolved {
		for j := 0; j < i; j++ {
			left, right := resolved[j], resolved[i]
			if key(left.RealPath) == key(right.RealPath) {
				return nil, fmt.Errorf("sources %q and %q resolve to the same directory", left.Relative, right.Relative)
			}
			if contained(left.RealPath, right.RealPath) || contained(right.RealPath, left.RealPath) {
				return nil, fmt.Errorf("sources %q and %q overlap", left.Relative, right.Relative)
			}
		}
	}
	return resolved, nil
}

func ResolveOne(workspaceRoot, value string) (Source, error) {
	values, err := ResolveAll(workspaceRoot, []string{value})
	if err != nil {
		return Source{}, err
	}
	return values[0], nil
}

func contained(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Contains reports whether target is root or a descendant after both paths
// have been resolved by the caller. It is exported for consumers that need to
// prove an item is under one of the validated source roots.
func Contains(root, target string) bool {
	return contained(root, target)
}

func key(value string) string {
	clean := filepath.Clean(value)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.ToLower(clean)
	}
	return clean
}

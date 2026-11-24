package itemwriter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/pathguard"
	"kode-stream/internal/filesystem/sourceguard"
)

func validateSource(workspace models.WorkspaceConfig, dir string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(dir)))
	for _, allowed := range workspace.SourcePaths() {
		if clean == allowed {
			if _, err := sourceguard.ResolveOne(workspace.Path, clean); err != nil {
				return "", err
			}
			return clean, nil
		}
	}
	return "", fmt.Errorf("source is not registered")
}

func isInsideConfiguredSource(workspace models.WorkspaceConfig, rel string) bool {
	_, err := safeItemPathForRelative(workspace, rel)
	return err == nil
}

func safeItemPath(workspace models.WorkspaceConfig, item models.ItemDetail) (string, error) {
	return safeItemPathForRelative(workspace, item.ItemPath)
}

// A source symlink can change after registration, so each write resolves the source again.
func safeItemPathForRelative(workspace models.WorkspaceConfig, relative string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(relative)))
	var selected string
	for _, source := range workspace.SourcePaths() {
		source = filepath.ToSlash(filepath.Clean(strings.TrimSpace(source)))
		if clean == source || strings.HasPrefix(clean, source+"/") {
			selected = source
			break
		}
	}
	if selected == "" {
		return "", fmt.Errorf("item path is outside configured sources")
	}
	resolved, err := sourceguard.ResolveOne(workspace.Path, selected)
	if err != nil {
		// Create only the lexical path below the root, then resolve it. An existing symlink is never followed.
		if !strings.Contains(err.Error(), "does not exist") {
			return "", err
		}
		realRoot, rootErr := filepath.EvalSymlinks(workspace.Path)
		if rootErr != nil {
			return "", rootErr
		}
		candidate, joinErr := safeJoin(realRoot, selected)
		if joinErr != nil {
			return "", joinErr
		}
		if mkdirErr := os.MkdirAll(candidate, 0o755); mkdirErr != nil {
			return "", mkdirErr
		}
		resolved, err = sourceguard.ResolveOne(workspace.Path, selected)
		if err != nil {
			return "", err
		}
	}
	full, err := safeJoin(workspace.Path, relative)
	if err != nil {
		return "", err
	}
	probe := full
	for {
		if _, statErr := os.Lstat(probe); statErr == nil {
			break
		} else if !os.IsNotExist(statErr) {
			return "", statErr
		}
		next := filepath.Dir(probe)
		if next == probe {
			return "", fmt.Errorf("item path is outside configured sources")
		}
		probe = next
	}
	realProbe, err := filepath.EvalSymlinks(probe)
	if err != nil {
		return "", err
	}
	if sourceguard.Contains(resolved.RealPath, realProbe) {
		return full, nil
	}
	return "", fmt.Errorf("item path is outside configured sources")
}

func safeJoin(root, rel string) (string, error) {
	return pathguard.SafeJoin(root, rel)
}

func isDocumentationRoot(item models.ItemDetail) bool {
	return item.MetadataSource == "docs" || item.MetadataSource == "wiki"
}

func cleanTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" && !seen[tag] {
			seen[tag] = true
			out = append(out, tag)
		}
	}
	return out
}

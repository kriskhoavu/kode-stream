package itemwriter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/writeguard"
	"kode-stream/internal/workspace/scanner"
)

func (w *Writer) CreateItem(workspace models.WorkspaceConfig, input models.NewItemInput) (models.WriteResult, error) {
	input.Source = strings.TrimSpace(input.Source)
	input.Scope = strings.TrimSpace(input.Scope)
	input.Identifier = strings.TrimSpace(input.Identifier)
	source, err := validateSource(workspace, input.Source)
	if err != nil {
		return models.WriteResult{}, err
	}
	if err := writeguard.ValidateScopeName(input.Scope); err != nil {
		return models.WriteResult{}, err
	}
	if err := writeguard.ValidateIdentifierName(input.Identifier); err != nil {
		return models.WriteResult{}, err
	}
	status := input.Status
	if status == "" {
		status = models.StatusDraft
	}
	if err := writeguard.ValidateStatus(status); err != nil {
		return models.WriteResult{}, err
	}
	itemRoot, err := w.createItemRoot(workspace, source, input)
	if err != nil {
		return models.WriteResult{}, err
	}
	fullRoot, err := safeItemPathForRelative(workspace, itemRoot)
	if err != nil {
		return models.WriteResult{}, err
	}
	if info, err := os.Stat(fullRoot); err == nil {
		// A manual deletion of an item's contents can leave the directory behind.
		// An empty directory holds no item, so reuse it instead of reporting a
		// duplicate the workspace cannot show.
		empty, emptyErr := isEmptyDir(fullRoot, info)
		if emptyErr != nil {
			return models.WriteResult{}, emptyErr
		}
		if !empty {
			return models.WriteResult{}, fmt.Errorf("item already exists")
		}
	} else if !os.IsNotExist(err) {
		return models.WriteResult{}, err
	}
	if err := os.MkdirAll(fullRoot, 0o755); err != nil {
		return models.WriteResult{}, err
	}
	if err := os.WriteFile(filepath.Join(fullRoot, "README.md"), []byte(input.InitialReadme), 0o644); err != nil {
		return models.WriteResult{}, err
	}
	if strings.TrimSpace(input.JiraKey) != "" {
		meta := planYAML{Plan: planFields{
			Identifier: input.Identifier,
			Title:      strings.TrimSpace(input.Title),
			Scope:      input.Scope,
			Status:     string(status),
			Owner:      strings.TrimSpace(input.Owner),
			Tags:       cleanTags(input.Tags),
		}}
		if err := writePlanMetadataAt(fullRoot, meta); err != nil {
			return models.WriteResult{}, err
		}
	}
	return w.refresh(workspace, itemRoot)
}

// isEmptyDir reports whether path is a directory holding no entries. A
// non-directory is never empty, so an existing file at the item path still
// counts as occupied.
func isEmptyDir(path string, info os.FileInfo) (bool, error) {
	if !info.IsDir() {
		return false, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

var createItemPatternVariable = regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9_]*)\}`)

func (w *Writer) createItemRoot(workspace models.WorkspaceConfig, source string, input models.NewItemInput) (string, error) {
	settings, hasSettings, warnings := scanner.ReadSourceStructureSettings(workspace.Path, source)
	if hasSettings && len(warnings) == 0 && len(settings.Cards) > 0 {
		rel, err := renderCreateItemPathPattern(settings.Cards[0].PathPattern, source, input)
		if err != nil {
			return "", err
		}
		return filepath.ToSlash(filepath.Join(source, rel)), nil
	}
	if isSourceRootScope(source, input.Scope) {
		return filepath.ToSlash(filepath.Join(source, input.Identifier)), nil
	}
	return filepath.ToSlash(filepath.Join(source, input.Scope, input.Identifier)), nil
}

func isSourceRootScope(source, scope string) bool {
	cleanSource := filepath.ToSlash(filepath.Clean(strings.TrimSpace(source)))
	cleanScope := filepath.ToSlash(filepath.Clean(strings.TrimSpace(scope)))
	return cleanScope == cleanSource || cleanScope == filepath.Base(cleanSource)
}

func renderCreateItemPathPattern(pattern, source string, input models.NewItemInput) (string, error) {
	values := map[string]string{
		"folder":     input.Scope,
		"scope":      input.Scope,
		"item":       input.Identifier,
		"identifier": input.Identifier,
		"source":     filepath.Base(source),
	}
	rendered := createItemPatternVariable.ReplaceAllStringFunc(pattern, func(match string) string {
		parts := createItemPatternVariable.FindStringSubmatch(match)
		if len(parts) != 2 {
			return ""
		}
		return values[parts[1]]
	})
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(rendered)))
	if clean == "." || clean == "" || strings.HasPrefix(clean, "../") || clean == ".." || strings.HasPrefix(clean, "/") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("source item path pattern produced an invalid path")
	}
	return clean, nil
}

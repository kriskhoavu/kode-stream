package itemwriter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/workspace/scanner"
)

func (w *Writer) importReviewedSnapshot(workspace models.WorkspaceConfig, item models.ItemDetail) error {
	if item.SourceMode != "snapshot" {
		return nil
	}
	if strings.TrimSpace(item.Commit) == "" {
		return fmt.Errorf("snapshot commit is missing")
	}
	reader := w.snapshotReader(workspace, item)
	scopeRoot := item.ItemPath
	copyOneFile := false
	var targetItemRoot string
	if !copyOneFile {
		var err error
		targetItemRoot, err = safeItemPathForRelative(workspace, item.ItemPath)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(targetItemRoot); err == nil {
			return fmt.Errorf("import target already exists in the current checkout branch")
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	var files []string
	if !copyOneFile {
		if err := reader.WalkDir(scopeRoot, func(path string, d scanner.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			files = append(files, path)
			return nil
		}); err != nil {
			return err
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("snapshot item has no files to materialize")
	}
	return w.publishStructuredSnapshot(workspace, item, reader, files, targetItemRoot)
}

func (w *Writer) publishStructuredSnapshot(workspace models.WorkspaceConfig, item models.ItemDetail, reader scanner.SourceReader, files []string, targetItemRoot string) error {
	parent := filepath.Dir(targetItemRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stagingRoot, err := os.MkdirTemp(parent, "."+filepath.Base(targetItemRoot)+".import-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stagingRoot)
	if err := os.Chmod(stagingRoot, 0o755); err != nil {
		return err
	}
	for _, rel := range files {
		if !isInsideConfiguredSource(workspace, rel) {
			return fmt.Errorf("materialized path is outside configured sources")
		}
		withinItem, err := filepath.Rel(filepath.FromSlash(item.ItemPath), filepath.FromSlash(rel))
		if err != nil || withinItem == ".." || strings.HasPrefix(withinItem, ".."+string(filepath.Separator)) {
			return fmt.Errorf("snapshot file is outside the reviewed item root")
		}
		data, err := reader.ReadFile(rel)
		if err != nil {
			return err
		}
		stagedPath, err := safeJoin(stagingRoot, withinItem)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(stagedPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(stagedPath, data, 0o644); err != nil {
			return err
		}
	}
	if _, err := os.Lstat(targetItemRoot); err == nil {
		return fmt.Errorf("import target already exists in the current checkout branch")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(stagingRoot, targetItemRoot); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("import target already exists in the current checkout branch")
		}
		return err
	}
	return nil
}

func (w *Writer) ImportSnapshotPlan(workspace models.WorkspaceConfig, item models.ItemDetail) (models.WriteResult, error) {
	if item.SourceMode != "snapshot" {
		return models.WriteResult{}, fmt.Errorf("reviewed plan must come from a snapshot")
	}
	if isDocumentationRoot(item) {
		return models.WriteResult{}, fmt.Errorf("only structured plans can be imported")
	}
	if err := w.importReviewedSnapshot(workspace, item); err != nil {
		return models.WriteResult{}, err
	}
	result, err := w.refresh(workspace, item.ItemPath)
	if err == nil {
		return result, nil
	}
	targetItemRoot, pathErr := safeItemPathForRelative(workspace, item.ItemPath)
	if pathErr != nil {
		return models.WriteResult{}, fmt.Errorf("refresh imported plan: %w; resolve rollback target: %v", err, pathErr)
	}
	if rollbackErr := os.RemoveAll(targetItemRoot); rollbackErr != nil {
		return models.WriteResult{}, fmt.Errorf("refresh imported plan: %w; rollback failed: %v", err, rollbackErr)
	}
	return models.WriteResult{}, err
}

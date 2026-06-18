package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"plan-manager/internal/fileaccess"
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/itemindex"
	"plan-manager/internal/itemwriter"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
	"plan-manager/internal/scanner"
	"plan-manager/internal/systemdialog"
	"plan-manager/internal/writeguard"
)

type API struct {
	registry *registry.Registry
	index    *itemindex.Index
	scanner  *scanner.Scanner
	files    *fileaccess.Access
	writer   *itemwriter.Writer
	git      *gitadapter.GitAdapter
	dialog   *systemdialog.Dialog
}

func New(reg *registry.Registry, idx *itemindex.Index, scan *scanner.Scanner, files *fileaccess.Access, writer *itemwriter.Writer, git *gitadapter.GitAdapter, dialog *systemdialog.Dialog) *API {
	return &API{registry: reg, index: idx, scanner: scan, files: files, writer: writer, git: git, dialog: dialog}
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/state", a.state)
	mux.HandleFunc("GET /api/workspaces", a.listWorkspaces)
	mux.HandleFunc("POST /api/workspaces", a.createWorkspace)
	mux.HandleFunc("PUT /api/workspaces/{id}", a.updateWorkspace)
	mux.HandleFunc("DELETE /api/workspaces/{id}", a.deleteWorkspace)
	mux.HandleFunc("POST /api/workspaces/{id}/scan", a.scanWorkspace)
	mux.HandleFunc("GET /api/workspaces/{id}/source-structure", a.getSourceStructure)
	mux.HandleFunc("PUT /api/workspaces/{id}/source-structure", a.saveSourceStructure)
	mux.HandleFunc("GET /api/items", a.listItems)
	mux.HandleFunc("GET /api/items/{id}", a.itemDetail)
	mux.HandleFunc("GET /api/items/{id}/files", a.itemFiles)
	mux.HandleFunc("GET /api/items/{id}/files/{fileID}", a.itemFileContent)
	mux.HandleFunc("POST /api/items/{id}/files/{fileID}", a.saveItemFile)
	mux.HandleFunc("POST /api/items/{id}/files/{fileID}/revert", a.revertItemFile)
	mux.HandleFunc("GET /api/items/{id}/diff", a.itemDiff)
	mux.HandleFunc("PATCH /api/items/{id}/metadata", a.saveItemMetadata)
	mux.HandleFunc("PATCH /api/items/{id}/status", a.updateItemStatus)
	mux.HandleFunc("POST /api/items", a.createItem)
	mux.HandleFunc("GET /api/workspaces/{id}/git/status", a.gitStatus)
	mux.HandleFunc("POST /api/workspaces/{id}/git/fetch", a.gitFetch)
	mux.HandleFunc("POST /api/workspaces/{id}/git/pull", a.gitPull)
	mux.HandleFunc("POST /api/workspaces/{id}/git/push", a.gitPush)
	mux.HandleFunc("POST /api/workspaces/{id}/git/commit", a.gitCommit)
	mux.HandleFunc("POST /api/workspaces/{id}/git/branches", a.gitCreateBranch)
	mux.HandleFunc("POST /api/workspaces/{id}/git/switch", a.gitSwitchBranch)
	mux.HandleFunc("POST /api/system/select-directory", a.selectDirectory)
	mux.HandleFunc("POST /api/system/open-path", a.openPath)
	return mux
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) state(w http.ResponseWriter, r *http.Request) {
	workspaces, err := a.registry.List()
	if err != nil {
		respond(w, nil, err)
		return
	}
	items, err := a.index.Query(itemindex.Query{})
	if err != nil {
		respond(w, nil, err)
		return
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
		respond(w, nil, err)
		return
	}
	sum := sha256.Sum256(data)
	writeJSON(w, http.StatusOK, map[string]any{
		"version":        hex.EncodeToString(sum[:]),
		"workspaceCount": len(workspaces),
		"itemCount":      len(items),
		"updatedAt":      latest,
	})
}

func (a *API) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := a.registry.List()
	respond(w, workspaces, err)
}

func (a *API) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	workspace, err := a.registry.Create(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, workspace)
}

func (a *API) updateWorkspace(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	workspace, err := a.registry.Update(r.PathValue("id"), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workspace)
}

func (a *API) deleteWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.registry.Delete(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.index.DeleteWorkspace(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) scanWorkspace(w http.ResponseWriter, r *http.Request) {
	workspace, ok, err := a.registry.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	data, err := a.scanner.Scan(workspace)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	scannedAt := time.Now().UTC()
	if err := a.index.ReplaceWorkspace(workspace.ID, data.Items, data.Warnings, scannedAt); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = a.registry.TouchScanned(workspace.ID, scannedAt)
	writeJSON(w, http.StatusOK, models.ScanResult{
		WorkspaceID: workspace.ID,
		ScannedAt:   scannedAt,
		ItemCount:   len(data.Items),
		Warnings:    data.Warnings,
	})
}

func (a *API) getSourceStructure(w http.ResponseWriter, r *http.Request) {
	workspace, root, directory, ok := a.sourceSettingsRoot(w, r)
	if !ok {
		return
	}
	_ = workspace
	settings, exists, warnings := scanner.ReadSourceStructureSettings(root)
	mode := scanner.SourceSettingsMode(root)
	if !exists && mode == "structured" {
		settings = scanner.BuiltInStructuredSettings()
	}
	if warnings == nil {
		warnings = []models.ScanWarning{}
	}
	writeJSON(w, http.StatusOK, models.SourceSettingsResult{
		Directory: directory,
		Exists:    exists,
		Mode:      mode,
		Settings:  settings,
		Warnings:  warnings,
	})
}

func (a *API) saveSourceStructure(w http.ResponseWriter, r *http.Request) {
	workspace, root, directory, ok := a.sourceSettingsRoot(w, r)
	if !ok {
		return
	}
	var settings models.SourceStructureSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if warnings := scanner.ValidateSourceStructureSettings(settings); len(warnings) > 0 {
		writeError(w, http.StatusBadRequest, warnings[0].Message)
		return
	}
	if err := scanner.WriteSourceStructureSettings(root, settings); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	scanResult, err := a.writer.RefreshWorkspace(workspace)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, struct {
		models.SourceSettingsResult
		Scan models.ScanResult `json:"scan" yaml:"scan"`
	}{
		SourceSettingsResult: models.SourceSettingsResult{
			Directory: directory,
			Exists:    true,
			Mode:      scanner.SourceSettingsMode(root),
			Settings:  settings,
			Warnings:  nonNilWarnings(scanResult.Warnings),
		},
		Scan: scanResult,
	})
}

func (a *API) listItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, err := a.index.Query(itemindex.Query{
		WorkspaceID: q.Get("workspaceId"),
		Branch:      q.Get("branch"),
		Status:      q.Get("status"),
		Text:        q.Get("q"),
	})
	for i := range items {
		items[i] = normalizeItemSummary(items[i])
	}
	respond(w, items, err)
}

func (a *API) itemDetail(w http.ResponseWriter, r *http.Request) {
	workspace, item, ok, err := a.workspaceAndItem(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	item.Description = fullReadmeDescription(workspace, item)
	item = normalizeItemDetail(item)
	writeJSON(w, http.StatusOK, item)
}

func (a *API) itemFiles(w http.ResponseWriter, r *http.Request) {
	workspace, item, ok, err := a.workspaceAndItem(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	tree, err := a.files.Tree(workspace, item)
	respond(w, tree, err)
}

func (a *API) itemFileContent(w http.ResponseWriter, r *http.Request) {
	workspace, item, ok, err := a.workspaceAndItem(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	content, err := a.files.Read(workspace, item, r.PathValue("fileID"))
	respond(w, content, err)
}

func (a *API) itemDiff(w http.ResponseWriter, r *http.Request) {
	workspace, item, ok, err := a.workspaceAndItem(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	diff, err := a.git.Diff(workspace.Path, item.ItemPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "diff unavailable: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": diff})
}

func (a *API) saveItemFile(w http.ResponseWriter, r *http.Request) {
	workspace, item, ok, err := a.workspaceAndItem(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	var input models.FileSaveInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	input.FileID = r.PathValue("fileID")
	result, err := a.files.WriteMarkdown(workspace, item, input)
	respond(w, result, err)
}

func (a *API) revertItemFile(w http.ResponseWriter, r *http.Request) {
	workspace, item, ok, err := a.workspaceAndItem(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	relPath, err := a.files.RelativePath(workspace, item, r.PathValue("fileID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	gitPath := filepath.ToSlash(filepath.Join(item.ItemPath, relPath))
	if err := validateGitPaths(workspace, []string{gitPath}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.git.RevertPaths(workspace.Path, []string{gitPath}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := a.writer.RefreshWorkspace(workspace)
	respond(w, result, err)
}

func (a *API) saveItemMetadata(w http.ResponseWriter, r *http.Request) {
	workspace, item, ok, err := a.workspaceAndItem(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	var input models.ItemMetadataUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.writer.SaveMetadata(workspace, item, input)
	respond(w, result, err)
}

func (a *API) updateItemStatus(w http.ResponseWriter, r *http.Request) {
	workspace, item, ok, err := a.workspaceAndItem(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	var input models.ItemStatusUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.writer.UpdateStatus(workspace, item, input)
	respond(w, result, err)
}

func (a *API) createItem(w http.ResponseWriter, r *http.Request) {
	var input models.NewItemInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	workspace, ok, err := a.workspace(input.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	result, err := a.writer.CreateItem(workspace, input)
	respond(w, result, err)
}

func (a *API) gitStatus(w http.ResponseWriter, r *http.Request) {
	workspace, ok, err := a.workspace(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	status, err := a.git.Status(workspace.ID, workspace.Path)
	respond(w, status, err)
}

func (a *API) gitFetch(w http.ResponseWriter, r *http.Request) {
	a.gitOperation(w, r, func(workspace models.WorkspaceConfig, input models.GitOperationInput) error {
		return a.git.Fetch(workspace.Path)
	})
}

func (a *API) gitPull(w http.ResponseWriter, r *http.Request) {
	a.gitOperation(w, r, func(workspace models.WorkspaceConfig, input models.GitOperationInput) error {
		status, err := a.git.Status(workspace.ID, workspace.Path)
		if err != nil {
			return err
		}
		if (status.Dirty || status.Conflicted) && !input.Confirm {
			return fmt.Errorf("working tree has local changes; confirm to pull")
		}
		if err := a.git.Pull(workspace.Path); err != nil {
			return err
		}
		_, err = a.writer.RefreshWorkspace(workspace)
		return err
	})
}

func (a *API) gitPush(w http.ResponseWriter, r *http.Request) {
	a.gitOperation(w, r, func(workspace models.WorkspaceConfig, input models.GitOperationInput) error {
		return a.git.Push(workspace.Path)
	})
}

func (a *API) gitCommit(w http.ResponseWriter, r *http.Request) {
	workspace, ok, err := a.workspace(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	var input models.GitCommitInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := writeguard.ValidateCommitMessage(input.Message); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateGitPaths(workspace, input.Paths); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	err = a.git.Commit(workspace.Path, input.Message, input.Paths)
	if err == nil {
		_, err = a.writer.RefreshWorkspace(workspace)
	}
	result := a.gitResult(workspace, err)
	status := http.StatusOK
	if err != nil {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, result)
}

func (a *API) gitCreateBranch(w http.ResponseWriter, r *http.Request) {
	workspace, ok, err := a.workspace(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	var input models.BranchCreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := writeguard.ValidateBranchName(input.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	err = a.git.CreateBranch(workspace.Path, input.Name, input.StartPoint, input.Checkout)
	if err == nil && input.Checkout {
		_, err = a.writer.RefreshWorkspace(workspace)
	}
	writeJSON(w, statusForError(err), a.gitResult(workspace, err))
}

func (a *API) gitSwitchBranch(w http.ResponseWriter, r *http.Request) {
	workspace, ok, err := a.workspace(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	var input models.BranchSwitchInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := writeguard.ValidateBranchName(input.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	status, err := a.git.Status(workspace.ID, workspace.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, a.gitResult(workspace, err))
		return
	}
	if (status.Dirty || status.Conflicted) && !input.Confirm {
		err = fmt.Errorf("working tree has local changes; confirm to switch branches")
		writeJSON(w, http.StatusBadRequest, a.gitResult(workspace, err))
		return
	}
	err = a.git.SwitchBranch(workspace.Path, input.Name)
	if err == nil {
		_, err = a.writer.RefreshWorkspace(workspace)
	}
	writeJSON(w, statusForError(err), a.gitResult(workspace, err))
}

func (a *API) gitOperation(w http.ResponseWriter, r *http.Request, run func(models.WorkspaceConfig, models.GitOperationInput) error) {
	workspace, ok, err := a.workspace(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	var input models.GitOperationInput
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}
	err = run(workspace, input)
	writeJSON(w, statusForError(err), a.gitResult(workspace, err))
}

func (a *API) selectDirectory(w http.ResponseWriter, r *http.Request) {
	path, err := a.dialog.SelectDirectory()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

func (a *API) openPath(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := a.dialog.OpenPath(input.Path); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) workspaceAndItem(itemID string) (models.WorkspaceConfig, models.ItemDetail, bool, error) {
	item, ok, err := a.index.Get(itemID)
	if err != nil || !ok {
		return models.WorkspaceConfig{}, models.ItemDetail{}, ok, err
	}
	workspace, ok, err := a.registry.Get(item.WorkspaceID)
	if err != nil || !ok {
		return workspace, item, ok, err
	}
	if item.ItemPath == "" {
		item.ItemPath = fallbackItemPath(workspace, item)
	}
	return workspace, item, ok, err
}

func (a *API) workspace(workspaceID string) (models.WorkspaceConfig, bool, error) {
	return a.registry.Get(workspaceID)
}

func nonNilWarnings(warnings []models.ScanWarning) []models.ScanWarning {
	if warnings == nil {
		return []models.ScanWarning{}
	}
	return warnings
}

func (a *API) sourceSettingsRoot(w http.ResponseWriter, r *http.Request) (models.WorkspaceConfig, string, string, bool) {
	workspace, ok, err := a.registry.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return models.WorkspaceConfig{}, "", "", false
	}
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return models.WorkspaceConfig{}, "", "", false
	}
	directory := filepath.ToSlash(filepath.Clean(strings.TrimSpace(r.URL.Query().Get("directory"))))
	if directory == "." || directory == "" || filepath.IsAbs(directory) || strings.HasPrefix(directory, "../") || directory == ".." {
		writeError(w, http.StatusBadRequest, "source directory is invalid")
		return models.WorkspaceConfig{}, "", "", false
	}
	allowed := false
	for _, source := range workspace.Sources {
		if directory == source {
			allowed = true
			break
		}
	}
	if !allowed {
		writeError(w, http.StatusBadRequest, "source directory is not registered")
		return models.WorkspaceConfig{}, "", "", false
	}
	root := filepath.Join(workspace.Path, filepath.FromSlash(directory))
	info, err := os.Stat(root)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return models.WorkspaceConfig{}, "", "", false
	}
	if !info.IsDir() {
		writeError(w, http.StatusBadRequest, "source directory is not a directory")
		return models.WorkspaceConfig{}, "", "", false
	}
	return workspace, root, directory, true
}

func (a *API) gitResult(workspace models.WorkspaceConfig, opErr error) models.GitOperationResult {
	status, statusErr := a.git.Status(workspace.ID, workspace.Path)
	if statusErr != nil && opErr == nil {
		opErr = statusErr
	}
	result := models.GitOperationResult{OK: opErr == nil, Status: status}
	if opErr != nil {
		result.Message = opErr.Error()
	}
	return result
}

func validateGitPaths(workspace models.WorkspaceConfig, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}
	for _, path := range paths {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
		if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "../") || clean == ".." {
			return fmt.Errorf("path %q is invalid", path)
		}
		allowed := false
		for _, dir := range workspace.Sources {
			if clean == dir || strings.HasPrefix(clean, dir+"/") {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("path %q is outside configured sources", path)
		}
	}
	return nil
}

func statusForError(err error) int {
	if err != nil {
		return http.StatusBadRequest
	}
	return http.StatusOK
}

func fallbackItemPath(workspace models.WorkspaceConfig, item models.ItemDetail) string {
	if len(workspace.Sources) == 0 || item.Scope == "" || item.Identifier == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(workspace.Sources[0], item.Scope, item.Identifier))
}

func fullReadmeDescription(workspace models.WorkspaceConfig, item models.ItemDetail) string {
	if item.ItemPath == "" {
		return item.Description
	}
	readme := filepath.Join(workspace.Path, filepath.FromSlash(item.ItemPath), "README.md")
	data, err := os.ReadFile(readme)
	if err != nil {
		return item.Description
	}
	if description := firstMarkdownParagraph(string(data)); description != "" {
		return description
	}
	return item.Description
}

func normalizeItemSummary(item models.ItemSummary) models.ItemSummary {
	if item.Tags == nil {
		item.Tags = []string{}
	}
	return item
}

func normalizeItemDetail(item models.ItemDetail) models.ItemDetail {
	item.ItemSummary = normalizeItemSummary(item.ItemSummary)
	if item.Documents == nil {
		item.Documents = []models.ItemDocument{}
	}
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	return item
}

func firstMarkdownParagraph(markdown string) string {
	for _, block := range strings.Split(markdown, "\n\n") {
		clean := strings.TrimSpace(block)
		if clean == "" || strings.HasPrefix(clean, "#") || strings.HasPrefix(clean, "|") {
			continue
		}
		return regexp.MustCompile(`\s+`).ReplaceAllString(clean, " ")
	}
	return ""
}

func respond(w http.ResponseWriter, data any, err error) {
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s\n", r.Method, r.URL.Path)
	})
}

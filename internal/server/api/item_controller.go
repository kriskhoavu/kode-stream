package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/guardedwrite"
	appitem "kode-stream/internal/item"
	itemwriter "kode-stream/internal/item/writer"
	knowledgeindex "kode-stream/internal/knowledge"
	appsearch "kode-stream/internal/search"
)

type itemController struct {
	items         *appitem.Service
	contentSearch *appsearch.ContentSearchService
	knowledge     *knowledgeindex.KnowledgeService
	audit         auditRecorder
}

func (a *itemController) importReviewedPlan(w http.ResponseWriter, r *http.Request) {
	var input models.ReviewedPlanImportInput
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	result, err := a.items.ImportReviewedPlan(r.PathValue("id"), input)
	switch {
	case errors.Is(err, apperrors.ErrItemNotFound):
		writeError(w, http.StatusNotFound, "item not found")
	case errors.Is(err, appitem.ErrReviewCommitMoved):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "review_commit_moved"})
	case errors.Is(err, appitem.ErrReviewCheckoutMoved):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "review_checkout_moved"})
	case errors.Is(err, appitem.ErrReviewedPlan):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "reviewed_plan_required"})
	case err != nil && strings.Contains(strings.ToLower(err.Error()), "already exist"):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "import_target_exists"})
	default:
		respond(w, result, err)
	}
}

func (a *itemController) listItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, err := a.items.List(appitem.ListInput{
		WorkspaceID: q.Get("workspaceId"),
		Branch:      q.Get("branch"),
		Status:      q.Get("status"),
		Text:        q.Get("q"),
	})
	respond(w, items, err)
}

func (a *itemController) itemDetail(w http.ResponseWriter, r *http.Request) {
	item, err := a.items.Detail(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if errors.Is(err, appitem.ErrSnapshotReviewOnly) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "snapshot_review_only"})
		return
	}
	respond(w, item, err)
}

func (a *itemController) itemFiles(w http.ResponseWriter, r *http.Request) {
	tree, err := a.items.FilesContext(r.Context(), r.PathValue("id"), r.URL.Query().Get("expectedCommit"))
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if errors.Is(err, appitem.ErrReviewCommitMoved) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "review_commit_moved"})
		return
	}
	if errors.Is(err, appitem.ErrSnapshotTreeTooLarge) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": err.Error(), "code": "snapshot_tree_too_large"})
		return
	}
	respond(w, tree, err)
}

func (a *itemController) itemContentSearch(w http.ResponseWriter, r *http.Request) {
	caseSensitive, err := optionalBool(r, "caseSensitive")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := a.contentSearch.SearchItem(r.Context(), r.PathValue("id"), models.WorkspaceContentSearchRequest{
		Query: r.URL.Query().Get("q"), CaseSensitive: caseSensitive,
	})
	respondContentSearch(w, result, err)
}

func (a *itemController) itemFileContent(w http.ResponseWriter, r *http.Request) {
	content, err := a.items.FileContentContext(r.Context(), r.PathValue("id"), r.PathValue("fileID"), r.URL.Query().Get("expectedCommit"))
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if errors.Is(err, appitem.ErrReviewCommitMoved) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "review_commit_moved"})
		return
	}
	if errors.Is(err, appitem.ErrSnapshotFileTooLarge) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": err.Error(), "code": "snapshot_file_too_large"})
		return
	}
	respond(w, content, err)
}

func (a *itemController) itemDiff(w http.ResponseWriter, r *http.Request) {
	diff, err := a.items.Diff(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if errors.Is(err, appitem.ErrSnapshotReviewOnly) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "snapshot_review_only"})
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": diff})
}

func (a *itemController) saveItemFile(w http.ResponseWriter, r *http.Request) {
	auditContext, contextErr := a.items.AuditContext(r.PathValue("id"))
	if errors.Is(contextErr, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if contextErr != nil {
		respond(w, nil, contextErr)
		return
	}
	var input models.FileSaveInput
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	started := time.Now()
	result, err := a.items.SaveFile(r.PathValue("id"), r.PathValue("fileID"), input)
	paths := []string{result.Path}
	if result.Path == "" {
		paths = []string{auditContext.ItemPath}
	}
	a.audit.record(auditContext.WorkspaceID, auditContext.ItemID, "save_file", "File saved.", paths, started, err)
	respondItemMutation(w, result, err)
}

func (a *itemController) revertItemFile(w http.ResponseWriter, r *http.Request) {
	result, err := a.items.RevertFile(r.PathValue("id"), r.PathValue("fileID"), validateGitPaths)
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	respondItemMutation(w, result, err)
}

func (a *itemController) saveItemMetadata(w http.ResponseWriter, r *http.Request) {
	var input models.ItemMetadataUpdateInput
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	auditContext, contextErr := a.items.AuditContext(r.PathValue("id"))
	if errors.Is(contextErr, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if contextErr != nil {
		respond(w, nil, contextErr)
		return
	}
	started := time.Now()
	result, err := a.items.SaveMetadata(r.PathValue("id"), input)
	a.audit.record(auditContext.WorkspaceID, auditContext.ItemID, "save_metadata", "Item metadata saved.", []string{auditContext.ItemPath}, started, err)
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	respondItemMutation(w, result, err)
}

func (a *itemController) itemVerificationTests(w http.ResponseWriter, r *http.Request) {
	tests, err := a.items.VerificationTests(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	respond(w, tests, err)
}

func (a *itemController) itemE2ERunbooks(w http.ResponseWriter, r *http.Request) {
	result, sources, err := a.items.E2ERunbooks(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if errors.Is(err, appitem.ErrSnapshotReviewOnly) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "snapshot_review_only"})
		return
	}
	if err != nil {
		respond(w, result, err)
		return
	}
	item, itemErr := a.items.Detail(r.PathValue("id"))
	if itemErr == nil && a.knowledge != nil && len(sources) > 0 {
		canonical, canonicalErr := a.knowledge.E2ERunbooksForSources(item.WorkspaceID, sources)
		if canonicalErr == nil {
			result.Runbooks = append(result.Runbooks, canonical.Runbooks...)
			if canonical.Diagnostic != "" {
				result.Diagnostic = canonical.Diagnostic
			}
		} else {
			result.Diagnostic = "Canonical E2E coverage could not be loaded."
		}
	}
	respond(w, result, nil)
}

func (a *itemController) saveItemVerificationTests(w http.ResponseWriter, r *http.Request) {
	var input models.VerificationTestSelection
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	tests, err := a.items.SaveVerificationTests(r.PathValue("id"), input)
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	respondItemMutation(w, tests, err)
}

func (a *itemController) updateItemStatus(w http.ResponseWriter, r *http.Request) {
	var input models.ItemStatusUpdateInput
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	auditContext, contextErr := a.items.AuditContext(r.PathValue("id"))
	if errors.Is(contextErr, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if contextErr != nil {
		respond(w, nil, contextErr)
		return
	}
	started := time.Now()
	result, err := a.items.UpdateStatus(r.PathValue("id"), input)
	a.audit.record(auditContext.WorkspaceID, auditContext.ItemID, "update_status", "Item status updated.", []string{auditContext.ItemPath}, started, err)
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	respondItemMutation(w, result, err)
}

func respondItemMutation(w http.ResponseWriter, result any, err error) {
	if errors.Is(err, itemwriter.ErrMetadataRevisionRequired) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "metadata_revision_required", "recoveryHint": "Reload the item before saving metadata."})
		return
	}
	if errors.Is(err, guardedwrite.ErrHashRequired) || errors.Is(err, guardedwrite.ErrStale) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "stale_file_content", "recoveryHint": "Reload the file and retry your edit."})
		return
	}
	if errors.Is(err, guardedwrite.ErrTooLarge) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": err.Error(), "code": "file_too_large"})
		return
	}
	if errors.Is(err, appitem.ErrSnapshotReadOnly) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "snapshot_read_only"})
		return
	}
	respond(w, result, err)
}

func (a *itemController) createItem(w http.ResponseWriter, r *http.Request) {
	var input models.NewItemInput
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	result, err := a.items.Create(input)
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, result, err)
}

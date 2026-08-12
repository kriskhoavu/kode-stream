package api

import (
	"errors"
	"net/http"
	"os"

	"kode-stream/internal/common/models"
	fileaccess "kode-stream/internal/filesystem/content"
	knowledgeindex "kode-stream/internal/knowledge"
)

type knowledgeController struct {
	knowledge *knowledgeindex.KnowledgeService
}

func (a *knowledgeController) knowledgeWikis(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	wikis, err := a.knowledge.WikisContext(r.Context(), r.URL.Query().Get("workspaceId"))
	a.respondKnowledge(w, wikis, err)
}

func (a *knowledgeController) knowledgePages(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	pages, warnings, err := a.knowledge.PagesContext(r.Context(), r.PathValue("workspaceID"), r.PathValue("root"))
	a.respondKnowledge(w, map[string]any{"pages": pages, "warnings": warnings}, err)
}

func (a *knowledgeController) knowledgePage(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	page, err := a.knowledge.PageContext(r.Context(), r.PathValue("workspaceID"), r.PathValue("root"), r.PathValue("slug"))
	a.respondKnowledge(w, page, err)
}

func (a *knowledgeController) knowledgeE2ERunbook(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	result, err := a.knowledge.E2ERunbook(r.PathValue("workspaceID"), r.PathValue("root"), r.PathValue("slug"))
	respond(w, result, err)
}

func (a *knowledgeController) knowledgeGraph(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	graph, err := a.knowledge.GraphContext(r.Context(), r.PathValue("workspaceID"), r.PathValue("root"))
	a.respondKnowledge(w, graph, err)
}

func (a *knowledgeController) knowledgeRescan(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	result, err := a.knowledge.Rescan(r.Context(), r.PathValue("workspaceID"), r.PathValue("root"))
	a.respondKnowledgeAction(w, result, err)
}

func (a *knowledgeController) knowledgeSync(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	var input models.GitOperationInput
	if !decodeLimitedJSON(w, r, &input, 16<<10, true) {
		return
	}
	result, err := a.knowledge.Sync(r.Context(), r.PathValue("workspaceID"), input)
	a.respondKnowledgeAction(w, result, err)
}

func (a *knowledgeController) knowledgeEnrich(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	var input models.KnowledgeConfirmationInput
	if !decodeLimitedJSON(w, r, &input, 16<<10, true) {
		return
	}
	result, err := a.knowledge.Enrich(r.Context(), r.PathValue("workspaceID"), input.Confirm)
	a.respondKnowledgeAction(w, result, err)
}

func (a *knowledgeController) respondKnowledgeAction(w http.ResponseWriter, result knowledgeindex.KnowledgeActionResult, err error) {
	switch {
	case errors.Is(err, knowledgeindex.ErrConfirmationRequired), errors.Is(err, knowledgeindex.ErrEnrichNotConfigured), errors.Is(err, knowledgeindex.ErrKnowledgeDisabled):
		writeError(w, http.StatusConflict, err.Error())
	case err != nil:
		a.respondKnowledge(w, result, err)
	case !result.OK:
		writeJSON(w, http.StatusUnprocessableEntity, result)
	default:
		writeJSON(w, http.StatusOK, result)
	}
}

func (a *knowledgeController) respondKnowledge(w http.ResponseWriter, data any, err error) {
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, data)
	case errors.Is(err, knowledgeindex.ErrWorkspaceNotFound), errors.Is(err, knowledgeindex.ErrWikiNotFound), errors.Is(err, knowledgeindex.ErrPageNotFound), errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, knowledgeindex.ErrUnsafePath), errors.Is(err, fileaccess.ErrUnsupportedContent):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, knowledgeindex.ErrKnowledgeDisabled):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "knowledge request failed")
	}
}

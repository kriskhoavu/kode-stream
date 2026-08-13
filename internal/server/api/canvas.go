package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	appcanvas "kode-stream/internal/canvas"
	appworkstream "kode-stream/internal/workstream"
)

const maxCanvasMutationBodyBytes int64 = 64 << 10

type canvasController struct {
	canvas     *appcanvas.Service
	workstream *appworkstream.Service
}

func (a *canvasController) resolveDefaultCanvas(w http.ResponseWriter, r *http.Request) {
	if a.canvas == nil || a.workstream == nil {
		writeError(w, http.StatusServiceUnavailable, "Canvas is unavailable")
		return
	}
	var input struct {
		WorkspaceID string `json:"workspaceId"`
		BranchKey   string `json:"branchKey,omitempty"`
	}
	if !decodeLimitedJSON(w, r, &input, maxCanvasMutationBodyBytes, true) {
		return
	}
	if strings.TrimSpace(input.WorkspaceID) == "" {
		writeError(w, http.StatusBadRequest, "workspaceId is required")
		return
	}
	checkout, err := a.workstream.LoadCheckout(input.WorkspaceID, false)
	if err != nil {
		a.respondCanvas(w, appcanvas.Projection{}, err)
		return
	}
	if requested := strings.TrimSpace(input.BranchKey); requested != "" && requested != checkout.Branch {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "Canvas branch differs from the current checkout", "code": "canvas_branch_mismatch", "checkoutBranch": checkout.Branch})
		return
	}
	projection, err := a.canvas.ResolveDefaultContext(r.Context(), a.canvasOwner(r), input.WorkspaceID, checkout.Branch)
	a.respondCanvas(w, projection, err)
}

func (a *canvasController) canvasLayout(w http.ResponseWriter, r *http.Request) {
	if a.canvas == nil {
		writeError(w, http.StatusServiceUnavailable, "Canvas is unavailable")
		return
	}
	projection, err := a.canvas.ProjectContext(r.Context(), a.canvasOwner(r), r.PathValue("id"))
	a.respondCanvas(w, projection, err)
}

func (a *canvasController) patchCanvasPlacements(w http.ResponseWriter, r *http.Request) {
	if a.canvas == nil {
		writeError(w, http.StatusServiceUnavailable, "Canvas is unavailable")
		return
	}
	var input struct {
		Patches []appcanvas.PlacementPatch `json:"patches"`
	}
	if !decodeLimitedJSON(w, r, &input, maxCanvasMutationBodyBytes, true) {
		return
	}
	if len(input.Patches) == 0 {
		writeError(w, http.StatusBadRequest, "patches are required")
		return
	}
	projection, err := a.canvas.PatchPlacementsContext(r.Context(), a.canvasOwner(r), r.PathValue("id"), input.Patches)
	a.respondCanvas(w, projection, err)
}

func (a *canvasController) patchCanvasViewport(w http.ResponseWriter, r *http.Request) {
	if a.canvas == nil {
		writeError(w, http.StatusServiceUnavailable, "Canvas is unavailable")
		return
	}
	var input struct {
		ExpectedVersion int64              `json:"expectedVersion"`
		Viewport        appcanvas.Viewport `json:"viewport"`
	}
	if !decodeLimitedJSON(w, r, &input, maxCanvasMutationBodyBytes, true) {
		return
	}
	if input.ExpectedVersion < 1 {
		writeError(w, http.StatusBadRequest, "expectedVersion is required")
		return
	}
	projection, err := a.canvas.SaveViewportContext(r.Context(), a.canvasOwner(r), r.PathValue("id"), input.ExpectedVersion, input.Viewport)
	a.respondCanvas(w, projection, err)
}

func (a *canvasController) removeCanvasPlacement(w http.ResponseWriter, r *http.Request) {
	if a.canvas == nil {
		writeError(w, http.StatusServiceUnavailable, "Canvas is unavailable")
		return
	}
	revision, err := strconv.ParseInt(r.URL.Query().Get("expectedRevision"), 10, 64)
	if err != nil || revision < 1 {
		writeError(w, http.StatusBadRequest, "expectedRevision is required")
		return
	}
	projection, err := a.canvas.RemovePlacementContext(r.Context(), a.canvasOwner(r), r.PathValue("id"), r.PathValue("nodeId"), revision)
	a.respondCanvas(w, projection, err)
}

func (a *canvasController) canvasOwner(r *http.Request) string {
	if session, ok := cloudSessionFromContext(r.Context()); ok {
		return session.User.ID
	}
	return ""
}

func (a *canvasController) respondCanvas(w http.ResponseWriter, projection appcanvas.Projection, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, projection)
		return
	}
	if errors.Is(err, appcanvas.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Canvas layout not found")
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	var conflict *appcanvas.PlacementConflictError
	if errors.As(err, &conflict) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": conflict.Error(), "code": "placement_conflict", "nodeIds": conflict.NodeIDs})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "code": "canvas_invalid_request"})
}

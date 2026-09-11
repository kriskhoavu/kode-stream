package api

import (
	"errors"
	"log"
	"net/http"
	"strings"

	appaisession "kode-stream/internal/ai"
)

// maxSessionDiffBytes bounds a single file diff response, matching the byte
// bound the workspace tree reader already applies to a file's content.
const maxSessionDiffBytes int64 = 1 << 20

// discardResponseWriter swallows a handler's response so authorizeWorkspace
// (which writes a 404 on denial) can be reused as a pure read predicate when
// filtering workspaces for the cross-workspace session list, instead of
// duplicating its authorization rule.
type discardResponseWriter struct {
	header http.Header
}

func (d *discardResponseWriter) Header() http.Header {
	if d.header == nil {
		d.header = http.Header{}
	}
	return d.header
}

func (d *discardResponseWriter) Write(body []byte) (int, error) { return len(body), nil }

func (d *discardResponseWriter) WriteHeader(int) {}

// sessionsController serves the cross-workspace session list and a single
// session's changes and file diffs. It embeds *aiController to reuse
// authorizeWorkspace rather than duplicating its Cloud-denial-looks-like-404
// rule.
type sessionsController struct {
	*aiController
}

func (a *sessionsController) globalSessions(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session records are unavailable")
		return
	}
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	if scope == "" {
		scope = appaisession.SessionScopeActive
	}
	if scope != appaisession.SessionScopeActive && scope != appaisession.SessionScopeAll {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scope must be active or all", "code": "session_request_invalid"})
		return
	}
	mayRead := func(workspaceID string) bool {
		return a.authorizeWorkspace(&discardResponseWriter{}, r, workspaceID)
	}
	rows, truncated, err := a.aiSessions.SessionRecordsAcrossWorkspaces(r.Context(), scope, mayRead)
	if err != nil {
		log.Printf("global_sessions_list_failure scope=%q error=%q", scope, err)
		writeError(w, http.StatusInternalServerError, "Sessions could not be listed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows, "truncated": truncated})
}

func (a *sessionsController) sessionChanges(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session records are unavailable")
		return
	}
	id := r.PathValue("id")
	if !a.authorizeSession(w, r, id) {
		return
	}
	changes, err := a.aiSessions.SessionChanges(r.Context(), id)
	if err != nil {
		if errors.Is(err, appaisession.ErrNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, changes)
}

func (a *sessionsController) sessionDiff(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session records are unavailable")
		return
	}
	id := r.PathValue("id")
	if !a.authorizeSession(w, r, id) {
		return
	}
	path := r.URL.Query().Get("path")
	if strings.TrimSpace(path) == "" || strings.HasPrefix(path, ":") || strings.HasPrefix(path, "-") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required and must not start with ':' or '-'", "code": "session_request_invalid"})
		return
	}
	diff, err := a.aiSessions.SessionFileDiff(r.Context(), id, path, maxSessionDiffBytes)
	if err != nil {
		if errors.Is(err, appaisession.ErrNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		var changesErr *appaisession.SessionChangesError
		if errors.As(err, &changesErr) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": changesErr.Error(), "code": changesErr.Reason})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, diff)
}

// authorizeSession resolves the session's workspace and applies the same
// Cloud-denial-looks-like-404 rule authorizeWorkspace already gives every
// other AI session endpoint. A session that does not exist gets the same
// 404 a denied workspace would, so a session ID is not an oracle either.
func (a *sessionsController) authorizeSession(w http.ResponseWriter, r *http.Request, sessionID string) bool {
	workspaceID, err := a.aiSessions.SessionRecordWorkspace(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return false
	}
	return a.authorizeWorkspace(w, r, workspaceID)
}

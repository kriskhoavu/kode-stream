package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"kode-stream/internal/common/models"
	appjira "kode-stream/internal/jira"
)

type jiraController struct{ jira *appjira.Service }

func (a *jiraController) jiraAttachment(w http.ResponseWriter, r *http.Request) {
	if a.jira == nil {
		writeError(w, http.StatusServiceUnavailable, "Jira integration is unavailable")
		return
	}
	content, err := a.jira.Attachment(r.Context(), r.PathValue("id"), r.PathValue("attachmentId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filename := sanitizeDownloadName(content.Filename)
	disposition := "attachment"
	if safeInlineMediaType(content.MediaType) {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", content.MediaType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename=%q`, disposition, filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(content.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content.Data)
}

func safeInlineMediaType(value string) bool {
	switch strings.ToLower(value) {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}
func sanitizeDownloadName(value string) string {
	value = filepath.Base(strings.ReplaceAll(value, "\\", "/"))
	value = strings.Map(func(r rune) rune {
		if r < ' ' || r == 127 || r == '"' {
			return -1
		}
		return r
	}, value)
	if strings.TrimSpace(value) == "" || value == "." {
		return "attachment"
	}
	return value
}

func (a *jiraController) jiraIssue(w http.ResponseWriter, r *http.Request) {
	a.respondJiraIssue(w, r, false)
}
func (a *jiraController) refreshJiraIssue(w http.ResponseWriter, r *http.Request) {
	a.respondJiraIssue(w, r, true)
}

func (a *jiraController) workspaceJiraIssue(w http.ResponseWriter, r *http.Request) {
	if a.jira == nil {
		writeError(w, http.StatusServiceUnavailable, "Jira integration is unavailable")
		return
	}
	result, err := a.jira.WorkspaceIssue(r.Context(), r.PathValue("id"), r.PathValue("issueKey"), false)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *jiraController) respondJiraIssue(w http.ResponseWriter, r *http.Request, refresh bool) {
	if a.jira == nil {
		writeError(w, http.StatusServiceUnavailable, "Jira integration is unavailable")
		return
	}
	result, err := a.jira.Issue(r.Context(), r.PathValue("id"), refresh)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *jiraController) testJiraConnection(w http.ResponseWriter, r *http.Request) {
	if a.jira == nil {
		writeError(w, http.StatusServiceUnavailable, "Jira integration is unavailable")
		return
	}
	var connection models.JiraConnection
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&connection); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.jira.TestConnection(r.Context(), r.PathValue("id"), &connection)
	respond(w, result, err)
}

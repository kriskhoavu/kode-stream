package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"kode-stream/internal/common/models"
	appjira "kode-stream/internal/jira"
)

const maxJiraConnectionBodyBytes int64 = 16 << 10

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
	defer content.Body.Close()
	filename := sanitizeDownloadName(content.Filename)
	disposition := "attachment"
	if safeInlineMediaType(content.MediaType) {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", content.MediaType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename=%q`, disposition, filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if err := copyBoundedAttachment(w, content.Body); err != nil && !errors.Is(err, context.Canceled) {
		// Headers may already be committed; ending the stream is the only safe
		// outcome for an unknown-length oversized upstream response.
		return
	}
}

func copyBoundedAttachment(w io.Writer, body io.Reader) error {
	buffer := make([]byte, 32<<10)
	var written int64
	for {
		n, readErr := body.Read(buffer)
		if n > 0 {
			if written+int64(n) > appjira.MaxAttachmentBytes {
				return errors.New("Jira attachment exceeds the size limit")
			}
			if _, err := w.Write(buffer[:n]); err != nil {
				return err
			}
			written += int64(n)
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
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
	if !decodeLimitedJSON(w, r, &connection, maxJiraConnectionBodyBytes, true) {
		return
	}
	result, err := a.jira.TestConnection(r.Context(), r.PathValue("id"), &connection)
	if err == nil {
		writeJSON(w, http.StatusOK, result)
		return
	}
	switch {
	case strings.Contains(err.Error(), "workspace not found"):
		writeError(w, http.StatusNotFound, "workspace not found")
	case strings.Contains(err.Error(), "Jira connection"), strings.Contains(err.Error(), "Jira deployment"), strings.Contains(err.Error(), "Jira base URL"), strings.Contains(err.Error(), "Jira project"), strings.Contains(err.Error(), "token environment"):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, appjira.ErrAuthentication):
		writeError(w, http.StatusUnauthorized, "Jira authentication failed")
	case errors.Is(err, appjira.ErrForbidden):
		writeError(w, http.StatusForbidden, "Jira access is forbidden")
	default:
		writeError(w, http.StatusBadGateway, "Jira connection test is unavailable")
	}
}

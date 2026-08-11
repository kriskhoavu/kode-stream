package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/httpx"
	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/guardedwrite"
	appgit "kode-stream/internal/git"
	appitem "kode-stream/internal/item"
	appworkspace "kode-stream/internal/workspace"
	workspaceaccess "kode-stream/internal/workspace/files"
)

const maxWorkspaceMutationBodyBytes int64 = (2 << 20) + (64 << 10)

func decodeLimitedJSON(w http.ResponseWriter, r *http.Request, target any, limit int64, disallowUnknown bool) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	if disallowUnknown {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request body exceeds the write limit", "code": "request_too_large"})
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func nonNilWarnings(warnings []models.ScanWarning) []models.ScanWarning {
	return appworkspace.NonNilWarnings(warnings)
}

func validateGitPaths(workspace models.WorkspaceConfig, paths []string) error {
	return appgit.ValidatePaths(workspace, paths)
}

func statusForError(err error) int {
	if err != nil {
		return http.StatusBadRequest
	}
	return http.StatusOK
}

func fallbackItemPath(workspace models.WorkspaceConfig, item models.ItemDetail) string {
	return appitem.FallbackPath(workspace, item)
}

func fullReadmeDescription(workspace models.WorkspaceConfig, item models.ItemDetail) string {
	return appitem.FullReadmeDescription(workspace, item)
}

func normalizeItemSummary(item models.ItemSummary) models.ItemSummary {
	return appitem.NormalizeSummary(item)
}

func normalizeItemDetail(item models.ItemDetail) models.ItemDetail {
	return appitem.NormalizeDetail(item)
}

func firstMarkdownParagraph(markdown string) string {
	return appitem.FirstMarkdownParagraph(markdown)
}

func respond(w http.ResponseWriter, data any, err error) {
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func respondWorkspaceResult(w http.ResponseWriter, data any, err error) {
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, data, err)
}

func respondWorkspaceFileResult(w http.ResponseWriter, data any, err error) {
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, data)
	case errors.Is(err, apperrors.ErrWorkspaceNotFound), errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, workspaceaccess.ErrHashRequired), errors.Is(err, workspaceaccess.ErrStaleContent):
		writeError(w, http.StatusConflict, workspaceaccess.ErrStaleContent.Error())
	case errors.Is(err, guardedwrite.ErrTooLarge):
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": err.Error(), "code": "file_too_large"})
	case errors.Is(err, workspaceaccess.ErrDestinationExists):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func respondContentSearch(w http.ResponseWriter, data any, err error) {
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, data)
	case errors.Is(err, appitem.ErrSnapshotReviewOnly):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "snapshot_review_only"})
	case errors.Is(err, apperrors.ErrItemNotFound), errors.Is(err, apperrors.ErrWorkspaceNotFound), errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, context.Canceled):
		writeError(w, 499, "content search canceled")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func optionalBool(r *http.Request, name string) (bool, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return value, nil
}

func respondGitResult(w http.ResponseWriter, result models.GitOperationResult) {
	if result.Message == apperrors.ErrWorkspaceNotFound.Error() {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	writeJSON(w, statusForErrorFromResult(result), result)
}

func withRecoveryHint(result models.GitOperationResult) models.GitOperationResult {
	if !result.OK && result.RecoveryHint == "" {
		result.RecoveryHint = recoveryHint(result.Message)
	}
	return result
}

func statusForErrorFromResult(result models.GitOperationResult) int {
	if !result.OK {
		return http.StatusBadRequest
	}
	return http.StatusOK
}

func writeSSE(w http.ResponseWriter, event string, data any) {
	payload, _ := json.Marshal(data)
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(string(payload), "\n") {
		_, _ = fmt.Fprintf(w, "data: %s\n", line)
	}
	_, _ = fmt.Fprint(w, "\n")
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	httpx.WriteJSON(w, status, data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	httpx.WriteError(w, status, message, recoveryHint)
}

func recoveryHint(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "jira token environment variable"):
		return "Set the configured environment variable or add it to ~/.creds.zsh or ~/.creds.sh, then restart Kode Stream."
	case strings.Contains(lower, "jira authentication"):
		return "Check the Jira account and token configured for this workspace."
	case strings.Contains(lower, "jira project"):
		return "Check the Jira project key and account permissions."
	case strings.Contains(lower, "jira is unavailable"):
		return "Check the Jira base URL, network access, and server availability."
	case strings.Contains(lower, "changed since it was loaded"):
		return "Reload the file to review the latest content, then apply your changes again."
	case strings.Contains(lower, "local changes"):
		return "Review local changes, then confirm the operation or commit them first."
	case strings.Contains(lower, "conflict"):
		return "Resolve or abort the current Git operation before continuing."
	case strings.Contains(lower, "outside configured sources"), strings.Contains(lower, "path escapes"):
		return "Choose a path inside a configured workspace source."
	default:
		return ""
	}
}

func Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s\n", r.Method, r.URL.Path)
	})
}

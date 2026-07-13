package httpx

// Package httpx provides shared HTTP response encoding.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	apperrors "kode-stream/internal/common"
)

// WriteJSON writes a JSON response using the provided HTTP status.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// WriteError writes the common API error envelope. The hint function may be nil.
func WriteError(w http.ResponseWriter, status int, message string, hint func(string) string) {
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	payload := map[string]string{"error": message}
	if hint != nil {
		if recoveryHint := hint(message); recoveryHint != "" {
			payload["recoveryHint"] = recoveryHint
		}
	}
	WriteJSON(w, status, payload)
}

func WriteAppError(w http.ResponseWriter, err error, hint func(string) string) {
	status, message, code := MapError(err)
	WriteCodedError(w, status, message, code, hint)
}

func WriteCodedError(w http.ResponseWriter, status int, message string, code apperrors.ErrorCode, hint func(string) string) {
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	payload := map[string]string{"error": message}
	if code != "" {
		payload["code"] = string(code)
	}
	if hint != nil {
		if recoveryHint := hint(message); recoveryHint != "" {
			payload["recoveryHint"] = recoveryHint
		}
	}
	WriteJSON(w, status, payload)
}

func MapError(err error) (int, string, apperrors.ErrorCode) {
	if err == nil {
		return http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), apperrors.ErrorCodeInfra
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return statusForCode(appErr.Code), appErr.Error(), appErr.Code
	}
	switch {
	case errors.Is(err, apperrors.ErrWorkspaceNotFound), errors.Is(err, apperrors.ErrItemNotFound):
		return http.StatusNotFound, err.Error(), apperrors.ErrorCodeNotFound
	default:
		return http.StatusInternalServerError, err.Error(), apperrors.ErrorCodeInfra
	}
}

func statusForCode(code apperrors.ErrorCode) int {
	switch code {
	case apperrors.ErrorCodeNotFound:
		return http.StatusNotFound
	case apperrors.ErrorCodeValidation:
		return http.StatusBadRequest
	case apperrors.ErrorCodeConflict:
		return http.StatusConflict
	case apperrors.ErrorCodeUnauthorized:
		return http.StatusUnauthorized
	case apperrors.ErrorCodeForbidden:
		return http.StatusForbidden
	case apperrors.ErrorCodeUnavailable:
		return http.StatusServiceUnavailable
	case apperrors.ErrorCodeInfra:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

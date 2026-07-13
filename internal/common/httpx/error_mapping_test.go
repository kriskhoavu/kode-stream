package httpx

import (
	"errors"
	"net/http"
	"testing"

	apperrors "kode-stream/internal/common"
)

func TestMapErrorStatuses(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   apperrors.ErrorCode
	}{
		{name: "not found", err: apperrors.NotFound("missing", nil), status: http.StatusNotFound, code: apperrors.ErrorCodeNotFound},
		{name: "validation", err: apperrors.Validation("invalid", nil), status: http.StatusBadRequest, code: apperrors.ErrorCodeValidation},
		{name: "conflict", err: apperrors.Conflict("conflict", nil), status: http.StatusConflict, code: apperrors.ErrorCodeConflict},
		{name: "unauthorized", err: apperrors.Unauthorized("unauthorized", nil), status: http.StatusUnauthorized, code: apperrors.ErrorCodeUnauthorized},
		{name: "forbidden", err: apperrors.Forbidden("forbidden", nil), status: http.StatusForbidden, code: apperrors.ErrorCodeForbidden},
		{name: "unavailable", err: apperrors.Unavailable("unavailable", nil), status: http.StatusServiceUnavailable, code: apperrors.ErrorCodeUnavailable},
		{name: "infra", err: apperrors.Infra("infra", nil), status: http.StatusInternalServerError, code: apperrors.ErrorCodeInfra},
		{name: "unknown", err: errors.New("unknown"), status: http.StatusInternalServerError, code: apperrors.ErrorCodeInfra},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _, code := MapError(tt.err)
			if status != tt.status || code != tt.code {
				t.Fatalf("status = %d code = %q", status, code)
			}
		})
	}
}

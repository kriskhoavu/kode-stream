package httpx

// Package httpx provides shared HTTP response encoding.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "kode-stream/internal/common"
)

func TestWriteJSON(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteJSON(recorder, http.StatusCreated, map[string]string{"id": "one"})

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}
	if got := strings.TrimSpace(recorder.Body.String()); got != `{"id":"one"}` {
		t.Fatalf("body = %s", got)
	}
}

func TestWriteErrorUsesStatusTextAndRecoveryHint(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteError(recorder, http.StatusConflict, " ", func(string) string { return "reload" })

	if got := strings.TrimSpace(recorder.Body.String()); got != `{"error":"Conflict","recoveryHint":"reload"}` {
		t.Fatalf("body = %s", got)
	}
}

func TestWriteAppErrorAddsCompatibleOptionalCode(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteAppError(recorder, apperrors.Validation("name is required", nil), nil)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := strings.TrimSpace(recorder.Body.String()); got != `{"code":"validation","error":"name is required"}` {
		t.Fatalf("body = %s", got)
	}
}

func TestMapErrorCoversSharedNotFoundErrors(t *testing.T) {
	status, message, code := MapError(apperrors.ErrItemNotFound)

	if status != http.StatusNotFound || message != "item not found" || code != apperrors.ErrorCodeNotFound {
		t.Fatalf("status = %d message = %q code = %q", status, message, code)
	}
}

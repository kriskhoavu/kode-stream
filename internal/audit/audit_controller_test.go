package audit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"kode-stream/internal/common/models"
)

func TestAuditControllerEventsPreservesLimitAndWorkspaceFilter(t *testing.T) {
	repository := New(filepath.Join(t.TempDir(), "audit.jsonl"))
	controller := NewController(repository)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	events := []models.AuditEvent{
		{WorkspaceID: "workspace-a", Operation: "old", Status: models.AuditStatusSuccess, Message: "old"},
		{WorkspaceID: "workspace-b", Operation: "other", Status: models.AuditStatusSuccess, Message: "other"},
		{WorkspaceID: "workspace-a", Operation: "new", Status: models.AuditStatusSuccess, Message: "new"},
	}
	for _, event := range events {
		if _, err := repository.Append(event); err != nil {
			t.Fatal(err)
		}
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/audit-events?workspaceId=workspace-a&limit=1", nil)
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("content type = %q", contentType)
	}
	var payload []models.AuditEvent
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 1 || payload[0].WorkspaceID != "workspace-a" || payload[0].Operation != "new" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestAuditControllerEventsDefaultsInvalidLimitToFifty(t *testing.T) {
	repository := New(filepath.Join(t.TempDir(), "audit.jsonl"))
	controller := NewController(repository)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	for i := 0; i < 55; i++ {
		if _, err := repository.Append(models.AuditEvent{Operation: "scan", Status: models.AuditStatusSuccess, Message: "scan"}); err != nil {
			t.Fatal(err)
		}
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/audit-events?limit=999", nil)
	mux.ServeHTTP(response, request)

	var payload []models.AuditEvent
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || len(payload) != 50 {
		t.Fatalf("status = %d events = %d", response.Code, len(payload))
	}
}

func BenchmarkAuditControllerEvents(b *testing.B) {
	repository := New(filepath.Join(b.TempDir(), "audit.jsonl"))
	for i := 0; i < 100; i++ {
		if _, err := repository.Append(models.AuditEvent{WorkspaceID: "workspace-a", Operation: "scan", Status: models.AuditStatusSuccess, Message: "scan"}); err != nil {
			b.Fatal(err)
		}
	}
	controller := NewController(repository)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	request := httptest.NewRequest(http.MethodGet, "/api/audit-events?workspaceId=workspace-a&limit=50", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			b.Fatalf("status = %d", response.Code)
		}
	}
}

package workspace

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthControllerReturnsStableContract(t *testing.T) {
	controller := NewHealthController()
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("content type = %q", contentType)
	}
	var payload map[string]bool
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload["ok"] {
		t.Fatalf("payload = %#v", payload)
	}
}

func BenchmarkHealthController(b *testing.B) {
	controller := NewHealthController()
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			b.Fatalf("status = %d", response.Code)
		}
	}
}

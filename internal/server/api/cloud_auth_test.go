package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kode-stream/internal/common/models"
	"kode-stream/internal/system"
)

func TestCloudModeRequiresSessionOutsideHealthAndAuth(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := apiHandler.WithRuntimeConfig(testCloudRuntimeConfig()).Routes()

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d", health.Code)
	}

	state := httptest.NewRecorder()
	handler.ServeHTTP(state, httptest.NewRequest(http.MethodGet, "/api/state", nil))
	if state.Code != http.StatusUnauthorized {
		t.Fatalf("state status = %d body = %s", state.Code, state.Body.String())
	}
}

func TestCloudCallbackBootstrapsAdminFromAllowlist(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := apiHandler.WithRuntimeConfig(testCloudRuntimeConfig()).Routes()

	callback := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/callback", nil)
	request.Header.Set("X-Kode-Stream-Email", "admin@example.com")
	request.Header.Set("X-Kode-Stream-Subject", "admin-subject")
	handler.ServeHTTP(callback, request)
	if callback.Code != http.StatusOK {
		t.Fatalf("callback status = %d body = %s", callback.Code, callback.Body.String())
	}

	state := httptest.NewRecorder()
	stateRequest := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	for _, cookie := range callback.Result().Cookies() {
		stateRequest.AddCookie(cookie)
	}
	handler.ServeHTTP(state, stateRequest)
	if state.Code != http.StatusOK {
		t.Fatalf("state status = %d body = %s", state.Code, state.Body.String())
	}
	var payload struct {
		Mode         models.RuntimeMode         `json:"mode"`
		Role         models.CloudRole           `json:"role"`
		User         models.CloudUser           `json:"user"`
		Capabilities map[models.Capability]bool `json:"capabilities"`
	}
	if err := json.Unmarshal(state.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Mode != models.RuntimeModeCloud || payload.Role != models.CloudRoleAdmin || payload.User.Email != "admin@example.com" || !payload.Capabilities[models.CapabilitySystem] {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestCloudViewerCannotMutateRoutes(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := apiHandler.WithRuntimeConfig(testCloudRuntimeConfig()).Routes()

	request := httptest.NewRequest(http.MethodPost, "/api/workspaces", strings.NewReader(`{}`))
	request.Header.Set("X-Kode-Stream-Subject", "viewer")
	request.Header.Set("X-Kode-Stream-Role", "viewer")
	request.Header.Set(csrfHeader, stableCloudUserID("viewer:csrf"))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestCloudEditorMutationsRequireCSRF(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := apiHandler.WithRuntimeConfig(testCloudRuntimeConfig()).Routes()

	request := httptest.NewRequest(http.MethodPost, "/api/workspaces", strings.NewReader(`{}`))
	request.Header.Set("X-Kode-Stream-Subject", "editor")
	request.Header.Set("X-Kode-Stream-Role", "editor")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status = %d body = %s", response.Code, response.Body.String())
	}

	withCSRF := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/workspaces", strings.NewReader(`{}`))
	request.Header.Set("X-Kode-Stream-Subject", "editor")
	request.Header.Set("X-Kode-Stream-Role", "editor")
	request.Header.Set(csrfHeader, stableCloudUserID("editor:csrf"))
	handler.ServeHTTP(withCSRF, request)
	if withCSRF.Code != http.StatusBadRequest {
		t.Fatalf("csrf status = %d body = %s", withCSRF.Code, withCSRF.Body.String())
	}
}

func TestCloudLogoutClearsSessionWithCSRF(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := apiHandler.WithRuntimeConfig(testCloudRuntimeConfig()).Routes()

	login := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/callback", nil)
	request.Header.Set("X-Kode-Stream-Subject", "editor")
	request.Header.Set("X-Kode-Stream-Role", "editor")
	handler.ServeHTTP(login, request)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d", login.Code)
	}

	logout := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	for _, cookie := range login.Result().Cookies() {
		logoutRequest.AddCookie(cookie)
	}
	logoutRequest.Header.Set(csrfHeader, stableCloudUserID("editor:csrf"))
	handler.ServeHTTP(logout, logoutRequest)
	if logout.Code != http.StatusOK || logout.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("logout status = %d cookies=%#v body=%s", logout.Code, logout.Result().Cookies(), logout.Body.String())
	}
}

func TestLocalModeBypassesCloudAuthPolicy(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := apiHandler.Routes()
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/state", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func testCloudRuntimeConfig() system.RuntimeConfig {
	config := system.RuntimeConfig{
		Mode:         models.RuntimeModeCloud,
		BindAddress:  "0.0.0.0",
		CookieSecret: "test-secret",
		OIDCIssuer:   "https://issuer.example.com",
		AdminUsers:   []string{"admin@example.com"},
		Capabilities: map[models.Capability]bool{models.CapabilityRead: true},
		Agent:        models.AgentConnection{Available: false, Status: "offline"},
	}
	return config
}

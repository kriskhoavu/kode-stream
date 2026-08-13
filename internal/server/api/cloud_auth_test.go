package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appaisession "kode-stream/internal/ai"
	"kode-stream/internal/audit"
	"kode-stream/internal/common/models"
	"kode-stream/internal/system"
)

func TestCloudModeRequiresSessionOutsideHealthAndAuth(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := withTestRuntime(apiHandler, testCloudRuntimeConfig()).Routes()

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

func TestCloudEmbeddedSessionRoutesAreTenantScopedBeforeWebSocketUpgrade(t *testing.T) {
	manager := appaisession.NewTerminalManager(appaisession.Config{})
	t.Cleanup(func() { _ = manager.Close() })
	session, grant, err := manager.Start(appaisession.StartRequest{ID: "tenant-terminal", WorkspaceID: "owner-workspace", Executable: "/bin/sh", Args: []string{"-c", "sleep 10"}, Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	apiHandler := withTestRuntime(New(Dependencies{AISessions: appaisession.New(nil).ConfigureEmbedded(manager)}), testAppOIDCRuntimeConfig())
	owner := stableCloudUserID("owner")
	other := stableCloudUserID("other")
	for _, workspace := range []models.WorkspaceConfig{{ID: session.WorkspaceID, OwnerUserID: owner}, {ID: "other-workspace", OwnerUserID: other}} {
		if _, err := apiHandler.cloud.workspaces.Upsert(context.Background(), workspace); err != nil {
			t.Fatal(err)
		}
	}
	handler := apiHandler.Routes()
	requestFor := func(subject, path string) *http.Request {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("X-Kode-Stream-Subject", subject)
		request.Header.Set("X-Kode-Stream-Role", "editor")
		return request
	}
	ownerResponse := httptest.NewRecorder()
	handler.ServeHTTP(ownerResponse, requestFor("owner", "/api/ai/sessions/"+session.ID))
	if ownerResponse.Code != http.StatusOK {
		t.Fatalf("owner status=%d body=%s", ownerResponse.Code, ownerResponse.Body.String())
	}
	for _, path := range []string{"/api/ai/sessions/" + session.ID, "/api/ai/sessions/" + session.ID + "/channel?token=" + grant.Token} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, requestFor("other", path))
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), session.ID) {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	for _, test := range []struct{ method, path string }{
		{http.MethodPost, "/api/ai/sessions/" + session.ID + "/grant"},
		{http.MethodDelete, "/api/ai/sessions/" + session.ID},
		{http.MethodGet, "/api/ai/providers/codex/capabilities?workspaceId=owner-workspace"},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(`{}`))
		request.Header.Set("X-Kode-Stream-Subject", "other")
		request.Header.Set("X-Kode-Stream-Role", "editor")
		request.Header.Set(csrfHeader, stableCloudUserID("other:csrf"))
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), session.ID) {
			t.Fatalf("%s %s status=%d body=%s", test.method, test.path, response.Code, response.Body.String())
		}
		stillLive, getErr := manager.Get(session.ID)
		if getErr != nil || stillLive.State != appaisession.StateRunning {
			t.Fatalf("denied %s changed session=%#v err=%v", test.path, stillLive, getErr)
		}
	}
	ownerGrant := httptest.NewRecorder()
	ownerGrantRequest := httptest.NewRequest(http.MethodPost, "/api/ai/sessions/"+session.ID+"/grant", nil)
	ownerGrantRequest.Header.Set("X-Kode-Stream-Subject", "owner")
	ownerGrantRequest.Header.Set("X-Kode-Stream-Role", "editor")
	ownerGrantRequest.Header.Set(csrfHeader, stableCloudUserID("owner:csrf"))
	handler.ServeHTTP(ownerGrant, ownerGrantRequest)
	if ownerGrant.Code != http.StatusOK {
		t.Fatalf("owner grant status=%d body=%s", ownerGrant.Code, ownerGrant.Body.String())
	}
	for _, test := range []struct {
		invoke func(http.ResponseWriter, *http.Request)
		body   string
	}{{apiHandler.aiSessions.launchWorkspaceAISession, `{}`}, {apiHandler.aiSessions.startEmbeddedWorkspaceAISession, `{"columns":80,"rows":24}`}} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/workspaces/owner-workspace/ai-sessions", strings.NewReader(test.body))
		request.SetPathValue("id", session.WorkspaceID)
		request = request.WithContext(context.WithValue(request.Context(), cloudSessionContextKey{}, cloudSession{User: models.CloudUser{ID: other, Role: models.CloudRoleEditor}}))
		test.invoke(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("direct launch authorization status=%d body=%s", response.Code, response.Body.String())
		}
	}
}

func TestCloudAuditReadsAreTenantScoped(t *testing.T) {
	apiHandler, _, _, store := reliabilityTestAPI(t)
	owner := stableCloudUserID("editor")
	for _, eventOwner := range []string{owner, stableCloudUserID("other"), ""} {
		if _, err := store.Append(models.AuditEvent{OwnerUserID: eventOwner, WorkspaceID: "workspace", Operation: "scan", Status: models.AuditStatusSuccess}); err != nil {
			t.Fatal(err)
		}
	}
	apiHandler = withTestRuntime(apiHandler, testCloudRuntimeConfig())
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/audit-events?limit=10", nil)
	request.Header.Set("X-Kode-Stream-Subject", "editor")
	request.Header.Set("X-Kode-Stream-Role", "editor")
	apiHandler.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var events []models.AuditEvent
	if err := json.Unmarshal(response.Body.Bytes(), &events); err != nil || len(events) != 1 || events[0].OwnerUserID != owner {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	if queried, err := store.QueryContext(context.Background(), audit.Query{OwnerUserID: owner}); err != nil || len(queried) != 1 {
		t.Fatalf("repository query=%#v err=%v", queried, err)
	}
}

func TestCloudCallbackBootstrapsAdminFromAllowlist(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := withTestRuntime(apiHandler, testAppOIDCRuntimeConfig()).Routes()

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
	handler := withTestRuntime(apiHandler, testAppOIDCRuntimeConfig()).Routes()

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

func TestCloudSystemAndStorageRoutesRequireSystemCapabilityForReads(t *testing.T) {
	apiHandler := &API{}
	apiHandler.initializeControllers()
	for _, route := range routeManifest(apiHandler) {
		if route.access != protectedRoute {
			continue
		}
		policy, ok := policyForRoute(route)
		if !ok {
			t.Fatalf("%s %s has no explicit policy", route.method, route.path)
		}
		if route.owner == "system" || route.owner == "storage" {
			if policy.capability != models.CapabilitySystem {
				t.Fatalf("%s %s policy = %#v", route.method, route.path, policy)
			}
			for _, role := range []models.CloudRole{models.CloudRoleViewer, models.CloudRoleEditor} {
				if roleCanAccess(role, policy) {
					t.Fatalf("role %q unexpectedly accesses %s %s", role, route.method, route.path)
				}
			}
			if !roleCanAccess(models.CloudRoleAdmin, policy) {
				t.Fatalf("admin lost System capability for %s %s", route.method, route.path)
			}
		}
	}
}

func TestDomain02SystemStorageRouteHTTPRoleMatrix(t *testing.T) {
	cloudAPI, _, _, _ := reliabilityTestAPI(t)
	cloudHandler := withTestRuntime(cloudAPI, testAppOIDCRuntimeConfig()).Routes()
	localHandler := (&API{}).Routes()
	for _, route := range routeManifest(cloudAPI) {
		if route.owner != "system" && route.owner != "storage" {
			continue
		}
		path := "/api" + route.path
		for _, role := range []models.CloudRole{models.CloudRoleViewer, models.CloudRoleEditor} {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(route.method, path, strings.NewReader(`{}`))
			request.Header.Set("X-Kode-Stream-Subject", string(role))
			request.Header.Set("X-Kode-Stream-Role", string(role))
			if isMutatingMethod(route.method) {
				request.Header.Set(csrfHeader, stableCloudUserID(string(role)+":csrf"))
			}
			cloudHandler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Errorf("%s %s role=%s status=%d body=%s", route.method, path, role, response.Code, response.Body.String())
			}
		}
		admin := httptest.NewRecorder()
		adminRequest := httptest.NewRequest(route.method, path, strings.NewReader(`{}`))
		adminRequest.Header.Set("X-Kode-Stream-Subject", "admin")
		adminRequest.Header.Set("X-Kode-Stream-Role", "admin")
		if isMutatingMethod(route.method) {
			adminRequest.Header.Set(csrfHeader, stableCloudUserID("admin:csrf"))
		}
		cloudHandler.ServeHTTP(admin, adminRequest)
		if admin.Code == http.StatusForbidden || admin.Code == http.StatusUnauthorized {
			t.Errorf("admin denied %s %s: %d %s", route.method, path, admin.Code, admin.Body.String())
		}

		local := httptest.NewRecorder()
		localHandler.ServeHTTP(local, httptest.NewRequest(route.method, path, strings.NewReader(`{}`)))
		if local.Code == http.StatusForbidden || local.Code == http.StatusUnauthorized {
			t.Errorf("local denied %s %s: %d %s", route.method, path, local.Code, local.Body.String())
		}
	}
}

func TestCloudEditorMutationsRequireCSRF(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := withTestRuntime(apiHandler, testAppOIDCRuntimeConfig()).Routes()

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
	handler := withTestRuntime(apiHandler, testAppOIDCRuntimeConfig()).Routes()

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

func TestCloudOauth2ProxyHeadersAuthenticateWithoutAppSession(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := withTestRuntime(apiHandler, testCloudRuntimeConfig()).Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	request.Header.Set("X-Auth-Request-User", "keycloak-subject")
	request.Header.Set("X-Auth-Request-Email", "admin@example.com")
	request.Header.Set("X-Auth-Request-Preferred-Username", "Admin")
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Mode models.RuntimeMode `json:"mode"`
		User models.CloudUser   `json:"user"`
		Role models.CloudRole   `json:"role"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Mode != models.RuntimeModeCloud || payload.User.Email != "admin@example.com" || payload.User.Name != "Admin" || payload.Role != models.CloudRoleAdmin {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestCloudOauth2ProxyMutationsRelyOnProxyCsrf(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := withTestRuntime(apiHandler, testCloudRuntimeConfig()).Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/agents/connect-token", strings.NewReader(`{}`))
	request.Header.Set("X-Auth-Request-User", "viewer")
	request.Header.Set("X-Auth-Request-Email", "viewer@example.com")
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
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
		AuthMode:     "oauth2_proxy",
		BindAddress:  "0.0.0.0",
		CookieSecret: "test-secret",
		AdminUsers:   []string{"admin@example.com"},
		Capabilities: map[models.Capability]bool{models.CapabilityRead: true},
		Agent:        models.AgentConnection{Available: false, Status: "offline"},
	}
	return config
}

func testAppOIDCRuntimeConfig() system.RuntimeConfig {
	config := testCloudRuntimeConfig()
	config.AuthMode = "app_oidc"
	config.OIDCIssuer = "https://issuer.example.com"
	config.OIDCClientID = "client"
	config.OIDCClientSecret = "secret"
	return config
}

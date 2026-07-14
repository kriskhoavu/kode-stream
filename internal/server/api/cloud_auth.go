package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"kode-stream/internal/common/models"
)

const (
	cloudSessionCookie = "kode_stream_session"
	csrfHeader         = "X-CSRF-Token"
)

type cloudSession struct {
	User      models.CloudUser `json:"user"`
	CSRFToken string           `json:"csrfToken"`
	ExpiresAt time.Time        `json:"expiresAt"`
}

type cloudSessionContextKey struct{}

func (a *API) registerCloudAuthRoutes(api *gin.RouterGroup) {
	api.GET("/auth/login", ginHTTPHandler(a.cloudLogin))
	api.GET("/auth/callback", ginHTTPHandler(a.cloudCallback))
	api.POST("/auth/logout", ginHTTPHandler(a.cloudLogout))
}

func (a *API) cloudAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if a.runtimeConfig.Mode != models.RuntimeModeCloud {
			c.Next()
			return
		}
		session, ok := a.readCloudSession(c.Request)
		if !ok {
			ginJSON(c, http.StatusUnauthorized, map[string]string{"error": "Cloud session is required", "code": "unauthorized"})
			c.Abort()
			return
		}
		if isMutatingMethod(c.Request.Method) && c.GetHeader(csrfHeader) != session.CSRFToken {
			ginJSON(c, http.StatusForbidden, map[string]string{"error": "CSRF token is required", "code": "forbidden"})
			c.Abort()
			return
		}
		if !roleCanAccess(session.User.Role, c.Request.Method, c.FullPath()) {
			ginJSON(c, http.StatusForbidden, map[string]string{"error": "role cannot access this route", "code": "forbidden"})
			c.Abort()
			return
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), cloudSessionContextKey{}, session))
		c.Next()
	}
}

func (a *API) cloudLogin(w http.ResponseWriter, r *http.Request) {
	session, ok := a.sessionFromTrustedHeaders(r)
	if !ok {
		http.Redirect(w, r, strings.TrimRight(a.runtimeConfig.OIDCIssuer, "/"), http.StatusFound)
		return
	}
	a.writeCloudSession(w, session)
	writeJSON(w, http.StatusOK, map[string]any{"user": session.User, "csrfToken": session.CSRFToken})
}

func (a *API) cloudCallback(w http.ResponseWriter, r *http.Request) {
	session, ok := a.sessionFromTrustedHeaders(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Cloud identity headers are required")
		return
	}
	a.writeCloudSession(w, session)
	writeJSON(w, http.StatusOK, map[string]any{"user": session.User, "csrfToken": session.CSRFToken})
}

func (a *API) cloudLogout(w http.ResponseWriter, r *http.Request) {
	session, ok := a.readCloudSession(r)
	if !ok || r.Header.Get(csrfHeader) != session.CSRFToken {
		writeError(w, http.StatusForbidden, "CSRF token is required")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cloudSessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) sessionFromTrustedHeaders(r *http.Request) (cloudSession, bool) {
	subject := strings.TrimSpace(r.Header.Get("X-Kode-Stream-Subject"))
	email := strings.TrimSpace(r.Header.Get("X-Kode-Stream-Email"))
	if subject == "" {
		subject = email
	}
	if subject == "" {
		return cloudSession{}, false
	}
	role := models.CloudRole(strings.TrimSpace(r.Header.Get("X-Kode-Stream-Role")))
	if role != models.CloudRoleAdmin && role != models.CloudRoleEditor && role != models.CloudRoleViewer {
		role = models.CloudRoleViewer
	}
	if slices.Contains(a.runtimeConfig.AdminUsers, email) || slices.Contains(a.runtimeConfig.AdminUsers, subject) {
		role = models.CloudRoleAdmin
	}
	user := models.CloudUser{
		ID:      stableCloudUserID(subject),
		Email:   email,
		Name:    strings.TrimSpace(r.Header.Get("X-Kode-Stream-Name")),
		Role:    role,
		Subject: subject,
	}
	return cloudSession{User: user, CSRFToken: stableCloudUserID(subject + ":csrf"), ExpiresAt: time.Now().UTC().Add(12 * time.Hour)}, true
}

func (a *API) writeCloudSession(w http.ResponseWriter, session cloudSession) {
	value := a.signCloudSession(session)
	http.SetCookie(w, &http.Cookie{Name: cloudSessionCookie, Value: value, Path: "/", Expires: session.ExpiresAt, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func (a *API) readCloudSession(r *http.Request) (cloudSession, bool) {
	if session, ok := a.sessionFromTrustedHeaders(r); ok {
		return session, true
	}
	cookie, err := r.Cookie(cloudSessionCookie)
	if err != nil {
		return cloudSession{}, false
	}
	return a.verifyCloudSession(cookie.Value)
}

func (a *API) signCloudSession(session cloudSession) string {
	data, _ := json.Marshal(session)
	payload := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, []byte(a.runtimeConfig.CookieSecret))
	mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + signature
}

func (a *API) verifyCloudSession(value string) (cloudSession, bool) {
	payload, signature, ok := strings.Cut(value, ".")
	if !ok || payload == "" || signature == "" || a.runtimeConfig.CookieSecret == "" {
		return cloudSession{}, false
	}
	mac := hmac.New(sha256.New, []byte(a.runtimeConfig.CookieSecret))
	mac.Write([]byte(payload))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return cloudSession{}, false
	}
	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return cloudSession{}, false
	}
	var session cloudSession
	if err := json.Unmarshal(data, &session); err != nil || time.Now().UTC().After(session.ExpiresAt) {
		return cloudSession{}, false
	}
	return session, true
}

func cloudSessionFromContext(ctx context.Context) (cloudSession, bool) {
	session, ok := ctx.Value(cloudSessionContextKey{}).(cloudSession)
	return session, ok
}

func stableCloudUserID(input string) string {
	sum := sha256.Sum256([]byte(input))
	return base64.RawURLEncoding.EncodeToString(sum[:])[:22]
}

func isMutatingMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func roleCanAccess(role models.CloudRole, method, path string) bool {
	if role == models.CloudRoleAdmin {
		return true
	}
	if !isMutatingMethod(method) {
		return true
	}
	if role == models.CloudRoleViewer {
		return false
	}
	return !strings.Contains(path, "/system/") && !strings.Contains(path, "/ai/settings") && !strings.Contains(path, "/config-paths")
}

func roleCapabilities(role models.CloudRole) map[models.Capability]bool {
	capabilities := map[models.Capability]bool{
		models.CapabilityRead:                  true,
		models.CapabilityGit:                   true,
		models.CapabilityWorkspaceRegistration: role == models.CloudRoleAdmin || role == models.CloudRoleEditor,
		models.CapabilityWrite:                 role == models.CloudRoleAdmin || role == models.CloudRoleEditor,
		models.CapabilitySystem:                role == models.CloudRoleAdmin,
		models.CapabilityTerminal:              role == models.CloudRoleAdmin || role == models.CloudRoleEditor,
		models.CapabilityAI:                    role == models.CloudRoleAdmin || role == models.CloudRoleEditor,
		models.CapabilityRuntime:               role == models.CloudRoleAdmin || role == models.CloudRoleEditor,
		models.CapabilityVerification:          role == models.CloudRoleAdmin || role == models.CloudRoleEditor,
	}
	return capabilities
}

package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"plan-manager/internal/models"
)

func TestCloudConnectionUsesBasicAuthAndV3(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		user, token, ok := r.BasicAuth()
		if !ok || user != "user@example.com" || token != "secret" || !strings.HasPrefix(r.URL.Path, "/rest/api/3/") {
			t.Fatalf("path=%s auth=%q,%q,%v", r.URL.Path, user, token, ok)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := New()
	client.getenv = func(string) string { return "secret" }
	result, err := client.TestConnection(context.Background(), models.JiraConnection{DeploymentType: "cloud", BaseURL: server.URL, ProjectKey: "DI", AccountEmail: "user@example.com", TokenEnvVar: "JIRA_TOKEN"})
	if err != nil || !result.OK || requests != 2 {
		t.Fatalf("result=%#v requests=%d err=%v", result, requests, err)
	}
}

func TestServerConnectionUsesBearerAuthAndReportsFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer pat" || !strings.HasPrefix(r.URL.Path, "/rest/api/2/") {
			t.Fatalf("path=%s auth=%s", r.URL.Path, r.Header.Get("Authorization"))
		}
		http.Error(w, "denied", http.StatusUnauthorized)
	}))
	defer server.Close()
	client := New()
	client.getenv = func(string) string { return "pat" }
	_, err := client.TestConnection(context.Background(), models.JiraConnection{DeploymentType: "server", BaseURL: server.URL, ProjectKey: "DI", TokenEnvVar: "JIRA_PAT"})
	if err == nil || err.Error() != "Jira authentication failed" {
		t.Fatalf("err = %v", err)
	}
}

func TestConnectionRequiresEnvironmentToken(t *testing.T) {
	client := New()
	client.getenv = func(string) string { return "" }
	_, err := client.TestConnection(context.Background(), models.JiraConnection{TokenEnvVar: "JIRA_TOKEN"})
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("err = %v", err)
	}
}

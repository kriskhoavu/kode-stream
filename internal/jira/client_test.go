package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestConnectionFallsBackToCredentialsFile(t *testing.T) {
	home := t.TempDir()
	credsPath := filepath.Join(home, ".creds.zsh")
	if err := os.WriteFile(credsPath, []byte("export CC_JIRA_API_TOKEN=\"secret\"\n"), 0o600); err != nil {
		t.Fatalf("write creds: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, token, ok := r.BasicAuth()
		if !ok || user != "user@example.com" || token != "secret" {
			t.Fatalf("auth=%q,%q,%v", user, token, ok)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := New()
	client.getenv = func(string) string { return "" }
	client.homeDir = func() (string, error) { return home, nil }
	client.credentialFiles = []string{"~/.creds.zsh"}
	result, err := client.TestConnection(context.Background(), models.JiraConnection{DeploymentType: "cloud", BaseURL: server.URL, ProjectKey: "DI", AccountEmail: "user@example.com", TokenEnvVar: "CC_JIRA_API_TOKEN"})
	if err != nil || !result.OK {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestConnectionIgnoresBareCredentialsAssignments(t *testing.T) {
	home := t.TempDir()
	credsPath := filepath.Join(home, ".creds.zsh")
	if err := os.WriteFile(credsPath, []byte("CC_JIRA_API_TOKEN=\"secret\"\n"), 0o600); err != nil {
		t.Fatalf("write creds: %v", err)
	}
	client := New()
	client.getenv = func(string) string { return "" }
	client.homeDir = func() (string, error) { return home, nil }
	client.credentialFiles = []string{"~/.creds.zsh"}
	token, err := client.resolveToken("CC_JIRA_API_TOKEN")
	if err != nil {
		t.Fatalf("resolveToken err=%v", err)
	}
	if token != "" {
		t.Fatalf("token=%q", token)
	}
}

func TestGetIssueNormalizesCloudADFAndAttachments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"key":"DI-170","fields":{"summary":"Search","description":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello Jira"}]}]},"status":{"name":"In Progress"},"issuetype":{"name":"Story"},"assignee":{"displayName":"Kim","accountId":"a1"},"reporter":null,"priority":{"name":"High"},"labels":["backend"],"created":"2026-01-01","updated":"2026-01-02","attachment":[{"id":"9","filename":"spec.pdf","mimeType":"application/pdf","size":12,"content":"https://files/9","author":{"displayName":"Kim"}}]}}`))
	}))
	defer server.Close()
	client := New()
	client.getenv = func(string) string { return "token" }
	issue, err := client.GetIssue(context.Background(), models.JiraConnection{DeploymentType: "cloud", BaseURL: server.URL, AccountEmail: "a@b.com", TokenEnvVar: "TOKEN"}, "DI-170")
	if err != nil || issue.Description != "Hello Jira" || issue.Status != "In Progress" || len(issue.Attachments) != 1 || issue.Attachments[0].ContentURL == "" {
		t.Fatalf("issue=%#v err=%v", issue, err)
	}
}

func TestGetIssueNormalizesServerTextAndStatusErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "missing", http.StatusNotFound) }))
	defer server.Close()
	client := New()
	client.getenv = func(string) string { return "token" }
	_, err := client.GetIssue(context.Background(), models.JiraConnection{DeploymentType: "server", BaseURL: server.URL, TokenEnvVar: "TOKEN"}, "DI-404")
	if err != ErrNotFound {
		t.Fatalf("err=%v", err)
	}
}

func TestGetIssueReportsMalformedAndTimedOutResponses(t *testing.T) {
	malformed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"key":`)) }))
	defer malformed.Close()
	client := New()
	client.getenv = func(string) string { return "token" }
	_, err := client.GetIssue(context.Background(), models.JiraConnection{DeploymentType: "server", BaseURL: malformed.URL, TokenEnvVar: "TOKEN"}, "DI-1")
	if err == nil || !strings.Contains(err.Error(), "decode Jira issue") {
		t.Fatalf("malformed err=%v", err)
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(50 * time.Millisecond) }))
	defer slow.Close()
	client.httpClient.Timeout = 5 * time.Millisecond
	_, err = client.GetIssue(context.Background(), models.JiraConnection{DeploymentType: "server", BaseURL: slow.URL, TokenEnvVar: "TOKEN"}, "DI-1")
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("timeout err=%v", err)
	}
}

func TestGetAttachmentChecksOriginSizeAndContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png; charset=binary")
		_, _ = w.Write([]byte("png"))
	}))
	defer server.Close()
	client := New()
	client.getenv = func(string) string { return "token" }
	connection := models.JiraConnection{DeploymentType: "server", BaseURL: server.URL, TokenEnvVar: "TOKEN"}
	content, err := client.GetAttachment(context.Background(), connection, Attachment{ID: "1", Filename: "image.png", ContentURL: server.URL + "/file"})
	if err != nil || string(content.Data) != "png" || content.MediaType != "image/png" {
		t.Fatalf("content=%#v err=%v", content, err)
	}
	_, err = client.GetAttachment(context.Background(), connection, Attachment{ContentURL: "https://evil.example/file"})
	if err == nil || !strings.Contains(err.Error(), "changed origin") {
		t.Fatalf("origin err=%v", err)
	}
}

func TestGetAttachmentRejectsDeclaredOversize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "30000000")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := New()
	client.getenv = func(string) string { return "token" }
	_, err := client.GetAttachment(context.Background(), models.JiraConnection{DeploymentType: "server", BaseURL: server.URL, TokenEnvVar: "TOKEN"}, Attachment{ContentURL: server.URL + "/large"})
	if err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("err=%v", err)
	}
}

func TestGetAttachmentRejectsCrossOriginRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("secret")) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer redirect.Close()
	client := New()
	client.getenv = func(string) string { return "token" }
	_, err := client.GetAttachment(context.Background(), models.JiraConnection{DeploymentType: "server", BaseURL: redirect.URL, TokenEnvVar: "TOKEN"}, Attachment{ContentURL: redirect.URL + "/file"})
	if err == nil || !strings.Contains(err.Error(), "changed origin") {
		t.Fatalf("err=%v", err)
	}
}

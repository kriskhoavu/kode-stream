package jira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"plan-manager/internal/models"
)

type ConnectionTest struct {
	OK             bool   `json:"ok"`
	DeploymentType string `json:"deploymentType"`
	ProjectKey     string `json:"projectKey"`
	Message        string `json:"message"`
	RecoveryHint   string `json:"recoveryHint,omitempty"`
}

type Client struct {
	httpClient      *http.Client
	getenv          func(string) string
	readFile        func(string) ([]byte, error)
	homeDir         func() (string, error)
	credentialFiles []string
}

var (
	ErrAuthentication = errors.New("Jira authentication failed")
	ErrForbidden      = errors.New("Jira access is forbidden")
	ErrNotFound       = errors.New("Jira issue was not found")
)

type Person struct {
	DisplayName string `json:"displayName"`
	AccountID   string `json:"accountId,omitempty"`
	Email       string `json:"email,omitempty"`
}

type Attachment struct {
	ID         string `json:"id"`
	Filename   string `json:"filename"`
	MediaType  string `json:"mediaType"`
	SizeBytes  int64  `json:"sizeBytes"`
	CreatedAt  string `json:"createdAt,omitempty"`
	Author     Person `json:"author"`
	ContentURL string `json:"-"`
}

type Issue struct {
	Key         string       `json:"key"`
	Summary     string       `json:"summary"`
	Status      string       `json:"status"`
	Description string       `json:"description"`
	IssueType   string       `json:"issueType"`
	Assignee    *Person      `json:"assignee,omitempty"`
	Reporter    *Person      `json:"reporter,omitempty"`
	Priority    string       `json:"priority,omitempty"`
	Labels      []string     `json:"labels"`
	CreatedAt   string       `json:"createdAt,omitempty"`
	UpdatedAt   string       `json:"updatedAt,omitempty"`
	BrowserURL  string       `json:"browserUrl"`
	Attachments []Attachment `json:"attachments"`
}

type AttachmentContent struct {
	Data      []byte
	MediaType string
	Filename  string
}

const MaxAttachmentBytes int64 = 25 * 1024 * 1024

func (c *Client) GetAttachment(ctx context.Context, connection models.JiraConnection, attachment Attachment) (AttachmentContent, error) {
	target, err := url.Parse(attachment.ContentURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return AttachmentContent{}, errors.New("Jira attachment URL is invalid")
	}
	base, _ := url.Parse(connection.BaseURL)
	if !strings.EqualFold(target.Scheme, base.Scheme) || !strings.EqualFold(target.Host, base.Host) {
		return AttachmentContent{}, errors.New("Jira attachment URL changed origin")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return AttachmentContent{}, err
	}
	if err := c.authorize(request, connection); err != nil {
		return AttachmentContent{}, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return AttachmentContent{}, fmt.Errorf("Jira attachment is unavailable: %w", err)
	}
	if err := responseError(response, errors.New("Jira attachment was not found")); err != nil {
		return AttachmentContent{}, err
	}
	defer response.Body.Close()
	if response.ContentLength > MaxAttachmentBytes {
		return AttachmentContent{}, errors.New("Jira attachment exceeds the size limit")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxAttachmentBytes+1))
	if err != nil {
		return AttachmentContent{}, err
	}
	if int64(len(data)) > MaxAttachmentBytes {
		return AttachmentContent{}, errors.New("Jira attachment exceeds the size limit")
	}
	mediaType := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	if mediaType == "" {
		mediaType = attachment.MediaType
	}
	return AttachmentContent{Data: data, MediaType: mediaType, Filename: attachment.Filename}, nil
}

type jiraIssueResponse struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		Status      struct {
			Name string `json:"name"`
		} `json:"status"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Assignee *jiraPerson `json:"assignee"`
		Reporter *jiraPerson `json:"reporter"`
		Priority *struct {
			Name string `json:"name"`
		} `json:"priority"`
		Labels     []string `json:"labels"`
		Created    string   `json:"created"`
		Updated    string   `json:"updated"`
		Attachment []struct {
			ID       string     `json:"id"`
			Filename string     `json:"filename"`
			MimeType string     `json:"mimeType"`
			Size     int64      `json:"size"`
			Created  string     `json:"created"`
			Content  string     `json:"content"`
			Author   jiraPerson `json:"author"`
		} `json:"attachment"`
	} `json:"fields"`
}

type jiraPerson struct {
	DisplayName  string `json:"displayName"`
	AccountID    string `json:"accountId"`
	EmailAddress string `json:"emailAddress"`
	Name         string `json:"name"`
}

func (c *Client) GetIssue(ctx context.Context, connection models.JiraConnection, key string) (Issue, error) {
	version := "2"
	if connection.DeploymentType == "cloud" {
		version = "3"
	}
	endpoint := connection.BaseURL + "/rest/api/" + version + "/issue/" + url.PathEscape(key) + "?fields=summary,status,description,issuetype,assignee,reporter,priority,labels,created,updated,attachment"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Issue{}, err
	}
	if err := c.authorize(request, connection); err != nil {
		return Issue{}, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return Issue{}, fmt.Errorf("Jira is unavailable: %w", err)
	}
	if err := responseError(response, ErrNotFound); err != nil {
		return Issue{}, err
	}
	var payload jiraIssueResponse
	if err := decodeBounded(response, &payload); err != nil {
		return Issue{}, fmt.Errorf("decode Jira issue: %w", err)
	}
	issue := Issue{Key: payload.Key, Summary: payload.Fields.Summary, Status: payload.Fields.Status.Name, Description: normalizeDescription(payload.Fields.Description), IssueType: payload.Fields.IssueType.Name, Assignee: normalizePerson(payload.Fields.Assignee), Reporter: normalizePerson(payload.Fields.Reporter), Labels: payload.Fields.Labels, CreatedAt: payload.Fields.Created, UpdatedAt: payload.Fields.Updated, BrowserURL: connection.BaseURL + "/browse/" + url.PathEscape(payload.Key), Attachments: []Attachment{}}
	if issue.Labels == nil {
		issue.Labels = []string{}
	}
	if payload.Fields.Priority != nil {
		issue.Priority = payload.Fields.Priority.Name
	}
	for _, value := range payload.Fields.Attachment {
		issue.Attachments = append(issue.Attachments, Attachment{ID: value.ID, Filename: value.Filename, MediaType: value.MimeType, SizeBytes: value.Size, CreatedAt: value.Created, ContentURL: value.Content, Author: *normalizePerson(&value.Author)})
	}
	return issue, nil
}

func (c *Client) authorize(request *http.Request, connection models.JiraConnection) error {
	token, err := c.resolveToken(connection.TokenEnvVar)
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("Jira token environment variable %s is not available in the process environment or supported credentials files", connection.TokenEnvVar)
	}
	request.Header.Set("Accept", "application/json")
	if connection.DeploymentType == "cloud" {
		request.SetBasicAuth(connection.AccountEmail, token)
	} else {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

func responseError(response *http.Response, notFound error) error {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
	switch response.StatusCode {
	case http.StatusUnauthorized:
		return ErrAuthentication
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return notFound
	default:
		return fmt.Errorf("Jira returned status %d", response.StatusCode)
	}
}

func normalizePerson(value *jiraPerson) *Person {
	if value == nil {
		return nil
	}
	id := value.AccountID
	if id == "" {
		id = value.Name
	}
	return &Person{DisplayName: value.DisplayName, AccountID: id, Email: value.EmailAddress}
}

func normalizeDescription(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	var parts []string
	var walk func(any)
	walk = func(node any) {
		switch current := node.(type) {
		case map[string]any:
			if current["type"] == "text" {
				if value, ok := current["text"].(string); ok {
					parts = append(parts, value)
				}
			}
			if content, ok := current["content"].([]any); ok {
				for _, child := range content {
					walk(child)
				}
				if current["type"] == "paragraph" {
					parts = append(parts, "\n")
				}
			}
		case []any:
			for _, child := range current {
				walk(child)
			}
		}
	}
	walk(value)
	return strings.TrimSpace(strings.Join(parts, ""))
}

func New() *Client {
	return &Client{
		httpClient:      &http.Client{Timeout: 12 * time.Second, CheckRedirect: sameOriginRedirect},
		getenv:          os.Getenv,
		readFile:        os.ReadFile,
		homeDir:         os.UserHomeDir,
		credentialFiles: []string{"~/.creds.zsh", "~/.creds.sh"},
	}
}

func (c *Client) resolveToken(name string) (string, error) {
	token := strings.TrimSpace(c.getenv(name))
	if token != "" {
		return token, nil
	}
	for _, candidate := range c.credentialFiles {
		path, err := c.expandHome(candidate)
		if err != nil {
			return "", err
		}
		value, ok, err := c.loadTokenFromFile(path, name)
		if err != nil {
			return "", err
		}
		if ok {
			return value, nil
		}
	}
	return "", nil
}

func (c *Client) expandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := c.homeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func (c *Client) loadTokenFromFile(path, name string) (string, bool, error) {
	data, err := c.readFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		value, ok := parseEnvAssignment(line, name)
		if ok {
			return value, true, nil
		}
	}
	return "", false, nil
}

func parseEnvAssignment(line, name string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	if !strings.HasPrefix(trimmed, "export ") {
		return "", false
	}
	trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
	key, raw, ok := strings.Cut(trimmed, "=")
	if !ok || strings.TrimSpace(key) != name {
		return "", false
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", true
	}
	if len(value) >= 2 {
		switch value[0] {
		case '"':
			if value[len(value)-1] == '"' {
				return value[1 : len(value)-1], true
			}
		case '\'':
			if value[len(value)-1] == '\'' {
				return value[1 : len(value)-1], true
			}
		}
	}
	if idx := strings.Index(value, " #"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	return value, true
}

func (c *Client) TestConnection(ctx context.Context, connection models.JiraConnection) (ConnectionTest, error) {
	version := "2"
	if connection.DeploymentType == "cloud" {
		version = "3"
	}
	for _, endpoint := range []string{"myself", "project/" + url.PathEscape(connection.ProjectKey)} {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, connection.BaseURL+"/rest/api/"+version+"/"+endpoint, nil)
		if err != nil {
			return ConnectionTest{}, err
		}
		if err := c.authorize(request, connection); err != nil {
			return ConnectionTest{}, err
		}
		response, err := c.httpClient.Do(request)
		if err != nil {
			return ConnectionTest{}, fmt.Errorf("Jira is unavailable: %w", err)
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
		response.Body.Close()
		if response.StatusCode == http.StatusUnauthorized {
			return ConnectionTest{}, errors.New("Jira authentication failed")
		}
		if response.StatusCode == http.StatusForbidden {
			return ConnectionTest{}, errors.New("Jira access is forbidden")
		}
		if response.StatusCode == http.StatusNotFound && strings.HasPrefix(endpoint, "project/") {
			return ConnectionTest{}, errors.New("Jira project was not found")
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return ConnectionTest{}, fmt.Errorf("Jira returned status %d", response.StatusCode)
		}
	}
	return ConnectionTest{OK: true, DeploymentType: connection.DeploymentType, ProjectKey: connection.ProjectKey, Message: "Jira connection succeeded"}, nil
}

func sameOriginRedirect(request *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	if !strings.EqualFold(request.URL.Scheme, via[0].URL.Scheme) || !strings.EqualFold(request.URL.Host, via[0].URL.Host) {
		return errors.New("Jira redirect changed origin")
	}
	if len(via) >= 5 {
		return errors.New("too many Jira redirects")
	}
	return nil
}

func decodeBounded(response *http.Response, target any) error {
	defer response.Body.Close()
	return json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(target)
}

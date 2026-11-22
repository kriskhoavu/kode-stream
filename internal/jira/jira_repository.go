package jira

// Client is the Jira HTTP repository used by JiraService.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"kode-stream/internal/common/models"
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
	Key     string `json:"key"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
	// StatusCategory is Jira's own coarse grouping ("new", "indeterminate",
	// "done"). Workflow status names are project-specific, so this is the only
	// reliable signal for colouring a status.
	StatusCategory string       `json:"statusCategory,omitempty"`
	Description    string       `json:"description"`
	IssueType      string       `json:"issueType"`
	Assignee       *Person      `json:"assignee,omitempty"`
	Reporter       *Person      `json:"reporter,omitempty"`
	Priority       string       `json:"priority,omitempty"`
	Labels         []string     `json:"labels"`
	CreatedAt      string       `json:"createdAt,omitempty"`
	UpdatedAt      string       `json:"updatedAt,omitempty"`
	BrowserURL     string       `json:"browserUrl"`
	Attachments    []Attachment `json:"attachments"`
	Truncated      bool         `json:"truncated,omitempty"`
	Warnings       []string     `json:"warnings,omitempty"`
}

const (
	maxDescriptionBytes = 64 << 10
	maxScalarBytes      = 8 << 10
	maxLabels           = 64
	maxAttachments      = 100
	maxADFDepth         = 64
)

type jiraIssueResponse struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		Status      struct {
			Name           string `json:"name"`
			StatusCategory struct {
				Key  string `json:"key"`
				Name string `json:"name"`
			} `json:"statusCategory"`
		} `json:"status"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Assignee *jiraPerson `json:"assignee"`
		Reporter *jiraPerson `json:"reporter"`
		Priority *struct {
			Name string `json:"name"`
		} `json:"priority"`
		Labels     []string         `json:"labels"`
		Created    string           `json:"created"`
		Updated    string           `json:"updated"`
		Attachment []jiraAttachment `json:"attachment"`
	} `json:"fields"`
}

type jiraAttachment struct {
	ID       string     `json:"id"`
	Filename string     `json:"filename"`
	MimeType string     `json:"mimeType"`
	Size     int64      `json:"size"`
	Created  string     `json:"created"`
	Content  string     `json:"content"`
	Author   jiraPerson `json:"author"`
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
	var payload jiraIssueResponse
	if err := c.getJSON(ctx, connection, endpoint, ErrNotFound, "Jira issue", &payload); err != nil {
		return Issue{}, err
	}
	clip := &clipper{}
	description, truncated := normalizeDescription(payload.Fields.Description)
	clip.truncated = truncated
	if description == "" {
		// Projects often keep the prose in a custom field ("User Story
		// Description" and friends) and leave the standard one empty. Only pay
		// for the wider lookup when the standard field gave us nothing.
		if custom, customTruncated, err := c.customDescription(ctx, connection, version, key); err == nil && custom != "" {
			description = custom
			clip.truncated = clip.truncated || customTruncated
		}
	}
	issue := issueFromResponse(payload, connection.BaseURL, description, clip)
	issue.Truncated = clip.truncated
	if clip.truncated {
		issue.Warnings = []string{"Jira issue content was truncated to safe limits."}
	}
	return issue, nil
}

// clipper bounds Jira strings and remembers whether any was cut.
type clipper struct{ truncated bool }

func (c *clipper) text(value string) string {
	bounded, clipped := boundedString(value, maxScalarBytes)
	c.truncated = c.truncated || clipped
	return bounded
}

func (c *clipper) person(value *jiraPerson) *Person {
	person, clipped := normalizePerson(value)
	c.truncated = c.truncated || clipped
	return person
}

func issueFromResponse(payload jiraIssueResponse, baseURL, description string, clip *clipper) Issue {
	fields := payload.Fields
	issue := Issue{
		Key:            clip.text(payload.Key),
		Summary:        clip.text(fields.Summary),
		Status:         clip.text(fields.Status.Name),
		IssueType:      clip.text(fields.IssueType.Name),
		CreatedAt:      clip.text(fields.Created),
		UpdatedAt:      clip.text(fields.Updated),
		Assignee:       clip.person(fields.Assignee),
		Reporter:       clip.person(fields.Reporter),
		StatusCategory: clip.text(fields.Status.StatusCategory.Key),
		Description:    description,
		BrowserURL:     baseURL + "/browse/" + url.PathEscape(payload.Key),
	}
	issue.Labels = boundedStrings(fields.Labels, maxLabels, maxScalarBytes, &clip.truncated)
	if fields.Priority != nil {
		issue.Priority = clip.text(fields.Priority.Name)
	}
	issue.Attachments = boundedAttachments(fields.Attachment, clip)
	return issue
}

// boundedAttachments keeps at most maxAttachments attachments. An attachment
// whose content URL was clipped is dropped: a clipped proxy URL is not a
// stable authorization target.
func boundedAttachments(values []jiraAttachment, clip *clipper) []Attachment {
	attachments := []Attachment{}
	for i, value := range values {
		if i >= maxAttachments {
			clip.truncated = true
			break
		}
		person := clip.person(&value.Author)
		if person == nil {
			person = &Person{}
		}
		contentURL, contentClipped := boundedString(value.Content, maxScalarBytes)
		clip.truncated = clip.truncated || contentClipped
		attachment := Attachment{ID: clip.text(value.ID), Filename: clip.text(value.Filename), MediaType: clip.text(value.MimeType), SizeBytes: value.Size, CreatedAt: clip.text(value.Created), ContentURL: contentURL, Author: *person}
		if contentClipped {
			continue
		}
		attachments = append(attachments, attachment)
	}
	return attachments
}

// getJSON sends an authorized GET and decodes a bounded JSON answer. A 404
// answers notFound.
func (c *Client) getJSON(ctx context.Context, connection models.JiraConnection, endpoint string, notFound error, what string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if err := c.authorize(request, connection); err != nil {
		return err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("Jira is unavailable: %w", err)
	}
	if err := responseError(response, notFound); err != nil {
		return err
	}
	if err := decodeBounded(response, target); err != nil {
		return fmt.Errorf("decode %s: %w", what, err)
	}
	return nil
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

func normalizePerson(value *jiraPerson) (*Person, bool) {
	if value == nil {
		return nil, false
	}
	id := value.AccountID
	if id == "" {
		id = value.Name
	}
	displayName, displayClipped := boundedString(value.DisplayName, maxScalarBytes)
	accountID, idClipped := boundedString(id, maxScalarBytes)
	email, emailClipped := boundedString(value.EmailAddress, maxScalarBytes)
	return &Person{DisplayName: displayName, AccountID: accountID, Email: email}, displayClipped || idClipped || emailClipped
}

func boundedString(value string, limit int) (string, bool) {
	if len(value) <= limit {
		return value, false
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut], true
}

func boundedStrings(values []string, count, bytes int, truncated *bool) []string {
	result := make([]string, 0, min(len(values), count))
	for i, value := range values {
		if i >= count {
			*truncated = true
			break
		}
		bounded, clipped := boundedString(value, bytes)
		if clipped {
			*truncated = true
		}
		result = append(result, bounded)
	}
	return result
}

// customDescription looks for a description held in a custom field. Jira only
// reports human-readable field names under expand=names, so the field is
// matched by its display name rather than a hard-coded customfield id.
func (c *Client) customDescription(ctx context.Context, connection models.JiraConnection, version, key string) (string, bool, error) {
	endpoint := connection.BaseURL + "/rest/api/" + version + "/issue/" + url.PathEscape(key) + "?expand=names&fields=*all"
	var payload struct {
		Names  map[string]string          `json:"names"`
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := c.getJSON(ctx, connection, endpoint, ErrNotFound, "Jira issue fields", &payload); err != nil {
		return "", false, err
	}
	ids := make([]string, 0, len(payload.Names))
	for id, name := range payload.Names {
		if id == "description" || !strings.Contains(strings.ToLower(name), "description") {
			continue
		}
		ids = append(ids, id)
	}
	// Stable order so the same ticket always resolves to the same field.
	sort.Strings(ids)
	for _, id := range ids {
		if text, truncated := normalizeDescription(payload.Fields[id]); text != "" {
			return text, truncated, nil
		}
	}
	return "", false, nil
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
			return ConnectionTest{}, ErrAuthentication
		}
		if response.StatusCode == http.StatusForbidden {
			return ConnectionTest{}, ErrForbidden
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

func decodeBounded(response *http.Response, target any) error {
	defer response.Body.Close()
	const maxResponseBytes = 2 * 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxResponseBytes {
		return errors.New("Jira response exceeds the size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("Jira response contains multiple JSON values")
		}
		return err
	}
	return nil
}

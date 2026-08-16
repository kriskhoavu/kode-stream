package jira

// Client is the Jira HTTP repository used by JiraService.

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
	"sort"
	"strconv"
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

type AttachmentContent struct {
	Body           io.ReadCloser
	MediaType      string
	Filename       string
	DeclaredLength int64
}

const MaxAttachmentBytes int64 = 25 * 1024 * 1024

const (
	maxDescriptionBytes = 64 << 10
	maxScalarBytes      = 8 << 10
	maxLabels           = 64
	maxAttachments      = 100
	maxADFDepth         = 64
)

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
	attachmentClient := *c.httpClient
	attachmentClient.CheckRedirect = attachmentRedirectPolicy(connection)
	response, err := attachmentClient.Do(request)
	if err != nil {
		return AttachmentContent{}, fmt.Errorf("Jira attachment is unavailable: %w", err)
	}
	if err := responseError(response, errors.New("Jira attachment was not found")); err != nil {
		return AttachmentContent{}, err
	}
	if response.ContentLength > MaxAttachmentBytes {
		response.Body.Close()
		return AttachmentContent{}, errors.New("Jira attachment exceeds the size limit")
	}
	mediaType := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	if mediaType == "" {
		mediaType = attachment.MediaType
	}
	return AttachmentContent{Body: response.Body, MediaType: mediaType, Filename: attachment.Filename, DeclaredLength: response.ContentLength}, nil
}

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
	description, truncated := normalizeDescription(payload.Fields.Description)
	if description == "" {
		// Projects often keep the prose in a custom field ("User Story
		// Description" and friends) and leave the standard one empty. Only pay
		// for the wider lookup when the standard field gave us nothing.
		if custom, customTruncated, err := c.customDescription(ctx, connection, version, key); err == nil && custom != "" {
			description = custom
			truncated = truncated || customTruncated
		}
	}
	keyValue, clipped := boundedString(payload.Key, maxScalarBytes)
	truncated = truncated || clipped
	summary, clipped := boundedString(payload.Fields.Summary, maxScalarBytes)
	truncated = truncated || clipped
	status, clipped := boundedString(payload.Fields.Status.Name, maxScalarBytes)
	truncated = truncated || clipped
	issueType, clipped := boundedString(payload.Fields.IssueType.Name, maxScalarBytes)
	truncated = truncated || clipped
	created, clipped := boundedString(payload.Fields.Created, maxScalarBytes)
	truncated = truncated || clipped
	updated, clipped := boundedString(payload.Fields.Updated, maxScalarBytes)
	truncated = truncated || clipped
	assignee, clipped := normalizePerson(payload.Fields.Assignee)
	truncated = truncated || clipped
	reporter, clipped := normalizePerson(payload.Fields.Reporter)
	truncated = truncated || clipped
	statusCategory, clipped := boundedString(payload.Fields.Status.StatusCategory.Key, maxScalarBytes)
	truncated = truncated || clipped
	issue := Issue{Key: keyValue, Summary: summary, Status: status, StatusCategory: statusCategory, Description: description, IssueType: issueType, Assignee: assignee, Reporter: reporter, Labels: boundedStrings(payload.Fields.Labels, maxLabels, maxScalarBytes, &truncated), CreatedAt: created, UpdatedAt: updated, BrowserURL: connection.BaseURL + "/browse/" + url.PathEscape(payload.Key), Attachments: []Attachment{}}
	if payload.Fields.Priority != nil {
		issue.Priority, clipped = boundedString(payload.Fields.Priority.Name, maxScalarBytes)
		truncated = truncated || clipped
	}
	for i, value := range payload.Fields.Attachment {
		if i >= maxAttachments {
			truncated = true
			break
		}
		person, personClipped := normalizePerson(&value.Author)
		truncated = truncated || personClipped
		if person == nil {
			person = &Person{}
		}
		id, idClipped := boundedString(value.ID, maxScalarBytes)
		filename, filenameClipped := boundedString(value.Filename, maxScalarBytes)
		mediaType, mediaTypeClipped := boundedString(value.MimeType, maxScalarBytes)
		attachmentCreated, createdClipped := boundedString(value.Created, maxScalarBytes)
		contentURL, contentClipped := boundedString(value.Content, maxScalarBytes)
		truncated = truncated || idClipped || filenameClipped || mediaTypeClipped || createdClipped || contentClipped
		// A clipped proxy URL is not a stable authorization target. Do not expose
		// that attachment at all.
		if contentClipped {
			continue
		}
		issue.Attachments = append(issue.Attachments, Attachment{ID: id, Filename: filename, MediaType: mediaType, SizeBytes: value.Size, CreatedAt: attachmentCreated, ContentURL: contentURL, Author: *person})
	}
	issue.Truncated = truncated
	if truncated {
		issue.Warnings = []string{"Jira issue content was truncated to safe limits."}
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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", false, err
	}
	if err := c.authorize(request, connection); err != nil {
		return "", false, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", false, err
	}
	if err := responseError(response, ErrNotFound); err != nil {
		return "", false, err
	}
	var payload struct {
		Names  map[string]string          `json:"names"`
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := decodeBounded(response, &payload); err != nil {
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

// adfNodeText returns the readable text an ADF leaf node contributes, if any.
func adfNodeText(node map[string]any) (string, bool) {
	attrString := func(names ...string) (string, bool) {
		attrs, ok := node["attrs"].(map[string]any)
		if !ok {
			return "", false
		}
		for _, name := range names {
			if value, ok := attrs[name].(string); ok && value != "" {
				return value, true
			}
		}
		return "", false
	}
	switch node["type"] {
	case "text":
		value, ok := node["text"].(string)
		return value, ok
	case "hardBreak":
		return "\n", true
	case "mention":
		return attrString("text", "displayName")
	case "emoji":
		return attrString("shortName", "text")
	case "inlineCard", "blockCard", "embedCard":
		return attrString("url")
	}
	return "", false
}

// adfBudget accumulates converted Markdown while enforcing the description
// byte ceiling, clipping on rune boundaries so output stays valid UTF-8.
type adfBudget struct {
	used      int
	truncated bool
}

func (b *adfBudget) take(text string) string {
	if text == "" {
		return ""
	}
	remaining := maxDescriptionBytes - b.used
	if remaining <= 0 {
		b.truncated = true
		return ""
	}
	part, clipped := boundedString(text, remaining)
	b.used += len(part)
	if clipped {
		b.truncated = true
	}
	return part
}

// adfInline converts a run of inline nodes, applying the marks Jira records
// alongside each text node.
func adfInline(nodes []any, depth int, budget *adfBudget) string {
	var out strings.Builder
	for _, raw := range nodes {
		node, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if depth > maxADFDepth {
			budget.truncated = true
			continue
		}
		if content, ok := node["content"].([]any); ok && node["type"] != "text" {
			out.WriteString(adfInline(content, depth+1, budget))
			continue
		}
		text, ok := adfNodeText(node)
		if !ok {
			continue
		}
		out.WriteString(adfApplyMarks(budget.take(text), node))
	}
	return out.String()
}

func adfApplyMarks(text string, node map[string]any) string {
	marks, ok := node["marks"].([]any)
	if !ok || text == "" || strings.TrimSpace(text) == "" {
		return text
	}
	for _, raw := range marks {
		mark, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch mark["type"] {
		case "strong":
			text = "**" + text + "**"
		case "em":
			text = "_" + text + "_"
		case "code":
			text = "`" + text + "`"
		case "strike":
			text = "~~" + text + "~~"
		case "link":
			if attrs, ok := mark["attrs"].(map[string]any); ok {
				if href, ok := attrs["href"].(string); ok && href != "" {
					text = "[" + text + "](" + href + ")"
				}
			}
		}
	}
	return text
}

// adfBlocks converts block-level nodes into Markdown blocks. Keeping blocks
// separate is what preserves headings and lists: the previous flat walk ran
// every text node together, gluing a heading onto the paragraph before it.
func adfBlocks(nodes []any, depth int, budget *adfBudget) []string {
	var blocks []string
	for _, raw := range nodes {
		node, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		blocks = append(blocks, adfBlock(node, depth, budget)...)
	}
	return blocks
}

func adfBlock(node map[string]any, depth int, budget *adfBudget) []string {
	if depth > maxADFDepth {
		budget.truncated = true
		return nil
	}
	content, _ := node["content"].([]any)
	switch node["type"] {
	case "doc":
		return adfBlocks(content, depth+1, budget)
	case "paragraph":
		if text := strings.TrimRight(adfInline(content, depth+1, budget), " \t"); text != "" {
			return []string{text}
		}
	case "heading":
		level := 3
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if raw, ok := attrs["level"].(float64); ok && raw >= 1 && raw <= 6 {
				level = int(raw)
			}
		}
		if text := adfInline(content, depth+1, budget); text != "" {
			return []string{strings.Repeat("#", level) + " " + text}
		}
	case "bulletList", "orderedList":
		ordered := node["type"] == "orderedList"
		var lines []string
		for index, raw := range content {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			itemContent, _ := item["content"].([]any)
			marker := "- "
			if ordered {
				marker = strconv.Itoa(index+1) + ". "
			}
			inner := adfBlocks(itemContent, depth+1, budget)
			for innerIndex, line := range inner {
				if innerIndex == 0 {
					lines = append(lines, marker+line)
					continue
				}
				lines = append(lines, adfIndent(line, strings.Repeat(" ", len(marker))))
			}
		}
		if len(lines) > 0 {
			return []string{strings.Join(lines, "\n")}
		}
	case "codeBlock":
		language := ""
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if value, ok := attrs["language"].(string); ok {
				language = value
			}
		}
		if text := adfInline(content, depth+1, budget); text != "" {
			return []string{"```" + language + "\n" + text + "\n```"}
		}
	case "blockquote":
		inner := adfBlocks(content, depth+1, budget)
		if len(inner) > 0 {
			return []string{adfIndent(strings.Join(inner, "\n\n"), "> ")}
		}
	case "rule":
		return []string{"---"}
	case "table":
		if rows := adfTable(content, depth+1, budget); rows != "" {
			return []string{rows}
		}
	case "mediaSingle", "mediaGroup", "media":
		return nil
	default:
		if len(content) > 0 {
			return adfBlocks(content, depth+1, budget)
		}
		if text := strings.TrimSpace(adfInline([]any{node}, depth+1, budget)); text != "" {
			return []string{text}
		}
	}
	return nil
}

func adfIndent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func adfTable(rows []any, depth int, budget *adfBudget) string {
	var lines []string
	for rowIndex, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok || row["type"] != "tableRow" {
			continue
		}
		cells, _ := row["content"].([]any)
		var values []string
		for _, rawCell := range cells {
			cell, ok := rawCell.(map[string]any)
			if !ok {
				continue
			}
			cellContent, _ := cell["content"].([]any)
			text := strings.Join(adfBlocks(cellContent, depth+1, budget), " ")
			values = append(values, strings.ReplaceAll(strings.TrimSpace(text), "|", "\\|"))
		}
		if len(values) == 0 {
			continue
		}
		lines = append(lines, "| "+strings.Join(values, " | ")+" |")
		if rowIndex == 0 {
			separator := make([]string, len(values))
			for i := range separator {
				separator[i] = "---"
			}
			lines = append(lines, "| "+strings.Join(separator, " | ")+" |")
		}
	}
	return strings.Join(lines, "\n")
}

// normalizeDescription converts a Jira description into Markdown. Cloud sends
// ADF; Server sends plain text, which is passed through unchanged.
func normalizeDescription(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return boundedString(text, maxDescriptionBytes)
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	budget := &adfBudget{}
	var blocks []string
	switch root := value.(type) {
	case map[string]any:
		blocks = adfBlock(root, 0, budget)
	case []any:
		blocks = adfBlocks(root, 0, budget)
	}
	return strings.TrimSpace(strings.Join(blocks, "\n\n")), budget.truncated
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

func attachmentRedirectPolicy(connection models.JiraConnection) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return nil
		}
		if len(via) >= 5 {
			return errors.New("too many Jira redirects")
		}
		if strings.EqualFold(request.URL.Scheme, via[0].URL.Scheme) && strings.EqualFold(request.URL.Host, via[0].URL.Host) {
			return nil
		}
		trustedCloudMedia := strings.EqualFold(connection.DeploymentType, "cloud") &&
			strings.EqualFold(request.URL.Scheme, "https") &&
			strings.EqualFold(request.URL.Hostname(), "api.media.atlassian.com") &&
			request.URL.Port() == ""
		if !trustedCloudMedia {
			return errors.New("Jira redirect changed origin")
		}
		request.Header.Del("Authorization")
		request.Header.Del("Cookie")
		request.Header.Del("Proxy-Authorization")
		return nil
	}
}

func decodeBounded(response *http.Response, target any) error {
	defer response.Body.Close()
	return json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(target)
}

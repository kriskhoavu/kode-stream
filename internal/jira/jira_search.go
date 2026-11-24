package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"kode-stream/internal/common/models"
)

const MaxSearchResults = 200

type SearchedIssue struct {
	Key                     string            `json:"key"`
	Summary                 string            `json:"summary"`
	Description             string            `json:"description"`
	Status                  string            `json:"status"`
	StatusCategory          string            `json:"statusCategory,omitempty"`
	IssueType               string            `json:"issueType"`
	Priority                string            `json:"priority,omitempty"`
	Assignee                string            `json:"assignee,omitempty"`
	Labels                  []string          `json:"labels"`
	UpdatedAt               string            `json:"updatedAt,omitempty"`
	BrowserURL              string            `json:"browserUrl"`
	BlockedBy               []string          `json:"blockedBy"`
	FixVersions             []string          `json:"fixVersions"`
	Epic                    string            `json:"epic,omitempty"`
	OriginalEstimateSeconds int64             `json:"originalEstimateSeconds,omitempty"`
	Reporter                *Person           `json:"reporter,omitempty"`
	Comments                []SearchedComment `json:"comments"`
}

type SearchedComment struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	CreatedAt string `json:"createdAt"`
	Body      string `json:"body"`
}

const (
	MaxSearchedComments = 10
	maxCommentBytes     = 4 << 10
)

type jiraSearchResponse struct {
	Issues []jiraSearchIssue `json:"issues"`
}

type jiraSearchComment struct {
	ID      string          `json:"id"`
	Author  *jiraPerson     `json:"author"`
	Body    json.RawMessage `json:"body"`
	Created string          `json:"created"`
}

type jiraSearchIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		Status      struct {
			Name           string `json:"name"`
			StatusCategory struct {
				Key string `json:"key"`
			} `json:"statusCategory"`
		} `json:"status"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Assignee *jiraPerson `json:"assignee"`
		Reporter *jiraPerson `json:"reporter"`
		Comment  *struct {
			Comments []jiraSearchComment `json:"comments"`
		} `json:"comment"`
		Priority *struct {
			Name string `json:"name"`
		} `json:"priority"`
		Labels      []string `json:"labels"`
		Updated     string   `json:"updated"`
		FixVersions []struct {
			Name string `json:"name"`
		} `json:"fixVersions"`
		Parent *struct {
			Key string `json:"key"`
		} `json:"parent"`
		TimeOriginalEstimate *int64 `json:"timeoriginalestimate"`
		IssueLinks           []struct {
			Type struct {
				Inward string `json:"inward"`
			} `json:"type"`
			InwardIssue *struct {
				Key string `json:"key"`
			} `json:"inwardIssue"`
		} `json:"issuelinks"`
	} `json:"fields"`
}

const searchFields = "summary,status,description,issuetype,assignee,reporter,comment,priority,labels,updated,fixVersions,parent,timeoriginalestimate,issuelinks"

func (c *Client) SearchIssues(ctx context.Context, connection models.JiraConnection, jql string, max int) ([]SearchedIssue, error) {
	if strings.TrimSpace(jql) == "" {
		return nil, fmt.Errorf("a JQL query is required")
	}
	if max <= 0 || max > MaxSearchResults {
		max = MaxSearchResults
	}
	query := url.Values{}
	query.Set("jql", jql)
	query.Set("maxResults", strconv.Itoa(max))
	query.Set("fields", searchFields)
	endpoint := connection.BaseURL + "/rest/api/2/search?" + query.Encode()
	if connection.DeploymentType == "cloud" {
		endpoint = connection.BaseURL + "/rest/api/3/search/jql?" + query.Encode()
	}
	var payload jiraSearchResponse
	if err := c.getJSON(ctx, connection, endpoint, fmt.Errorf("Jira search was not found"), "Jira search", &payload); err != nil {
		return nil, err
	}
	issues := make([]SearchedIssue, 0, len(payload.Issues))
	for i, raw := range payload.Issues {
		if i >= max {
			break
		}
		if issue, ok := searchedIssue(raw, connection.BaseURL); ok {
			issues = append(issues, issue)
		}
	}
	return issues, nil
}

func searchedIssue(raw jiraSearchIssue, baseURL string) (SearchedIssue, bool) {
	var truncated bool
	key, _ := boundedString(raw.Key, maxScalarBytes)
	if key == "" {
		return SearchedIssue{}, false
	}
	fields := raw.Fields
	summary, _ := boundedString(fields.Summary, maxScalarBytes)
	description, _ := normalizeDescription(fields.Description)
	status, _ := boundedString(fields.Status.Name, maxScalarBytes)
	category, _ := boundedString(fields.Status.StatusCategory.Key, maxScalarBytes)
	issueType, _ := boundedString(fields.IssueType.Name, maxScalarBytes)
	updated, _ := boundedString(fields.Updated, maxScalarBytes)
	issue := SearchedIssue{Key: key, Summary: summary, Description: description, Status: status, StatusCategory: category, IssueType: issueType, Labels: boundedStrings(fields.Labels, maxLabels, maxScalarBytes, &truncated), UpdatedAt: updated, BrowserURL: baseURL + "/browse/" + url.PathEscape(key), BlockedBy: []string{}, FixVersions: []string{}, Comments: []SearchedComment{}}
	if person, _ := normalizePerson(fields.Reporter); person != nil {
		issue.Reporter = person
	}
	if fields.Comment != nil {
		issue.Comments = searchedComments(fields.Comment.Comments)
	}
	if fields.Priority != nil {
		issue.Priority, _ = boundedString(fields.Priority.Name, maxScalarBytes)
	}
	if person, _ := normalizePerson(fields.Assignee); person != nil {
		issue.Assignee = person.DisplayName
	}
	for _, version := range fields.FixVersions {
		if name, _ := boundedString(version.Name, maxScalarBytes); name != "" && len(issue.FixVersions) < maxLabels {
			issue.FixVersions = append(issue.FixVersions, name)
		}
	}
	if fields.Parent != nil {
		issue.Epic, _ = boundedString(fields.Parent.Key, maxScalarBytes)
	}
	if fields.TimeOriginalEstimate != nil && *fields.TimeOriginalEstimate > 0 {
		issue.OriginalEstimateSeconds = *fields.TimeOriginalEstimate
	}
	for _, link := range fields.IssueLinks {
		if link.InwardIssue == nil || !strings.Contains(strings.ToLower(link.Type.Inward), "blocked by") {
			continue
		}
		if blocker, _ := boundedString(link.InwardIssue.Key, maxScalarBytes); blocker != "" && len(issue.BlockedBy) < maxLabels {
			issue.BlockedBy = append(issue.BlockedBy, blocker)
		}
	}
	return issue, true
}

func searchedComments(comments []jiraSearchComment) []SearchedComment {
	if len(comments) > MaxSearchedComments {
		comments = comments[len(comments)-MaxSearchedComments:]
	}
	out := make([]SearchedComment, 0, len(comments))
	for _, comment := range comments {
		body, _ := normalizeDescription(comment.Body)
		body, _ = boundedString(body, maxCommentBytes)
		id, _ := boundedString(comment.ID, maxScalarBytes)
		created, _ := boundedString(comment.Created, maxScalarBytes)
		author := ""
		if person, _ := normalizePerson(comment.Author); person != nil {
			author = person.DisplayName
		}
		out = append(out, SearchedComment{ID: id, Author: author, CreatedAt: created, Body: body})
	}
	return out
}

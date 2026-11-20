package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"kode-stream/internal/common/models"
)

// MaxSearchResults bounds one search.
const MaxSearchResults = 200

// SearchedIssue is one search hit with the fields a queue ranks and orders
// on. Every string is untrusted and bounded.
type SearchedIssue struct {
	Key                     string   `json:"key"`
	Summary                 string   `json:"summary"`
	Description             string   `json:"description"`
	Status                  string   `json:"status"`
	StatusCategory          string   `json:"statusCategory,omitempty"`
	IssueType               string   `json:"issueType"`
	Priority                string   `json:"priority,omitempty"`
	Assignee                string   `json:"assignee,omitempty"`
	Labels                  []string `json:"labels"`
	UpdatedAt               string   `json:"updatedAt,omitempty"`
	BrowserURL              string   `json:"browserUrl"`
	BlockedBy               []string `json:"blockedBy"`
	FixVersions             []string `json:"fixVersions"`
	Epic                    string   `json:"epic,omitempty"`
	OriginalEstimateSeconds int64    `json:"originalEstimateSeconds,omitempty"`
	// Reporter is who opened the issue; nil when Jira names nobody.
	Reporter *Person `json:"reporter,omitempty"`
	// Comments are the issue's latest comments, oldest first, at most
	// MaxSearchedComments, each bounded.
	Comments []SearchedComment `json:"comments"`
}

// SearchedComment is one comment on a searched issue. Its body is untrusted.
type SearchedComment struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	CreatedAt string `json:"createdAt"`
	Body      string `json:"body"`
}

// MaxSearchedComments and maxCommentBytes bound what a search keeps of an
// issue's comments.
const (
	MaxSearchedComments = 10
	maxCommentBytes     = 4 << 10
)

type jiraSearchResponse struct {
	Issues []struct {
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
				Comments []struct {
					ID      string          `json:"id"`
					Author  *jiraPerson     `json:"author"`
					Body    json.RawMessage `json:"body"`
					Created string          `json:"created"`
				} `json:"comments"`
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
	} `json:"issues"`
}

const searchFields = "summary,status,description,issuetype,assignee,reporter,comment,priority,labels,updated,fixVersions,parent,timeoriginalestimate,issuelinks"

// SearchIssues runs a JQL search: Jira Server and Data Center through API 2
// search, Jira Cloud through API 3 search/jql. It reads only.
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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if err := c.authorize(request, connection); err != nil {
		return nil, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Jira is unavailable: %w", err)
	}
	if err := responseError(response, fmt.Errorf("Jira search was not found")); err != nil {
		return nil, err
	}
	var payload jiraSearchResponse
	if err := decodeBounded(response, &payload); err != nil {
		return nil, fmt.Errorf("decode Jira search: %w", err)
	}
	issues := make([]SearchedIssue, 0, len(payload.Issues))
	for i, raw := range payload.Issues {
		if i >= max {
			break
		}
		var truncated bool
		key, _ := boundedString(raw.Key, maxScalarBytes)
		if key == "" {
			continue
		}
		summary, _ := boundedString(raw.Fields.Summary, maxScalarBytes)
		description, _ := normalizeDescription(raw.Fields.Description)
		status, _ := boundedString(raw.Fields.Status.Name, maxScalarBytes)
		category, _ := boundedString(raw.Fields.Status.StatusCategory.Key, maxScalarBytes)
		issueType, _ := boundedString(raw.Fields.IssueType.Name, maxScalarBytes)
		updated, _ := boundedString(raw.Fields.Updated, maxScalarBytes)
		issue := SearchedIssue{Key: key, Summary: summary, Description: description, Status: status, StatusCategory: category, IssueType: issueType, Labels: boundedStrings(raw.Fields.Labels, maxLabels, maxScalarBytes, &truncated), UpdatedAt: updated, BrowserURL: connection.BaseURL + "/browse/" + url.PathEscape(key), BlockedBy: []string{}, FixVersions: []string{}, Comments: []SearchedComment{}}
		if person, _ := normalizePerson(raw.Fields.Reporter); person != nil {
			issue.Reporter = person
		}
		if raw.Fields.Comment != nil {
			comments := raw.Fields.Comment.Comments
			if len(comments) > MaxSearchedComments {
				comments = comments[len(comments)-MaxSearchedComments:]
			}
			for _, comment := range comments {
				body, _ := normalizeDescription(comment.Body)
				body, _ = boundedString(body, maxCommentBytes)
				id, _ := boundedString(comment.ID, maxScalarBytes)
				created, _ := boundedString(comment.Created, maxScalarBytes)
				author := ""
				if person, _ := normalizePerson(comment.Author); person != nil {
					author = person.DisplayName
				}
				issue.Comments = append(issue.Comments, SearchedComment{ID: id, Author: author, CreatedAt: created, Body: body})
			}
		}
		if raw.Fields.Priority != nil {
			issue.Priority, _ = boundedString(raw.Fields.Priority.Name, maxScalarBytes)
		}
		if person, _ := normalizePerson(raw.Fields.Assignee); person != nil {
			issue.Assignee = person.DisplayName
		}
		for _, version := range raw.Fields.FixVersions {
			if name, _ := boundedString(version.Name, maxScalarBytes); name != "" && len(issue.FixVersions) < maxLabels {
				issue.FixVersions = append(issue.FixVersions, name)
			}
		}
		if raw.Fields.Parent != nil {
			issue.Epic, _ = boundedString(raw.Fields.Parent.Key, maxScalarBytes)
		}
		if raw.Fields.TimeOriginalEstimate != nil && *raw.Fields.TimeOriginalEstimate > 0 {
			issue.OriginalEstimateSeconds = *raw.Fields.TimeOriginalEstimate
		}
		for _, link := range raw.Fields.IssueLinks {
			if link.InwardIssue == nil || !strings.Contains(strings.ToLower(link.Type.Inward), "blocked by") {
				continue
			}
			if blocker, _ := boundedString(link.InwardIssue.Key, maxScalarBytes); blocker != "" && len(issue.BlockedBy) < maxLabels {
				issue.BlockedBy = append(issue.BlockedBy, blocker)
			}
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

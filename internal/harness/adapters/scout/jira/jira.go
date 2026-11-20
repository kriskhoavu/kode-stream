// Package jira lists a workspace's Jira issues by the JQL its harness settings
// name, through the workspace's own Jira connection and token.
package jira

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
	appjira "kode-stream/internal/jira"
)

type Searcher interface {
	SearchIssues(ctx context.Context, connection models.JiraConnection, jql string, max int) ([]appjira.SearchedIssue, error)
}

type Scout struct{ client Searcher }

func New(client Searcher) *Scout { return &Scout{client: client} }

// jiraTime is how Jira writes a timestamp.
const jiraTime = "2006-01-02T15:04:05.000-0700"

// mention is Jira's wiki markup that mentions a person in a comment: an
// account id on Jira Cloud, a user name on Server and Data Center.
func mention(connection models.JiraConnection, id string) string {
	id = strings.TrimSpace(id)
	if id == "" || strings.ContainsAny(id, "[]~|") {
		return ""
	}
	if connection.DeploymentType == "cloud" {
		return "[~accountid:" + id + "]"
	}
	return "[~" + id + "]"
}

func (s *Scout) Kind() domain.SourceKind { return domain.SourceJira }

func (s *Scout) Enabled(workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) bool {
	return settings.Jira != nil && settings.Jira.Enabled && workspace.Jira != nil
}

func (s *Scout) Poll(ctx context.Context, workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) ([]domain.Candidate, error) {
	if !s.Enabled(workspace, settings) {
		return nil, fmt.Errorf("workspace %s has no Jira connection", workspace.ID)
	}
	issues, err := s.client.SearchIssues(ctx, *workspace.Jira, settings.Jira.JQL, settings.Jira.Max)
	if err != nil {
		return nil, err
	}
	candidates := make([]domain.Candidate, 0, len(issues))
	for _, issue := range issues {
		if issue.StatusCategory == "done" {
			continue
		}
		source := domain.Source{Kind: domain.SourceJira, Key: issue.Key, URL: issue.BrowserURL, Status: issue.Status, Labels: issue.Labels, Priority: issue.Priority, Assignee: issue.Assignee, Epic: issue.Epic}
		if len(issue.FixVersions) > 0 {
			source.Milestone = issue.FixVersions[0]
		}
		if updated, err := time.Parse(jiraTime, issue.UpdatedAt); err == nil {
			updated = updated.UTC()
			source.UpdatedAt = &updated
		}
		if issue.Reporter != nil && issue.Reporter.DisplayName != "" {
			source.Reporter = &domain.Person{Name: issue.Reporter.DisplayName, Mention: mention(*workspace.Jira, issue.Reporter.AccountID)}
		}
		comments := make([]domain.Comment, 0, len(issue.Comments))
		for _, comment := range issue.Comments {
			created, _ := time.Parse(jiraTime, comment.CreatedAt)
			comments = append(comments, domain.Comment{ID: comment.ID, Author: comment.Author, CreatedAt: created.UTC(), Body: comment.Body})
		}
		candidates = append(candidates, domain.Candidate{WorkspaceID: workspace.ID, Source: source, Title: issue.Summary, Body: issue.Description, DependsOnKeys: issue.BlockedBy, EstimatedMinutes: int(issue.OriginalEstimateSeconds / 60), Comments: comments})
	}
	return candidates, nil
}

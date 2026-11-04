// Package jira lists a workspace's Jira issues by the JQL its harness settings
// name, through the workspace's own Jira connection and token.
package jira

import (
	"context"
	"fmt"
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
		if updated, err := time.Parse("2006-01-02T15:04:05.000-0700", issue.UpdatedAt); err == nil {
			updated = updated.UTC()
			source.UpdatedAt = &updated
		}
		candidates = append(candidates, domain.Candidate{WorkspaceID: workspace.ID, Source: source, Title: issue.Summary, Body: issue.Description, DependsOnKeys: issue.BlockedBy, EstimatedMinutes: int(issue.OriginalEstimateSeconds / 60)})
	}
	return candidates, nil
}

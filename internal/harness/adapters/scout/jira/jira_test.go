package jira

import (
	"context"
	"testing"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
	appjira "kode-stream/internal/jira"
)

type fakeSearcher struct{ jql string }

func (f *fakeSearcher) SearchIssues(_ context.Context, _ models.JiraConnection, jql string, _ int) ([]appjira.SearchedIssue, error) {
	f.jql = jql
	return []appjira.SearchedIssue{
		{Key: "PM-2", Summary: "Open", StatusCategory: "new", Priority: "High", Labels: []string{"bug"}, UpdatedAt: "2026-11-01T10:00:00.000+0100", BlockedBy: []string{"PM-1"}, FixVersions: []string{"M1"}, Epic: "PM-0", OriginalEstimateSeconds: 7200, BrowserURL: "https://jira/browse/PM-2"},
		{Key: "PM-3", Summary: "Closed", StatusCategory: "done"},
	}, nil
}

func TestJiraScoutMapsOpenIssuesToCandidates(t *testing.T) {
	searcher := &fakeSearcher{}
	scout := New(searcher)
	workspace := models.WorkspaceConfig{ID: "ws", Jira: &models.JiraConnection{BaseURL: "https://jira"}}
	settings := domain.WorkspaceSettings{WorkspaceID: "ws", Jira: &domain.JiraSourceSettings{Enabled: true, JQL: "project = PM", Max: 10}}
	if !scout.Enabled(workspace, settings) || scout.Enabled(models.WorkspaceConfig{ID: "ws"}, settings) {
		t.Fatal("enabled check is wrong")
	}
	candidates, err := scout.Poll(context.Background(), workspace, settings)
	if err != nil || len(candidates) != 1 || searcher.jql != "project = PM" {
		t.Fatalf("candidates = %#v err=%v", candidates, err)
	}
	candidate := candidates[0]
	if candidate.Source.Key != "PM-2" || candidate.Source.Milestone != "M1" || candidate.Source.Epic != "PM-0" || candidate.EstimatedMinutes != 120 || candidate.DependsOnKeys[0] != "PM-1" || candidate.Source.UpdatedAt == nil || candidate.Source.UpdatedAt.Hour() != 9 {
		t.Fatalf("candidate = %#v", candidate)
	}
}

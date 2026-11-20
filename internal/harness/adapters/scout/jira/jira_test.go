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
		{Key: "PM-2", Summary: "Open", StatusCategory: "new", Priority: "High", Labels: []string{"bug"}, UpdatedAt: "2026-11-01T10:00:00.000+0100", BlockedBy: []string{"PM-1"}, FixVersions: []string{"M1"}, Epic: "PM-0", OriginalEstimateSeconds: 7200, BrowserURL: "https://jira/browse/PM-2",
			Reporter: &appjira.Person{DisplayName: "Bao", AccountID: "5b10"}, Comments: []appjira.SearchedComment{{ID: "10", Author: "Bao", CreatedAt: "2026-11-02T09:00:00.000+0000", Body: "CSV."}}},
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

func TestJiraScoutCarriesTheReporterAndComments(t *testing.T) {
	settings := domain.WorkspaceSettings{WorkspaceID: "ws", Jira: &domain.JiraSourceSettings{Enabled: true, JQL: "project = PM"}}
	for deployment, want := range map[string]string{"cloud": "[~accountid:5b10]", "server": "[~5b10]"} {
		workspace := models.WorkspaceConfig{ID: "ws", Jira: &models.JiraConnection{BaseURL: "https://jira", DeploymentType: deployment}}
		candidates, err := New(&fakeSearcher{}).Poll(context.Background(), workspace, settings)
		if err != nil {
			t.Fatal(err)
		}
		candidate := candidates[0]
		if candidate.Source.Reporter == nil || candidate.Source.Reporter.Name != "Bao" || candidate.Source.Reporter.Mention != want {
			t.Fatalf("%s reporter = %#v", deployment, candidate.Source.Reporter)
		}
		if len(candidate.Comments) != 1 || candidate.Comments[0].Author != "Bao" || candidate.Comments[0].CreatedAt.Hour() != 9 || candidate.Comments[0].Body != "CSV." {
			t.Fatalf("comments = %#v", candidate.Comments)
		}
	}
	if mention(models.JiraConnection{}, "a]b") != "" {
		t.Fatal("an id that would break the markup was mentioned")
	}
}

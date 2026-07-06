package workspacefiles

// Package workspacefiles provides Workspace-owned file operations.

// Package workspacefiles provides bounded workspace file operations.

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"plan-manager/internal/common/models"
)

func TestContentSearchReturnsLiteralUnicodeLineMatches(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "guide.md"), "Header\nCafé CAFÉ café\nFooter")
	workspace := models.WorkspaceConfig{ID: "ws", Name: "Workspace", Path: root}
	response, err := NewWithIgnoreChecker(nil).ContentSearch(context.Background(), workspace, []models.WorkspaceContentSearchRoot{{Path: ""}}, models.WorkspaceContentSearchRequest{Query: "café"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 3 {
		t.Fatalf("results = %#v", response.Results)
	}
	first := response.Results[0]
	if first.LineNumber != 2 || first.ColumnStart != 1 || first.ColumnEnd != 5 || first.Kind != models.FileKindMarkdown {
		t.Fatalf("first = %#v", first)
	}
}

func TestContentSearchSupportsCaseSensitivityAndBoundedSnippets(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "notes.txt"), strings.Repeat("a", 40)+"Needle"+strings.Repeat("z", 40))
	budget := DefaultContentSearchBudget()
	budget.MaxSnippetLength = 20
	response, err := NewWithIgnoreChecker(nil).ContentSearch(context.Background(), models.WorkspaceConfig{ID: "ws", Path: root}, []models.WorkspaceContentSearchRoot{{}}, models.WorkspaceContentSearchRequest{Query: "Needle", CaseSensitive: true}, &budget)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || len([]rune(response.Results[0].Snippet)) > 20 || !strings.Contains(response.Results[0].Snippet, "Needle") {
		t.Fatalf("response = %#v", response)
	}
}

func TestContentSearchEnforcesBudgetsAndCancellation(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "match match")
	mustWrite(t, filepath.Join(root, "b.txt"), "match")
	budget := DefaultContentSearchBudget()
	budget.MaxResults = 1
	response, err := NewWithIgnoreChecker(nil).ContentSearch(context.Background(), models.WorkspaceConfig{ID: "ws", Path: root}, []models.WorkspaceContentSearchRoot{{}}, models.WorkspaceContentSearchRequest{Query: "match"}, &budget)
	if err != nil || !response.Truncated || len(response.Results) != 1 {
		t.Fatalf("response = %#v, err = %v", response, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = NewWithIgnoreChecker(nil).ContentSearch(ctx, models.WorkspaceConfig{ID: "ws", Path: root}, []models.WorkspaceContentSearchRoot{{}}, models.WorkspaceContentSearchRequest{Query: "match"}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateContentSearchQuery(t *testing.T) {
	if ValidateContentSearchQuery("x", 200) == nil {
		t.Fatal("expected short query error")
	}
	if ValidateContentSearchQuery(strings.Repeat("x", 201), 200) == nil {
		t.Fatal("expected long query error")
	}
}

package planindex

import (
	"path/filepath"
	"testing"
	"time"

	"plan-manager/internal/models"
)

func TestDeleteRepositoryRemovesPlansAndKeepsOthers(t *testing.T) {
	idx := New(filepath.Join(t.TempDir(), "plans.json"))
	if err := idx.ReplaceRepository("repo-a", []models.PlanDetail{
		{PlanSummary: models.PlanSummary{ID: "a-1", RepositoryID: "repo-a", Title: "A"}},
	}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := idx.ReplaceRepository("repo-b", []models.PlanDetail{
		{PlanSummary: models.PlanSummary{ID: "b-1", RepositoryID: "repo-b", Title: "B"}},
	}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}

	if err := idx.DeleteRepository("repo-a"); err != nil {
		t.Fatal(err)
	}

	plans, err := idx.Query(Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].ID != "b-1" {
		t.Fatalf("plans = %#v, want only repo-b plan", plans)
	}
	if _, ok, err := idx.Get("a-1"); err != nil || ok {
		t.Fatalf("repo-a plan still exists: ok=%v err=%v", ok, err)
	}
}

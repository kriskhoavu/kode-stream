package queue

import (
	"context"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/harness/domain"
)

func TestRuleRankerScoresPriorityLabelsAgeOverrideAndPin(t *testing.T) {
	now := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	settings := domain.DefaultSettings()
	settings.Ranker.LabelBoosts = map[string]float64{"Bug": 5}
	old := domain.Task{ID: "a", Source: domain.Source{Priority: "High", Labels: []string{"bug"}}, DiscoveredAt: now.Add(-20 * 24 * time.Hour)}
	score, reasons := RuleRanker{}.Score(old, settings, now)
	if score != 30+5+5 || len(reasons) != 3 || !strings.Contains(strings.Join(reasons, ";"), "label bug +5") {
		t.Fatalf("score=%v reasons=%v", score, reasons)
	}
	pinned := domain.Task{ID: "b", Source: domain.Source{}, Pinned: true, PriorityOverride: -3, DiscoveredAt: now}
	score, reasons = RuleRanker{}.Score(pinned, settings, now)
	if score != 15-3+1000 || reasons[len(reasons)-1] != "pinned" {
		t.Fatalf("score=%v reasons=%v", score, reasons)
	}
	ranked, err := RuleRanker{}.Rank(context.Background(), []domain.Task{old, pinned}, settings, now)
	if err != nil || ranked[0].Task.ID != "b" {
		t.Fatalf("ranked = %#v err=%v", ranked, err)
	}
}

func TestApplyOrderReordersOnlyUnpinnedTopTasks(t *testing.T) {
	ranked := []Scored{
		{Task: domain.Task{ID: "pin", Pinned: true}, Score: 1000},
		{Task: domain.Task{ID: "a"}, Score: 40},
		{Task: domain.Task{ID: "b"}, Score: 30},
		{Task: domain.Task{ID: "c"}, Score: 20},
	}
	out, ok := ApplyOrder(ranked, 3, []string{"b", "a"})
	if !ok || out[0].Task.ID != "pin" || out[1].Task.ID != "b" || out[1].Score != 40 || out[2].Task.ID != "a" || out[2].Score != 30 || out[3].Task.ID != "c" {
		t.Fatalf("out = %#v", out)
	}
	if out[1].Reasons[len(out[1].Reasons)-1] != "reordered by the model" {
		t.Fatalf("reasons = %v", out[1].Reasons)
	}
	for _, bad := range [][]string{{"a"}, {"a", "a"}, {"a", "c"}, {"pin", "a"}} {
		if _, ok := ApplyOrder(ranked, 3, bad); ok {
			t.Fatalf("order %v was accepted", bad)
		}
	}
}

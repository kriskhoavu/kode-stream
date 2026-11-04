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

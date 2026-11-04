// Package queue ranks harness tasks. RuleRanker scores every task by rules a
// person can read; a model re-ranker may only reorder the rules' top N.
package queue

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"kode-stream/internal/harness/domain"
)

// Scored is a task with its new score and the reasons behind it.
type Scored struct {
	Task    domain.Task
	Score   float64
	Reasons []string
}

// Ranker orders a workspace's ranked tasks, best first.
type Ranker interface {
	Rank(ctx context.Context, tasks []domain.Task, settings domain.Settings, now time.Time) ([]Scored, error)
}

type RuleRanker struct{}

const (
	pinnedBonus = 1000
	agePerDay   = 0.5
	ageCap      = 5
	noPriority  = 15
)

var priorityBase = map[string]float64{
	"highest": 40, "blocker": 40, "p0": 40,
	"high": 30, "critical": 30, "major": 30, "p1": 30,
	"medium": 20, "normal": 20, "p2": 20,
	"low": 10, "minor": 10, "p3": 10,
	"lowest": 5, "trivial": 5, "p4": 5,
}

// Score computes one task's score and the reasons, one per non-zero term.
func (RuleRanker) Score(task domain.Task, settings domain.Settings, now time.Time) (float64, []string) {
	reasons := []string{}
	base, ok := priorityBase[strings.ToLower(strings.TrimSpace(task.Source.Priority))]
	if !ok {
		base = noPriority
		reasons = append(reasons, fmt.Sprintf("no priority +%g", base))
	} else {
		reasons = append(reasons, fmt.Sprintf("priority %s +%g", task.Source.Priority, base))
	}
	score := base
	for _, label := range task.Source.Labels {
		if boost, ok := lookupBoost(settings.Ranker.LabelBoosts, label); ok && boost != 0 {
			score += boost
			reasons = append(reasons, fmt.Sprintf("label %s %+g", label, boost))
		}
	}
	if !task.DiscoveredAt.IsZero() {
		days := now.Sub(task.DiscoveredAt).Hours() / 24
		if age := math.Min(math.Max(days, 0)*agePerDay, ageCap); age >= 0.1 {
			age = math.Round(age*10) / 10
			score += age
			reasons = append(reasons, fmt.Sprintf("waiting %.0f days +%g", math.Floor(days), age))
		}
	}
	if task.PriorityOverride != 0 {
		score += task.PriorityOverride
		reasons = append(reasons, fmt.Sprintf("set by hand %+g", task.PriorityOverride))
	}
	if task.Pinned {
		score += pinnedBonus
		reasons = append(reasons, "pinned")
	}
	return score, reasons
}

func lookupBoost(boosts map[string]float64, label string) (float64, bool) {
	if boost, ok := boosts[label]; ok {
		return boost, true
	}
	for key, boost := range boosts {
		if strings.EqualFold(key, label) {
			return boost, true
		}
	}
	return 0, false
}

func (r RuleRanker) Rank(_ context.Context, tasks []domain.Task, settings domain.Settings, now time.Time) ([]Scored, error) {
	scored := make([]Scored, len(tasks))
	for i, task := range tasks {
		score, reasons := r.Score(task, settings, now)
		scored[i] = Scored{Task: task, Score: score, Reasons: reasons}
	}
	SortScored(scored)
	return scored, nil
}

// SortScored orders best first; ties go to the task found first.
func SortScored(scored []Scored) {
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		if !scored[i].Task.DiscoveredAt.Equal(scored[j].Task.DiscoveredAt) {
			return scored[i].Task.DiscoveredAt.Before(scored[j].Task.DiscoveredAt)
		}
		return scored[i].Task.ID < scored[j].Task.ID
	})
}

// Reranker may reorder the rules' top N; it never adds, drops or moves a
// task in or out of the top N, and never moves a pinned task.
type Reranker interface {
	Rerank(ctx context.Context, ranked []Scored, settings domain.Settings) ([]Scored, error)
}

// ApplyOrder reorders the unpinned tasks among ranked's first topN by order
// (task ids, best first) and hands them the same scores in the new order, so
// the stored ranking follows. An order that is not a permutation of those
// ids is refused and ranked comes back unchanged.
func ApplyOrder(ranked []Scored, topN int, order []string) ([]Scored, bool) {
	if topN > len(ranked) {
		topN = len(ranked)
	}
	var slots []int
	byID := map[string]Scored{}
	for i := 0; i < topN; i++ {
		if ranked[i].Task.Pinned {
			continue
		}
		slots = append(slots, i)
		byID[ranked[i].Task.ID] = ranked[i]
	}
	if len(order) != len(slots) {
		return ranked, false
	}
	seen := map[string]bool{}
	for _, id := range order {
		if _, ok := byID[id]; !ok || seen[id] {
			return ranked, false
		}
		seen[id] = true
	}
	out := append([]Scored{}, ranked...)
	for position, slot := range slots {
		entry := byID[order[position]]
		if entry.Task.ID != ranked[slot].Task.ID {
			entry.Reasons = append(append([]string{}, entry.Reasons...), "reordered by the model")
		}
		entry.Score = ranked[slot].Score
		out[slot] = entry
	}
	return out, true
}

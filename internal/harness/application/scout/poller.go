// Package scout polls every configured issue source and keeps the task table
// in step with it: new issues become discovered tasks, known ones are
// refreshed, and an issue a source stops listing goes stale after three polls.
package scout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

// StaleAfterMissedPolls is how many successful polls may miss a task before
// it is marked stale.
const StaleAfterMissedPolls = 3

type Emit func(context.Context, domain.Event) domain.Event

type Health struct {
	WorkspaceID  string
	Kind         domain.SourceKind
	LastPolledAt *time.Time
	Found        int
	Added        int
	LastError    string
}

type Poller struct {
	repository domain.Repository
	workspaces ports.Workspaces
	scouts     []ports.Scout
	now        func() time.Time
	emit       Emit

	mu     sync.Mutex
	health map[string]Health
}

func NewPoller(repository domain.Repository, workspaces ports.Workspaces, scouts []ports.Scout, now func() time.Time, emit Emit) *Poller {
	return &Poller{repository: repository, workspaces: workspaces, scouts: scouts, now: now, emit: emit, health: map[string]Health{}}
}

// Poll asks every enabled source of every configured workspace, or of one
// workspace when workspaceID is set. It returns the sources it asked.
func (p *Poller) Poll(ctx context.Context, settings domain.Settings, workspaceID string) ([]Health, error) {
	var polled []Health
	var errs []error
	for _, workspaceSettings := range settings.Workspaces {
		if workspaceID != "" && workspaceSettings.WorkspaceID != workspaceID {
			continue
		}
		workspace, found, err := p.workspaces.Get(workspaceSettings.WorkspaceID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !found {
			continue
		}
		for _, scout := range p.scouts {
			if !scout.Enabled(workspace, workspaceSettings) {
				continue
			}
			health := p.pollOne(ctx, scout, workspace.ID, func(ctx context.Context) ([]domain.Candidate, error) {
				return scout.Poll(ctx, workspace, workspaceSettings)
			})
			polled = append(polled, health)
		}
	}
	return polled, errors.Join(errs...)
}

func (p *Poller) pollOne(ctx context.Context, scout ports.Scout, workspaceID string, poll func(context.Context) ([]domain.Candidate, error)) Health {
	now := p.now()
	health := Health{WorkspaceID: workspaceID, Kind: scout.Kind(), LastPolledAt: &now}
	candidates, err := poll(ctx)
	if err == nil {
		health.Found = len(candidates)
		health.Added, err = p.apply(ctx, workspaceID, scout.Kind(), candidates)
	}
	if err != nil {
		health.LastError = err.Error()
	}
	p.mu.Lock()
	p.health[workspaceID+"\x00"+string(scout.Kind())] = health
	p.mu.Unlock()
	return health
}

// apply upserts the candidates of one successful poll and ages the tasks it
// no longer lists. It returns how many tasks it added.
func (p *Poller) apply(ctx context.Context, workspaceID string, kind domain.SourceKind, candidates []domain.Candidate) (int, error) {
	seen := map[string]bool{}
	added := 0
	for _, candidate := range candidates {
		if candidate.Source.Kind != kind || strings.TrimSpace(candidate.Source.Key) == "" {
			continue
		}
		seen[candidate.Source.Key] = true
		created, err := p.upsert(ctx, workspaceID, candidate)
		if err != nil {
			return added, err
		}
		if created {
			added++
		}
	}
	open, err := p.repository.ListTasks(ctx, domain.TaskFilter{WorkspaceID: workspaceID, States: []domain.TaskState{domain.TaskDiscovered, domain.TaskRanked}, Limit: 1000})
	if err != nil {
		return added, err
	}
	for _, task := range open {
		if task.Source.Kind != kind || seen[task.Source.Key] {
			continue
		}
		expect := domain.Expect{State: string(task.State), Revision: task.Revision}
		task.MissedPolls++
		task.UpdatedAt = p.now()
		if task.MissedPolls >= StaleAfterMissedPolls {
			task.State = domain.TaskStale
		}
		saved, err := p.repository.SaveTask(ctx, task, expect)
		if errors.Is(err, domain.ErrStateMoved) {
			continue
		}
		if err != nil {
			return added, err
		}
		if saved.State == domain.TaskStale {
			p.emit(ctx, domain.Event{Kind: domain.EventTaskUpdated, TaskID: saved.ID, Payload: taskPayload(saved, "no longer listed by its source")})
		}
	}
	return added, nil
}

func (p *Poller) upsert(ctx context.Context, workspaceID string, candidate domain.Candidate) (bool, error) {
	now := p.now()
	existing, found, err := p.repository.FindTaskBySource(ctx, candidate.Source.Kind, candidate.Source.Key)
	if err != nil {
		return false, err
	}
	if !found {
		task := domain.Task{
			ID: domain.NewID("tsk"), WorkspaceID: workspaceID, Source: normalizedSource(candidate.Source), Title: bounded(candidate.Title, 1024),
			Body: bounded(candidate.Body, domain.MaxTaskBodyBytes), State: domain.TaskDiscovered, ScoreReasons: []string{}, DependsOn: []string{},
			DependsOnKeys: candidate.DependsOnKeys, EstimatedMinutes: candidate.EstimatedMinutes, DiscoveredAt: now, UpdatedAt: now,
		}
		if err := p.repository.InsertTask(ctx, task); errors.Is(err, domain.ErrDuplicate) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		p.emit(ctx, domain.Event{Kind: domain.EventTaskDiscovered, TaskID: task.ID, Payload: taskPayload(task, "")})
		return true, nil
	}
	// The first workspace to find an issue owns it; a task never reopens.
	if existing.WorkspaceID != workspaceID || domain.TaskTerminal(existing.State) {
		return false, nil
	}
	expect := domain.Expect{State: string(existing.State), Revision: existing.Revision}
	before := fingerprint(existing)
	updated := existing
	updated.Source = normalizedSource(candidate.Source)
	updated.Title = bounded(candidate.Title, 1024)
	updated.Body = bounded(candidate.Body, domain.MaxTaskBodyBytes)
	updated.DependsOnKeys = candidate.DependsOnKeys
	updated.EstimatedMinutes = candidate.EstimatedMinutes
	reopened := updated.State == domain.TaskStale
	if reopened {
		updated.State = domain.TaskDiscovered
	}
	missed := updated.MissedPolls
	updated.MissedPolls = 0
	if fingerprint(updated) == before && !reopened && missed == 0 {
		return false, nil
	}
	updated.UpdatedAt = now
	saved, err := p.repository.SaveTask(ctx, updated, expect)
	if errors.Is(err, domain.ErrStateMoved) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if fingerprint(saved) != before || reopened {
		p.emit(ctx, domain.Event{Kind: domain.EventTaskUpdated, TaskID: saved.ID, Payload: taskPayload(saved, "")})
	}
	return false, nil
}

func normalizedSource(source domain.Source) domain.Source {
	if source.Labels == nil {
		source.Labels = []string{}
	}
	source.Key = bounded(source.Key, 256)
	source.URL = bounded(source.URL, 2048)
	return source
}

func fingerprint(task domain.Task) string {
	labels := append([]string{}, task.Source.Labels...)
	sort.Strings(labels)
	deps := append([]string{}, task.DependsOnKeys...)
	sort.Strings(deps)
	return strings.Join([]string{string(task.State), task.Title, task.Body, task.Source.Status, task.Source.Priority, task.Source.Assignee, task.Source.Milestone, task.Source.Epic, task.Source.URL, strings.Join(labels, ","), strings.Join(deps, ","), fmt.Sprint(task.EstimatedMinutes)}, "\x00")
}

// bounded cuts value to at most limit bytes without splitting a rune.
func bounded(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	cut := value[:limit]
	for !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

func taskPayload(task domain.Task, note string) []byte {
	payload := map[string]any{"sourceKey": task.Source.Key, "title": bounded(task.Title, 200), "state": task.State}
	if note != "" {
		payload["note"] = note
	}
	data, _ := json.Marshal(payload)
	return data
}

// Health lists every source polled so far, ordered by workspace and kind.
func (p *Poller) Health() []Health {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Health, 0, len(p.health))
	for _, health := range p.health {
		out = append(out, health)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].WorkspaceID != out[j].WorkspaceID {
			return out[i].WorkspaceID < out[j].WorkspaceID
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

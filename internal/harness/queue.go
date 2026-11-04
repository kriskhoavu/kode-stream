package harness

import (
	"context"
	"errors"
	"log"
	"strings"

	"kode-stream/internal/harness/application/queue"
	"kode-stream/internal/harness/domain"
)

// atomic runs fn in one transaction and publishes the events it stored only
// after the commit, so a subscriber never sees an event for a rolled-back
// change.
func (s *Service) atomic(ctx context.Context, fn func(tx domain.Repository, record func(domain.Event) error) error) error {
	var stored []domain.Event
	err := s.repository.Atomic(ctx, func(tx domain.Repository) error {
		stored = stored[:0]
		return fn(tx, func(event domain.Event) error {
			if event.At.IsZero() {
				event.At = s.now()
			}
			saved, err := tx.AppendEvent(ctx, event)
			if err != nil {
				return err
			}
			stored = append(stored, saved)
			return s.enqueueFor(ctx, tx, saved)
		})
	})
	if err == nil {
		s.publish(stored...)
	}
	return err
}

// Sort scores every discovered and ranked task, moves discovered ones to
// ranked, resolves dependencies to task ids, and saves what changed.
func (s *Service) Sort(ctx context.Context) error {
	if !s.Supported() {
		return domain.ErrUnsupported
	}
	settings := s.settingsNow(ctx)
	tasks, err := s.repository.ListTasks(ctx, domain.TaskFilter{States: []domain.TaskState{domain.TaskDiscovered, domain.TaskRanked}, Limit: 1000})
	if err != nil {
		return err
	}
	all, err := s.repository.ListTasks(ctx, domain.TaskFilter{Limit: 1000})
	if err != nil {
		return err
	}
	idsByKey := map[string]string{}
	for _, task := range all {
		idsByKey[string(task.Source.Kind)+"\x00"+task.Source.Key] = task.ID
	}
	byWorkspace := map[string][]domain.Task{}
	for _, task := range tasks {
		byWorkspace[task.WorkspaceID] = append(byWorkspace[task.WorkspaceID], task)
	}
	now := s.now()
	for _, group := range byWorkspace {
		scored, err := s.rules.Rank(ctx, group, settings, now)
		if err != nil {
			return err
		}
		if s.reranker != nil && settings.Ranker.LLM.Enabled {
			reordered, err := s.reranker.Rerank(ctx, scored, settings)
			if err != nil {
				log.Printf("harness: the re-ranker kept the rules' order: %v", err)
			} else if len(reordered) == len(scored) {
				scored = reordered
			}
		}
		for _, entry := range scored {
			task := entry.Task
			expect := domain.Expect{State: string(task.State), Revision: task.Revision}
			changed := task.State == domain.TaskDiscovered || task.Score != entry.Score || strings.Join(task.ScoreReasons, "\x00") != strings.Join(entry.Reasons, "\x00")
			dependsOn := []string{}
			for _, key := range task.DependsOnKeys {
				if id, ok := idsByKey[string(task.Source.Kind)+"\x00"+key]; ok && id != task.ID {
					dependsOn = append(dependsOn, id)
				}
			}
			if strings.Join(dependsOn, ",") != strings.Join(task.DependsOn, ",") {
				changed = true
			}
			if !changed {
				continue
			}
			task.State = domain.TaskRanked
			task.Score = entry.Score
			task.ScoreReasons = entry.Reasons
			task.DependsOn = dependsOn
			task.UpdatedAt = now
			saved, err := s.repository.SaveTask(ctx, task, expect)
			if errors.Is(err, domain.ErrStateMoved) {
				continue
			}
			if err != nil {
				return err
			}
			s.emit(ctx, domain.Event{Kind: domain.EventTaskRanked, TaskID: saved.ID, Payload: payload(map[string]any{"score": saved.Score, "scoreReasons": saved.ScoreReasons, "state": saved.State})})
		}
	}
	s.wakeWork()
	return nil
}

// updateTask applies change to a task the caller loaded at expectedRevision.
// A task in a terminal state never changes.
func (s *Service) updateTask(ctx context.Context, id string, expectedRevision int64, change func(*domain.Task) error, events func(domain.Task) []domain.Event) (domain.Task, error) {
	if err := s.mutable(); err != nil {
		return domain.Task{}, err
	}
	var saved domain.Task
	err := s.atomic(ctx, func(tx domain.Repository, record func(domain.Event) error) error {
		task, err := tx.GetTask(ctx, id)
		if err != nil {
			return err
		}
		if task.Revision != expectedRevision {
			return domain.ErrStateMoved
		}
		if domain.TaskTerminal(task.State) {
			return domain.IllegalTransitionError{Entity: "task", From: string(task.State), To: "changed"}
		}
		expect := domain.Expect{State: string(task.State), Revision: task.Revision}
		if err := change(&task); err != nil {
			return err
		}
		task.UpdatedAt = s.now()
		saved, err = tx.SaveTask(ctx, task, expect)
		if err != nil {
			return err
		}
		for _, event := range events(saved) {
			if err := record(event); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.Task{}, err
	}
	saved.Body = ""
	return saved, nil
}

// rescore recomputes a task's score by the rules at once, so a pin or a
// priority change moves it in the queue without waiting for the sorter.
//
// The settings are read before any transaction opens: SQLite has one
// connection, so a read beside an open transaction would wait for itself.
func (s *Service) rescore(settings domain.Settings, task *domain.Task) {
	score, reasons := s.rules.Score(*task, settings, s.now())
	task.Score = score
	task.ScoreReasons = reasons
}

func (s *Service) PinTask(ctx context.Context, id string, pinned bool, expectedRevision int64, actor domain.Actor) (domain.Task, error) {
	settings := s.settingsNow(ctx)
	return s.updateTask(ctx, id, expectedRevision, func(task *domain.Task) error {
		task.Pinned = pinned
		s.rescore(settings, task)
		return nil
	}, func(task domain.Task) []domain.Event {
		return []domain.Event{{Kind: domain.EventTaskPinned, TaskID: task.ID, Actor: actorOrNil(actor), Payload: payload(map[string]any{"pinned": task.Pinned, "score": task.Score})}}
	})
}

// maxPriorityOverride bounds a hand-set priority, so it reorders without
// outweighing a pin.
const maxPriorityOverride = 500

func (s *Service) SetPriority(ctx context.Context, id string, override float64, expectedRevision int64, actor domain.Actor) (domain.Task, error) {
	if override < -maxPriorityOverride || override > maxPriorityOverride {
		return domain.Task{}, domain.Invalid("priorityOverride must be between -%d and %d", maxPriorityOverride, maxPriorityOverride)
	}
	settings := s.settingsNow(ctx)
	return s.updateTask(ctx, id, expectedRevision, func(task *domain.Task) error {
		task.PriorityOverride = override
		s.rescore(settings, task)
		return nil
	}, func(task domain.Task) []domain.Event {
		return []domain.Event{{Kind: domain.EventTaskRanked, TaskID: task.ID, Actor: actorOrNil(actor), Payload: payload(map[string]any{"priorityOverride": task.PriorityOverride, "score": task.Score, "scoreReasons": task.ScoreReasons})}}
	})
}

func (s *Service) DismissTask(ctx context.Context, id, reason string, expectedRevision int64, actor domain.Actor) (domain.Task, error) {
	return s.updateTask(ctx, id, expectedRevision, func(task *domain.Task) error {
		if err := domain.CheckTask(task.State, domain.TaskDismissed); err != nil {
			return err
		}
		if task.State == domain.TaskBlocked {
			return domain.Invalid("cancel the task's run before dismissing it")
		}
		task.State = domain.TaskDismissed
		task.Note = boundedText(strings.TrimSpace(reason), 600)
		task.NeedsYou = false
		return nil
	}, func(task domain.Task) []domain.Event {
		return []domain.Event{{Kind: domain.EventTaskUpdated, TaskID: task.ID, Actor: actorOrNil(actor), Payload: payload(map[string]any{"state": task.State, "note": task.Note})}}
	})
}

// StartTask records a person's start approval. The dispatcher takes the task
// on its next cycle; with autoStart off this is the only way work begins.
// Replaying the same idempotency key returns the task unchanged.
func (s *Service) StartTask(ctx context.Context, id string, expectedRevision int64, idempotencyKey string, actor domain.Actor, channel domain.Channel) (domain.Task, error) {
	if err := s.mutable(); err != nil {
		return domain.Task{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) > 200 {
		return domain.Task{}, domain.Invalid("idempotencyKey is longer than 200 characters")
	}
	if idempotencyKey != "" {
		if approval, found, err := s.repository.FindApprovalByIdempotencyKey(ctx, idempotencyKey); err != nil {
			return domain.Task{}, err
		} else if found {
			if approval.Gate != domain.GateStart || approval.TaskID != id {
				return domain.Task{}, domain.Invalid("idempotencyKey was used for another decision")
			}
			task, err := s.repository.GetTask(ctx, id)
			task.Body = ""
			return task, err
		}
	}
	if channel == "" {
		channel = domain.ChannelDashboard
	}
	settings := s.settingsNow(ctx)
	var saved domain.Task
	err := s.atomic(ctx, func(tx domain.Repository, record func(domain.Event) error) error {
		task, err := tx.GetTask(ctx, id)
		if err != nil {
			return err
		}
		if task.Revision != expectedRevision {
			return domain.ErrStateMoved
		}
		if task.State != domain.TaskRanked && task.State != domain.TaskDiscovered {
			return domain.IllegalTransitionError{Entity: "task", From: string(task.State), To: "started"}
		}
		if task.StartRequested {
			saved = task
			return nil
		}
		expect := domain.Expect{State: string(task.State), Revision: task.Revision}
		task.StartRequested = true
		if task.State == domain.TaskDiscovered {
			task.State = domain.TaskRanked
			s.rescore(settings, &task)
		}
		now := s.now()
		task.UpdatedAt = now
		if saved, err = tx.SaveTask(ctx, task, expect); err != nil {
			return err
		}
		approval := domain.Approval{ID: domain.NewID("apr"), TaskID: task.ID, SourceKey: task.Source.Key, Title: task.Title, Gate: domain.GateStart, State: domain.ApprovalApproved, RequestedAt: now, DecidedAt: &now, DecidedBy: &actor, Channel: channel, IdempotencyKey: idempotencyKey}
		if err := tx.InsertApproval(ctx, approval); err != nil {
			return err
		}
		if err := record(domain.Event{Kind: domain.EventApprovalDecided, TaskID: task.ID, ApprovalID: approval.ID, Actor: actorOrNil(actor), Payload: payload(map[string]any{"gate": approval.Gate, "decision": "approve", "channel": channel})}); err != nil {
			return err
		}
		return record(domain.Event{Kind: domain.EventTaskUpdated, TaskID: task.ID, Actor: actorOrNil(actor), Payload: payload(map[string]any{"state": saved.State, "startRequested": true})})
	})
	if err != nil {
		return domain.Task{}, err
	}
	saved.Body = ""
	s.wakeWork()
	return saved, nil
}

// PollNow asks the sources at once, for one workspace or all of them.
func (s *Service) PollNow(ctx context.Context, workspaceID string) ([]SourceHealth, error) {
	if err := s.mutable(); err != nil {
		return nil, err
	}
	if s.poller == nil {
		return []SourceHealth{}, nil
	}
	settings := s.settingsNow(ctx)
	if workspaceID != "" {
		if _, ok := settings.Workspace(workspaceID); !ok {
			return nil, domain.Invalid("workspace %s has no harness sources", workspaceID)
		}
	}
	polled, err := s.poller.Poll(ctx, settings, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]SourceHealth, 0, len(polled))
	for _, health := range polled {
		out = append(out, SourceHealth{WorkspaceID: health.WorkspaceID, Kind: string(health.Kind), LastPolledAt: health.LastPolledAt, Found: health.Found, Added: health.Added, LastError: health.LastError})
	}
	if err := s.Sort(ctx); err != nil {
		return out, err
	}
	return out, nil
}

// SetRolePaused pauses or resumes one worker. A paused role finishes what it
// holds and claims nothing new.
func (s *Service) SetRolePaused(ctx context.Context, key domain.RoleKey, paused bool, actor domain.Actor) (RoleHealth, error) {
	if err := s.mutable(); err != nil {
		return RoleHealth{}, err
	}
	if !domain.ValidRoleKey(key) {
		return RoleHealth{}, domain.ErrNotFound
	}
	var saved domain.Settings
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		settings := s.settingsNow(ctx)
		if settings.Paused == nil {
			settings.Paused = map[domain.RoleKey]bool{}
		}
		if paused {
			settings.Paused[key] = true
		} else {
			delete(settings.Paused, key)
		}
		saved, err = s.repository.SaveSettings(ctx, settings, settings.Revision)
		if !errors.Is(err, domain.ErrStateMoved) {
			break
		}
	}
	if err != nil {
		return RoleHealth{}, err
	}
	kind := domain.EventRoleResumed
	if paused {
		kind = domain.EventRolePaused
		s.cancelPausedRole(key)
	}
	s.emit(ctx, domain.Event{Kind: kind, Actor: actorOrNil(actor), Payload: payload(map[string]any{"role": key})})
	if !paused {
		s.wakeWork()
		if key == domain.RoleKeyScout {
			wake(s.pollWake)
		}
		if key == domain.RoleKeySorter {
			wake(s.sortWake)
		}
	}
	status, err := s.Status(ctx)
	if err != nil {
		return RoleHealth{}, err
	}
	for _, role := range status.Roles {
		if role.Role == key {
			role.Paused = saved.Paused[key]
			return role, nil
		}
	}
	return RoleHealth{Role: key, Paused: paused}, nil
}

func boundedText(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	runes := []rune(value)
	for len(string(runes)) > limit {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

var _ queue.Ranker = queue.RuleRanker{}

// MoveBefore sets a task's hand-set priority so that it ranks just above
// another task, the way dragging a card above another does. The override is
// bounded, so a pinned neighbour may stay above.
func (s *Service) MoveBefore(ctx context.Context, id, beforeID string, expectedRevision int64, actor domain.Actor) (domain.Task, error) {
	if !s.Supported() {
		return domain.Task{}, domain.ErrUnsupported
	}
	if beforeID == id {
		return domain.Task{}, domain.Invalid("a task cannot move before itself")
	}
	above, err := s.repository.GetTask(ctx, beforeID)
	if err != nil {
		return domain.Task{}, err
	}
	task, err := s.repository.GetTask(ctx, id)
	if err != nil {
		return domain.Task{}, err
	}
	override := task.PriorityOverride + (above.Score + 0.5 - task.Score)
	override = max(-maxPriorityOverride, min(maxPriorityOverride, override))
	return s.SetPriority(ctx, id, override, expectedRevision, actor)
}

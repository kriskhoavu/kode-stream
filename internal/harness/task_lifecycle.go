package harness

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"kode-stream/internal/harness/domain"
)

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

// A pushed branch and a pull request stay at the remote.
func (s *Service) RemoveTask(ctx context.Context, id string, expectedRevision int64, actor domain.Actor) (domain.Task, error) {
	if err := s.mutable(); err != nil {
		return domain.Task{}, err
	}
	task, err := s.repository.GetTask(ctx, id)
	if err != nil {
		return domain.Task{}, err
	}
	if task.Revision != expectedRevision {
		return domain.Task{}, domain.ErrStateMoved
	}
	runs, err := s.discardTaskWork(ctx, task, "removed", actor)
	if err != nil {
		return domain.Task{}, err
	}
	err = s.atomic(ctx, func(tx domain.Repository, record func(domain.Event) error) error {
		if _, err := tx.GetTask(ctx, id); err != nil {
			return err
		}
		if err := tx.DeleteTaskHistory(ctx, id); err != nil {
			return err
		}
		return record(domain.Event{Kind: domain.EventTaskRemoved, TaskID: id, Actor: actorOrNil(actor), Payload: payload(map[string]any{"sourceKey": task.Source.Key, "title": boundedText(task.Title, 200), "state": task.State, "runs": len(runs)})})
	})
	if err != nil {
		return domain.Task{}, err
	}
	task.Body = ""
	return task, nil
}

func (s *Service) discardTaskWork(ctx context.Context, task domain.Task, reason string, actor domain.Actor) ([]domain.Run, error) {
	if task.RunID != "" {
		run, err := s.repository.GetRun(ctx, task.RunID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		if err == nil && !domain.RunTerminal(run.State) {
			if _, err := s.CancelRun(ctx, run.ID, reason, nil, actor); err != nil && !errors.Is(err, domain.ErrNotFound) {
				return nil, err
			}
		}
	}
	runs, err := s.repository.ListRuns(ctx, domain.RunFilter{TaskID: task.ID, Limit: 1000})
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		s.killRun(run.ID)
		if s.sessions != nil && run.Branch != "" {
			if err := s.sessions(ctx, task.WorkspaceID, run.Branch); err != nil {
				log.Printf("harness: close the sessions of run %s: %v", run.ID, err)
			}
		}
	}
	if s.workspaces != nil {
		if workspace, found, err := s.workspaces.Get(task.WorkspaceID); err == nil && found {
			for _, run := range runs {
				if run.WorktreePath != "" {
					s.removeWorktree(ctx, run, workspace, true)
				}
			}
		}
	}
	return runs, nil
}

// A pushed branch and a pull request stay at the remote.
func (s *Service) ResetTask(ctx context.Context, id string, expectedRevision int64, actor domain.Actor) (domain.Task, error) {
	if err := s.mutable(); err != nil {
		return domain.Task{}, err
	}
	task, err := s.repository.GetTask(ctx, id)
	if err != nil {
		return domain.Task{}, err
	}
	if task.Revision != expectedRevision {
		return domain.Task{}, domain.ErrStateMoved
	}
	if task.State == domain.TaskDismissed || task.State == domain.TaskStale {
		return domain.Task{}, domain.Invalid("%s is not in the queue; only a task in the queue can be reset", task.Source.Key)
	}
	runs, err := s.discardTaskWork(ctx, task, "reset", actor)
	if err != nil {
		return domain.Task{}, err
	}
	settings := s.settingsNow(ctx)
	var fresh domain.Task
	err = s.atomic(ctx, func(tx domain.Repository, record func(domain.Event) error) error {
		current, err := tx.GetTask(ctx, id)
		if err != nil {
			return err
		}
		if err := tx.DeleteTaskHistory(ctx, id); err != nil {
			return err
		}
		fresh = resetTask(current, s.now())
		s.rescore(settings, &fresh)
		if err := tx.InsertTask(ctx, fresh); err != nil {
			return err
		}
		return record(domain.Event{Kind: domain.EventTaskUpdated, TaskID: id, Actor: actorOrNil(actor), Payload: payload(map[string]any{"state": fresh.State, "reset": true, "runs": len(runs)})})
	})
	if err != nil {
		return domain.Task{}, err
	}
	fresh.Body = ""
	return fresh, nil
}

func resetTask(task domain.Task, now time.Time) domain.Task {
	return domain.Task{
		ID:               task.ID,
		WorkspaceID:      task.WorkspaceID,
		Source:           task.Source,
		Title:            task.Title,
		Body:             task.Body,
		State:            domain.TaskRanked,
		PriorityOverride: task.PriorityOverride,
		Pinned:           task.Pinned,
		AskQuestions:     task.AskQuestions,
		DependsOn:        task.DependsOn,
		DependsOnKeys:    task.DependsOnKeys,
		EstimatedMinutes: task.EstimatedMinutes,
		Revision:         task.Revision + 1,
		DiscoveredAt:     task.DiscoveredAt,
		UpdatedAt:        now,
	}
}

func (s *Service) ClearQueue(ctx context.Context, workspaceID string, actor domain.Actor) (int, error) {
	if err := s.mutable(); err != nil {
		return 0, err
	}
	if workspaceID != "" {
		if _, ok := s.settingsNow(ctx).Workspace(workspaceID); !ok {
			return 0, domain.Invalid("workspace %s has no harness sources", workspaceID)
		}
	}
	removed := 0
	err := s.atomic(ctx, func(tx domain.Repository, record func(domain.Event) error) error {
		tasks, err := tx.DeleteTasks(ctx, domain.TaskFilter{WorkspaceID: workspaceID, States: []domain.TaskState{domain.TaskDiscovered, domain.TaskRanked, domain.TaskStale, domain.TaskDismissed}})
		if err != nil {
			return err
		}
		removed = len(tasks)
		for _, task := range tasks {
			if err := record(domain.Event{Kind: domain.EventTaskRemoved, TaskID: task.ID, Actor: actorOrNil(actor), Payload: payload(map[string]any{"sourceKey": task.Source.Key, "title": boundedText(task.Title, 200), "state": task.State})}); err != nil {
				return err
			}
		}
		return nil
	})
	return removed, err
}

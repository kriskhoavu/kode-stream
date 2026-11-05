package harness

import (
	"context"
	"encoding/json"
	"time"

	"kode-stream/internal/harness/application/notify"
	"kode-stream/internal/harness/application/scheduler"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

const notifierInterval = 5 * time.Second

// enqueueFor writes the outbox row of a stored event the bot is told about,
// through the same repository that stored the event.
func (s *Service) enqueueFor(ctx context.Context, repository domain.Repository, event domain.Event) error {
	if s.notifier == nil || !notify.Notifiable[event.Kind] || event.Seq == 0 {
		return nil
	}
	return repository.EnqueueNotification(ctx, domain.Notification{ID: domain.NewID("ntf"), EventSeq: event.Seq, Target: notify.Target, State: domain.NotificationPending, NextAt: s.now()})
}

func (s *Service) notifierLoop() scheduler.Loop {
	return scheduler.Loop{Name: "notifier", Interval: func() time.Duration { return notifierInterval }, Run: s.deliverDue}
}

// deliverDue sends every due outbox row once. A failure waits 5 s doubling
// to 10 minutes; after five attempts the row fails and a
// notification.failed event says so.
func (s *Service) deliverDue(ctx context.Context) error {
	if s.notifier == nil {
		return nil
	}
	due, err := s.repository.DueNotifications(ctx, s.now(), 20)
	if err != nil {
		return err
	}
	for _, notification := range due {
		body, err := s.webhookBody(ctx, notification.EventSeq)
		if err == nil {
			err = s.notifier.Deliver(ctx, ports.Delivery{EventID: notify.EventID(notification.EventSeq), Attempt: notification.Attempts + 1, Body: body})
		}
		notification.Attempts++
		switch {
		case err == nil:
			notification.State, notification.LastError = domain.NotificationDelivered, ""
		case notification.Attempts >= notify.MaxAttempts:
			notification.State, notification.LastError = domain.NotificationFailed, boundedText(err.Error(), 400)
			s.emit(ctx, domain.Event{Kind: domain.EventNotificationFailed, Payload: payload(map[string]any{"eventId": notify.EventID(notification.EventSeq), "attempts": notification.Attempts, "error": notification.LastError})})
		default:
			notification.LastError = boundedText(err.Error(), 400)
			notification.NextAt = s.now().Add(notify.Backoff(notification.Attempts))
		}
		if err := s.repository.SaveNotification(ctx, notification); err != nil {
			return err
		}
	}
	return nil
}

// webhookBody describes one stored event for the bot, with the task, run and
// approval it names.
func (s *Service) webhookBody(ctx context.Context, seq int64) ([]byte, error) {
	events, err := s.repository.EventsAfter(ctx, seq-1, 1)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 || events[0].Seq != seq {
		return nil, domain.ErrNotFound
	}
	event := events[0]
	body := notify.Payload{EventID: notify.EventID(seq), Seq: seq, Kind: event.Kind, At: event.At, Links: map[string]string{}}
	var fields map[string]any
	_ = json.Unmarshal(event.Payload, &fields)
	for _, key := range []string{"summary", "reason"} {
		if text, ok := fields[key].(string); ok && text != "" && body.Summary == "" {
			body.Summary = boundedText(text, domain.MaxSummaryChars)
		}
	}
	if event.TaskID != "" {
		if task, err := s.repository.GetTask(ctx, event.TaskID); err == nil {
			body.Task = &notify.TaskRef{ID: task.ID, SourceKey: task.Source.Key, Title: task.Title, URL: task.Source.URL}
		}
	}
	if event.RunID != "" {
		if run, err := s.repository.GetRun(ctx, event.RunID); err == nil {
			ref := &notify.RunRef{ID: run.ID, Stage: string(run.CurrentStageKind), CostUSD: roundCents(run.Totals.CostUSD), Turns: run.Totals.Turns, State: string(run.State), PRURL: run.PRURL}
			if stage, err := s.repository.GetStage(ctx, run.CurrentStageID); err == nil {
				ref.Round = stage.Round
			}
			body.Run = ref
			if s.config.DashboardURL != "" {
				body.Links["dashboard"] = s.config.DashboardURL + "/harness/runs/" + run.ID
			}
		}
	}
	if event.ApprovalID != "" {
		if approval, err := s.repository.GetApproval(ctx, event.ApprovalID); err == nil {
			body.Approval = &notify.ApprovalRef{ID: approval.ID, Gate: string(approval.Gate), Revision: approval.Revision, ExpiresAt: approval.ExpiresAt, Questions: approval.Context.Questions, Role: string(approval.Context.Role)}
			if body.Summary == "" {
				body.Summary = approval.Summary
			}
		}
	}
	if _, ok := body.Links["dashboard"]; !ok && s.config.DashboardURL != "" {
		body.Links["dashboard"] = s.config.DashboardURL + "/harness"
	}
	return json.Marshal(body)
}

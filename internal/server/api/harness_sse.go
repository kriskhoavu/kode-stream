package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kode-stream/internal/harness/domain"
)

const (
	harnessCatchUpLimit     = 500
	harnessKeepAlive        = 15 * time.Second
	harnessStreamLifetime   = 10 * time.Minute
	harnessSubscriberBuffer = 256
)

// events streams harness events. Stored events after the client's sequence
// come first, then live ones. Each message carries `id: <seq>` and no event
// name, so EventSource.onmessage receives every type; a subscriber that falls
// behind gets `event: reset` and refetches its snapshot.
func (a *harnessController) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeHarnessError(w, fmt.Errorf("streaming is not supported"))
		return
	}
	after := harnessAfter(r)
	// The transport's request timeout would end a long stream; the stream
	// ends on its own lifetime or on the first failed write instead, and the
	// client reconnects with Last-Event-ID.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), harnessStreamLifetime)
	defer cancel()
	subscription := a.harness.Subscribe(harnessSubscriberBuffer)
	defer subscription.Cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	stored, err := a.harness.EventsAfter(ctx, after, harnessCatchUpLimit+1)
	if err != nil {
		return
	}
	if len(stored) > harnessCatchUpLimit {
		last, _ := a.harness.LastEventSeq(ctx)
		writeHarnessReset(w, last)
		flusher.Flush()
		return
	}
	for _, event := range stored {
		if !a.writeHarnessEvent(ctx, w, event) {
			return
		}
		after = event.Seq
	}
	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	keepAlive := time.NewTicker(harnessKeepAlive)
	defer keepAlive.Stop()
	// The request context ends both when the client goes away (Canceled) and
	// when the transport's timeout fires (DeadlineExceeded); only the first
	// ends the stream.
	disconnected := r.Context().Done()
	for {
		select {
		case <-disconnected:
			if errors.Is(r.Context().Err(), context.Canceled) {
				return
			}
			disconnected = nil
		case <-ctx.Done():
			return
		case <-subscription.Behind:
			last, _ := a.harness.LastEventSeq(ctx)
			writeHarnessReset(w, last)
			flusher.Flush()
			return
		case event := <-subscription.Events:
			if event.Seq != 0 && event.Seq <= after {
				continue
			}
			if !a.writeHarnessEvent(ctx, w, event) {
				return
			}
			if event.Seq != 0 {
				after = event.Seq
			}
			flusher.Flush()
		case <-keepAlive.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func harnessAfter(r *http.Request) int64 {
	raw := strings.TrimSpace(r.URL.Query().Get("after"))
	if raw == "" {
		raw = strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

// writeHarnessEvent sends one event with the entity it is about; an event
// the dashboard does not show is skipped.
func (a *harnessController) writeHarnessEvent(ctx context.Context, w http.ResponseWriter, event domain.Event) bool {
	view, ok := a.harness.DashboardEvent(ctx, event)
	if !ok {
		return true
	}
	data, err := json.Marshal(view)
	if err != nil {
		return true
	}
	if event.Seq != 0 {
		if _, err := fmt.Fprintf(w, "id: %d\n", event.Seq); err != nil {
			return false
		}
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	return err == nil
}

func writeHarnessReset(w http.ResponseWriter, last int64) {
	writeSSE(w, "reset", map[string]int64{"lastEventSeq": last})
}

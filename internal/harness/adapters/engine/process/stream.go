package process

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"time"

	"kode-stream/internal/harness/ports"
)

// MaxLineBytes bounds one JSON line a CLI prints.
const MaxLineBytes = 4 << 20

const stderrBytes = 64 << 10

// Parser turns a CLI's JSON lines into progress events and a result. Line
// and Result are called from one goroutine.
type Parser interface {
	Line(raw []byte, emit func(ports.EngineEvent))
	// Result returns what the stream reported; finished is false when the
	// CLI never printed its final event.
	Result() (result ports.Result, finished bool)
}

// Spec is one CLI process to start.
type Spec struct {
	Path    string
	Args    []string
	Dir     string
	Env     []string
	Stdin   string
	Timeout time.Duration
	// SessionID is the session the harness chose, used until the stream
	// names one.
	SessionID string
	// TranscriptRef returns the path of the CLI's transcript for a session.
	TranscriptRef func(sessionID string) string
	// Cleanup runs once the process has exited.
	Cleanup func()
}

// Start runs spec in its own process group and parses its stdout with
// parser. Cancel and the timeout stop the whole group.
func Start(ctx context.Context, spec Spec, parser Parser) (ports.Handle, error) {
	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	cmd.Stdin = strings.NewReader(spec.Stdin)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := NewRing(stderrBytes)
	cmd.Stderr = stderr
	Prepare(cmd)
	started := time.Now()
	if err := cmd.Start(); err != nil {
		if spec.Cleanup != nil {
			spec.Cleanup()
		}
		return nil, err
	}
	done := make(chan struct{})
	h := &handle{events: make(chan ports.EngineEvent, 64), done: done, stop: NewStop(cmd, done)}
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 45 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	go func() {
		select {
		case <-runCtx.Done():
			if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
				h.setStopReason("timeout")
			} else {
				h.setStopReason("cancelled")
			}
			h.stop.Terminate()
		case <-done:
		}
	}()
	go func() {
		defer cancel()
		parseErr := Lines(stdout, MaxLineBytes, func(line []byte) { parser.Line(line, h.emit) })
		waitErr := cmd.Wait()
		if spec.Cleanup != nil {
			spec.Cleanup()
		}
		result, finished := parser.Result()
		if result.SessionID == "" {
			result.SessionID = spec.SessionID
		}
		result.DurationMs = max(result.DurationMs, time.Since(started).Milliseconds())
		result.Stderr = stderr.String()
		var exit *exec.ExitError
		if errors.As(waitErr, &exit) {
			result.ExitCode = exit.ExitCode()
		} else if waitErr != nil {
			result.ExitCode = -1
		}
		h.mu.Lock()
		switch {
		case h.forcedStop != "":
			result.StopReason, result.IsError = h.forcedStop, true
		case !finished:
			result.IsError, result.StopReason = true, "no_result"
			if parseErr != nil {
				result.StopReason = "unreadable_output"
			}
		case result.ExitCode != 0:
			result.IsError = true
		}
		if result.SessionID != "" && spec.TranscriptRef != nil {
			result.TranscriptRef = spec.TranscriptRef(result.SessionID)
		}
		h.result = result
		h.mu.Unlock()
		close(h.events)
		close(done)
	}()
	return h, nil
}

type handle struct {
	events     chan ports.EngineEvent
	done       chan struct{}
	stop       *Stop
	mu         sync.Mutex
	result     ports.Result
	forcedStop string
	finished   bool
}

func (h *handle) Events() <-chan ports.EngineEvent { return h.events }

func (h *handle) Cancel() {
	h.setStopReason("cancelled")
	go h.stop.Terminate()
}

func (h *handle) Wait() ports.Result {
	<-h.done
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.result
}

func (h *handle) setStopReason(reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.forcedStop == "" {
		h.forcedStop = reason
	}
}

// emit never blocks the parser: a reader that falls behind loses progress
// events, never the result.
func (h *handle) emit(event ports.EngineEvent) {
	select {
	case h.events <- event:
	default:
	}
}

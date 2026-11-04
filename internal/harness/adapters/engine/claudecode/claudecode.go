// Package claudecode runs Claude Code headless for one agent run:
// `claude -p --output-format stream-json` in the run's worktree, the brief on
// stdin, the CLI's own --max-budget-usd as the only cost stop. It parses the
// JSON lines into turns, usage, cost and the session id, and keeps no text
// beyond a 200-character progress summary and the final answer.
package claudecode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"kode-stream/internal/harness/adapters/engine/process"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

const (
	ID           = "claudecode"
	maxLineBytes = 4 << 20
	stderrBytes  = 64 << 10
)

// DefaultEnv lists the variables Claude Code needs beyond PATH and HOME.
var DefaultEnv = []string{"CLAUDE_CONFIG_DIR", "ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "AWS_PROFILE", "AWS_REGION", "HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "SSL_CERT_FILE"}

type Engine struct {
	Binary         string
	Getenv         func(string) string
	EnvAllowList   []string
	TranscriptPath func(workingDir, sessionID string) (string, error)
}

func New(transcriptPath func(string, string) (string, error)) *Engine {
	return &Engine{Binary: "claude", Getenv: os.Getenv, EnvAllowList: DefaultEnv, TranscriptPath: transcriptPath}
}

func (e *Engine) ID() string { return ID }

func (e *Engine) Detect(ctx context.Context) ports.Detection {
	path, err := exec.LookPath(e.Binary)
	if err != nil {
		return ports.Detection{Unsupported: []string{}}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	detection := ports.Detection{Available: err == nil, Path: path, Unsupported: []string{}}
	if err == nil {
		detection.Version = strings.TrimSpace(string(out))
	}
	return detection
}

// Args builds the argv after the binary. Tools are passed one argument each
// so that a rule such as `Bash(git diff:*)` keeps its space.
func Args(invocation ports.Invocation) []string {
	args := []string{"-p", "--output-format", "stream-json", "--verbose"}
	if invocation.Model != "" {
		args = append(args, "--model", invocation.Model)
	}
	if invocation.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(invocation.MaxTurns))
	}
	if invocation.PermissionMode != "" {
		args = append(args, "--permission-mode", invocation.PermissionMode)
	}
	if len(invocation.AllowedTools) > 0 {
		args = append(append(args, "--allowedTools"), invocation.AllowedTools...)
	}
	if len(invocation.DisallowedTools) > 0 {
		args = append(append(args, "--disallowedTools"), invocation.DisallowedTools...)
	}
	if invocation.SessionID != "" {
		args = append(args, "--session-id", invocation.SessionID)
	}
	if invocation.MaxBudgetUSD > 0 {
		args = append(args, "--max-budget-usd", strconv.FormatFloat(invocation.MaxBudgetUSD, 'f', 2, 64))
	}
	return args
}

func (e *Engine) Start(ctx context.Context, invocation ports.Invocation) (ports.Handle, error) {
	if strings.TrimSpace(invocation.WorktreePath) == "" {
		return nil, errors.New("an agent run needs a worktree")
	}
	path, err := exec.LookPath(e.Binary)
	if err != nil {
		return nil, fmt.Errorf("Claude Code is not installed: %w", err)
	}
	cmd := exec.Command(path, Args(invocation)...)
	cmd.Dir = invocation.WorktreePath
	cmd.Env = process.Environment(e.Getenv, e.EnvAllowList, invocation.Env)
	cmd.Stdin = strings.NewReader(invocation.Brief)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := process.NewRing(stderrBytes)
	cmd.Stderr = stderr
	process.Prepare(cmd)
	started := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	h := &handle{events: make(chan ports.EngineEvent, 64), done: done, stop: process.NewStop(cmd, done)}
	h.result.SessionID = invocation.SessionID
	timeout := invocation.Timeout
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
		parseErr := process.Lines(stdout, maxLineBytes, h.visit)
		waitErr := cmd.Wait()
		h.mu.Lock()
		h.result.DurationMs = max(h.result.DurationMs, time.Since(started).Milliseconds())
		h.result.Stderr = stderr.String()
		if exit := (*exec.ExitError)(nil); errors.As(waitErr, &exit) {
			h.result.ExitCode = exit.ExitCode()
		} else if waitErr != nil {
			h.result.ExitCode = -1
		}
		if h.forcedStop != "" {
			h.result.StopReason = h.forcedStop
			h.result.IsError = true
		} else if !h.sawResult {
			h.result.IsError = true
			h.result.StopReason = "no_result"
			if parseErr != nil {
				h.result.StopReason = "unreadable_output"
			}
		} else if h.result.ExitCode != 0 && !h.result.IsError {
			h.result.IsError = true
		}
		if h.result.SessionID != "" && e.TranscriptPath != nil {
			if ref, err := e.TranscriptPath(invocation.WorktreePath, h.result.SessionID); err == nil {
				h.result.TranscriptRef = ref
			}
		}
		h.mu.Unlock()
		close(h.events)
		close(done)
	}()
	return h, nil
}

type handle struct {
	events     chan ports.EngineEvent
	done       chan struct{}
	stop       *process.Stop
	mu         sync.Mutex
	result     ports.Result
	sawResult  bool
	forcedStop string
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
	if h.forcedStop == "" && !h.sawResult {
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

type streamLine struct {
	Type         string  `json:"type"`
	Subtype      string  `json:"subtype"`
	SessionID    string  `json:"session_id"`
	IsError      bool    `json:"is_error"`
	DurationMs   int64   `json:"duration_ms"`
	NumTurns     int     `json:"num_turns"`
	Result       string  `json:"result"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        *struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
	Message *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
			Name string `json:"name"`
		} `json:"content"`
	} `json:"message"`
}

func (h *handle) visit(raw []byte) {
	var line streamLine
	if json.Unmarshal(raw, &line) != nil {
		return
	}
	switch line.Type {
	case "system":
		if line.Subtype == "init" && line.SessionID != "" {
			h.mu.Lock()
			h.result.SessionID = line.SessionID
			h.mu.Unlock()
			h.emit(ports.EngineEvent{Kind: ports.EngineSession, SessionID: line.SessionID})
		}
	case "assistant":
		if line.Message == nil {
			return
		}
		var tools []string
		var text strings.Builder
		for _, block := range line.Message.Content {
			switch block.Type {
			case "tool_use":
				tools = append(tools, block.Name)
			case "text":
				text.WriteString(block.Text)
				text.WriteByte(' ')
			}
		}
		h.mu.Lock()
		h.result.Turns++
		turns := h.result.Turns
		h.mu.Unlock()
		h.emit(ports.EngineEvent{Kind: ports.EngineTurn, Turns: turns, Tools: tools, Summary: process.Summary(text.String(), domain.MaxProgressChars)})
	case "result":
		h.mu.Lock()
		defer h.mu.Unlock()
		h.sawResult = true
		h.result.IsError = line.IsError || (line.Subtype != "" && line.Subtype != "success")
		h.result.StopReason = line.Subtype
		if line.NumTurns > 0 {
			h.result.Turns = line.NumTurns
		}
		h.result.DurationMs = line.DurationMs
		h.result.CostUSD = line.TotalCostUSD
		h.result.FinalText = line.Result
		if line.SessionID != "" {
			h.result.SessionID = line.SessionID
		}
		if line.Usage != nil {
			h.result.Usage = domain.Usage{InputTokens: line.Usage.InputTokens, OutputTokens: line.Usage.OutputTokens, CacheReadTokens: line.Usage.CacheReadInputTokens, CacheWriteTokens: line.Usage.CacheCreationInputTokens}
		}
		h.result.Usage.CostUSD = line.TotalCostUSD
	}
}

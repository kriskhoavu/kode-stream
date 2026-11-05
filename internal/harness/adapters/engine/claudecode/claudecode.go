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
	"time"

	"kode-stream/internal/harness/adapters/engine/process"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

const ID = "claudecode"

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
	spec := process.Spec{Path: path, Args: Args(invocation), Dir: invocation.WorktreePath, Env: process.Environment(e.Getenv, e.EnvAllowList, invocation.Env), Stdin: invocation.Brief, Timeout: invocation.Timeout, SessionID: invocation.SessionID}
	if e.TranscriptPath != nil {
		spec.TranscriptRef = func(sessionID string) string {
			ref, _ := e.TranscriptPath(invocation.WorktreePath, sessionID)
			return ref
		}
	}
	return process.Start(ctx, spec, &parser{})
}

// TranscriptRef is where Claude Code writes the session's transcript; "" when
// the adapter was built without a locator.
func (e *Engine) TranscriptRef(worktreePath, sessionID string) string {
	if e.TranscriptPath == nil {
		return ""
	}
	ref, _ := e.TranscriptPath(worktreePath, sessionID)
	return ref
}

// parser reads Claude Code's stream-json lines.
type parser struct {
	result   ports.Result
	finished bool
}

func (p *parser) Result() (ports.Result, bool) { return p.result, p.finished }

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

func (p *parser) Line(raw []byte, emit func(ports.EngineEvent)) {
	var line streamLine
	if json.Unmarshal(raw, &line) != nil {
		return
	}
	switch line.Type {
	case "system":
		if line.Subtype == "init" && line.SessionID != "" {
			p.result.SessionID = line.SessionID
			emit(ports.EngineEvent{Kind: ports.EngineSession, SessionID: line.SessionID})
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
		p.result.Turns++
		emit(ports.EngineEvent{Kind: ports.EngineTurn, Turns: p.result.Turns, Tools: tools, Summary: process.Summary(text.String(), domain.MaxProgressChars)})
	case "result":
		p.finished = true
		p.result.IsError = line.IsError || (line.Subtype != "" && line.Subtype != "success")
		p.result.StopReason = line.Subtype
		if line.NumTurns > 0 {
			p.result.Turns = line.NumTurns
		}
		p.result.DurationMs = line.DurationMs
		p.result.CostUSD = line.TotalCostUSD
		p.result.FinalText = line.Result
		if line.SessionID != "" {
			p.result.SessionID = line.SessionID
		}
		if line.Usage != nil {
			p.result.Usage = domain.Usage{InputTokens: line.Usage.InputTokens, OutputTokens: line.Usage.OutputTokens, CacheReadTokens: line.Usage.CacheReadInputTokens, CacheWriteTokens: line.Usage.CacheCreationInputTokens}
		}
		p.result.Usage.CostUSD = line.TotalCostUSD
	}
}

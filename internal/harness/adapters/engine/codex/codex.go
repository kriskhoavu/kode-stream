// Package codex runs the Codex CLI headless. Its sandbox stands in for the permission mode.
package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"kode-stream/internal/harness/adapters/engine/process"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

const ID = "codex"

var DefaultEnv = []string{"CODEX_HOME", "OPENAI_API_KEY", "OPENAI_BASE_URL", "HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "SSL_CERT_FILE"}

var Unsupported = []string{"maxTurns", "maxBudgetUsd", "allowedTools", "disallowedTools", "sessionId"}

type Engine struct {
	Binary       string
	Getenv       func(string) string
	EnvAllowList []string
}

func New() *Engine { return &Engine{Binary: "codex", Getenv: os.Getenv, EnvAllowList: DefaultEnv} }

func (e *Engine) ID() string { return ID }

func (e *Engine) Detect(ctx context.Context) ports.Detection {
	detection := ports.Detection{Unsupported: append([]string{}, Unsupported...)}
	path, err := exec.LookPath(e.Binary)
	if err != nil {
		return detection
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	detection.Path, detection.Available = path, err == nil
	if err == nil {
		detection.Version = strings.TrimSpace(string(out))
	}
	return detection
}

func Sandbox(permissionMode string) string {
	if permissionMode == "plan" {
		return "read-only"
	}
	return "workspace-write"
}

func Args(invocation ports.Invocation) []string {
	args := []string{"exec", "--json", "--skip-git-repo-check", "--cd", invocation.WorktreePath, "--sandbox", Sandbox(invocation.PermissionMode)}
	if invocation.Model != "" {
		args = append(args, "--model", invocation.Model)
	}
	return append(args, "-")
}

func (e *Engine) Start(ctx context.Context, invocation ports.Invocation) (ports.Handle, error) {
	if strings.TrimSpace(invocation.WorktreePath) == "" {
		return nil, errors.New("an agent run needs a worktree")
	}
	path, err := exec.LookPath(e.Binary)
	if err != nil {
		return nil, fmt.Errorf("Codex is not installed: %w", err)
	}
	spec := process.Spec{Path: path, Args: Args(invocation), Dir: invocation.WorktreePath, Env: process.Environment(e.Getenv, e.EnvAllowList, invocation.Env), Stdin: invocation.Brief, Timeout: invocation.Timeout}
	return process.Start(ctx, spec, &parser{})
}

type parser struct {
	result   ports.Result
	finished bool
	failed   bool
}

func (p *parser) Result() (ports.Result, bool) {
	if p.failed {
		p.result.IsError = true
		if p.result.StopReason == "" {
			p.result.StopReason = "error"
		}
	} else if p.finished && p.result.StopReason == "" {
		p.result.StopReason = "success"
	}
	return p.result, p.finished
}

type event struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id"`
	Message  string `json:"message"`
	Item     *struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Command string `json:"command"`
	} `json:"item"`
	Usage *struct {
		InputTokens       int64 `json:"input_tokens"`
		CachedInputTokens int64 `json:"cached_input_tokens"`
		OutputTokens      int64 `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Codex reports no turns, so each completed command or file change counts as one.
func (p *parser) Line(raw []byte, emit func(ports.EngineEvent)) {
	var line event
	if json.Unmarshal(raw, &line) != nil {
		return
	}
	switch line.Type {
	case "thread.started":
		p.result.SessionID = line.ThreadID
		emit(ports.EngineEvent{Kind: ports.EngineSession, SessionID: line.ThreadID})
	case "item.completed":
		if line.Item == nil {
			return
		}
		switch line.Item.Type {
		case "agent_message":
			p.result.FinalText = line.Item.Text
			emit(ports.EngineEvent{Kind: ports.EngineText, Turns: p.result.Turns, Summary: process.Summary(line.Item.Text, domain.MaxProgressChars)})
		case "command_execution", "file_change", "mcp_tool_call":
			p.result.Turns++
			emit(ports.EngineEvent{Kind: ports.EngineTurn, Turns: p.result.Turns, Tools: []string{line.Item.Type}, Summary: process.Summary(line.Item.Command, domain.MaxProgressChars)})
		}
	case "turn.completed":
		p.finished = true
		if line.Usage != nil {
			p.result.Usage.InputTokens += line.Usage.InputTokens
			p.result.Usage.CacheReadTokens += line.Usage.CachedInputTokens
			p.result.Usage.OutputTokens += line.Usage.OutputTokens
		}
	case "turn.failed", "error":
		p.finished, p.failed = true, true
		message := line.Message
		if line.Error != nil {
			message = line.Error.Message
		}
		p.result.StopReason = "error: " + process.Summary(message, 300)
	}
}

// Package opencode runs OpenCode headless. The brief goes in a file and never reaches argv.
package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"kode-stream/internal/harness/adapters/engine/process"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

const ID = "opencode"

const BriefFile = ".harness/brief.md"

const message = "Follow the attached brief exactly. It names your goal, scope, constraints, deliverable and verification."

var DefaultEnv = []string{"OPENCODE_CONFIG", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OPENAI_BASE_URL", "HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "SSL_CERT_FILE"}

var Unsupported = []string{"maxTurns", "maxBudgetUsd", "allowedTools", "disallowedTools", "permissionMode"}

type Engine struct {
	Binary       string
	Getenv       func(string) string
	EnvAllowList []string
}

func New() *Engine { return &Engine{Binary: "opencode", Getenv: os.Getenv, EnvAllowList: DefaultEnv} }

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

func Args(invocation ports.Invocation) []string {
	args := []string{"run", "--format", "json", "--file", BriefFile}
	if invocation.Model != "" {
		args = append(args, "--model", invocation.Model)
	}
	if invocation.SessionID != "" {
		args = append(args, "--session", invocation.SessionID)
	}
	if invocation.PermissionMode != "plan" {
		args = append(args, "--auto")
	}
	return append(args, message)
}

func (e *Engine) Start(ctx context.Context, invocation ports.Invocation) (ports.Handle, error) {
	if strings.TrimSpace(invocation.WorktreePath) == "" {
		return nil, errors.New("an agent run needs a worktree")
	}
	if invocation.Model != "" && !strings.Contains(invocation.Model, "/") {
		return nil, fmt.Errorf("OpenCode needs a provider/model name, not %q", invocation.Model)
	}
	path, err := exec.LookPath(e.Binary)
	if err != nil {
		return nil, fmt.Errorf("OpenCode is not installed: %w", err)
	}
	briefPath := filepath.Join(invocation.WorktreePath, BriefFile)
	if err := os.MkdirAll(filepath.Dir(briefPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(briefPath, []byte(invocation.Brief), 0o600); err != nil {
		return nil, err
	}
	spec := process.Spec{Path: path, Args: Args(invocation), Dir: invocation.WorktreePath, Env: process.Environment(e.Getenv, e.EnvAllowList, invocation.Env), Timeout: invocation.Timeout, SessionID: invocation.SessionID, Cleanup: func() { _ = os.Remove(briefPath) }}
	return process.Start(ctx, spec, &parser{})
}

type parser struct {
	result   ports.Result
	finished bool
	failed   bool
	text     strings.Builder
}

func (p *parser) Result() (ports.Result, bool) {
	p.result.FinalText = p.text.String()
	if p.failed {
		p.result.IsError = true
	} else if p.finished && p.result.StopReason == "" {
		p.result.StopReason = "success"
	}
	p.result.Usage.CostUSD = p.result.CostUSD
	return p.result, p.finished
}

type event struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionID"`
	Error     *struct {
		Name string `json:"name"`
		Data *struct {
			Message string `json:"message"`
		} `json:"data"`
	} `json:"error"`
	Part *struct {
		Type   string  `json:"type"`
		Text   string  `json:"text"`
		Tool   string  `json:"tool"`
		Reason string  `json:"reason"`
		Cost   float64 `json:"cost"`
		Tokens *struct {
			Input  int64 `json:"input"`
			Output int64 `json:"output"`
			Cache  *struct {
				Read  int64 `json:"read"`
				Write int64 `json:"write"`
			} `json:"cache"`
		} `json:"tokens"`
	} `json:"part"`
}

func (p *parser) Line(raw []byte, emit func(ports.EngineEvent)) {
	var line event
	if json.Unmarshal(raw, &line) != nil {
		return
	}
	if line.SessionID != "" && p.result.SessionID != line.SessionID {
		p.result.SessionID = line.SessionID
		emit(ports.EngineEvent{Kind: ports.EngineSession, SessionID: line.SessionID})
	}
	switch line.Type {
	case "text":
		if line.Part != nil {
			p.text.Reset()
			p.text.WriteString(line.Part.Text)
			emit(ports.EngineEvent{Kind: ports.EngineText, Turns: p.result.Turns, Summary: process.Summary(line.Part.Text, domain.MaxProgressChars)})
		}
	case "tool_use", "tool":
		if line.Part != nil {
			emit(ports.EngineEvent{Kind: ports.EngineTurn, Turns: p.result.Turns, Tools: []string{line.Part.Tool}})
		}
	case "step_finish":
		p.finished = true
		p.result.Turns++
		if line.Part != nil {
			p.result.CostUSD += line.Part.Cost
			if tokens := line.Part.Tokens; tokens != nil {
				p.result.Usage.InputTokens += tokens.Input
				p.result.Usage.OutputTokens += tokens.Output
				if tokens.Cache != nil {
					p.result.Usage.CacheReadTokens += tokens.Cache.Read
					p.result.Usage.CacheWriteTokens += tokens.Cache.Write
				}
			}
		}
	case "error":
		p.finished, p.failed = true, true
		reason := "error"
		if line.Error != nil {
			reason = "error: " + line.Error.Name
			if line.Error.Data != nil && line.Error.Data.Message != "" {
				reason += ": " + process.Summary(line.Error.Data.Message, 300)
			}
		}
		p.result.StopReason = reason
	}
}

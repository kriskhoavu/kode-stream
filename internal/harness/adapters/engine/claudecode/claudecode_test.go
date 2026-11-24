package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/harness/ports"
)

func fakeClaude(t *testing.T, mode string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	record := filepath.Join(dir, "record")
	script := `#!/bin/sh
printf '%s\n' "$@" > "` + record + `.args"
cat > "` + record + `.stdin"
env > "` + record + `.env"
case "` + mode + `" in
sleep)
  sleep 30 &
  echo '{"type":"system","subtype":"init","session_id":"sess-sleep"}'
  wait
  ;;
budget)
  echo '{"type":"system","subtype":"init","session_id":"sess-budget"}'
  echo '{"type":"result","subtype":"error_max_budget_usd","is_error":true,"duration_ms":10,"num_turns":3,"total_cost_usd":0.05,"session_id":"sess-budget"}'
  ;;
*)
  echo '{"type":"system","subtype":"init","session_id":"sess-123","model":"opus"}'
  echo 'not json at all'
  echo '{"type":"assistant","message":{"content":[{"type":"text","text":"Reading the plan folder now"},{"type":"tool_use","name":"Read"}]},"session_id":"sess-123"}'
  echo '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write"}]},"session_id":"sess-123"}'
  echo '{"type":"result","subtype":"success","is_error":false,"duration_ms":4200,"num_turns":2,"result":"Done. ` + "```json harness-report" + `","session_id":"sess-123","total_cost_usd":0.42,"usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":7,"cache_creation_input_tokens":3}}'
  ;;
esac
`
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path, record
}

func newEngine(binary string) *Engine {
	return &Engine{Binary: binary, Getenv: func(name string) string {
		return map[string]string{"PATH": os.Getenv("PATH"), "HOME": "/home/test", "SECRET_TOKEN": "leak", "CLAUDE_CONFIG_DIR": "/cfg"}[name]
	}, EnvAllowList: DefaultEnv, TranscriptPath: func(dir, session string) (string, error) { return "/cfg/projects/x/" + session + ".jsonl", nil }}
}

func TestClaudeCodeAdapterParsesTheStreamAndKeepsTheBriefOffArgv(t *testing.T) {
	binary, record := fakeClaude(t, "success")
	worktree := t.TempDir()
	handle, err := newEngine(binary).Start(context.Background(), ports.Invocation{Brief: "the brief\n<task-data>x</task-data>", WorktreePath: worktree, Model: "opus", MaxTurns: 60, PermissionMode: "acceptEdits", AllowedTools: []string{"Read", "Bash(git diff:*)"}, DisallowedTools: []string{"Bash(git push:*)"}, SessionID: "sess-123", MaxBudgetUSD: 5})
	if err != nil {
		t.Fatal(err)
	}
	var events []ports.EngineEvent
	for event := range handle.Events() {
		events = append(events, event)
	}
	result := handle.Wait()
	if result.IsError || result.StopReason != "success" || result.Turns != 2 || result.CostUSD != 0.42 || result.Usage.CacheReadTokens != 7 || result.SessionID != "sess-123" || result.TranscriptRef != "/cfg/projects/x/sess-123.jsonl" || !strings.HasPrefix(result.FinalText, "Done.") {
		t.Fatalf("result = %#v", result)
	}
	if len(events) != 3 || events[0].Kind != ports.EngineSession || events[1].Summary != "Reading the plan folder now" || events[1].Tools[0] != "Read" || events[2].Turns != 2 {
		t.Fatalf("events = %#v", events)
	}
	args, _ := os.ReadFile(record + ".args")
	argv := string(args)
	for _, want := range []string{"-p\n", "--output-format\nstream-json\n", "--model\nopus\n", "--max-turns\n60\n", "--permission-mode\nacceptEdits\n", "--allowedTools\nRead\nBash(git diff:*)\n", "--disallowedTools\nBash(git push:*)\n", "--session-id\nsess-123\n", "--max-budget-usd\n5.00\n"} {
		if !strings.Contains(argv, want) {
			t.Fatalf("argv %q lacks %q", argv, want)
		}
	}
	if strings.Contains(argv, "the brief") {
		t.Fatal("the brief reached argv")
	}
	if stdin, _ := os.ReadFile(record + ".stdin"); string(stdin) != "the brief\n<task-data>x</task-data>" {
		t.Fatalf("stdin = %q", stdin)
	}
	env, _ := os.ReadFile(record + ".env")
	if strings.Contains(string(env), "SECRET_TOKEN") || !strings.Contains(string(env), "CLAUDE_CONFIG_DIR=/cfg") {
		t.Fatalf("env = %q", env)
	}
}

func TestClaudeCodeAdapterReportsTheBudgetStop(t *testing.T) {
	binary, _ := fakeClaude(t, "budget")
	handle, err := newEngine(binary).Start(context.Background(), ports.Invocation{WorktreePath: t.TempDir(), SessionID: "sess-budget"})
	if err != nil {
		t.Fatal(err)
	}
	result := handle.Wait()
	if !result.IsError || result.StopReason != "error_max_budget_usd" || result.CostUSD != 0.05 {
		t.Fatalf("result = %#v", result)
	}
}

func TestClaudeCodeAdapterCancelsTheProcessGroup(t *testing.T) {
	binary, _ := fakeClaude(t, "sleep")
	handle, err := newEngine(binary).Start(context.Background(), ports.Invocation{WorktreePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	<-handle.Events()
	started := time.Now()
	handle.Cancel()
	result := handle.Wait()
	if !result.IsError || result.StopReason != "cancelled" || time.Since(started) > 4*time.Second {
		t.Fatalf("result = %#v after %s", result, time.Since(started))
	}

	timed, err := newEngine(binary).Start(context.Background(), ports.Invocation{WorktreePath: t.TempDir(), Timeout: 300 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if result := timed.Wait(); result.StopReason != "timeout" {
		t.Fatalf("timeout result = %#v", result)
	}
	if _, err := newEngine(filepath.Join(t.TempDir(), "missing")).Start(context.Background(), ports.Invocation{WorktreePath: t.TempDir()}); err == nil {
		t.Fatal("a missing binary started")
	}
	quick, _ := fakeClaude(t, "success")
	if detection := newEngine(quick).Detect(context.Background()); !detection.Available || detection.Path != quick {
		t.Fatalf("detection = %#v", detection)
	}
}

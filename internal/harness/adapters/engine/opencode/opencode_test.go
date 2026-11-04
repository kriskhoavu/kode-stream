package opencode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kode-stream/internal/harness/ports"
)

func TestOpenCodeAdapterAttachesTheBriefAndSumsSteps(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "record")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + record + ".args\ncp .harness/brief.md " + record + ".brief\n" + `echo '{"type":"step_start","sessionID":"ses-1"}'
echo '{"type":"tool_use","sessionID":"ses-1","part":{"type":"tool","tool":"edit"}}'
echo '{"type":"step_finish","sessionID":"ses-1","part":{"type":"step-finish","cost":0.02,"tokens":{"input":5,"output":2,"cache":{"read":1,"write":0}}}}'
echo '{"type":"text","sessionID":"ses-1","part":{"type":"text","text":"All done."}}'
echo '{"type":"step_finish","sessionID":"ses-1","part":{"type":"step-finish","cost":0.03,"tokens":{"input":6,"output":3}}}'
`
	binary := filepath.Join(dir, "opencode")
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	worktree := t.TempDir()
	engine := &Engine{Binary: binary, Getenv: os.Getenv}
	if _, err := engine.Start(context.Background(), ports.Invocation{WorktreePath: worktree, Model: "sonnet"}); err == nil {
		t.Fatal("a model without a provider was accepted")
	}
	handle, err := engine.Start(context.Background(), ports.Invocation{Brief: "secret-free brief", WorktreePath: worktree, Model: "anthropic/claude-sonnet", PermissionMode: "acceptEdits", SessionID: "ses-1"})
	if err != nil {
		t.Fatal(err)
	}
	for range handle.Events() {
	}
	result := handle.Wait()
	if result.IsError || result.Turns != 2 || result.CostUSD != 0.05 || result.Usage.InputTokens != 11 || result.FinalText != "All done." || result.SessionID != "ses-1" {
		t.Fatalf("result = %#v", result)
	}
	args, _ := os.ReadFile(record + ".args")
	if strings.Contains(string(args), "secret-free brief") || !strings.Contains(string(args), "--file\n.harness/brief.md\n") || !strings.Contains(string(args), "--auto\n") {
		t.Fatalf("args = %q", args)
	}
	if brief, _ := os.ReadFile(record + ".brief"); string(brief) != "secret-free brief" {
		t.Fatalf("brief file = %q", brief)
	}
	if _, err := os.Stat(filepath.Join(worktree, BriefFile)); !os.IsNotExist(err) {
		t.Fatal("the brief file outlived the run")
	}
}

package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kode-stream/internal/harness/ports"
)

func fakeCodex(t *testing.T, body string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	record := filepath.Join(dir, "record")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + record + ".args\ncat > " + record + ".stdin\n" + body
	path := filepath.Join(dir, "codex")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path, record
}

func TestCodexAdapterParsesExecJSONAndMapsTheSandbox(t *testing.T) {
	binary, record := fakeCodex(t, `echo '{"type":"thread.started","thread_id":"th-1"}'
echo '{"type":"turn.started"}'
echo '{"type":"item.completed","item":{"type":"command_execution","command":"go test ./..."}}'
echo '{"type":"item.completed","item":{"type":"agent_message","text":"Done."}}'
echo '{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":3}}'
`)
	engine := &Engine{Binary: binary, Getenv: os.Getenv}
	worktree := t.TempDir()
	handle, err := engine.Start(context.Background(), ports.Invocation{Brief: "brief text", WorktreePath: worktree, Model: "gpt-5", PermissionMode: "plan"})
	if err != nil {
		t.Fatal(err)
	}
	for range handle.Events() {
	}
	result := handle.Wait()
	if result.IsError || result.StopReason != "success" || result.SessionID != "th-1" || result.Turns != 1 || result.FinalText != "Done." || result.Usage.CacheReadTokens != 4 {
		t.Fatalf("result = %#v", result)
	}
	args, _ := os.ReadFile(record + ".args")
	if !strings.Contains(string(args), "--sandbox\nread-only\n") || !strings.Contains(string(args), "--cd\n"+worktree+"\n") || !strings.HasSuffix(string(args), "-\n") || strings.Contains(string(args), "brief text") {
		t.Fatalf("args = %q", args)
	}
	if stdin, _ := os.ReadFile(record + ".stdin"); string(stdin) != "brief text" {
		t.Fatalf("stdin = %q", stdin)
	}
	if Sandbox("acceptEdits") != "workspace-write" || len(engine.Detect(context.Background()).Unsupported) == 0 {
		t.Fatal("sandbox or detection is wrong")
	}

	failing, _ := fakeCodex(t, `echo '{"type":"turn.failed","error":{"message":"quota exceeded"}}'
`)
	handle, err = (&Engine{Binary: failing, Getenv: os.Getenv}).Start(context.Background(), ports.Invocation{WorktreePath: worktree})
	if err != nil {
		t.Fatal(err)
	}
	if result := handle.Wait(); !result.IsError || !strings.Contains(result.StopReason, "quota exceeded") {
		t.Fatalf("failed result = %#v", result)
	}
}

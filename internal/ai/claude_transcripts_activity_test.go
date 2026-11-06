package ai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadClaudeTranscriptActivityKeepsThinkingAndToolCallsAndDropsToolOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	lines := `{"type":"user","sessionId":"s","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"Plan APP-562"}}
{"type":"assistant","sessionId":"s","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Read the export first."},{"type":"tool_use","name":"Read","input":{"file_path":"src/Export.java","limit":40}},{"type":"text","text":"Reading the export."}]}}
{"type":"user","sessionId":"s","timestamp":"2026-01-01T00:00:02Z","message":{"role":"user","content":[{"type":"tool_result","content":"package export;"}]}}
{"type":"assistant","sessionId":"s","timestamp":"2026-01-01T00:00:03Z","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./...","description":"Run the tests"}}]}}
`
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	turns, err := ReadClaudeTranscriptActivity(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []ClaudeTurn{
		{Role: "user", Text: "Plan APP-562"},
		{Role: "thinking", Text: "Read the export first."},
		{Role: "tool", Text: "Read · src/Export.java"},
		{Role: "assistant", Text: "Reading the export."},
		{Role: "tool", Text: "Bash · Run the tests"},
	}
	if len(turns) != len(want) {
		t.Fatalf("got %d turns, want %d: %+v", len(turns), len(want), turns)
	}
	for i := range want {
		if turns[i].Role != want[i].Role || turns[i].Text != want[i].Text {
			t.Errorf("turn %d: got %q %q, want %q %q", i, turns[i].Role, turns[i].Text, want[i].Role, want[i].Text)
		}
	}
	if tail, _ := ReadClaudeTranscriptTail(path, 10); len(tail) != 2 {
		t.Errorf("the text-only tail changed: %+v", tail)
	}
}

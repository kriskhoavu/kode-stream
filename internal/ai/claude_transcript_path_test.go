package ai

import (
	"path/filepath"
	"testing"
)

func TestClaudeTranscriptPathEncodesTheWorkingDirectory(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/cfg")
	path, err := ClaudeTranscriptPath("/work/run 1", "sess")
	if err != nil || path != filepath.Join("/cfg", "projects", "-work-run-1", "sess.jsonl") {
		t.Fatalf("path = %q err=%v", path, err)
	}
}

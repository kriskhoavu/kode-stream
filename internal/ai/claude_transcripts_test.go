package ai

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
)

const (
	conversationA = "11111111-1111-4111-8111-111111111111"
	conversationB = "22222222-2222-4222-8222-222222222222"
	conversationC = "33333333-3333-4333-8333-333333333333"
)

func transcriptTestService(t *testing.T) (*Service, models.ItemDetail, models.WorkspaceConfig, string) {
	t.Helper()
	service, item, workspace, _, _, _ := launchTestService(t, true)
	service.SetRuntimeMode(models.RuntimeModeLocal)
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	projectDir := filepath.Join(configDir, "projects", claudeProjectDirName(workspace.Path))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return service, item, workspace, projectDir
}

func writeTranscript(t *testing.T, projectDir, name, cwd string, lines ...string) {
	t.Helper()
	header := fmt.Sprintf(`{"type":"user","sessionId":%q,"cwd":%q,"gitBranch":"main","timestamp":"2026-10-01T09:00:00Z","message":{"role":"user","content":"Look at the plan"}}`, strings.TrimSuffix(name, ".jsonl"), cwd)
	content := strings.Join(append([]string{header}, lines...), "\n") + "\n"
	if err := os.WriteFile(filepath.Join(projectDir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeProjectDirNameMatchesClaudeCode(t *testing.T) {
	if got := claudeProjectDirName("/Users/me/Personal/01. happy-learning/fen"); got != "-Users-me-Personal-01--happy-learning-fen" {
		t.Fatalf("got %q", got)
	}
}

func TestClaudeTranscriptsListWorkspaceConversationsByItem(t *testing.T) {
	service, item, workspace, projectDir := transcriptTestService(t)
	writeTranscript(t, projectDir, conversationA+".jsonl", workspace.Path,
		`{"type":"assistant","timestamp":"2026-10-01T09:01:00Z","message":{"role":"assistant","content":[{"type":"text","text":"Reading plans/platform/PM-018/plan.yaml"},{"type":"tool_use","name":"Read"}]}}`,
		`{"type":"user","timestamp":"2026-10-01T09:02:00Z","message":{"role":"user","content":[{"type":"tool_result","content":"secret"}]}}`,
		`{"type":"user","isMeta":true,"timestamp":"2026-10-01T09:03:00Z","message":{"role":"user","content":"meta"}}`,
	)
	writeTranscript(t, projectDir, conversationB+".jsonl", workspace.Path, `{"type":"assistant","timestamp":"2026-10-01T10:00:00Z","message":{"role":"assistant","content":[{"type":"text","text":"PM-0180 is a different ticket"}]}}`)
	writeTranscript(t, projectDir, conversationC+".jsonl", filepath.Join(workspace.Path, "elsewhere"), `{"type":"assistant","message":{"role":"assistant","content":"PM-018"}}`)
	writeTranscript(t, projectDir, "not-a-conversation.jsonl", workspace.Path)

	transcripts, err := service.ClaudeTranscripts(workspace.ID, item.Branch)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]ClaudeTranscript{}
	for _, transcript := range transcripts {
		byID[transcript.SessionID] = transcript
	}
	if len(transcripts) != 2 {
		t.Fatalf("transcripts = %#v, want A and B only", transcripts)
	}
	a := byID[conversationA]
	if len(a.ItemIDs) != 1 || a.ItemIDs[0] != item.ID || a.Messages != 2 || a.Branch != "main" || !a.LastActiveAt.Equal(time.Date(2026, 10, 1, 9, 3, 0, 0, time.UTC)) {
		t.Fatalf("A = %#v", a)
	}
	if b := byID[conversationB]; len(b.ItemIDs) != 0 {
		t.Fatalf("PM-0180 matched PM-018: %#v", b)
	}
}

func TestClaudeTranscriptsLeaveOutConversationsKodeStreamStarted(t *testing.T) {
	service, item, workspace, projectDir := transcriptTestService(t)
	records := NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "records.yaml"))
	service.ConfigureSessionRecords(records, gitadapter.New())
	now := time.Now().UTC()
	for _, id := range []string{"embedded-" + conversationA, "embedded-" + conversationB + "-resumed"} {
		if _, err := records.Upsert(SessionRecord{ID: id, WorkspaceID: workspace.ID, Provider: "claude", Intent: "card_context", RequestedBranch: "main", State: StateExited, StartedAt: now, LastKnownAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	writeTranscript(t, projectDir, conversationA+".jsonl", workspace.Path)
	writeTranscript(t, projectDir, conversationB+".jsonl", workspace.Path)
	writeTranscript(t, projectDir, conversationC+".jsonl", workspace.Path)

	transcripts, err := service.ClaudeTranscripts(workspace.ID, item.Branch)
	if err != nil {
		t.Fatal(err)
	}
	if len(transcripts) != 1 || transcripts[0].SessionID != conversationC {
		t.Fatalf("transcripts = %#v, want only C", transcripts)
	}
}

func TestClaudeTranscriptTailReturnsLastTextTurns(t *testing.T) {
	service, _, workspace, projectDir := transcriptTestService(t)
	writeTranscript(t, projectDir, conversationA+".jsonl", workspace.Path,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"first answer"}]}}`,
		`{"type":"user","message":{"role":"user","content":"<command-name>/clear</command-name>"}}`,
		`{"type":"user","message":{"role":"user","content":"second question"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"hidden"},{"type":"text","text":"second answer"}]}}`,
	)
	turns, err := service.ClaudeTranscriptTail(workspace.ID, conversationA, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 || turns[0].Role != "user" || turns[0].Text != "second question" || turns[1].Text != "second answer" {
		t.Fatalf("turns = %#v", turns)
	}
}

func TestClaudeTranscriptTailRefusesForeignOrInvalidConversations(t *testing.T) {
	service, _, workspace, projectDir := transcriptTestService(t)
	writeTranscript(t, projectDir, conversationC+".jsonl", filepath.Join(workspace.Path, "elsewhere"))
	var launchErr *LaunchError
	if _, err := service.ClaudeTranscriptTail(workspace.ID, "../../secrets", 5); !errors.As(err, &launchErr) || launchErr.Code != "invalid_claude_session" {
		t.Fatalf("traversal err = %v", err)
	}
	if _, err := service.ClaudeTranscriptTail(workspace.ID, conversationA, 5); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	if _, err := service.ClaudeTranscriptTail(workspace.ID, conversationC, 5); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other cwd err = %v", err)
	}
}

func TestClaudeTranscriptsAreLocalOnly(t *testing.T) {
	service, item, workspace, _ := transcriptTestService(t)
	service.SetRuntimeMode(models.RuntimeModeCloud)
	if _, err := service.ClaudeTranscripts(workspace.ID, item.Branch); !errors.Is(err, ErrClaudeTranscriptsUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestResumeClaudeConversationInEmbeddedTerminal(t *testing.T) {
	service, item, workspace, projectDir := transcriptTestService(t)
	argsPath := filepath.Join(t.TempDir(), "args")
	executable := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+argsPath+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Save(Settings{DefaultProvider: "claude", DefaultTerminal: "wezterm", Providers: map[string]LaunchTemplate{"claude": {Enabled: true, Executable: executable, Args: []string{"{prompt}"}}}}); err != nil {
		t.Fatal(err)
	}
	manager := NewTerminalManager(Config{})
	t.Cleanup(func() { _ = manager.Close() })
	service.ConfigureEmbedded(manager).ConfigureSessionRecords(NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "records.yaml")), gitadapter.New())
	writeTranscript(t, projectDir, conversationA+".jsonl", workspace.Path)

	var launchErr *LaunchError
	if _, err := service.StartEmbedded(item.ID, EmbeddedInput{ResumeClaudeSessionID: conversationB, ExpectedBranch: item.Branch}); !errors.As(err, &launchErr) || launchErr.Code != "claude_session_not_found" {
		t.Fatalf("unknown conversation err = %v", err)
	}
	result, err := service.StartEmbedded(item.ID, EmbeddedInput{ResumeClaudeSessionID: conversationA, ExpectedBranch: item.Branch, IdempotencyKey: "resume-once"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Record == nil || result.Record.Intent != "resume" || result.Record.PlanRef == nil || result.Record.PlanRef.ItemID != item.ID {
		t.Fatalf("record = %#v", result.Record)
	}
	if conversation, ok := claudeConversationOf(result.Record.ID); !ok || conversation != conversationA {
		t.Fatalf("record id %q does not name conversation A", result.Record.ID)
	}
	var args []byte
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if args, err = os.ReadFile(argsPath); err == nil && len(args) > 0 {
			break
		}
	}
	if string(args) != "--resume\n"+conversationA+"\n" {
		t.Fatalf("claude args = %q", args)
	}
	transcripts, err := service.ClaudeTranscripts(workspace.ID, item.Branch)
	if err != nil || len(transcripts) != 0 {
		t.Fatalf("resumed conversation still listed as the user's: %#v err=%v", transcripts, err)
	}
}

func TestClaudeTranscriptsMatchOnlyWhatTheUserOrClaudeWrote(t *testing.T) {
	service, item, workspace, projectDir := transcriptTestService(t)
	toolOutput := `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"plans/platform/PM-018/plan.yaml\nPM-018 notes"}]},"toolUseResult":{"stdout":"PM-018"}}`
	injected := `{"type":"user","isMeta":true,"message":{"role":"user","content":"git status: plans/platform/PM-018/README.md"}}`
	writeTranscript(t, projectDir, conversationA+".jsonl", workspace.Path, toolOutput, injected)
	writeTranscript(t, projectDir, conversationB+".jsonl", workspace.Path, `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"`+workspace.Path+`/plans/platform/PM-018/implementation-plan.md"}}]}}`)
	writeTranscript(t, projectDir, conversationC+".jsonl", workspace.Path, `{"type":"user","message":{"role":"user","content":"What does PM-018 still need?"}}`)

	transcripts, err := service.ClaudeTranscripts(workspace.ID, item.Branch)
	if err != nil {
		t.Fatal(err)
	}
	matched := map[string]bool{}
	for _, transcript := range transcripts {
		matched[transcript.SessionID] = len(transcript.ItemIDs) == 1 && transcript.ItemIDs[0] == item.ID
	}
	if matched[conversationA] || !matched[conversationB] || !matched[conversationC] {
		t.Fatalf("matched = %v; want only the tool call (B) and the prompt (C), not tool output or injected context (A)", matched)
	}
}

func TestTranscriptNeedlesIgnorePlainWordIdentifiers(t *testing.T) {
	needles := transcriptNeedles([]models.ItemSummary{{ItemPath: "wiki/platform", Identifier: "platform"}, {ItemPath: "plans/api/PM-365", Identifier: "PM-365"}})
	if strings.Join(needles, ",") != "PM-365,plans/api/PM-365,wiki/platform" {
		t.Fatalf("needles = %v", needles)
	}
}

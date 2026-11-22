package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"kode-stream/internal/common/models"
)

// Claude Code keeps each conversation as a JSONL transcript under
// <config dir>/projects/<encoded working directory>/<session id>.jsonl. These
// readers find the conversations held in a workspace's root, so a card can show
// the ones that worked on it and resume one in an embedded terminal. They read
// on request and store nothing: a transcript holds prompts and output, which
// never enter a session record.

const (
	claudeTranscriptWindow   = 14 * 24 * time.Hour
	claudeTranscriptMaxFiles = 50
	claudeTranscriptMaxLine  = 16 << 20
	claudeTurnMaxText        = 4000
	ClaudeTranscriptMaxTail  = 50
)

var ErrClaudeTranscriptsUnavailable = errors.New("claude transcripts are read in Local mode only")

// ClaudeTranscript is a conversation's safe metadata: no prompt or output.
type ClaudeTranscript struct {
	SessionID    string    `json:"sessionId"`
	Branch       string    `json:"branch,omitempty"`
	StartedAt    time.Time `json:"startedAt"`
	LastActiveAt time.Time `json:"lastActiveAt"`
	Messages     int       `json:"messages"`
	ItemIDs      []string  `json:"itemIds"`
}

// ClaudeTurn is one user or assistant text turn of a transcript tail.
type ClaudeTurn struct {
	Role string    `json:"role"`
	Text string    `json:"text"`
	At   time.Time `json:"at,omitempty"`
}

type claudeTranscriptLine struct {
	Type        string    `json:"type"`
	SessionID   string    `json:"sessionId"`
	Cwd         string    `json:"cwd"`
	GitBranch   string    `json:"gitBranch"`
	Timestamp   time.Time `json:"timestamp"`
	IsSidechain bool      `json:"isSidechain"`
	IsMeta      bool      `json:"isMeta"`
	Message     *struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type claudeTranscriptScan struct {
	size     int64
	modified time.Time
	summary  ClaudeTranscript
	cwd      string
	needles  string
	mentions []string
}

func claudeConfigDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// claudeProjectDirName encodes a working directory the way Claude Code names
// its project directory: every rune that is not an ASCII letter or digit is '-'.
func claudeProjectDirName(path string) string {
	var name strings.Builder
	for _, r := range path {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			name.WriteRune(r)
			continue
		}
		name.WriteByte('-')
	}
	return name.String()
}

// ClaudeTranscriptPath is where Claude Code writes the transcript of the
// session sessionID started in workingDir. Callers store this path, never
// the transcript.
func ClaudeTranscriptPath(workingDir, sessionID string) (string, error) {
	configDir, err := claudeConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "projects", claudeProjectDirName(workingDir), sessionID+".jsonl"), nil
}

func (s *Service) claudeProject(workspaceID string) (models.WorkspaceConfig, string, error) {
	if s.runtimeMode != models.RuntimeModeLocal {
		return models.WorkspaceConfig{}, "", ErrClaudeTranscriptsUnavailable
	}
	if s.launch == nil || s.launch.registry == nil {
		return models.WorkspaceConfig{}, "", ErrClaudeTranscriptsUnavailable
	}
	workspace, found, err := s.launch.registry.Get(strings.TrimSpace(workspaceID))
	if err != nil {
		return models.WorkspaceConfig{}, "", err
	}
	if !found {
		return models.WorkspaceConfig{}, "", ErrNotFound
	}
	configDir, err := claudeConfigDir()
	if err != nil {
		return models.WorkspaceConfig{}, "", err
	}
	return workspace, filepath.Join(configDir, "projects", claudeProjectDirName(workspace.Path)), nil
}

// ClaudeTranscripts lists the workspace's recent Claude conversations, newest
// first, each with the items on branch whose path or identifier it mentions.
// Conversations kode-stream started itself are left out: their session record
// already represents them.
func (s *Service) ClaudeTranscripts(workspaceID, branch string) ([]ClaudeTranscript, error) {
	workspace, projectDir, err := s.claudeProject(workspaceID)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(projectDir)
	if errors.Is(err, os.ErrNotExist) {
		return []ClaudeTranscript{}, nil
	}
	if err != nil {
		return nil, err
	}
	type candidate struct {
		path     string
		info     os.FileInfo
		modified time.Time
	}
	cutoff := time.Now().Add(-claudeTranscriptWindow)
	candidates := []candidate{}
	for _, entry := range entries {
		id, ok := strings.CutSuffix(entry.Name(), ".jsonl")
		if !ok || entry.IsDir() || uuid.Validate(id) != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().Before(cutoff) {
			continue
		}
		candidates = append(candidates, candidate{path: filepath.Join(projectDir, entry.Name()), info: info, modified: info.ModTime()})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].modified.After(candidates[j].modified) })
	if len(candidates) > claudeTranscriptMaxFiles {
		candidates = candidates[:claudeTranscriptMaxFiles]
	}
	known, err := s.claudeConversationsWithRecords(workspace.ID)
	if err != nil {
		return nil, err
	}
	items := []models.ItemSummary{}
	if s.launch.index != nil {
		if items, err = s.launch.index.BranchItems(workspace.ID, strings.TrimSpace(branch)); err != nil {
			return nil, err
		}
	}
	needles := transcriptNeedles(items)
	transcripts := []ClaudeTranscript{}
	for _, candidate := range candidates {
		scan, err := transcriptCache.scan(candidate.path, candidate.info, needles)
		if err != nil || !sameDirectory(scan.cwd, workspace.Path) || known[scan.summary.SessionID] || scan.summary.Messages == 0 {
			continue
		}
		summary := scan.summary
		summary.ItemIDs = matchedItemIDs(items, scan.mentions)
		transcripts = append(transcripts, summary)
	}
	return transcripts, nil
}

// ClaudeTranscriptTail returns the last limit user and assistant text turns of
// one of the workspace's conversations, read now and kept nowhere.
func (s *Service) ClaudeTranscriptTail(workspaceID, sessionID string, limit int) ([]ClaudeTurn, error) {
	path, err := s.claudeTranscriptPath(workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	return ReadClaudeTranscriptTail(path, limit)
}

// ReadClaudeTranscriptTail returns the last limit user and assistant text
// turns of the transcript at path, read now and kept nowhere. A file that
// does not exist yet reads as no turns, since Claude Code creates it on its
// first turn.
func ReadClaudeTranscriptTail(path string, limit int) ([]ClaudeTurn, error) {
	if limit <= 0 || limit > ClaudeTranscriptMaxTail {
		limit = ClaudeTranscriptMaxTail
	}
	turns := []ClaudeTurn{}
	err := readTranscriptLines(path, func(line claudeTranscriptLine, _ []byte) {
		if turn, ok := transcriptTurn(line); ok {
			turns = append(turns, turn)
			if len(turns) > limit {
				turns = turns[1:]
			}
		}
	})
	if errors.Is(err, os.ErrNotExist) {
		return turns, nil
	}
	if err != nil {
		return nil, err
	}
	return turns, nil
}

// claudeTranscriptPath resolves a conversation of the workspace, refusing an ID
// that is not a UUID or a transcript held in another directory.
func (s *Service) claudeTranscriptPath(workspaceID, sessionID string) (string, error) {
	workspace, projectDir, err := s.claudeProject(workspaceID)
	if err != nil {
		return "", err
	}
	sessionID = strings.TrimSpace(sessionID)
	if uuid.Validate(sessionID) != nil {
		return "", launchError("invalid_claude_session", "Claude session ID must be a UUID")
	}
	path := filepath.Join(projectDir, sessionID+".jsonl")
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	scan, err := transcriptCache.scan(path, info, nil)
	if err != nil {
		return "", err
	}
	if !sameDirectory(scan.cwd, workspace.Path) {
		return "", ErrNotFound
	}
	return path, nil
}

// claudeConversationsWithRecords names the conversations kode-stream launched
// or resumed: a Claude session record is embedded-<conversation>[-<suffix>].
func (s *Service) claudeConversationsWithRecords(workspaceID string) (map[string]bool, error) {
	known := map[string]bool{}
	if s.records == nil {
		return known, nil
	}
	records, err := s.records.List(workspaceID, "")
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if conversation, ok := claudeConversationOf(record.ID); ok {
			known[conversation] = true
		}
	}
	return known, nil
}

func claudeConversationOf(recordID string) (string, bool) {
	rest, ok := strings.CutPrefix(recordID, "embedded-")
	if !ok || len(rest) < 36 {
		return "", false
	}
	conversation := rest[:36]
	if uuid.Validate(conversation) != nil || (len(rest) > 36 && rest[36] != '-') {
		return "", false
	}
	return conversation, true
}

func transcriptTurn(line claudeTranscriptLine) (ClaudeTurn, bool) {
	if (line.Type != "user" && line.Type != "assistant") || line.IsSidechain || line.IsMeta || line.Message == nil {
		return ClaudeTurn{}, false
	}
	text := strings.TrimSpace(transcriptText(line.Message.Content))
	if text == "" || strings.HasPrefix(text, "<") {
		return ClaudeTurn{}, false
	}
	if runes := []rune(text); len(runes) > claudeTurnMaxText {
		text = string(runes[:claudeTurnMaxText]) + "…"
	}
	return ClaudeTurn{Role: line.Type, Text: text, At: line.Timestamp}, true
}

func transcriptText(content json.RawMessage) string {
	var plain string
	if json.Unmarshal(content, &plain) == nil {
		return plain
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &blocks) != nil {
		return ""
	}
	parts := []string{}
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// authoredText is what the user or Claude wrote in a line: the user's prompts,
// Claude's replies, and the inputs of the tools Claude called, such as a file
// it read or edited. Tool output, injected context, and metadata are left out:
// a search result or git status that happens to list a plan's files says
// nothing about whether the conversation worked on it.
func authoredText(line claudeTranscriptLine) []byte {
	if (line.Type != "user" && line.Type != "assistant") || line.IsMeta || line.Message == nil {
		return nil
	}
	var plain string
	if json.Unmarshal(line.Message.Content, &plain) == nil {
		if line.Type == "user" {
			return []byte(plain)
		}
		return nil
	}
	var blocks []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		Input json.RawMessage `json:"input"`
	}
	if json.Unmarshal(line.Message.Content, &blocks) != nil {
		return nil
	}
	var text bytes.Buffer
	for _, block := range blocks {
		switch {
		case block.Type == "text":
			text.WriteString(block.Text)
		case block.Type == "tool_use" && line.Type == "assistant":
			text.Write(block.Input)
		default:
			continue
		}
		text.WriteByte('\n')
	}
	return text.Bytes()
}

func sameDirectory(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	resolvedA, errA := filepath.EvalSymlinks(a)
	resolvedB, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && resolvedA == resolvedB
}

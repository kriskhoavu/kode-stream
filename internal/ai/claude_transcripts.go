package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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

// claudeTranscriptCache keeps each file's scan until the file changes, so a
// board polling every few seconds re-reads only the conversations still moving.
type claudeTranscriptCache struct {
	mu    sync.Mutex
	scans map[string]claudeTranscriptScan
}

var transcriptCache = &claudeTranscriptCache{scans: map[string]claudeTranscriptScan{}}

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

func (c *claudeTranscriptCache) scan(path string, info os.FileInfo, needles []string) (claudeTranscriptScan, error) {
	c.mu.Lock()
	cached, ok := c.scans[path]
	c.mu.Unlock()
	if ok && cached.size == info.Size() && cached.modified.Equal(info.ModTime()) && (needles == nil || cached.needles == needlesKey(needles)) {
		return cached, nil
	}
	scan := claudeTranscriptScan{size: info.Size(), modified: info.ModTime(), needles: needlesKey(needles), summary: ClaudeTranscript{SessionID: strings.TrimSuffix(filepath.Base(path), ".jsonl"), ItemIDs: []string{}}}
	pending := map[string]bool{}
	for _, needle := range needles {
		pending[needle] = true
	}
	err := readTranscriptLines(path, func(line claudeTranscriptLine, _ []byte) {
		if scan.cwd == "" && line.Cwd != "" {
			scan.cwd = line.Cwd
		}
		if line.GitBranch != "" {
			scan.summary.Branch = line.GitBranch
		}
		if !line.Timestamp.IsZero() {
			if scan.summary.StartedAt.IsZero() {
				scan.summary.StartedAt = line.Timestamp
			}
			scan.summary.LastActiveAt = line.Timestamp
		}
		if _, ok := transcriptTurn(line); ok {
			scan.summary.Messages++
		}
		authored := authoredText(line)
		for needle := range pending {
			if len(authored) > 0 && mentions(authored, needle) {
				scan.mentions = append(scan.mentions, needle)
				delete(pending, needle)
			}
		}
	})
	if err != nil {
		return claudeTranscriptScan{}, err
	}
	if scan.summary.LastActiveAt.IsZero() {
		scan.summary.LastActiveAt = info.ModTime()
	}
	if needles != nil {
		c.mu.Lock()
		c.scans[path] = scan
		c.mu.Unlock()
	}
	return scan, nil
}

// needlesKey records which items a cached scan searched for, so a changed item
// list rescans the file.
func needlesKey(needles []string) string { return "\x00" + strings.Join(needles, "\x00") }

func readTranscriptLines(path string, visit func(claudeTranscriptLine, []byte)) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64<<10), claudeTranscriptMaxLine)
	for scanner.Scan() {
		raw := scanner.Bytes()
		var line claudeTranscriptLine
		if json.Unmarshal(raw, &line) != nil {
			continue
		}
		visit(line, raw)
	}
	if errors.Is(scanner.Err(), bufio.ErrTooLong) {
		return nil
	}
	return scanner.Err()
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

// transcriptNeedles are the strings that tie a conversation to an item: its
// repository path, and its identifier when that is ticket-like. A plain-word
// identifier such as a wiki section's "platform" would match any conversation.
func transcriptNeedles(items []models.ItemSummary) []string {
	seen := map[string]bool{}
	needles := []string{}
	for _, item := range items {
		identifier := strings.TrimSpace(item.Identifier)
		if !strings.ContainsAny(identifier, "0123456789") {
			identifier = ""
		}
		for _, needle := range []string{strings.Trim(item.ItemPath, "/"), identifier} {
			if len(needle) >= 3 && !seen[needle] {
				seen[needle] = true
				needles = append(needles, needle)
			}
		}
	}
	sort.Strings(needles)
	return needles
}

func matchedItemIDs(items []models.ItemSummary, found []string) []string {
	mentioned := map[string]bool{}
	for _, needle := range found {
		mentioned[needle] = true
	}
	ids := []string{}
	for _, item := range items {
		if mentioned[strings.Trim(item.ItemPath, "/")] || mentioned[strings.TrimSpace(item.Identifier)] {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

// mentions finds needle as a whole token: PM-04 does not match PM-042.
func mentions(raw []byte, needle string) bool {
	for offset := 0; ; {
		index := bytes.Index(raw[offset:], []byte(needle))
		if index < 0 {
			return false
		}
		start, end := offset+index, offset+index+len(needle)
		if (start == 0 || !isTokenByte(raw[start-1])) && (end == len(raw) || !isTokenByte(raw[end])) {
			return true
		}
		offset = start + 1
	}
}

func isTokenByte(b byte) bool {
	return b == '-' || b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
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

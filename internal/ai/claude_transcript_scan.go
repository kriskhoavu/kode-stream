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

	"kode-stream/internal/common/models"
)

// A file's scan is kept until the file changes, so board polling re-reads only active conversations.
type claudeTranscriptCache struct {
	mu    sync.Mutex
	scans map[string]claudeTranscriptScan
}

var transcriptCache = &claudeTranscriptCache{scans: map[string]claudeTranscriptScan{}}

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

// Only ticket-like identifiers count: a plain word such as "platform" would match any conversation.
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

// Whole tokens only: PM-04 must not match PM-042.
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

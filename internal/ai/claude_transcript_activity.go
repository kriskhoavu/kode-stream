package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
)

const claudeActivityMaxText = 600

// Tool output stays out: it says nothing about what the model decided.
func ReadClaudeTranscriptActivity(path string, limit int) ([]ClaudeTurn, error) {
	if limit <= 0 || limit > ClaudeTranscriptMaxTail {
		limit = ClaudeTranscriptMaxTail
	}
	turns := []ClaudeTurn{}
	err := readTranscriptLines(path, func(line claudeTranscriptLine, _ []byte) {
		for _, turn := range transcriptActivity(line) {
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

func transcriptActivity(line claudeTranscriptLine) []ClaudeTurn {
	if (line.Type != "user" && line.Type != "assistant") || line.IsSidechain || line.IsMeta || line.Message == nil {
		return nil
	}
	var plain string
	if json.Unmarshal(line.Message.Content, &plain) == nil {
		if turn, ok := transcriptTurn(line); ok {
			return []ClaudeTurn{turn}
		}
		return nil
	}
	var blocks []struct {
		Type     string          `json:"type"`
		Text     string          `json:"text"`
		Thinking string          `json:"thinking"`
		Name     string          `json:"name"`
		Input    json.RawMessage `json:"input"`
	}
	if json.Unmarshal(line.Message.Content, &blocks) != nil {
		return nil
	}
	turns := []ClaudeTurn{}
	add := func(role, text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		if runes := []rune(text); len(runes) > claudeTurnMaxText {
			text = string(runes[:claudeTurnMaxText]) + "…"
		}
		turns = append(turns, ClaudeTurn{Role: role, Text: text, At: line.Timestamp})
	}
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if line.Type == "user" && strings.HasPrefix(strings.TrimSpace(block.Text), "<") {
				continue
			}
			add(line.Type, block.Text)
		case "thinking":
			add("thinking", block.Thinking)
		case "tool_use":
			add("tool", toolCallText(block.Name, block.Input))
		}
	}
	return turns
}

func toolCallText(name string, input json.RawMessage) string {
	var fields map[string]json.RawMessage
	if json.Unmarshal(input, &fields) != nil || len(fields) == 0 {
		return name
	}
	for _, key := range []string{"description", "command", "file_path", "path", "pattern", "query", "url", "prompt", "skill"} {
		var value string
		if raw, ok := fields[key]; ok && json.Unmarshal(raw, &value) == nil && strings.TrimSpace(value) != "" {
			return name + " · " + clipText(value, claudeActivityMaxText)
		}
	}
	compact := bytes.Buffer{}
	if json.Compact(&compact, input) != nil {
		return name
	}
	return name + " · " + clipText(compact.String(), claudeActivityMaxText)
}

func clipText(text string, max int) string {
	text = strings.TrimSpace(text)
	if runes := []rune(text); len(runes) > max {
		return string(runes[:max]) + "…"
	}
	return text
}

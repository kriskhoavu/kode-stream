package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
)

const claudeActivityMaxText = 600

// ReadClaudeTranscriptActivity is ReadClaudeTranscriptTail with the model's
// work between its words: each tool call as a "tool" turn naming the tool and
// what it was given, and each thinking block as a "thinking" turn. Tool output
// stays out, since a page or command result says nothing about what the model
// decided. The harness Live tab reads this so a person can follow an agent that
// has no terminal.
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

// transcriptActivity returns a line's turns in order: thinking, text and tool
// calls as the model wrote them, or the person's text.
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

// toolCallText is one line for a tool call: the tool, then the argument a
// person would ask about first (a path, a command, a pattern), else the input.
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

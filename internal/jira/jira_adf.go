package jira

import (
	"encoding/json"
	"strconv"
	"strings"
)

// adfNodeText returns the readable text an ADF leaf node contributes, if any.
func adfNodeText(node map[string]any) (string, bool) {
	attrString := func(names ...string) (string, bool) {
		attrs, ok := node["attrs"].(map[string]any)
		if !ok {
			return "", false
		}
		for _, name := range names {
			if value, ok := attrs[name].(string); ok && value != "" {
				return value, true
			}
		}
		return "", false
	}
	switch node["type"] {
	case "text":
		value, ok := node["text"].(string)
		return value, ok
	case "hardBreak":
		return "\n", true
	case "mention":
		return attrString("text", "displayName")
	case "emoji":
		return attrString("shortName", "text")
	case "inlineCard", "blockCard", "embedCard":
		return attrString("url")
	}
	return "", false
}

// adfBudget accumulates converted Markdown while enforcing the description
// byte ceiling, clipping on rune boundaries so output stays valid UTF-8.
type adfBudget struct {
	used      int
	truncated bool
}

func (b *adfBudget) take(text string) string {
	if text == "" {
		return ""
	}
	remaining := maxDescriptionBytes - b.used
	if remaining <= 0 {
		b.truncated = true
		return ""
	}
	part, clipped := boundedString(text, remaining)
	b.used += len(part)
	if clipped {
		b.truncated = true
	}
	return part
}

// adfInline converts a run of inline nodes, applying the marks Jira records
// alongside each text node.
func adfInline(nodes []any, depth int, budget *adfBudget) string {
	var out strings.Builder
	for _, raw := range nodes {
		node, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if depth > maxADFDepth {
			budget.truncated = true
			continue
		}
		if content, ok := node["content"].([]any); ok && node["type"] != "text" {
			out.WriteString(adfInline(content, depth+1, budget))
			continue
		}
		text, ok := adfNodeText(node)
		if !ok {
			continue
		}
		out.WriteString(adfApplyMarks(budget.take(text), node))
	}
	return out.String()
}

func adfApplyMarks(text string, node map[string]any) string {
	marks, ok := node["marks"].([]any)
	if !ok || text == "" || strings.TrimSpace(text) == "" {
		return text
	}
	for _, raw := range marks {
		mark, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch mark["type"] {
		case "strong":
			text = "**" + text + "**"
		case "em":
			text = "_" + text + "_"
		case "code":
			text = "`" + text + "`"
		case "strike":
			text = "~~" + text + "~~"
		case "link":
			if attrs, ok := mark["attrs"].(map[string]any); ok {
				if href, ok := attrs["href"].(string); ok && href != "" {
					text = "[" + text + "](" + href + ")"
				}
			}
		}
	}
	return text
}

// adfBlocks converts block-level nodes into Markdown blocks. Keeping blocks
// separate is what preserves headings and lists: the previous flat walk ran
// every text node together, gluing a heading onto the paragraph before it.
func adfBlocks(nodes []any, depth int, budget *adfBudget) []string {
	var blocks []string
	for _, raw := range nodes {
		node, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		blocks = append(blocks, adfBlock(node, depth, budget)...)
	}
	return blocks
}

func adfBlock(node map[string]any, depth int, budget *adfBudget) []string {
	if depth > maxADFDepth {
		budget.truncated = true
		return nil
	}
	content, _ := node["content"].([]any)
	switch node["type"] {
	case "doc":
		return adfBlocks(content, depth+1, budget)
	case "paragraph":
		if text := strings.TrimRight(adfInline(content, depth+1, budget), " \t"); text != "" {
			return []string{text}
		}
	case "heading":
		level := 3
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if raw, ok := attrs["level"].(float64); ok && raw >= 1 && raw <= 6 {
				level = int(raw)
			}
		}
		if text := adfInline(content, depth+1, budget); text != "" {
			return []string{strings.Repeat("#", level) + " " + text}
		}
	case "bulletList", "orderedList":
		ordered := node["type"] == "orderedList"
		var lines []string
		for index, raw := range content {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			itemContent, _ := item["content"].([]any)
			marker := "- "
			if ordered {
				marker = strconv.Itoa(index+1) + ". "
			}
			inner := adfBlocks(itemContent, depth+1, budget)
			for innerIndex, line := range inner {
				if innerIndex == 0 {
					lines = append(lines, marker+line)
					continue
				}
				lines = append(lines, adfIndent(line, strings.Repeat(" ", len(marker))))
			}
		}
		if len(lines) > 0 {
			return []string{strings.Join(lines, "\n")}
		}
	case "codeBlock":
		language := ""
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if value, ok := attrs["language"].(string); ok {
				language = value
			}
		}
		if text := adfInline(content, depth+1, budget); text != "" {
			return []string{"```" + language + "\n" + text + "\n```"}
		}
	case "blockquote":
		inner := adfBlocks(content, depth+1, budget)
		if len(inner) > 0 {
			return []string{adfIndent(strings.Join(inner, "\n\n"), "> ")}
		}
	case "rule":
		return []string{"---"}
	case "table":
		if rows := adfTable(content, depth+1, budget); rows != "" {
			return []string{rows}
		}
	case "mediaSingle", "mediaGroup", "media":
		return nil
	default:
		if len(content) > 0 {
			return adfBlocks(content, depth+1, budget)
		}
		if text := strings.TrimSpace(adfInline([]any{node}, depth+1, budget)); text != "" {
			return []string{text}
		}
	}
	return nil
}

func adfIndent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func adfTable(rows []any, depth int, budget *adfBudget) string {
	var lines []string
	for rowIndex, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok || row["type"] != "tableRow" {
			continue
		}
		cells, _ := row["content"].([]any)
		var values []string
		for _, rawCell := range cells {
			cell, ok := rawCell.(map[string]any)
			if !ok {
				continue
			}
			cellContent, _ := cell["content"].([]any)
			text := strings.Join(adfBlocks(cellContent, depth+1, budget), " ")
			values = append(values, strings.ReplaceAll(strings.TrimSpace(text), "|", "\\|"))
		}
		if len(values) == 0 {
			continue
		}
		lines = append(lines, "| "+strings.Join(values, " | ")+" |")
		if rowIndex == 0 {
			separator := make([]string, len(values))
			for i := range separator {
				separator[i] = "---"
			}
			lines = append(lines, "| "+strings.Join(separator, " | ")+" |")
		}
	}
	return strings.Join(lines, "\n")
}

// normalizeDescription converts a Jira description into Markdown. Cloud sends
// ADF; Server sends plain text, which is passed through unchanged.
func normalizeDescription(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return boundedString(text, maxDescriptionBytes)
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	budget := &adfBudget{}
	var blocks []string
	switch root := value.(type) {
	case map[string]any:
		blocks = adfBlock(root, 0, budget)
	case []any:
		blocks = adfBlocks(root, 0, budget)
	}
	return strings.TrimSpace(strings.Join(blocks, "\n\n")), budget.truncated
}

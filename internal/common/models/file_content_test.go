package models

// Package models contains compatibility contracts pending domain ownership.

import (
	"encoding/json"
	"testing"
)

func TestFileContentJSONKeepsExistingFieldsAndAddsViewerMetadata(t *testing.T) {
	content := FileContent{
		ID:        "README_md",
		Path:      "README.md",
		Content:   "# Plan",
		Language:  "markdown",
		Hash:      "abc123",
		Kind:      FileKindMarkdown,
		SizeBytes: 6,
		Editable:  true,
	}

	data, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"id", "path", "content", "language", "hash", "kind", "sizeBytes", "editable"} {
		if _, ok := payload[field]; !ok {
			t.Fatalf("response is missing %q", field)
		}
	}
	if _, ok := payload["truncated"]; ok {
		t.Fatal("false truncated flag should be omitted")
	}
}

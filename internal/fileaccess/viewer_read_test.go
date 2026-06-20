package fileaccess

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"plan-manager/internal/models"
)

func TestReadFileContentReturnsViewerMetadata(t *testing.T) {
	tests := []struct {
		name     string
		kind     models.FileKind
		language string
		editable bool
	}{
		{"README.md", models.FileKindMarkdown, "markdown", true},
		{"page.html", models.FileKindHTML, "html", false},
		{"data.json", models.FileKindJSON, "json", false},
		{"plan.yaml", models.FileKindYAML, "yaml", false},
		{"main.go", models.FileKindCode, "go", false},
		{"notes.txt", models.FileKindText, "text", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), test.name)
			if err := os.WriteFile(path, []byte("content\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			content, err := readFileContent(test.name, path)
			if err != nil {
				t.Fatal(err)
			}
			if content.Kind != test.kind || content.Language != test.language || content.Editable != test.editable {
				t.Fatalf("metadata = kind:%q language:%q editable:%v", content.Kind, content.Language, content.Editable)
			}
			if content.SizeBytes != int64(len("content\n")) || content.Truncated || content.Hash == "" {
				t.Fatalf("unexpected file metadata: %+v", content)
			}
		})
	}
}

func TestReadFileContentRejectsBinaryAndInvalidUTF8(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
	}{
		{"binary.bin", []byte{'P', 'N', 'G', 0, 1}},
		{"invalid.txt", []byte{0xff, 0xfe}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), test.name)
			if err := os.WriteFile(path, test.data, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := readFileContent(test.name, path); !errors.Is(err, ErrUnsupportedContent) {
				t.Fatalf("error = %v, want unsupported content", err)
			}
		})
	}
}

func TestReadFileContentTruncatesLargeTextOnUTF8Boundary(t *testing.T) {
	prefix := strings.Repeat("a", int(MaxTextResponseBytes-1))
	data := []byte(prefix + "€")
	path := filepath.Join(t.TempDir(), "large.md")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	content, err := readFileContent("large.md", path)
	if err != nil {
		t.Fatal(err)
	}
	if !content.Truncated || content.Editable {
		t.Fatalf("large Markdown metadata = %+v", content)
	}
	if content.SizeBytes != int64(len(data)) {
		t.Fatalf("size = %d, want %d", content.SizeBytes, len(data))
	}
	if len(content.Content) > int(MaxTextResponseBytes) || !utf8.ValidString(content.Content) {
		t.Fatal("truncated content must stay within the limit and contain valid UTF-8")
	}
	if content.Hash != contentHash(data) {
		t.Fatal("truncated response must retain the full-file hash")
	}
}

func TestReadRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	itemRoot := filepath.Join(root, "items", "platform", "PM-006")
	if err := os.MkdirAll(itemRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(itemRoot, "escape.txt")); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	access := New()
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"items"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ItemPath: "items/platform/PM-006"}}
	if _, err := access.Read(workspace, item, "escape_txt"); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

package fileaccess

import (
	"strings"
	"testing"
)

func TestClassifyPath(t *testing.T) {
	tests := []struct {
		path     string
		kind     FileKind
		language string
	}{
		{"README.md", FileKindMarkdown, "markdown"},
		{"page.HTML", FileKindHTML, "html"},
		{"data.json", FileKindJSON, "json"},
		{"plan.yml", FileKindYAML, "yaml"},
		{"src/main.go", FileKindCode, "go"},
		{"src/App.tsx", FileKindCode, "tsx"},
		{"Dockerfile", FileKindCode, "dockerfile"},
		{"Makefile", FileKindCode, "makefile"},
		{"notes.txt", FileKindText, "text"},
		{"LICENSE", FileKindText, "text"},
		{"unknown.custom", FileKindText, "text"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			classification := ClassifyPath(test.path)
			if classification.Kind != test.kind {
				t.Fatalf("kind = %q, want %q", classification.Kind, test.kind)
			}
			if classification.Language != test.language {
				t.Fatalf("language = %q, want %q", classification.Language, test.language)
			}
		})
	}
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", nil, false},
		{"utf8 text", []byte("hello\nworld"), false},
		{"unicode text", []byte("xin chao"), false},
		{"nul byte", []byte{'a', 0, 'b'}, true},
		{"invalid utf8", []byte{0xff, 0xfe}, true},
		{"nul after sample", append([]byte(strings.Repeat("a", binarySampleBytes)), 0), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isBinary(test.data); got != test.want {
				t.Fatalf("isBinary() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPreviewThresholdsRemainOrdered(t *testing.T) {
	if RichPreviewThresholdBytes <= 0 {
		t.Fatal("rich preview threshold must be positive")
	}
	if MaxTextResponseBytes <= RichPreviewThresholdBytes {
		t.Fatal("maximum text response must exceed rich preview threshold")
	}
}

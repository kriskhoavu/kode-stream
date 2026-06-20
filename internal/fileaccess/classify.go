package fileaccess

import (
	"bytes"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"plan-manager/internal/models"
)

type FileKind = models.FileKind

const (
	FileKindMarkdown    = models.FileKindMarkdown
	FileKindHTML        = models.FileKindHTML
	FileKindJSON        = models.FileKindJSON
	FileKindYAML        = models.FileKindYAML
	FileKindCode        = models.FileKindCode
	FileKindText        = models.FileKindText
	FileKindUnsupported = models.FileKindUnsupported

	RichPreviewThresholdBytes int64 = 1 << 20
	MaxTextResponseBytes      int64 = 2 << 20
	binarySampleBytes               = 8 << 10
)

type Classification struct {
	Kind     FileKind
	Language string
}

var extensionClassifications = map[string]Classification{
	".c":          {FileKindCode, "c"},
	".cc":         {FileKindCode, "cpp"},
	".conf":       {FileKindText, "text"},
	".cpp":        {FileKindCode, "cpp"},
	".cs":         {FileKindCode, "csharp"},
	".css":        {FileKindCode, "css"},
	".go":         {FileKindCode, "go"},
	".h":          {FileKindCode, "c"},
	".hpp":        {FileKindCode, "cpp"},
	".htm":        {FileKindHTML, "html"},
	".html":       {FileKindHTML, "html"},
	".java":       {FileKindCode, "java"},
	".js":         {FileKindCode, "javascript"},
	".json":       {FileKindJSON, "json"},
	".jsx":        {FileKindCode, "jsx"},
	".kt":         {FileKindCode, "kotlin"},
	".kts":        {FileKindCode, "kotlin"},
	".log":        {FileKindText, "text"},
	".md":         {FileKindMarkdown, "markdown"},
	".markdown":   {FileKindMarkdown, "markdown"},
	".properties": {FileKindText, "properties"},
	".py":         {FileKindCode, "python"},
	".rb":         {FileKindCode, "ruby"},
	".rs":         {FileKindCode, "rust"},
	".sh":         {FileKindCode, "shell"},
	".sql":        {FileKindCode, "sql"},
	".toml":       {FileKindCode, "toml"},
	".ts":         {FileKindCode, "typescript"},
	".tsx":        {FileKindCode, "tsx"},
	".txt":        {FileKindText, "text"},
	".xml":        {FileKindCode, "xml"},
	".yaml":       {FileKindYAML, "yaml"},
	".yml":        {FileKindYAML, "yaml"},
}

var specialFileClassifications = map[string]Classification{
	"dockerfile": {FileKindCode, "dockerfile"},
	"makefile":   {FileKindCode, "makefile"},
}

func ClassifyPath(path string) Classification {
	name := strings.ToLower(filepath.Base(path))
	if classification, ok := specialFileClassifications[name]; ok {
		return classification
	}
	if classification, ok := extensionClassifications[strings.ToLower(filepath.Ext(name))]; ok {
		return classification
	}
	return Classification{Kind: FileKindText, Language: "text"}
}

func language(path string) string {
	return ClassifyPath(path).Language
}

func isBinary(data []byte) bool {
	if len(data) > binarySampleBytes {
		data = data[:binarySampleBytes]
	}
	return bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data)
}

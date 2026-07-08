package fileaccess

// Package fileaccess provides bounded content access.

import (
	"bytes"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"kode-stream/internal/common/models"
)

type FileKind = models.FileKind

const (
	FileKindMarkdown    = models.FileKindMarkdown
	FileKindHTML        = models.FileKindHTML
	FileKindJSON        = models.FileKindJSON
	FileKindYAML        = models.FileKindYAML
	FileKindCode        = models.FileKindCode
	FileKindText        = models.FileKindText
	FileKindImage       = models.FileKindImage
	FileKindUnsupported = models.FileKindUnsupported

	RichPreviewThresholdBytes int64 = 1 << 20
	MaxTextResponseBytes      int64 = 2 << 20
	MaxImageResponseBytes     int64 = 8 << 20
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
	".gif":        {FileKindImage, "image/gif"},
	".h":          {FileKindCode, "c"},
	".hpp":        {FileKindCode, "cpp"},
	".htm":        {FileKindHTML, "html"},
	".html":       {FileKindHTML, "html"},
	".java":       {FileKindCode, "java"},
	".js":         {FileKindCode, "javascript"},
	".json":       {FileKindJSON, "json"},
	".jpeg":       {FileKindImage, "image/jpeg"},
	".jpg":        {FileKindImage, "image/jpeg"},
	".jsx":        {FileKindCode, "jsx"},
	".kt":         {FileKindCode, "kotlin"},
	".kts":        {FileKindCode, "kotlin"},
	".log":        {FileKindText, "text"},
	".md":         {FileKindMarkdown, "markdown"},
	".markdown":   {FileKindMarkdown, "markdown"},
	".properties": {FileKindText, "properties"},
	".png":        {FileKindImage, "image/png"},
	".py":         {FileKindCode, "python"},
	".rb":         {FileKindCode, "ruby"},
	".rs":         {FileKindCode, "rust"},
	".sh":         {FileKindCode, "shell"},
	".sql":        {FileKindCode, "sql"},
	".toml":       {FileKindCode, "toml"},
	".ts":         {FileKindCode, "typescript"},
	".tsx":        {FileKindCode, "tsx"},
	".txt":        {FileKindText, "text"},
	".webp":       {FileKindImage, "image/webp"},
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

func IsBinary(data []byte) bool {
	return isBinary(data)
}

func IsEditableKind(kind FileKind) bool {
	return kind != FileKindUnsupported && kind != FileKindImage
}

func ValidateEditableContent(current, next []byte) error {
	if int64(len(current)) > MaxTextResponseBytes || int64(len(next)) > MaxTextResponseBytes || isBinary(current) || isBinary(next) {
		return ErrUnsupportedContent
	}
	return nil
}

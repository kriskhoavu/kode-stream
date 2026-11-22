package models

import "time"

type ItemSummary struct {
	ID               string     `json:"id" yaml:"id"`
	WorkspaceID      string     `json:"workspaceId" yaml:"workspaceId"`
	WorkspaceName    string     `json:"workspaceName" yaml:"workspaceName"`
	Branch           string     `json:"branch" yaml:"branch"`
	BranchRef        string     `json:"branchRef,omitempty" yaml:"branchRef,omitempty"`
	Commit           string     `json:"commit,omitempty" yaml:"commit,omitempty"`
	SourceMode       string     `json:"sourceMode,omitempty" yaml:"sourceMode,omitempty"`
	Editable         bool       `json:"editable" yaml:"editable"`
	Scope            string     `json:"scope" yaml:"scope"`
	Identifier       string     `json:"identifier" yaml:"identifier"`
	Title            string     `json:"title" yaml:"title"`
	Status           ItemStatus `json:"status" yaml:"status"`
	Owner            string     `json:"owner,omitempty" yaml:"owner,omitempty"`
	Author           string     `json:"author,omitempty" yaml:"author,omitempty"`
	Tags             []string   `json:"tags" yaml:"tags"`
	UpdatedAt        time.Time  `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
	Description      string     `json:"description,omitempty" yaml:"description,omitempty"`
	MetadataSource   string     `json:"metadataSource" yaml:"metadataSource"`
	ItemPath         string     `json:"itemPath,omitempty" yaml:"itemPath,omitempty"`
	MetadataRevision string     `json:"metadataRevision,omitempty" yaml:"metadataRevision,omitempty"`
}

type ItemDetail struct {
	ItemSummary
	Documents []ItemDocument      `json:"documents" yaml:"documents"`
	Metadata  map[string]any      `json:"metadata" yaml:"metadata"`
	Warnings  []ScanWarning       `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Counts    ItemWorkspaceCounts `json:"counts" yaml:"counts"`
}

type ItemWorkspaceCounts struct {
	Files int `json:"files" yaml:"files"`
}

type ItemDocument struct {
	ID    string `json:"id" yaml:"id,omitempty"`
	Role  string `json:"role" yaml:"role,omitempty"`
	Track string `json:"track,omitempty" yaml:"track,omitempty"`
	Path  string `json:"path" yaml:"path,omitempty"`
	Label string `json:"label" yaml:"label,omitempty"`
}

type FileNode struct {
	ID        string     `json:"id" yaml:"id"`
	Name      string     `json:"name" yaml:"name"`
	Path      string     `json:"path" yaml:"path"`
	Type      string     `json:"type" yaml:"type"`
	Children  []FileNode `json:"children,omitempty" yaml:"children,omitempty"`
	Truncated bool       `json:"truncated,omitempty" yaml:"truncated,omitempty"`
}

type FileKind string

const (
	FileKindMarkdown    FileKind = "markdown"
	FileKindHTML        FileKind = "html"
	FileKindJSON        FileKind = "json"
	FileKindYAML        FileKind = "yaml"
	FileKindCode        FileKind = "code"
	FileKindText        FileKind = "text"
	FileKindImage       FileKind = "image"
	FileKindUnsupported FileKind = "unsupported"
)

type FileContent struct {
	ID              string   `json:"id" yaml:"id"`
	Path            string   `json:"path" yaml:"path"`
	Content         string   `json:"content" yaml:"content"`
	Language        string   `json:"language" yaml:"language"`
	Hash            string   `json:"hash" yaml:"hash"`
	Kind            FileKind `json:"kind" yaml:"kind"`
	SizeBytes       int64    `json:"sizeBytes" yaml:"sizeBytes"`
	Truncated       bool     `json:"truncated,omitempty" yaml:"truncated,omitempty"`
	Editable        bool     `json:"editable" yaml:"editable"`
	Committed       bool     `json:"committed,omitempty" yaml:"committed,omitempty"`
	RefreshRequired bool     `json:"refreshRequired,omitempty" yaml:"refreshRequired,omitempty"`
	RefreshError    string   `json:"refreshError,omitempty" yaml:"refreshError,omitempty"`
}

type ItemStatus string

const (
	StatusUnsorted   ItemStatus = "unsorted"
	StatusDraft      ItemStatus = "draft"
	StatusInProgress ItemStatus = "in_progress"
	StatusReview     ItemStatus = "review"
	StatusDone       ItemStatus = "done"
)

var StatusOrder = []ItemStatus{StatusUnsorted, StatusDraft, StatusInProgress, StatusReview, StatusDone}

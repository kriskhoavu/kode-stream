package models

import "time"

type AuditStatus string

const (
	AuditStatusSuccess AuditStatus = "success"
	AuditStatusBlocked AuditStatus = "blocked"
	AuditStatusFailed  AuditStatus = "failed"
)

type AuditEvent struct {
	ID          string      `json:"id" yaml:"id"`
	Time        time.Time   `json:"time" yaml:"time"`
	WorkspaceID string      `json:"workspaceId,omitempty" yaml:"workspaceId,omitempty"`
	ItemID      string      `json:"itemId,omitempty" yaml:"itemId,omitempty"`
	Operation   string      `json:"operation" yaml:"operation"`
	Status      AuditStatus `json:"status" yaml:"status"`
	Message     string      `json:"message" yaml:"message"`
	Paths       []string    `json:"paths" yaml:"paths"`
	DurationMS  int64       `json:"durationMs" yaml:"durationMs"`
	Error       string      `json:"error,omitempty" yaml:"error,omitempty"`
}

type ItemStatus string

const (
	StatusUnsorted   ItemStatus = "unsorted"
	StatusIdeas      ItemStatus = "ideas"
	StatusDraft      ItemStatus = "draft"
	StatusInProgress ItemStatus = "in_progress"
	StatusReview     ItemStatus = "review"
	StatusDone       ItemStatus = "done"
)

var StatusOrder = []ItemStatus{StatusUnsorted, StatusIdeas, StatusDraft, StatusInProgress, StatusReview, StatusDone}

type WorkspaceConfig struct {
	ID             string    `json:"id" yaml:"id"`
	Name           string    `json:"name" yaml:"name"`
	Path           string    `json:"path" yaml:"path"`
	BaselineBranch string    `json:"baselineBranch" yaml:"baselineBranch"`
	Sources        []string  `json:"sources" yaml:"sources"`
	CreatedAt      time.Time `json:"createdAt" yaml:"createdAt"`
	LastScannedAt  time.Time `json:"lastScannedAt,omitempty" yaml:"lastScannedAt,omitempty"`
}

type WorkspaceInput struct {
	Name           string   `json:"name" yaml:"name"`
	Path           string   `json:"path" yaml:"path"`
	BaselineBranch string   `json:"baselineBranch" yaml:"baselineBranch"`
	Sources        []string `json:"sources" yaml:"sources"`
}

type SourceStructureSettings struct {
	Version int                   `json:"version" yaml:"version"`
	Cards   []SourceStructureCard `json:"cards" yaml:"cards"`
}

type SourceStructureCard struct {
	PathPattern string                `json:"pathPattern" yaml:"pathPattern"`
	Fields      SourceStructureFields `json:"fields" yaml:"fields"`
}

type SourceStructureFields struct {
	Scope      string   `json:"scope" yaml:"scope"`
	Identifier string   `json:"identifier" yaml:"identifier"`
	Title      string   `json:"title,omitempty" yaml:"title,omitempty"`
	Status     string   `json:"status,omitempty" yaml:"status,omitempty"`
	Owner      string   `json:"owner,omitempty" yaml:"owner,omitempty"`
	Tags       []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type SourceSettingsResult struct {
	Directory string                  `json:"directory" yaml:"directory"`
	Exists    bool                    `json:"exists" yaml:"exists"`
	Mode      string                  `json:"mode" yaml:"mode"`
	Settings  SourceStructureSettings `json:"settings" yaml:"settings"`
	Warnings  []ScanWarning           `json:"warnings" yaml:"warnings"`
}

type ItemSummary struct {
	ID             string     `json:"id" yaml:"id"`
	WorkspaceID    string     `json:"workspaceId" yaml:"workspaceId"`
	WorkspaceName  string     `json:"workspaceName" yaml:"workspaceName"`
	Branch         string     `json:"branch" yaml:"branch"`
	Scope          string     `json:"scope" yaml:"scope"`
	Identifier     string     `json:"identifier" yaml:"identifier"`
	Title          string     `json:"title" yaml:"title"`
	Status         ItemStatus `json:"status" yaml:"status"`
	Owner          string     `json:"owner,omitempty" yaml:"owner,omitempty"`
	Author         string     `json:"author,omitempty" yaml:"author,omitempty"`
	Tags           []string   `json:"tags" yaml:"tags"`
	UpdatedAt      time.Time  `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
	Description    string     `json:"description,omitempty" yaml:"description,omitempty"`
	MetadataSource string     `json:"metadataSource" yaml:"metadataSource"`
	ItemPath       string     `json:"itemPath,omitempty" yaml:"itemPath,omitempty"`
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
	ID    string `json:"id" yaml:"id"`
	Role  string `json:"role" yaml:"role"`
	Track string `json:"track,omitempty" yaml:"track,omitempty"`
	Path  string `json:"path" yaml:"path"`
	Label string `json:"label" yaml:"label"`
}

type FileNode struct {
	ID       string     `json:"id" yaml:"id"`
	Name     string     `json:"name" yaml:"name"`
	Path     string     `json:"path" yaml:"path"`
	Type     string     `json:"type" yaml:"type"`
	Children []FileNode `json:"children,omitempty" yaml:"children,omitempty"`
}

type FileContent struct {
	ID       string `json:"id" yaml:"id"`
	Path     string `json:"path" yaml:"path"`
	Content  string `json:"content" yaml:"content"`
	Language string `json:"language" yaml:"language"`
	Hash     string `json:"hash" yaml:"hash"`
}

type ScanWarning struct {
	ItemPath string `json:"itemPath,omitempty" yaml:"itemPath,omitempty"`
	Message  string `json:"message" yaml:"message"`
}

type ScanResult struct {
	WorkspaceID string        `json:"workspaceId" yaml:"workspaceId"`
	ScannedAt   time.Time     `json:"scannedAt" yaml:"scannedAt"`
	ItemCount   int           `json:"itemCount" yaml:"itemCount"`
	Warnings    []ScanWarning `json:"warnings" yaml:"warnings"`
}

type FileSaveInput struct {
	FileID       string `json:"fileId" yaml:"fileId"`
	Content      string `json:"content" yaml:"content"`
	ExpectedHash string `json:"expectedHash,omitempty" yaml:"expectedHash,omitempty"`
}

type ItemMetadataUpdateInput struct {
	Title      string     `json:"title,omitempty" yaml:"title,omitempty"`
	Scope      string     `json:"scope,omitempty" yaml:"scope,omitempty"`
	Identifier string     `json:"identifier,omitempty" yaml:"identifier,omitempty"`
	Status     ItemStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Owner      string     `json:"owner,omitempty" yaml:"owner,omitempty"`
	Tags       []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type ItemStatusUpdateInput struct {
	Status ItemStatus `json:"status" yaml:"status"`
}

type NewItemInput struct {
	WorkspaceID string     `json:"workspaceId" yaml:"workspaceId"`
	Source      string     `json:"source" yaml:"source"`
	Scope       string     `json:"scope" yaml:"scope"`
	Identifier  string     `json:"identifier" yaml:"identifier"`
	Title       string     `json:"title" yaml:"title"`
	Status      ItemStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Owner       string     `json:"owner,omitempty" yaml:"owner,omitempty"`
	Tags        []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type WriteResult struct {
	Item      ItemDetail `json:"item" yaml:"item"`
	ScannedAt time.Time  `json:"scannedAt" yaml:"scannedAt"`
}

type GitChangeStatus string

const (
	GitChangeModified   GitChangeStatus = "modified"
	GitChangeAdded      GitChangeStatus = "added"
	GitChangeDeleted    GitChangeStatus = "deleted"
	GitChangeRenamed    GitChangeStatus = "renamed"
	GitChangeCopied     GitChangeStatus = "copied"
	GitChangeUntracked  GitChangeStatus = "untracked"
	GitChangeConflicted GitChangeStatus = "conflicted"
)

type GitChange struct {
	Path     string          `json:"path" yaml:"path"`
	OldPath  string          `json:"oldPath,omitempty" yaml:"oldPath,omitempty"`
	Status   GitChangeStatus `json:"status" yaml:"status"`
	Staged   bool            `json:"staged" yaml:"staged"`
	Conflict bool            `json:"conflict" yaml:"conflict"`
}

type GitStatus struct {
	WorkspaceID string      `json:"workspaceId" yaml:"workspaceId"`
	Branch      string      `json:"branch" yaml:"branch"`
	Upstream    string      `json:"upstream,omitempty" yaml:"upstream,omitempty"`
	Ahead       int         `json:"ahead" yaml:"ahead"`
	Behind      int         `json:"behind" yaml:"behind"`
	Dirty       bool        `json:"dirty" yaml:"dirty"`
	Conflicted  bool        `json:"conflicted" yaml:"conflicted"`
	Changes     []GitChange `json:"changes" yaml:"changes"`
}

type GitCommitInput struct {
	Message string   `json:"message" yaml:"message"`
	Paths   []string `json:"paths" yaml:"paths"`
}

type GitOperationInput struct {
	Confirm bool `json:"confirm,omitempty" yaml:"confirm,omitempty"`
}

type BranchCreateInput struct {
	Name       string `json:"name" yaml:"name"`
	StartPoint string `json:"startPoint,omitempty" yaml:"startPoint,omitempty"`
	Checkout   bool   `json:"checkout,omitempty" yaml:"checkout,omitempty"`
}

type BranchSwitchInput struct {
	Name    string `json:"name" yaml:"name"`
	Confirm bool   `json:"confirm,omitempty" yaml:"confirm,omitempty"`
}

type GitOperationResult struct {
	OK      bool      `json:"ok" yaml:"ok"`
	Message string    `json:"message,omitempty" yaml:"message,omitempty"`
	Status  GitStatus `json:"status" yaml:"status"`
}

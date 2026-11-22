package models

import "time"

type WorkspaceDirectoryListing struct {
	WorkspaceID string               `json:"workspaceId" yaml:"workspaceId"`
	Path        string               `json:"path" yaml:"path"`
	Entries     []WorkspaceTreeEntry `json:"entries" yaml:"entries"`
	HiddenCount int                  `json:"hiddenCount" yaml:"hiddenCount"`
}

type WorkspaceTreeEntry struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Path        string   `json:"path" yaml:"path"`
	Type        string   `json:"type" yaml:"type"`
	HasChildren bool     `json:"hasChildren" yaml:"hasChildren"`
	Ignored     bool     `json:"ignored" yaml:"ignored"`
	Hidden      bool     `json:"hidden" yaml:"hidden"`
	Kind        FileKind `json:"kind,omitempty" yaml:"kind,omitempty"`
	Language    string   `json:"language,omitempty" yaml:"language,omitempty"`
	SizeBytes   int64    `json:"sizeBytes,omitempty" yaml:"sizeBytes,omitempty"`
	Editable    bool     `json:"editable" yaml:"editable"`
}

type WorkspaceFileSaveInput struct {
	Path         string `json:"path" yaml:"path"`
	Content      string `json:"content" yaml:"content"`
	ExpectedHash string `json:"expectedHash" yaml:"expectedHash"`
}

type WorkspaceFileRevertInput struct {
	Path string `json:"path" yaml:"path"`
}

type WorkspaceFileWriteResult struct {
	File            FileContent `json:"file" yaml:"file"`
	Refreshed       bool        `json:"refreshed" yaml:"refreshed"`
	Committed       bool        `json:"committed" yaml:"committed"`
	RefreshRequired bool        `json:"refreshRequired,omitempty" yaml:"refreshRequired,omitempty"`
	RefreshError    string      `json:"refreshError,omitempty" yaml:"refreshError,omitempty"`
}

type WorkspaceFileCreateInput struct {
	ParentPath string `json:"parentPath" yaml:"parentPath"`
	Name       string `json:"name" yaml:"name"`
	Content    string `json:"content" yaml:"content"`
}

type WorkspaceDirectoryCreateInput struct {
	ParentPath string `json:"parentPath" yaml:"parentPath"`
	Name       string `json:"name" yaml:"name"`
}

type WorkspacePathRenameInput struct {
	Path            string `json:"path" yaml:"path"`
	DestinationPath string `json:"destinationPath" yaml:"destinationPath"`
}

type WorkspacePathMutationResult struct {
	WorkspaceID      string   `json:"workspaceId" yaml:"workspaceId"`
	Path             string   `json:"path" yaml:"path"`
	Type             string   `json:"type" yaml:"type"`
	InvalidatedPaths []string `json:"invalidatedPaths" yaml:"invalidatedPaths"`
	Refreshed        bool     `json:"refreshed" yaml:"refreshed"`
	Committed        bool     `json:"committed" yaml:"committed"`
	RefreshRequired  bool     `json:"refreshRequired,omitempty" yaml:"refreshRequired,omitempty"`
	RefreshError     string   `json:"refreshError,omitempty" yaml:"refreshError,omitempty"`
}

type WorkspacePathSearchResult struct {
	ID            string `json:"id" yaml:"id"`
	WorkspaceID   string `json:"workspaceId" yaml:"workspaceId"`
	WorkspaceName string `json:"workspaceName" yaml:"workspaceName"`
	Name          string `json:"name" yaml:"name"`
	Path          string `json:"path" yaml:"path"`
	Type          string `json:"type" yaml:"type"`
	Ignored       bool   `json:"ignored" yaml:"ignored"`
	Context       string `json:"context" yaml:"context"`
}

type WorkspacePathSearchResponse struct {
	Results   []WorkspacePathSearchResult `json:"results" yaml:"results"`
	Truncated bool                        `json:"truncated" yaml:"truncated"`
}

type WorkspaceContentSearchRequest struct {
	Query          string `json:"query" yaml:"query"`
	CaseSensitive  bool   `json:"caseSensitive" yaml:"caseSensitive"`
	IncludeIgnored bool   `json:"includeIgnored" yaml:"includeIgnored"`
}

type WorkspaceContentSearchRoot struct {
	Path string `json:"path" yaml:"path"`
}

type WorkspaceContentSearchResult struct {
	ID            string   `json:"id" yaml:"id"`
	WorkspaceID   string   `json:"workspaceId" yaml:"workspaceId"`
	WorkspaceName string   `json:"workspaceName" yaml:"workspaceName"`
	ItemID        string   `json:"itemId,omitempty" yaml:"itemId,omitempty"`
	Path          string   `json:"path" yaml:"path"`
	FileID        string   `json:"fileId,omitempty" yaml:"fileId,omitempty"`
	Name          string   `json:"name" yaml:"name"`
	Kind          FileKind `json:"kind" yaml:"kind"`
	Language      string   `json:"language" yaml:"language"`
	LineNumber    int      `json:"lineNumber" yaml:"lineNumber"`
	ColumnStart   int      `json:"columnStart" yaml:"columnStart"`
	ColumnEnd     int      `json:"columnEnd" yaml:"columnEnd"`
	Snippet       string   `json:"snippet" yaml:"snippet"`
	Ignored       bool     `json:"ignored" yaml:"ignored"`
}

type WorkspaceContentSearchResponse struct {
	Results      []WorkspaceContentSearchResult `json:"results" yaml:"results"`
	Truncated    bool                           `json:"truncated" yaml:"truncated"`
	FilesVisited int                            `json:"filesVisited" yaml:"filesVisited"`
	BytesRead    int64                          `json:"bytesRead" yaml:"bytesRead"`
	SkippedFiles int                            `json:"skippedFiles" yaml:"skippedFiles"`
}

type WorkspaceContentSearchBudget struct {
	MaxResults       int   `json:"maxResults" yaml:"maxResults"`
	MaxFiles         int   `json:"maxFiles" yaml:"maxFiles"`
	MaxBytes         int64 `json:"maxBytes" yaml:"maxBytes"`
	MaxFileSize      int64 `json:"maxFileSize" yaml:"maxFileSize"`
	MaxQueryLength   int   `json:"maxQueryLength" yaml:"maxQueryLength"`
	MaxSnippetLength int   `json:"maxSnippetLength" yaml:"maxSnippetLength"`
	Results          int   `json:"-" yaml:"-"`
	FilesVisited     int   `json:"-" yaml:"-"`
	BytesRead        int64 `json:"-" yaml:"-"`
}

type WorkspacePathGitState struct {
	Path     string          `json:"path" yaml:"path"`
	OldPath  string          `json:"oldPath,omitempty" yaml:"oldPath,omitempty"`
	Status   GitChangeStatus `json:"status" yaml:"status"`
	Staged   bool            `json:"staged" yaml:"staged"`
	Conflict bool            `json:"conflict" yaml:"conflict"`
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

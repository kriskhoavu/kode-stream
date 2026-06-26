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

type HealthStatus string

const (
	HealthStatusOK      HealthStatus = "ok"
	HealthStatusWarning HealthStatus = "warning"
	HealthStatusFailed  HealthStatus = "failed"
)

type HealthCheck struct {
	Name         string       `json:"name" yaml:"name"`
	Status       HealthStatus `json:"status" yaml:"status"`
	Message      string       `json:"message" yaml:"message"`
	RecoveryHint string       `json:"recoveryHint,omitempty" yaml:"recoveryHint,omitempty"`
}

type WorkspaceHealth struct {
	WorkspaceID string        `json:"workspaceId" yaml:"workspaceId"`
	CheckedAt   time.Time     `json:"checkedAt" yaml:"checkedAt"`
	Checks      []HealthCheck `json:"checks" yaml:"checks"`
	Summary     HealthStatus  `json:"summary" yaml:"summary"`
}

type SafetyCheck struct {
	OK           bool   `json:"ok" yaml:"ok"`
	Message      string `json:"message,omitempty" yaml:"message,omitempty"`
	RecoveryHint string `json:"recoveryHint,omitempty" yaml:"recoveryHint,omitempty"`
}

type SearchQuery struct {
	Text        string   `json:"q" yaml:"q"`
	WorkspaceID string   `json:"workspaceId,omitempty" yaml:"workspaceId,omitempty"`
	Types       []string `json:"types,omitempty" yaml:"types,omitempty"`
	Limit       int      `json:"limit,omitempty" yaml:"limit,omitempty"`
}

type SearchResult struct {
	ID          string `json:"id" yaml:"id"`
	Type        string `json:"type" yaml:"type"`
	Title       string `json:"title" yaml:"title"`
	Subtitle    string `json:"subtitle" yaml:"subtitle"`
	Context     string `json:"context" yaml:"context"`
	WorkspaceID string `json:"workspaceId,omitempty" yaml:"workspaceId,omitempty"`
	ItemID      string `json:"itemId,omitempty" yaml:"itemId,omitempty"`
	Route       string `json:"route" yaml:"route"`
	Score       int    `json:"score" yaml:"score"`
}

type SavedFilter struct {
	ID          string         `json:"id" yaml:"id"`
	Name        string         `json:"name" yaml:"name"`
	Route       string         `json:"route" yaml:"route"`
	WorkspaceID string         `json:"workspaceId,omitempty" yaml:"workspaceId,omitempty"`
	Filters     map[string]any `json:"filters" yaml:"filters"`
	CreatedAt   time.Time      `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt" yaml:"updatedAt"`
}

type RecentItem struct {
	ItemID      string    `json:"itemId" yaml:"itemId"`
	WorkspaceID string    `json:"workspaceId" yaml:"workspaceId"`
	Title       string    `json:"title" yaml:"title"`
	Subtitle    string    `json:"subtitle,omitempty" yaml:"subtitle,omitempty"`
	Route       string    `json:"route" yaml:"route"`
	OpenedAt    time.Time `json:"openedAt" yaml:"openedAt"`
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

type WorkspaceConfig struct {
	ID                 string    `json:"id" yaml:"id"`
	Name               string    `json:"name" yaml:"name"`
	Path               string    `json:"path" yaml:"path"`
	BaselineBranch     string    `json:"baselineBranch" yaml:"baselineBranch"`
	LastSelectedBranch string    `json:"lastSelectedBranch,omitempty" yaml:"lastSelectedBranch,omitempty"`
	Sources            []string  `json:"sources" yaml:"sources"`
	CreatedAt          time.Time `json:"createdAt" yaml:"createdAt"`
	LastScannedAt      time.Time `json:"lastScannedAt,omitempty" yaml:"lastScannedAt,omitempty"`
}

type BranchScanMetadata struct {
	WorkspaceID             string        `json:"workspaceId" yaml:"workspaceId"`
	Branch                  string        `json:"branch" yaml:"branch"`
	BranchRef               string        `json:"branchRef,omitempty" yaml:"branchRef,omitempty"`
	Commit                  string        `json:"commit,omitempty" yaml:"commit,omitempty"`
	SourceMode              string        `json:"sourceMode,omitempty" yaml:"sourceMode,omitempty"`
	Editable                bool          `json:"editable" yaml:"editable"`
	SourceConfigurationHash string        `json:"sourceConfigurationHash,omitempty" yaml:"sourceConfigurationHash,omitempty"`
	ScannedAt               time.Time     `json:"scannedAt" yaml:"scannedAt"`
	Warnings                []ScanWarning `json:"warnings" yaml:"warnings"`
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
	Source     string   `json:"source,omitempty" yaml:"source,omitempty"`
	Item       string   `json:"item,omitempty" yaml:"item,omitempty"`
	Scope      string   `json:"scope,omitempty" yaml:"scope,omitempty"`
	Identifier string   `json:"identifier,omitempty" yaml:"identifier,omitempty"`
	Title      string   `json:"title,omitempty" yaml:"title,omitempty"`
	Status     string   `json:"status,omitempty" yaml:"status,omitempty"`
	Owner      string   `json:"owner,omitempty" yaml:"owner,omitempty"`
	Tags       []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type SourceStructureProposal struct {
	ID         string                   `json:"id" yaml:"id"`
	Label      string                   `json:"label" yaml:"label"`
	Summary    string                   `json:"summary" yaml:"summary"`
	Confidence string                   `json:"confidence" yaml:"confidence"`
	Card       SourceStructureCard      `json:"card" yaml:"card"`
	Preview    []SourceStructurePreview `json:"preview" yaml:"preview"`
}

type SourceStructurePreview struct {
	Path       string     `json:"path" yaml:"path"`
	Source     string     `json:"source,omitempty" yaml:"source,omitempty"`
	Item       string     `json:"item,omitempty" yaml:"item,omitempty"`
	Scope      string     `json:"scope" yaml:"scope"`
	Identifier string     `json:"identifier" yaml:"identifier"`
	Title      string     `json:"title" yaml:"title"`
	Status     ItemStatus `json:"status" yaml:"status"`
	Tags       []string   `json:"tags" yaml:"tags"`
}

type SourceSettingsResult struct {
	Directory string                    `json:"directory" yaml:"directory"`
	Exists    bool                      `json:"exists" yaml:"exists"`
	Mode      string                    `json:"mode" yaml:"mode"`
	Settings  SourceStructureSettings   `json:"settings" yaml:"settings"`
	Warnings  []ScanWarning             `json:"warnings" yaml:"warnings"`
	Proposals []SourceStructureProposal `json:"proposals" yaml:"proposals"`
	Preview   []SourceStructurePreview  `json:"preview" yaml:"preview"`
}

type ItemSummary struct {
	ID             string     `json:"id" yaml:"id"`
	WorkspaceID    string     `json:"workspaceId" yaml:"workspaceId"`
	WorkspaceName  string     `json:"workspaceName" yaml:"workspaceName"`
	Branch         string     `json:"branch" yaml:"branch"`
	BranchRef      string     `json:"branchRef,omitempty" yaml:"branchRef,omitempty"`
	Commit         string     `json:"commit,omitempty" yaml:"commit,omitempty"`
	SourceMode     string     `json:"sourceMode,omitempty" yaml:"sourceMode,omitempty"`
	Editable       bool       `json:"editable" yaml:"editable"`
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
	ID    string `json:"id" yaml:"id,omitempty"`
	Role  string `json:"role" yaml:"role,omitempty"`
	Track string `json:"track,omitempty" yaml:"track,omitempty"`
	Path  string `json:"path" yaml:"path,omitempty"`
	Label string `json:"label" yaml:"label,omitempty"`
}

type FileNode struct {
	ID       string     `json:"id" yaml:"id"`
	Name     string     `json:"name" yaml:"name"`
	Path     string     `json:"path" yaml:"path"`
	Type     string     `json:"type" yaml:"type"`
	Children []FileNode `json:"children,omitempty" yaml:"children,omitempty"`
}

type FileKind string

const (
	FileKindMarkdown    FileKind = "markdown"
	FileKindHTML        FileKind = "html"
	FileKindJSON        FileKind = "json"
	FileKindYAML        FileKind = "yaml"
	FileKindCode        FileKind = "code"
	FileKindText        FileKind = "text"
	FileKindUnsupported FileKind = "unsupported"
)

type FileContent struct {
	ID        string   `json:"id" yaml:"id"`
	Path      string   `json:"path" yaml:"path"`
	Content   string   `json:"content" yaml:"content"`
	Language  string   `json:"language" yaml:"language"`
	Hash      string   `json:"hash" yaml:"hash"`
	Kind      FileKind `json:"kind" yaml:"kind"`
	SizeBytes int64    `json:"sizeBytes" yaml:"sizeBytes"`
	Truncated bool     `json:"truncated,omitempty" yaml:"truncated,omitempty"`
	Editable  bool     `json:"editable" yaml:"editable"`
}

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
	File      FileContent `json:"file" yaml:"file"`
	Refreshed bool        `json:"refreshed" yaml:"refreshed"`
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

type BranchLoadInput struct {
	Branch string `json:"branch,omitempty" yaml:"branch,omitempty"`
	Force  bool   `json:"force,omitempty" yaml:"force,omitempty"`
}

type BranchLoadResult struct {
	WorkspaceID           string        `json:"workspaceId" yaml:"workspaceId"`
	Branch                string        `json:"branch" yaml:"branch"`
	SelectedBranch        string        `json:"selectedBranch" yaml:"selectedBranch"`
	BranchRef             string        `json:"branchRef" yaml:"branchRef"`
	Commit                string        `json:"commit" yaml:"commit"`
	CurrentCheckoutBranch string        `json:"currentCheckoutBranch" yaml:"currentCheckoutBranch"`
	SourceMode            string        `json:"sourceMode" yaml:"sourceMode"`
	Mode                  string        `json:"mode" yaml:"mode"`
	Editable              bool          `json:"editable" yaml:"editable"`
	ScannedAt             time.Time     `json:"scannedAt" yaml:"scannedAt"`
	ItemCount             int           `json:"itemCount" yaml:"itemCount"`
	Warnings              []ScanWarning `json:"warnings" yaml:"warnings"`
	Items                 []ItemSummary `json:"items" yaml:"items"`
}

type FileSaveInput struct {
	FileID               string `json:"fileId" yaml:"fileId"`
	Content              string `json:"content" yaml:"content"`
	ExpectedHash         string `json:"expectedHash,omitempty" yaml:"expectedHash,omitempty"`
	MaterializeConfirmed bool   `json:"materializeConfirmed,omitempty" yaml:"materializeConfirmed,omitempty"`
}

type ItemMetadataUpdateInput struct {
	Title                string     `json:"title,omitempty" yaml:"title,omitempty"`
	Scope                string     `json:"scope,omitempty" yaml:"scope,omitempty"`
	Identifier           string     `json:"identifier,omitempty" yaml:"identifier,omitempty"`
	Status               ItemStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Owner                string     `json:"owner,omitempty" yaml:"owner,omitempty"`
	Tags                 []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
	MaterializeConfirmed bool       `json:"materializeConfirmed,omitempty" yaml:"materializeConfirmed,omitempty"`
}

type ItemStatusUpdateInput struct {
	Status               ItemStatus `json:"status" yaml:"status"`
	MaterializeConfirmed bool       `json:"materializeConfirmed,omitempty" yaml:"materializeConfirmed,omitempty"`
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

type WorkspaceBranches struct {
	WorkspaceID string   `json:"workspaceId" yaml:"workspaceId"`
	Current     string   `json:"current" yaml:"current"`
	Branches    []string `json:"branches" yaml:"branches"`
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
	OK           bool      `json:"ok" yaml:"ok"`
	Message      string    `json:"message,omitempty" yaml:"message,omitempty"`
	RecoveryHint string    `json:"recoveryHint,omitempty" yaml:"recoveryHint,omitempty"`
	Status       GitStatus `json:"status" yaml:"status"`
}

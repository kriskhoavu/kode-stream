package models

// Package models contains compatibility contracts pending domain ownership.

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
	ID                 string                    `json:"id" yaml:"id"`
	Name               string                    `json:"name" yaml:"name"`
	Path               string                    `json:"path" yaml:"path"`
	Location           WorkspaceLocation         `json:"location,omitempty" yaml:"location,omitempty"`
	OwnerUserID        string                    `json:"ownerUserId,omitempty" yaml:"ownerUserId,omitempty"`
	AgentID            string                    `json:"agentId,omitempty" yaml:"agentId,omitempty"`
	LocalRootLabel     string                    `json:"localRootLabel,omitempty" yaml:"localRootLabel,omitempty"`
	PublishedSummary   bool                      `json:"publishedSummary,omitempty" yaml:"publishedSummary,omitempty"`
	ScanStatus         string                    `json:"scanStatus,omitempty" yaml:"scanStatus,omitempty"`
	BaselineBranch     string                    `json:"baselineBranch" yaml:"baselineBranch"`
	RegistrationMode   WorkspaceRegistrationMode `json:"registrationMode,omitempty" yaml:"registrationMode,omitempty"`
	RemoteURL          string                    `json:"remoteUrl,omitempty" yaml:"remoteUrl,omitempty"`
	ClonePathManaged   bool                      `json:"clonePathManaged,omitempty" yaml:"clonePathManaged,omitempty"`
	LastSelectedBranch string                    `json:"lastSelectedBranch,omitempty" yaml:"lastSelectedBranch,omitempty"`
	Sources            []string                  `json:"sources" yaml:"sources"`
	CreatedAt          time.Time                 `json:"createdAt" yaml:"createdAt"`
	LastScannedAt      time.Time                 `json:"lastScannedAt,omitempty" yaml:"lastScannedAt,omitempty"`
	Jira               *JiraConnection           `json:"jira,omitempty" yaml:"jira,omitempty"`
	Knowledge          *KnowledgeSettings        `json:"knowledge,omitempty" yaml:"knowledge,omitempty"`
	Runtime            *WorkspaceRuntimeConfig   `json:"runtime,omitempty" yaml:"runtime,omitempty"`
}

type RuntimeType string

const (
	RuntimeTypeDockerCompose RuntimeType = "docker-compose"
	RuntimeTypeProcfile      RuntimeType = "procfile"
	RuntimeTypeMakefile      RuntimeType = "makefile"
	RuntimeTypeCustom        RuntimeType = "custom"
)

type RebuildPolicy string

const (
	RebuildPolicyNever       RebuildPolicy = "never"
	RebuildPolicyChangedOnly RebuildPolicy = "changed-only"
	RebuildPolicyAlways      RebuildPolicy = "always"
)

type RuntimeHealthCheck struct {
	Name           string `json:"name,omitempty" yaml:"name,omitempty"`
	Type           string `json:"type" yaml:"type"`
	Target         string `json:"target" yaml:"target"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
}

type RuntimeVerifyCommands struct {
	Smoke    string `json:"smoke" yaml:"smoke"`
	Critical string `json:"critical,omitempty" yaml:"critical,omitempty"`
	Full     string `json:"full,omitempty" yaml:"full,omitempty"`
}

type RuntimeCommandSet struct {
	Up             string                `json:"up" yaml:"up"`
	Down           string                `json:"down" yaml:"down"`
	RebuildChanged string                `json:"rebuildChanged,omitempty" yaml:"rebuildChanged,omitempty"`
	Verify         RuntimeVerifyCommands `json:"verify" yaml:"verify"`
}

type RuntimeArtifacts struct {
	Paths []string `json:"paths,omitempty" yaml:"paths,omitempty"`
}

type AutomationRunner string

const (
	AutomationRunnerCypress    AutomationRunner = "cypress"
	AutomationRunnerPlaywright AutomationRunner = "playwright"
)

type RuntimeAutomationConfig struct {
	Enabled            bool             `json:"enabled" yaml:"enabled"`
	RepositoryPath     string           `json:"repositoryPath,omitempty" yaml:"repositoryPath,omitempty"`
	Runner             AutomationRunner `json:"runner,omitempty" yaml:"runner,omitempty"`
	DefaultEnvironment string           `json:"defaultEnvironment,omitempty" yaml:"defaultEnvironment,omitempty"`
	CommandTemplate    string           `json:"commandTemplate,omitempty" yaml:"commandTemplate,omitempty"`
	ArtifactPaths      []string         `json:"artifactPaths,omitempty" yaml:"artifactPaths,omitempty"`
}

type AutomationDisplayMode string

const (
	AutomationDisplayModeSilent  AutomationDisplayMode = "silent"
	AutomationDisplayModeVisible AutomationDisplayMode = "visible"
)

type WorkspaceRuntimeConfig struct {
	Type          RuntimeType              `json:"type" yaml:"type"`
	ConfigPath    string                   `json:"configPath,omitempty" yaml:"configPath,omitempty"`
	RebuildPolicy RebuildPolicy            `json:"rebuildPolicy,omitempty" yaml:"rebuildPolicy,omitempty"`
	Commands      RuntimeCommandSet        `json:"commands" yaml:"commands"`
	HealthChecks  []RuntimeHealthCheck     `json:"healthChecks,omitempty" yaml:"healthChecks,omitempty"`
	Artifacts     RuntimeArtifacts         `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
	Automation    *RuntimeAutomationConfig `json:"automation,omitempty" yaml:"automation,omitempty"`
}

type KnowledgeSettings struct {
	Enabled          *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	EnrichExecutable string   `json:"enrichExecutable,omitempty" yaml:"enrichExecutable,omitempty"`
	EnrichArgs       []string `json:"enrichArgs,omitempty" yaml:"enrichArgs,omitempty"`
}

type JiraConnection struct {
	DeploymentType string `json:"deploymentType" yaml:"deploymentType"`
	BaseURL        string `json:"baseUrl" yaml:"baseUrl"`
	ProjectKey     string `json:"projectKey" yaml:"projectKey"`
	AccountEmail   string `json:"accountEmail,omitempty" yaml:"accountEmail,omitempty"`
	TokenEnvVar    string `json:"tokenEnvVar" yaml:"tokenEnvVar"`
}

type WorkspaceRegistrationMode string

const (
	WorkspaceRegistrationModeLocalPath   WorkspaceRegistrationMode = "local_path"
	WorkspaceRegistrationModeRemoteClone WorkspaceRegistrationMode = "remote_clone"
	WorkspaceRegistrationModeExisting    WorkspaceRegistrationMode = "existing_workspace"
)

type WorkspaceLocation string

const (
	WorkspaceLocationLocalPath  WorkspaceLocation = "local_path"
	WorkspaceLocationCloudAgent WorkspaceLocation = "cloud_agent"
)

type RuntimeMode string

const (
	RuntimeModeLocal RuntimeMode = "local"
	RuntimeModeCloud RuntimeMode = "cloud"
)

type CloudRole string

const (
	CloudRoleAdmin  CloudRole = "admin"
	CloudRoleEditor CloudRole = "editor"
	CloudRoleViewer CloudRole = "viewer"
)

type Capability string

const (
	CapabilityRead                  Capability = "read"
	CapabilityWrite                 Capability = "write"
	CapabilityWorkspaceRegistration Capability = "workspace_registration"
	CapabilityGit                   Capability = "git"
	CapabilitySystem                Capability = "system"
	CapabilityTerminal              Capability = "terminal"
	CapabilityAI                    Capability = "ai"
	CapabilityRuntime               Capability = "runtime"
	CapabilityVerification          Capability = "verification"
)

type CloudUser struct {
	ID      string    `json:"id"`
	Email   string    `json:"email,omitempty"`
	Name    string    `json:"name,omitempty"`
	Role    CloudRole `json:"role"`
	Subject string    `json:"subject,omitempty"`
}

type AgentConnection struct {
	Available bool   `json:"available"`
	Status    string `json:"status"`
}

type CloudAgent struct {
	ID         string    `json:"id"`
	UserID     string    `json:"userId"`
	Name       string    `json:"name"`
	Platform   string    `json:"platform,omitempty"`
	Status     string    `json:"status"`
	LastSeenAt time.Time `json:"lastSeenAt,omitempty"`
}

type WorkspaceImportIssue struct {
	Field   string `json:"field" yaml:"field"`
	Code    string `json:"code" yaml:"code"`
	Message string `json:"message" yaml:"message"`
}

type WorkspaceImportCandidate struct {
	CandidateKey string                 `json:"candidateKey" yaml:"candidateKey"`
	Position     int                    `json:"position" yaml:"position"`
	Workspace    WorkspaceInput         `json:"workspace" yaml:"workspace"`
	Status       string                 `json:"status" yaml:"status"`
	Issues       []WorkspaceImportIssue `json:"issues" yaml:"issues"`
	Selected     bool                   `json:"selected" yaml:"selected"`
}

type WorkspaceImportSummary struct {
	Valid             int `json:"valid" yaml:"valid"`
	Invalid           int `json:"invalid" yaml:"invalid"`
	Duplicate         int `json:"duplicate" yaml:"duplicate"`
	AlreadyRegistered int `json:"alreadyRegistered" yaml:"alreadyRegistered"`
}

type WorkspaceImportPreview struct {
	SourcePath        string                     `json:"sourcePath" yaml:"sourcePath"`
	DestinationPath   string                     `json:"destinationPath" yaml:"destinationPath"`
	SourceFingerprint string                     `json:"sourceFingerprint" yaml:"sourceFingerprint"`
	Candidates        []WorkspaceImportCandidate `json:"candidates" yaml:"candidates"`
	Summary           WorkspaceImportSummary     `json:"summary" yaml:"summary"`
}

type WorkspaceImportRequest struct {
	SourcePath    string   `json:"sourcePath" yaml:"sourcePath"`
	CandidateKeys []string `json:"candidateKeys" yaml:"candidateKeys"`
}

type WorkspaceImportResult struct {
	CandidateKey string           `json:"candidateKey" yaml:"candidateKey"`
	Workspace    *WorkspaceConfig `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Status       string           `json:"status" yaml:"status"`
	Scan         *ScanResult      `json:"scan,omitempty" yaml:"scan,omitempty"`
	Message      string           `json:"message,omitempty" yaml:"message,omitempty"`
}

type BranchScanMetadata struct {
	WorkspaceID             string        `json:"workspaceId" yaml:"workspaceId"`
	Branch                  string        `json:"branch" yaml:"branch"`
	BranchRef               string        `json:"branchRef,omitempty" yaml:"branchRef,omitempty"`
	Commit                  string        `json:"commit,omitempty" yaml:"commit,omitempty"`
	SourceMode              string        `json:"sourceMode,omitempty" yaml:"sourceMode,omitempty"`
	Editable                bool          `json:"editable" yaml:"editable"`
	SourceConfigurationHash string        `json:"sourceConfigurationHash,omitempty" yaml:"sourceConfigurationHash,omitempty"`
	WorkingTreeHash         string        `json:"workingTreeHash,omitempty" yaml:"workingTreeHash,omitempty"`
	ScannedAt               time.Time     `json:"scannedAt" yaml:"scannedAt"`
	Warnings                []ScanWarning `json:"warnings" yaml:"warnings"`
}

type WorkspaceInput struct {
	Name             string                    `json:"name" yaml:"name"`
	Path             string                    `json:"path" yaml:"path"`
	BaselineBranch   string                    `json:"baselineBranch" yaml:"baselineBranch"`
	Sources          []string                  `json:"sources" yaml:"sources"`
	RegistrationMode WorkspaceRegistrationMode `json:"registrationMode,omitempty" yaml:"registrationMode,omitempty"`
	RemoteURL        string                    `json:"remoteUrl,omitempty" yaml:"remoteUrl,omitempty"`
	CloneRoot        string                    `json:"cloneRoot,omitempty" yaml:"cloneRoot,omitempty"`
	Jira             *JiraConnection           `json:"jira,omitempty" yaml:"jira,omitempty"`
	Knowledge        *KnowledgeSettings        `json:"knowledge,omitempty" yaml:"knowledge,omitempty"`
	Runtime          *WorkspaceRuntimeConfig   `json:"runtime,omitempty" yaml:"runtime,omitempty"`
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
	FileKindImage       FileKind = "image"
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

type WorkstreamBranchLoadInput struct {
	Branch string `json:"branch,omitempty" yaml:"branch,omitempty"`
	Force  bool   `json:"force,omitempty" yaml:"force,omitempty"`
}

type WorkstreamBranchLoadResult struct {
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

type VerificationTestSelection struct {
	SelectedSpecs []string              `json:"selectedSpecs" yaml:"selectedSpecs"`
	Environment   string                `json:"environment,omitempty" yaml:"environment,omitempty"`
	DisplayMode   AutomationDisplayMode `json:"displayMode,omitempty" yaml:"displayMode,omitempty"`
	UpdatedAt     time.Time             `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
}

type AutomationTestPath struct {
	Path string `json:"path" yaml:"path"`
}

type DiscoveredVerificationSpec struct {
	Path       string `json:"path" yaml:"path"`
	Runner     string `json:"runner" yaml:"runner"`
	SourcePath string `json:"sourcePath,omitempty" yaml:"sourcePath,omitempty"`
}

type ItemVerificationTests struct {
	Selection       VerificationTestSelection    `json:"selection" yaml:"selection"`
	DiscoveredSpecs []DiscoveredVerificationSpec `json:"discoveredSpecs" yaml:"discoveredSpecs"`
}

type ItemStatusUpdateInput struct {
	Status               ItemStatus `json:"status" yaml:"status"`
	MaterializeConfirmed bool       `json:"materializeConfirmed,omitempty" yaml:"materializeConfirmed,omitempty"`
}

type NewItemInput struct {
	WorkspaceID   string     `json:"workspaceId" yaml:"workspaceId"`
	Source        string     `json:"source" yaml:"source"`
	Scope         string     `json:"scope" yaml:"scope"`
	Identifier    string     `json:"identifier" yaml:"identifier"`
	Title         string     `json:"title" yaml:"title"`
	Status        ItemStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Owner         string     `json:"owner,omitempty" yaml:"owner,omitempty"`
	Tags          []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
	JiraKey       string     `json:"jiraKey,omitempty" yaml:"jiraKey,omitempty"`
	InitialReadme string     `json:"initialReadme,omitempty" yaml:"initialReadme,omitempty"`
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

type GitActivityPath struct {
	Path    string          `json:"path" yaml:"path"`
	OldPath string          `json:"oldPath,omitempty" yaml:"oldPath,omitempty"`
	Status  GitChangeStatus `json:"status" yaml:"status"`
}

type GitActivityEntry struct {
	Commit      string            `json:"commit" yaml:"commit"`
	CommittedAt time.Time         `json:"committedAt" yaml:"committedAt"`
	Author      string            `json:"author" yaml:"author"`
	Message     string            `json:"message" yaml:"message"`
	Paths       []GitActivityPath `json:"paths" yaml:"paths"`
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

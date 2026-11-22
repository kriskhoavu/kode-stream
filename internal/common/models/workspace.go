package models

import "time"

type WorkspaceConfig struct {
	ID                         string                    `json:"id" yaml:"id"`
	Name                       string                    `json:"name" yaml:"name"`
	Path                       string                    `json:"path" yaml:"path"`
	Location                   WorkspaceLocation         `json:"location,omitempty" yaml:"location,omitempty"`
	AccessMode                 WorkspaceAccessMode       `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	OwnerUserID                string                    `json:"ownerUserId,omitempty" yaml:"ownerUserId,omitempty"`
	AgentID                    string                    `json:"agentId,omitempty" yaml:"agentId,omitempty"`
	Provider                   string                    `json:"provider,omitempty" yaml:"provider,omitempty"`
	ProviderInstanceID         string                    `json:"providerInstanceId,omitempty" yaml:"providerInstanceId,omitempty"`
	ProviderRepository         string                    `json:"providerRepository,omitempty" yaml:"providerRepository,omitempty"`
	SelectedRef                string                    `json:"selectedRef,omitempty" yaml:"selectedRef,omitempty"`
	ResolvedCommitSHA          string                    `json:"resolvedCommitSha,omitempty" yaml:"resolvedCommitSha,omitempty"`
	CloudPublicationRevision   int64                     `json:"cloudPublicationRevision,omitempty" yaml:"cloudPublicationRevision,omitempty"`
	LocalRootLabel             string                    `json:"localRootLabel,omitempty" yaml:"localRootLabel,omitempty"`
	PublishedSummary           bool                      `json:"publishedSummary,omitempty" yaml:"publishedSummary,omitempty"`
	ScanStatus                 string                    `json:"scanStatus,omitempty" yaml:"scanStatus,omitempty"`
	BaselineBranch             string                    `json:"baselineBranch" yaml:"baselineBranch"`
	RegistrationMode           WorkspaceRegistrationMode `json:"registrationMode,omitempty" yaml:"registrationMode,omitempty"`
	RemoteURL                  string                    `json:"remoteUrl,omitempty" yaml:"remoteUrl,omitempty"`
	ClonePathManaged           bool                      `json:"clonePathManaged,omitempty" yaml:"clonePathManaged,omitempty"`
	ManagedCloneRoot           string                    `json:"managedCloneRoot,omitempty" yaml:"managedCloneRoot,omitempty"`
	ManagedCloneID             string                    `json:"managedCloneId,omitempty" yaml:"managedCloneId,omitempty"`
	ManagedCloneVerified       bool                      `json:"managedCloneVerified,omitempty" yaml:"managedCloneVerified,omitempty"`
	ManagedCloneCleanupPending bool                      `json:"managedCloneCleanupPending,omitempty" yaml:"managedCloneCleanupPending,omitempty"`
	LastSelectedBranch         string                    `json:"lastSelectedBranch,omitempty" yaml:"lastSelectedBranch,omitempty"`
	Sources                    []WorkspaceSource         `json:"sources" yaml:"sources"`
	CreatedAt                  time.Time                 `json:"createdAt" yaml:"createdAt"`
	LastScannedAt              time.Time                 `json:"lastScannedAt,omitempty" yaml:"lastScannedAt,omitempty"`
	Jira                       *JiraConnection           `json:"jira,omitempty" yaml:"jira,omitempty"`
	Knowledge                  *KnowledgeSettings        `json:"knowledge,omitempty" yaml:"knowledge,omitempty"`
	Runtime                    *WorkspaceRuntimeConfig   `json:"runtime,omitempty" yaml:"runtime,omitempty"`
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
	WorkspaceLocationLocalPath           WorkspaceLocation = "local_path"
	WorkspaceLocationCloudAgent          WorkspaceLocation = "cloud_agent"
	WorkspaceLocationCloudRemoteSnapshot WorkspaceLocation = "cloud_remote_snapshot"
)

type WorkspaceAccessMode string

const (
	WorkspaceAccessModeAgentBacked    WorkspaceAccessMode = "agent_backed"
	WorkspaceAccessModeRemoteSnapshot WorkspaceAccessMode = "remote_snapshot"
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
	CapabilityHarness               Capability = "harness"
)

type DeploymentTopology string

const (
	DeploymentTopologyLocalApplication  DeploymentTopology = "local_application"
	DeploymentTopologyCloudControlPlane DeploymentTopology = "cloud_control_plane"
)

type WorkspaceContentProvider string

const (
	WorkspaceContentProviderLocalCheckout    WorkspaceContentProvider = "local_checkout"
	WorkspaceContentProviderAgentCheckout    WorkspaceContentProvider = "agent_checkout"
	WorkspaceContentProviderProviderSnapshot WorkspaceContentProvider = "provider_snapshot"
)

type WorkspaceExecutionProvider string

const (
	WorkspaceExecutionProviderLocalProcess WorkspaceExecutionProvider = "local_process"
	WorkspaceExecutionProviderCloudAgent   WorkspaceExecutionProvider = "cloud_agent"
	WorkspaceExecutionProviderNone         WorkspaceExecutionProvider = "none"
)

type AppStateDatastore string

const (
	AppStateDatastoreDataDir  AppStateDatastore = "data_dir"
	AppStateDatastoreSQLite   AppStateDatastore = "sqlite"
	AppStateDatastorePostgres AppStateDatastore = "postgres"
)

type WorkspaceProviderAxes struct {
	Topology          DeploymentTopology         `json:"topology"`
	ContentProvider   WorkspaceContentProvider   `json:"contentProvider"`
	ExecutionProvider WorkspaceExecutionProvider `json:"executionProvider"`
	Datastore         AppStateDatastore          `json:"datastore,omitempty"`
}

type WorkspaceAction string

const (
	WorkspaceActionLayoutMove      WorkspaceAction = "layout.move"
	WorkspaceActionRepositoryRead  WorkspaceAction = "repo.read"
	WorkspaceActionGitStatus       WorkspaceAction = "git.status"
	WorkspaceActionTerminalLaunch  WorkspaceAction = "terminal.launch"
	WorkspaceActionVerificationRun WorkspaceAction = "verification.run"
)

type ActionCapabilityState string

const (
	ActionCapabilityAvailable   ActionCapabilityState = "available"
	ActionCapabilityUnavailable ActionCapabilityState = "unavailable"
	ActionCapabilityUnsupported ActionCapabilityState = "unsupported"
	ActionCapabilityForbidden   ActionCapabilityState = "forbidden"
	ActionCapabilityConflicted  ActionCapabilityState = "conflicted"
)

type CapabilityRecoveryAction struct {
	Action string `json:"action"`
	Label  string `json:"label"`
}

type ActionCapability struct {
	Action          WorkspaceAction            `json:"action"`
	State           ActionCapabilityState      `json:"state"`
	ReasonCode      string                     `json:"reasonCode,omitempty"`
	Message         string                     `json:"message,omitempty"`
	RecoveryActions []CapabilityRecoveryAction `json:"recoveryActions"`
}

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

type CommandEnvelope struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	WorkspaceID string            `json:"workspaceId"`
	UserID      string            `json:"userId"`
	AgentID     string            `json:"agentId"`
	Capability  Capability        `json:"capability"`
	Payload     map[string]string `json:"payload,omitempty"`
}

type CommandResult struct {
	Accepted bool            `json:"accepted"`
	Command  CommandEnvelope `json:"command"`
	Log      string          `json:"log,omitempty"`
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
	SourcePath        string   `json:"sourcePath" yaml:"sourcePath"`
	SourceFingerprint string   `json:"sourceFingerprint" yaml:"sourceFingerprint"`
	CandidateKeys     []string `json:"candidateKeys" yaml:"candidateKeys"`
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
	Name               string                    `json:"name" yaml:"name"`
	Path               string                    `json:"path" yaml:"path"`
	AccessMode         WorkspaceAccessMode       `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	Provider           string                    `json:"provider,omitempty" yaml:"provider,omitempty"`
	ProviderInstanceID string                    `json:"providerInstanceId,omitempty" yaml:"providerInstanceId,omitempty"`
	ProviderRepository string                    `json:"providerRepository,omitempty" yaml:"providerRepository,omitempty"`
	SelectedRef        string                    `json:"selectedRef,omitempty" yaml:"selectedRef,omitempty"`
	BaselineBranch     string                    `json:"baselineBranch" yaml:"baselineBranch"`
	Sources            []WorkspaceSource         `json:"sources" yaml:"sources"`
	RegistrationMode   WorkspaceRegistrationMode `json:"registrationMode,omitempty" yaml:"registrationMode,omitempty"`
	RemoteURL          string                    `json:"remoteUrl,omitempty" yaml:"remoteUrl,omitempty"`
	CloneRoot          string                    `json:"cloneRoot,omitempty" yaml:"cloneRoot,omitempty"`
	Jira               *JiraConnection           `json:"jira,omitempty" yaml:"jira,omitempty"`
	Knowledge          *KnowledgeSettings        `json:"knowledge,omitempty" yaml:"knowledge,omitempty"`
	Runtime            *WorkspaceRuntimeConfig   `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	ManagedCloneRoot   string                    `json:"-" yaml:"-"`
	ManagedCloneID     string                    `json:"-" yaml:"-"`
}

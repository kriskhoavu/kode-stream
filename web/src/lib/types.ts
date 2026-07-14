export type ItemStatus = 'unsorted' | 'draft' | 'in_progress' | 'review' | 'done';

export type AuditStatus = 'success' | 'blocked' | 'failed';
export type HealthStatus = 'ok' | 'warning' | 'failed';

export interface AuditEvent {
  id: string;
  time: string;
  workspaceId?: string;
  itemId?: string;
  operation: string;
  status: AuditStatus;
  message: string;
  paths: string[];
  durationMs: number;
  error?: string;
}

export interface HealthCheck {
  name: string;
  status: HealthStatus;
  message: string;
  recoveryHint?: string;
}

export interface WorkspaceHealth {
  workspaceId: string;
  checkedAt: string;
  checks: HealthCheck[];
  summary: HealthStatus;
}

export interface SearchResult {
  id: string;
  type: 'item' | 'workspace' | 'branch' | 'savedFilter';
  title: string;
  subtitle: string;
  context: string;
  workspaceId?: string;
  itemId?: string;
  route: string;
  score: number;
}

export interface SavedFilter {
  id: string;
  name: string;
  route: string;
  workspaceId?: string;
  filters: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

export interface RecentItem {
  itemId: string;
  workspaceId: string;
  title: string;
  subtitle?: string;
  route: string;
  openedAt: string;
}

export interface WorkspaceConfig {
  id: string;
  name: string;
  path: string;
  location?: WorkspaceLocation;
  baselineBranch: string;
  registrationMode?: WorkspaceRegistrationMode;
  remoteUrl?: string;
  clonePathManaged?: boolean;
  lastSelectedBranch?: string;
  sources: string[];
  createdAt: string;
  lastScannedAt?: string;
  jira?: JiraConnection;
  knowledge?: KnowledgeSettings;
  runtime?: WorkspaceRuntimeConfig;
}

export type RuntimeType = 'docker-compose' | 'procfile' | 'makefile' | 'custom';
export type RebuildPolicy = 'never' | 'changed-only' | 'always';
export type RuntimeHealthCheckType = 'http' | 'command';

export interface RuntimeHealthCheck {
  name?: string;
  type: RuntimeHealthCheckType;
  target: string;
  timeoutSeconds?: number;
}

export interface RuntimeVerifyCommands {
  smoke: string;
  critical?: string;
  full?: string;
}

export interface RuntimeCommandSet {
  up: string;
  down: string;
  rebuildChanged?: string;
  verify: RuntimeVerifyCommands;
}

export interface RuntimeArtifacts {
  paths?: string[];
}

export type AutomationRunner = 'cypress' | 'playwright';

export interface RuntimeAutomationConfig {
  enabled: boolean;
  repositoryPath?: string;
  runner?: AutomationRunner;
  defaultEnvironment?: string;
  commandTemplate?: string;
  artifactPaths?: string[];
}

export interface WorkspaceRuntimeConfig {
  type: RuntimeType;
  configPath?: string;
  rebuildPolicy?: RebuildPolicy;
  commands: RuntimeCommandSet;
  healthChecks?: RuntimeHealthCheck[];
  artifacts?: RuntimeArtifacts;
  automation?: RuntimeAutomationConfig;
}

export type VerifyProfile = 'smoke' | 'critical' | 'full';
export type VerificationRunMode = 'runtime' | 'automation';
export type AutomationDisplayMode = 'silent' | 'visible';
export type VerificationStatus = 'queued' | 'running' | 'passed' | 'failed';
export type VerificationFailureType = 'boot_failure' | 'test_failure' | 'infra_failure';

export interface VerificationStepResult {
  step: string;
  status: 'ok' | 'failed' | string;
  message?: string;
  durationMs: number;
  at: string;
}

export interface RunArtifact {
  kind: string;
  path: string;
  sizeBytes: number;
  createdAt: string;
}

export interface VerificationJob {
  id: string;
  workspaceId: string;
  mode?: VerificationRunMode;
  profile: VerifyProfile;
  environment?: string;
  displayMode?: AutomationDisplayMode | string;
  selectedSpecs?: string[];
  automationRepoPath?: string;
  renderedCommand?: string;
  status: VerificationStatus;
  failureType?: VerificationFailureType;
  exitCode: number;
  trigger?: string;
  provider?: string;
  sessionId?: string;
  terminalMode?: 'embedded' | 'external' | string;
  startedAt?: string;
  finishedAt?: string;
  steps: VerificationStepResult[];
  artifacts: RunArtifact[];
  runtime?: WorkspaceRuntimeConfig;
}

export interface VerificationTestSelection {
  selectedSpecs: string[];
  environment?: string;
  displayMode?: AutomationDisplayMode;
  updatedAt?: string;
}

export interface DiscoveredVerificationSpec {
  path: string;
  runner: AutomationRunner | string;
  sourcePath?: string;
}

export interface ItemVerificationTests {
  selection: VerificationTestSelection;
  discoveredSpecs: DiscoveredVerificationSpec[];
}

export interface CreateVerificationJobInput {
  profile?: VerifyProfile;
  mode?: VerificationRunMode;
  environment?: string;
  displayMode?: AutomationDisplayMode;
  selectedSpecs?: string[];
  trigger?: string;
  provider?: string;
  sessionId?: string;
  terminalMode?: string;
}

export interface KnowledgeSettings {
  enabled?: boolean;
  enrichExecutable?: string;
  enrichArgs?: string[];
}

export type KnowledgeLinkResolution = 'resolved' | 'unresolved';
export interface KnowledgeLink { sourceSlug: string; rawTarget: string; label?: string; targetSlug?: string; resolution: KnowledgeLinkResolution; }
export interface KnowledgeWarning { workspaceId?: string; wikiRoot?: string; path?: string; slug?: string; code: string; message: string; }
export interface KnowledgePage { slug: string; title: string; path: string; domain: string; pageType?: string; roles: string[]; topics: string[]; summary?: string; sourceRefs: string[]; sourceCount?: number; links: KnowledgeLink[]; backlinks: string[]; }
export interface KnowledgeWiki { workspaceId: string; root: string; displayName: string; pages: KnowledgePage[]; warnings: KnowledgeWarning[]; indexedAt: string; }
export interface KnowledgePageDetail extends KnowledgePage { content: FileContent; warnings: KnowledgeWarning[]; }
export interface KnowledgePagesResponse { pages: KnowledgePage[]; warnings: KnowledgeWarning[]; }
export interface KnowledgeGraphNode { id: string; title: string; domain: string; pageType?: string; roles: string[]; topics: string[]; path: string; inbound: number; outbound: number; }
export interface KnowledgeGraphEdge { source: string; target: string; }
export interface KnowledgeGraph { nodes: KnowledgeGraphNode[]; edges: KnowledgeGraphEdge[]; totalNodes: number; totalEdges: number; truncated: boolean; }
export interface KnowledgeActionResult { ok: boolean; operation: 'rescan' | 'sync' | 'enrich'; message?: string; wikis: KnowledgeWiki[]; warnings: KnowledgeWarning[]; log?: string; logTruncated: boolean; completedAt: string; }

export interface JiraConnection {
  deploymentType: 'cloud' | 'server';
  baseUrl: string;
  projectKey: string;
  accountEmail?: string;
  tokenEnvVar: string;
}

export interface JiraConnectionTest {
  ok: boolean;
  deploymentType: 'cloud' | 'server';
  projectKey: string;
  message: string;
  recoveryHint?: string;
}

export type JiraIssueStateName = 'not_configured' | 'invalid_identifier' | 'project_mismatch' | 'not_found' | 'available' | 'authentication_failed' | 'forbidden' | 'unavailable';
export interface JiraPerson { displayName: string; accountId?: string; email?: string; }
export interface JiraAttachment { id: string; filename: string; mediaType: string; sizeBytes: number; createdAt?: string; author: JiraPerson; }
export interface JiraIssue { key: string; summary: string; status: string; description: string; issueType: string; assignee?: JiraPerson; reporter?: JiraPerson; priority?: string; labels: string[]; createdAt?: string; updatedAt?: string; browserUrl: string; attachments: JiraAttachment[]; }
export interface JiraIssueState { state: JiraIssueStateName; issue?: JiraIssue; message?: string; recoveryHint?: string; refreshedAt?: string; }

export interface WorkspaceInput {
  name: string;
  path?: string;
  baselineBranch: string;
  sources: string[];
  registrationMode?: WorkspaceRegistrationMode;
  remoteUrl?: string;
  cloneRoot?: string;
  jira?: JiraConnection;
  knowledge?: KnowledgeSettings;
  runtime?: WorkspaceRuntimeConfig;
}

export type WorkspaceRegistrationMode = 'local_path' | 'remote_clone' | 'existing_workspace';
export type WorkspaceLocation = 'local_path' | 'cloud_agent';
export type RuntimeMode = 'local' | 'cloud';
export type CloudRole = 'admin' | 'editor' | 'viewer';
export type Capability = 'read' | 'write' | 'workspace_registration' | 'git' | 'system' | 'terminal' | 'ai' | 'runtime' | 'verification';

export interface CloudUser {
  id: string;
  email?: string;
  name?: string;
  role: CloudRole;
  subject?: string;
}

export interface AgentConnection {
  available: boolean;
  status: string;
}

export interface RuntimeContext {
  mode: RuntimeMode;
  user?: CloudUser;
  role?: CloudRole;
  capabilities: Record<Capability, boolean>;
  agent: AgentConnection;
}

export interface WorkspaceImportIssue {
  field: string;
  code: string;
  message: string;
}

export type WorkspaceImportCandidateStatus = 'valid' | 'invalid' | 'duplicate' | 'already_registered';
export type WorkspaceImportResultStatus = 'indexed' | 'scan_failed' | 'skipped' | 'failed';

export interface WorkspaceImportCandidate {
  candidateKey: string;
  position: number;
  workspace: WorkspaceInput;
  status: WorkspaceImportCandidateStatus;
  issues: WorkspaceImportIssue[];
  selected: boolean;
}

export interface WorkspaceImportSummary {
  valid: number;
  invalid: number;
  duplicate: number;
  alreadyRegistered: number;
}

export interface WorkspaceImportPreview {
  sourcePath: string;
  destinationPath: string;
  sourceFingerprint: string;
  candidates: WorkspaceImportCandidate[];
  summary: WorkspaceImportSummary;
}

export interface WorkspaceImportRequest {
  sourcePath: string;
  candidateKeys: string[];
}

export interface WorkspaceImportResult {
  candidateKey: string;
  workspace?: WorkspaceConfig;
  status: WorkspaceImportResultStatus;
  scan?: ScanResult;
  message?: string;
}

export interface WorkspaceCreateResult {
  workspace: WorkspaceConfig;
  operationLog?: string;
}

export interface SystemConfigPaths {
  dataDir: string;
  defaultDataDir: string;
  cloneRootDir: string;
  registryFile?: string;
  restartRequired?: boolean;
}

export type AICapabilityKind = 'provider' | 'terminal';

export interface AICapability {
  id: string;
  kind: AICapabilityKind;
  detected: boolean;
  configured: boolean;
  executable: string;
  reason?: string;
}

export interface AICapabilityDescriptor {
  id: string;
  name: string;
  description?: string;
  kind: 'skill' | 'agent' | string;
  provider: string;
  scope: 'workspace' | 'global' | string;
  sourcePath: string;
}

export interface AIProviderCapabilityCatalog {
  provider: string;
  skills: AICapabilityDescriptor[];
  agents: AICapabilityDescriptor[];
  supportsNativeSelection: boolean;
  supportsPromptFallback: boolean;
}

export interface AILaunchTemplate {
  enabled: boolean;
  executable: string;
  args: string[];
}

export interface AISettings {
  defaultProvider: string;
  defaultTerminal: string;
  providers: Record<string, AILaunchTemplate>;
  terminals: Record<string, AILaunchTemplate>;
}

export interface AIPlanPreset {
  id: string;
  name: string;
  prompt: string;
  contextMode: AISessionLaunchInput['contextMode'];
  provider?: string;
}

export interface AISessionEligibility {
  editable: boolean;
  cardContextAvailable: boolean;
  missing: string[];
}

export interface AISessionLaunchInput {
  provider: string;
  terminal: string;
  contextMode: 'workspace_only' | 'card_context';
	presetId?: string;
	promptDraft?: string;
	customPrompt?: string;
	selectedSkills?: string[];
	selectedAgents?: string[];
	surface?: 'external' | 'embedded';
}

export interface AISessionLaunchResult extends AISessionLaunchInput {
  accepted: boolean;
  startedAt: string;
}

export type EmbeddedAISessionState = 'starting' | 'running' | 'exited' | 'cancelled' | 'failed';

export interface EmbeddedAISession {
	id: string;
	itemId: string;
	itemIdentifier?: string;
	itemTitle?: string;
	workspaceId: string;
	provider: string;
	intent: AISessionLaunchInput['contextMode'];
	state: EmbeddedAISessionState;
	startedAt: string;
	exitCode?: number;
}

export interface EmbeddedAISessionResult {
	session: EmbeddedAISession;
	grant: { sessionId: string; token: string; expiresAt: string };
}

export interface SourceStructureSettings {
  version: number;
  cards: SourceStructureCard[];
}

export interface SourceStructureCard {
  pathPattern: string;
  fields: SourceStructureFields;
}

export interface SourceStructureFields {
  source?: string;
  item?: string;
  scope: string;
  identifier: string;
  title?: string;
  status?: string;
  owner?: string;
  tags?: string[];
}

export interface SourceStructureProposal {
  id: string;
  label: string;
  summary: string;
  confidence: 'high' | 'medium' | 'low' | string;
  card: SourceStructureCard;
  preview: SourceStructurePreview[];
}

export interface SourceStructurePreview {
  path: string;
  source?: string;
  item?: string;
  scope: string;
  identifier: string;
  title: string;
  status: ItemStatus;
  tags: string[];
}

export interface SourceSettingsResult {
  directory: string;
  exists: boolean;
  mode?: 'structured' | 'unstructured' | 'empty' | 'unknown';
  settings: SourceStructureSettings;
  warnings: { itemPath?: string; message: string }[];
  proposals?: SourceStructureProposal[];
  preview?: SourceStructurePreview[];
  scan?: ScanResult;
}

export interface ItemSummary {
  id: string;
  workspaceId: string;
  workspaceName: string;
  branch: string;
  branchRef?: string;
  commit?: string;
  sourceMode?: SourceMode;
  editable?: boolean;
  scope: string;
  identifier: string;
  title: string;
  status: ItemStatus;
  owner?: string;
  author?: string;
  tags: string[];
  updatedAt?: string;
  description?: string;
  metadataSource: string;
  itemPath?: string;
}

export interface ItemDocument {
  id: string;
  role: string;
  track?: string;
  path: string;
  label: string;
}

export interface ItemDetail extends ItemSummary {
  documents: ItemDocument[];
  metadata: Record<string, unknown>;
  warnings?: { itemPath?: string; message: string }[];
  counts: { files: number };
}

export interface FileNode {
  id: string;
  name: string;
  path: string;
  type: 'file' | 'directory';
  children?: FileNode[];
}

export type FileKind = 'markdown' | 'html' | 'json' | 'yaml' | 'code' | 'text' | 'image' | 'unsupported';

export interface FileContent {
  id: string;
  path: string;
  content: string;
  language: string;
  hash: string;
  kind: FileKind;
  sizeBytes: number;
  truncated?: boolean;
  editable: boolean;
}

export interface FileSaveInput {
  content: string;
  expectedHash?: string;
  materializeConfirmed?: boolean;
}

export type SourceMode = 'working_tree' | 'snapshot';

export interface BranchScanMetadata {
  workspaceId: string;
  branch: string;
  branchRef?: string;
  commit?: string;
  sourceMode?: SourceMode;
  editable: boolean;
  sourceConfigurationHash?: string;
  scannedAt: string;
  warnings: { itemPath?: string; message: string }[];
}

export interface WorkstreamBranchLoadResult {
  workspaceId: string;
  branch: string;
  selectedBranch: string;
  branchRef: string;
  commit: string;
  currentCheckoutBranch: string;
  sourceMode: SourceMode;
  mode: SourceMode;
  editable: boolean;
  scannedAt: string;
  itemCount: number;
  warnings: { itemPath?: string; message: string }[];
  items: ItemSummary[];
}

export interface WorkspaceTreeEntry {
  id: string;
  name: string;
  path: string;
  type: 'file' | 'directory';
  hasChildren: boolean;
  ignored: boolean;
  hidden: boolean;
  kind?: FileKind;
  language?: string;
  sizeBytes?: number;
  editable: boolean;
}

export interface WorkspaceDirectoryListing {
  workspaceId: string;
  path: string;
  entries: WorkspaceTreeEntry[];
  hiddenCount: number;
}

export interface WorkspaceFileSaveInput {
  path: string;
  content: string;
  expectedHash: string;
}

export interface WorkspaceFileRevertInput {
  path: string;
}

export interface WorkspaceFileWriteResult {
  file: FileContent;
  refreshed: boolean;
}

export interface WorkspacePathSearchResult {
  id: string;
  workspaceId: string;
  workspaceName: string;
  name: string;
  path: string;
  type: 'file' | 'directory';
  ignored: boolean;
  context: string;
}

export interface WorkspacePathSearchResponse {
  results: WorkspacePathSearchResult[];
  truncated: boolean;
}

export type ExplorerTreeMode = 'sources' | 'all';

export interface WorkspaceContentSearchResult {
  id: string;
  workspaceId: string;
  workspaceName: string;
  itemId?: string;
  path: string;
  fileId?: string;
  name: string;
  kind: FileKind;
  language: string;
  lineNumber: number;
  columnStart: number;
  columnEnd: number;
  snippet: string;
  ignored: boolean;
}

export interface WorkspaceContentSearchResponse {
  results: WorkspaceContentSearchResult[];
  truncated: boolean;
  filesVisited: number;
  bytesRead: number;
  skippedFiles: number;
}

export interface ContentSearchSelection {
  workspaceId: string;
  itemId?: string;
  path: string;
  fileId?: string;
  lineNumber: number;
  columnStart: number;
  columnEnd: number;
}

export interface WorkspacePathGitState {
  path: string;
  oldPath?: string;
  status: GitChangeStatus;
  staged: boolean;
  conflict: boolean;
}

export interface WorkspaceFileCreateInput {
  parentPath: string;
  name: string;
  content: string;
}

export interface WorkspaceDirectoryCreateInput {
  parentPath: string;
  name: string;
}

export interface WorkspacePathRenameInput {
  path: string;
  destinationPath: string;
}

export interface WorkspacePathMutationResult {
  workspaceId: string;
  path: string;
  type: 'file' | 'directory';
  invalidatedPaths: string[];
  refreshed: boolean;
}

export interface ItemMetadataUpdateInput {
  title?: string;
  scope?: string;
  identifier?: string;
  status?: ItemStatus;
  owner?: string;
  tags?: string[];
  materializeConfirmed?: boolean;
}

export interface ItemStatusUpdateInput {
  status: ItemStatus;
  materializeConfirmed?: boolean;
}

export interface NewItemInput {
  workspaceId: string;
  source: string;
  scope: string;
  identifier: string;
  title?: string;
  status?: ItemStatus;
  owner?: string;
  tags?: string[];
  jiraKey?: string;
  initialReadme?: string;
}

export interface WriteResult {
  item: ItemDetail;
  scannedAt: string;
}

export interface ScanResult {
  workspaceId: string;
  scannedAt: string;
  itemCount: number;
  warnings: { itemPath?: string; message: string }[];
}

export interface PathSelection {
  path: string;
}

export interface AppState {
  version: string;
  workspaceCount: number;
  itemCount: number;
  updatedAt: string;
  mode?: RuntimeMode;
  user?: CloudUser;
  role?: CloudRole;
  capabilities?: Record<Capability, boolean>;
  agent?: AgentConnection;
}

export type GitChangeStatus = 'modified' | 'added' | 'deleted' | 'renamed' | 'copied' | 'untracked' | 'conflicted';

export interface GitChange {
  path: string;
  oldPath?: string;
  status: GitChangeStatus;
  staged: boolean;
  conflict: boolean;
}

export interface GitActivityPath {
  path: string;
  oldPath?: string;
  status: GitChangeStatus;
}

export interface GitActivityEntry {
  commit: string;
  committedAt: string;
  author: string;
  message: string;
  paths: GitActivityPath[];
}

export interface GitStatus {
  workspaceId: string;
  branch: string;
  upstream?: string;
  ahead: number;
  behind: number;
  dirty: boolean;
  conflicted: boolean;
  changes: GitChange[];
}

export interface WorkspaceBranches {
  workspaceId: string;
  current: string;
  branches: string[];
}

export interface GitCommitInput {
  message: string;
  paths: string[];
}

export interface GitOperationInput {
  confirm?: boolean;
}

export interface BranchCreateInput {
  name: string;
  startPoint?: string;
  checkout?: boolean;
}

export interface BranchSwitchInput {
  name: string;
  confirm?: boolean;
}

export interface GitOperationResult {
  ok: boolean;
  message?: string;
  recoveryHint?: string;
  status: GitStatus;
}

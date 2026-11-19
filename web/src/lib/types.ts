export type ItemStatus = 'unsorted' | 'draft' | 'in_progress' | 'review' | 'done';

export type AuditStatus = 'success' | 'blocked' | 'failed';
export type HealthStatus = 'ok' | 'warning' | 'failed';

export interface AuditEvent {
  id: string;
  time: string;
  ownerUserId?: string;
  actorUserId?: string;
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

export type SourceSurface = 'workstream' | 'knowledge';
export interface WorkspaceSource { path: string; shownIn: SourceSurface[]; }
// The server accepts a bare path and answers the object form; readers go
// through lib/sources.ts, which applies the same defaults.
export type WorkspaceSourceLike = string | WorkspaceSource;

export interface WorkspaceConfig {
  id: string;
  name: string;
  path: string;
  location?: WorkspaceLocation;
  accessMode?: WorkspaceAccessMode;
  ownerUserId?: string;
  agentId?: string;
  provider?: string;
  providerInstanceId?: string;
  providerRepository?: string;
  selectedRef?: string;
  resolvedCommitSha?: string;
  localRootLabel?: string;
  publishedSummary?: boolean;
  scanStatus?: string;
  baselineBranch: string;
  registrationMode?: WorkspaceRegistrationMode;
  remoteUrl?: string;
  clonePathManaged?: boolean;
  managedCloneRoot?: string;
  managedCloneId?: string;
  managedCloneVerified?: boolean;
  lastSelectedBranch?: string;
  sources: WorkspaceSourceLike[];
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
  root?: 'workspace' | 'automation' | string;
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
	startFingerprint?: RepositoryFingerprint;
	finishFingerprint?: RepositoryFingerprint;
	currentFingerprint?: RepositoryFingerprint;
	freshness?: VerificationFreshness;
}

export type VerificationFreshness = 'fresh' | 'stale' | 'inconclusive';
export interface RepositoryFingerprint { value: string; branch: string; commit: string; }

export interface VerificationTestSelection {
	 expectedRevision?: string;
	 revision?: string;
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

export interface E2ELatestResult {
  status: string;
  freshness?: 'fresh' | 'stale' | 'unknown';
  recordedAt?: string;
  provider?: string;
  environment?: string;
  failedStep?: string;
  evidence: string[];
}

export interface E2ERunbook {
  title: string;
  path: string;
  source: 'plan' | 'wiki' | string;
  resultPath: string;
  latestResult?: E2ELatestResult;
  diagnostic?: string;
}

export interface E2ERunbookList {
  runbooks: E2ERunbook[];
  diagnostic?: string;
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
export interface KnowledgePage { slug: string; title: string; path: string; domain: string; bucket?: string; area?: string; tier?: string; pageType?: string; roles: string[]; topics: string[]; summary?: string; sourceRefs: string[]; sourceCount?: number; links: KnowledgeLink[]; backlinks: string[]; }
export interface KnowledgeWiki { workspaceId: string; root: string; displayName: string; journeysBucket?: string; pages: KnowledgePage[]; warnings: KnowledgeWarning[]; indexedAt: string; }
export interface KnowledgePageDetail extends KnowledgePage { content: FileContent; warnings: KnowledgeWarning[]; }
export interface KnowledgePagesResponse { pages: KnowledgePage[]; warnings: KnowledgeWarning[]; }
export interface KnowledgeGraphNode { id: string; title: string; domain: string; bucket?: string; area?: string; tier?: string; pageType?: string; roles: string[]; topics: string[]; path: string; inbound: number; outbound: number; }
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
export interface JiraIssue { key: string; summary: string; status: string; statusCategory?: string; description: string; issueType: string; assignee?: JiraPerson; reporter?: JiraPerson; priority?: string; labels: string[]; createdAt?: string; updatedAt?: string; browserUrl: string; attachments: JiraAttachment[]; truncated?: boolean; warnings?: string[]; }
export interface JiraIssueState { state: JiraIssueStateName; issue?: JiraIssue; message?: string; recoveryHint?: string; refreshedAt?: string; }

export interface WorkspaceInput {
  name: string;
  path?: string;
  baselineBranch: string;
  sources: WorkspaceSourceLike[];
  registrationMode?: WorkspaceRegistrationMode;
  remoteUrl?: string;
  cloneRoot?: string;
  jira?: JiraConnection;
  knowledge?: KnowledgeSettings;
  runtime?: WorkspaceRuntimeConfig;
}

export type WorkspaceRegistrationMode = 'local_path' | 'remote_clone' | 'existing_workspace';
export type WorkspaceLocation = 'local_path' | 'cloud_agent' | 'cloud_remote_snapshot';
export type WorkspaceAccessMode = 'agent_backed' | 'remote_snapshot';
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

export interface CloudAgent {
  id: string;
  userId: string;
  name: string;
  platform?: string;
  status: string;
  lastSeenAt?: string;
}

export interface AgentConnectToken {
  token: string;
  expiresAt: string;
  deepLink: string;
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
  sourceFingerprint: string;
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
  dataDirEnvironmentLocked?: boolean;
  restartRequired?: boolean;
}

export type StorageOption = 'database' | 'datadir';
export type StorageDriver = 'file' | 'sqlite' | 'postgres';
export type StorageSyncDirection = 'datadir_to_database' | 'database_to_datadir';

export interface DatabaseHealth {
  driver: StorageDriver;
  ok: boolean;
  migrationVersion: number;
  error?: string;
}

export interface StorageStatus {
  mode: RuntimeMode;
  storageOption: StorageOption;
  storageDriver: StorageDriver;
  environmentLocked: boolean;
  storageOptionEnv: string;
  storageDriverEnv: string;
  dataDir: string;
  databasePath?: string;
  databaseUrlConfigured: boolean;
  database?: DatabaseHealth;
}

export interface StorageSyncResult {
  ok: boolean;
  direction: StorageSyncDirection;
  backupPath: string;
  summary: Record<string, number>;
  warnings: string[];
  skippedStores: string[];
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
  /** Each workspace's own default provider, over `defaultProvider`. */
  workspaceProviders?: Record<string, string>;
}

/** The provider a session in a workspace starts with. */
export function providerFor(settings: Pick<AISettings, 'defaultProvider' | 'workspaceProviders'>, workspaceId?: string): string {
  return (workspaceId && settings.workspaceProviders?.[workspaceId]) || settings.defaultProvider;
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
	includeJiraDescription?: boolean;
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
	record?: SafeSessionRecord;
}

export interface EmbeddedAISessionLaunchInput extends Pick<AISessionLaunchInput, 'provider' | 'contextMode' | 'presetId' | 'promptDraft' | 'customPrompt' | 'selectedSkills' | 'selectedAgents'> {
	contextPath?: string;
	expectedWorkspaceId?: string;
	expectedBranch?: string;
	observedCommit?: string;
	idempotencyKey?: string;
	/** Resumes one of the workspace's Claude conversations instead of starting a new one. */
	resumeClaudeSessionId?: string;
	columns?: number;
	rows?: number;
}

export type WorkspaceAction = 'layout.move' | 'repo.read' | 'git.status' | 'terminal.launch' | 'verification.run';
export type ActionCapabilityState = 'available' | 'unavailable' | 'unsupported' | 'forbidden' | 'conflicted';
export interface CapabilityRecoveryAction { action: string; label: string; }
export interface ActionCapability { action: WorkspaceAction; state: ActionCapabilityState; reasonCode?: string; message?: string; recoveryActions: CapabilityRecoveryAction[]; }
export interface WorkspaceProviderAxes { topology: string; contentProvider: string; executionProvider: string; datastore?: string; }

export type CanvasEntityKind = 'workspace' | 'plan' | 'session';
export interface CanvasEntityRef { kind: CanvasEntityKind; workspaceId: string; itemId?: string; itemPath?: string; identifier?: string; branchKey?: string; observedCommit?: string; sessionId?: string; }
export interface CanvasPosition { x: number; y: number; }
export interface CanvasSize { width: number; height: number; }
export interface CanvasViewport extends CanvasPosition { zoom: number; }
export interface CanvasLayout { id: string; ownerUserId?: string; workspaceId: string; branchKey: string; viewport: CanvasViewport; version: number; createdAt: string; updatedAt: string; }
export interface CanvasPlacementPatch { nodeId: string; entityRef: CanvasEntityRef; position: CanvasPosition; size?: CanvasSize; collapsed: boolean; expectedRevision: number; }
export interface SafeSessionPlanRef { itemId: string; itemPath: string; identifier?: string; branchKey: string; observedCommit?: string; }
export interface SafeSessionRecord { id: string; workspaceId: string; planRef?: SafeSessionPlanRef; provider: string; intent: string; requestedBranch: string; observedCommit?: string; state: EmbeddedAISessionState | 'interrupted'; startedAt: string; endedAt?: string; exitCode?: number; lastKnownAt: string; live?: boolean; }

export type SessionFileStatus = 'A' | 'M' | 'D' | 'R' | 'C' | '?';
export interface SessionFile { path: string; status: SessionFileStatus; oldPath?: string; }
export type SessionChangesAvailability = 'available' | 'unavailable' | 'unsupported';
export type SessionChangesReason = 'launch_commit_unknown' | 'launch_commit_missing' | 'workspace_missing' | 'content_provider_unsupported';
export interface SessionChanges { sessionId: string; launchCommit: string; headCommit: string; headMoved: number; files: SessionFile[]; additions: number; deletions: number; truncated: boolean; availability: SessionChangesAvailability; reason?: SessionChangesReason; /** A harness run's changes: where they were read, its own copy or its branch. */ from?: 'worktree' | 'branch'; branch?: string; }
/** A Claude Code conversation held in the workspace root, outside kode-stream; metadata only. */
export interface ClaudeTranscript { sessionId: string; branch?: string; startedAt: string; lastActiveAt: string; messages: number; itemIds: string[]; }
/** `tool` is a call the model made (its name and main argument); `thinking` is its reasoning. Both appear only where the reader asked for the model's work, not just its words. */
export interface ClaudeTurn { role: 'user' | 'assistant' | 'tool' | 'thinking'; text: string; at?: string; }
export type SessionScope = 'active' | 'all';
export interface GlobalSessionRow { record: SafeSessionRecord; workspaceName: string; checkoutBranch: string; planTitle?: string; /** The plan's key (its identifier), which joins it to a harness task on the same ticket. */ planKey?: string; changesCount?: number; }
export interface GlobalSessionsResponse { rows: GlobalSessionRow[]; truncated: boolean; }
export interface CanvasWorkspaceNode { id: string; name?: string; branch?: string; commit?: string; git?: GitStatus; providerAxes: WorkspaceProviderAxes; actions: Partial<Record<WorkspaceAction, ActionCapability>>; verification?: VerificationJob; }
export interface CanvasPlanNode { itemId: string; identifier?: string; title?: string; service?: string; status: ItemStatus; branch?: string; commit?: string; editable: boolean; actions: Partial<Record<WorkspaceAction, ActionCapability>>; }
export interface CanvasSessionNode { record: SafeSessionRecord; }
/** `ephemeral` marks a live session drawn beside its plan without a placement; it leaves the Canvas when the session stops unless pinned. */
export interface CanvasNode { id: string; kind: CanvasEntityKind; state: 'resolved' | 'stale' | 'forbidden'; entityRef: CanvasEntityRef; position: CanvasPosition; size?: CanvasSize; collapsed: boolean; revision: number; workspace?: CanvasWorkspaceNode; plan?: CanvasPlanNode; session?: CanvasSessionNode; ephemeral?: boolean; }
export interface CanvasSection { id: string; title: string; position: CanvasPosition; width: number; height: number; nodeIds: string[]; }
export interface CanvasConnection { id: string; source: string; target: string; kind: 'repository_contains' | 'session_launched_from' | string; sourceOfTruth: 'derived'; }
export interface CanvasProjection { layout: CanvasLayout; nodes: CanvasNode[]; connections: CanvasConnection[]; unplaced: CanvasEntityRef[]; }

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
	metadataRevision?: string;
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
  truncated?: boolean;
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
	committed?: boolean;
	refreshRequired?: boolean;
	refreshError?: string;
}

export interface FileSaveInput {
  content: string;
  expectedHash?: string;
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

export interface ReviewedPlanImportInput {
	sourceBranch: string;
	expectedCommit: string;
	expectedCheckoutBranch: string;
	itemId: string;
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
  committed: boolean;
  refreshRequired?: boolean;
  refreshError?: string;
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
  committed: boolean;
  refreshRequired?: boolean;
  refreshError?: string;
}

export interface ItemMetadataUpdateInput {
	expectedRevision?: string;
  title?: string;
  scope?: string;
  identifier?: string;
  status?: ItemStatus;
  owner?: string;
  tags?: string[];
}

export interface ItemStatusUpdateInput {
  expectedRevision?: string;
  status: ItemStatus;
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
	committed: boolean;
	refreshRequired?: boolean;
	refreshError?: string;
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
  /** When the remote-tracking ref that `ahead`/`behind` are measured against was last fetched. Absent means nothing has fetched since the clone. */
  fetchedAt?: string;
  dirty: boolean;
  conflicted: boolean;
  changes: GitChange[];
}

export interface WorkspaceBranches {
  workspaceId: string;
  current: string;
  branches: string[];
}

export interface GitStashEntry {
	ref: string;
	message: string;
}

export interface GitCommitInput {
  message: string;
  paths: string[];
}

export interface GitOperationInput {
}

export interface BranchCreateInput {
  name: string;
  startPoint?: string;
  checkout?: boolean;
}

export interface BranchSwitchInput {
  name: string;
  strategy?: 'carry' | 'stash';
  stashMessage?: string;
}

export interface BranchSwitchDecision {
  sourceBranch: string;
  targetBranch: string;
  canCarryChanges: boolean;
}

export interface GitOperationResult {
  ok: boolean;
	committed?: boolean;
	refreshRequired?: boolean;
	refreshError?: string;
  message?: string;
  recoveryHint?: string;
  status: GitStatus;
	code?: string;
	decision?: BranchSwitchDecision;
	stashRef?: string;
	stashMessage?: string;
}

// ── PM-044 engineering harness ───────────────────────────────────────────────
// Mirrors the backend contract in plans/platform/PM-044 (API section). Every
// timestamp is an RFC 3339 string; every cost is US dollars as a number.

/** `local` reads tasks from Markdown files in a folder; its hand-off pushes and opens no pull request. */
export type HarnessSourceKind = 'jira' | 'github' | 'bitbucket' | 'local';
export type HarnessTaskState = 'discovered' | 'ranked' | 'running' | 'blocked' | 'done' | 'dismissed' | 'stale' | 'cancelled';
export type HarnessRunState = 'running' | 'blocked' | 'passed' | 'failed' | 'cancelled';
export type HarnessStageKind = 'plan' | 'plan_approval' | 'implement' | 'qa' | 'review' | 'draft_pr';
export type HarnessStageState = 'pending' | 'running' | 'awaiting_approval' | 'passed' | 'failed' | 'retry_wait' | 'cancelled' | 'skipped';
export type HarnessAgentRole = 'planner' | 'plan_reviewer' | 'coder' | 'code_reviewer' | 'qa_fixer';
export type HarnessAgentRunState = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled' | 'interrupted';
export type HarnessApprovalGate = 'start' | 'plan' | 'resume' | 'review' | 'question';
export type HarnessApprovalState = 'requested' | 'approved' | 'rejected' | 'expired' | 'superseded';
export type HarnessChannel = 'dashboard' | 'webex' | 'api';
/** The orchestrator's loops a person can pause; `reviewer` covers plan and code review. */
export type HarnessRoleId = 'scout' | 'sorter' | 'planner' | 'coder' | 'reviewer' | 'qa';
export type HarnessEngineId = 'claudecode' | 'codex' | 'opencode';
/** `replan` answers Review only: it sends the change back to Plan; `reject` sends it back to Build. */
export type HarnessDecision = 'approve' | 'reject' | 'replan';

export interface HarnessActor { actorId: string; displayName: string; email?: string }

export interface HarnessTaskSource {
  kind: HarnessSourceKind;
  key: string;
  url?: string;
  status?: string;
  labels: string[];
  priority?: string;
  assignee?: string;
  updatedAt?: string;
}

export interface HarnessTask {
  id: string;
  workspaceId: string;
  source: HarnessTaskSource;
  title: string;
  state: HarnessTaskState;
  score: number;
  scoreReasons: string[];
  priorityOverride: number;
  pinned: boolean;
  needsYou: boolean;
  /** A person pressed Start; the next free Planner takes it. */
  startRequested?: boolean;
  /** This task's own answer to "may an agent stop and ask"; absent follows the workspace, then the settings. */
  askQuestions?: boolean | null;
  note?: string;
  runId?: string;
  attempts: number;
  revision: number;
  /** Ids of the tasks this one waits for: the Task Queue says "Waits for" and starts it only after them. */
  dependsOn: string[];
  milestone?: string;
  epic?: string;
  risk?: 'low' | 'medium' | 'high';
  estimatedMinutes?: number;
  /** Set when the source closed the task outside the harness ("Done by hand"). */
  finishedOutside?: { by?: string; at?: string };
  createdAt?: string;
  updatedAt?: string;
}

/** What a card hands the harness to run it: the ticket's key, title and body. */
export interface HarnessCreateTaskInput { workspaceId: string; key: string; title: string; body?: string; url?: string; /** Start at once (the default) or only put it in the queue. */ startNow?: boolean; idempotencyKey: string }

/** One file a run wrote under the plan folder; `content` is empty when the file is too large to show. */
export interface HarnessRunFile { path: string; status: 'added' | 'modified' | 'deleted'; content: string; truncated?: boolean }
/** The plan a run wrote, readable before it is merged: live from the run's own copy, or from its branch. */
/** The diff of one path a run changed, read from its own copy or its branch. */
export interface HarnessRunFileDiff { path: string; diff: string; truncated: boolean; from: 'worktree' | 'branch' | '' }
export interface HarnessRunPlan { branch: string; from: 'worktree' | 'branch' | ''; files: HarnessRunFile[] }

export interface HarnessTotals { turns: number; durationMs: number; costUsd: number; blocking: number; advisory: number }

export interface HarnessStage {
  id: string;
  runId: string;
  kind: HarnessStageKind;
  ordinal: number;
  state: HarnessStageState;
  round: number;
  maxRounds: number;
  attempt: number;
  maxAttempts: number;
  engine?: HarnessEngineId;
  model?: string;
  turns: number;
  durationMs: number;
  costUsd: number;
  blocking: number;
  advisory: number;
  startedAt?: string;
  finishedAt?: string;
  failureReason?: string;
}

export interface HarnessFinding {
  severity: 'blocking' | 'advisory';
  title: string;
  path?: string;
  line?: number;
  detail?: string;
  /** True once a later round no longer reports it. */
  resolved?: boolean;
}

export interface HarnessAgentRun {
  id: string;
  runId: string;
  stageId: string;
  role: HarnessAgentRole;
  round: number;
  /** The try within the round; a run interrupted or without a report is tried again. */
  attempt?: number;
  engine: HarnessEngineId;
  model: string;
  state: HarnessAgentRunState;
  startedAt?: string;
  finishedAt?: string;
  turns: number;
  durationMs: number;
  costUsd: number;
  verdict?: 'approve' | 'changes_requested';
  blocking: number;
  advisory: number;
  summary?: string;
  /** The last ≤200 characters the engine reported, for live progress. */
  progress?: string;
  findings?: HarnessFinding[];
  stopReason?: string;
}

export interface HarnessRun {
  id: string;
  taskId: string;
  workspaceId: string;
  branch: string;
  state: HarnessRunState;
  currentStageId?: string;
  totals: HarnessTotals;
  prUrl?: string;
  /** The name the branch is pushed under. */
  remoteBranch?: string;
  startedAt: string;
  finishedAt?: string;
  revision: number;
  /** Embedded by the list endpoint so the Inbox needs no second request. */
  currentStage?: HarnessStage;
  activeAgentRun?: HarnessAgentRun;
}

export interface HarnessApproval {
  id: string;
  runId?: string;
  /** The stage that asked, when a stage did: two Reviews of one run are told apart by it. */
  stageId?: string;
  taskId: string;
  gate: HarnessApprovalGate;
  state: HarnessApprovalState;
  requestedAt: string;
  expiresAt?: string;
  decidedAt?: string;
  decidedBy?: HarnessActor;
  channel?: HarnessChannel;
  reason?: string;
  revision: number;
  /** Plain summary of what is being approved, written by the harness (≤600). */
  summary?: string;
  /** What the next step usually takes, shown in the consequence sentence. */
  estimate?: { minMinutes?: number; maxMinutes?: number; costUsd?: number };
  context?: { difficulty?: 'easy' | 'hard'; subtasks?: number; rounds?: number; blocking?: number; advisory?: number; costUsd?: number; durationMs?: number; verdict?: 'approve' | 'changes_requested'; questions?: string[]; role?: HarnessAgentRole };
}

/** The last turns of an agent run's own transcript, read from the coding tool's file when asked for. */
export interface HarnessTranscript { agentRunId: string; sessionId?: string; available: boolean; turns: ClaudeTurn[] }

export interface HarnessRunDetail {
  run: HarnessRun;
  task: HarnessTask;
  stages: HarnessStage[];
  agentRuns: HarnessAgentRun[];
  approvals: HarnessApproval[];
}

export interface HarnessRoleHealth {
  role: HarnessRoleId;
  paused: boolean;
  pausedBy?: string;
  pausedAt?: string;
  working: number;
  slots: number;
  queued: number;
  lastRunAt?: string;
  /** Finder only: tasks found by the last look. */
  found?: number;
  model?: string;
  engine?: HarnessEngineId;
}

export interface HarnessEngineStatus {
  id: HarnessEngineId;
  available: boolean;
  version?: string;
  /** Plain notes on what this engine cannot do (from Detect). */
  limits?: string[];
}

export interface HarnessStatus {
  /** False means a task waits for a person to press Start. */
  autoStart?: boolean;
  /** Whether an agent may stop and ask a person, unless a workspace or task says otherwise. */
  askQuestions?: boolean;
  enabled: boolean;
  storageSupported: boolean;
  mode: 'local' | 'cloud';
  running: boolean;
  waitingCount: number;
  lastEventSeq: number;
  roles: HarnessRoleHealth[];
  engines: HarnessEngineStatus[];
}

/** What one source returned when the Finder asked it. */
export interface HarnessSourcePoll {
  workspaceId: string;
  kind: string;
  lastPolledAt?: string;
  found: number;
  added: number;
  lastError?: string;
}

export interface HarnessCostGroup { key: string; costUsd: number; agentRuns: number; turns: number; durationMs: number }
export interface HarnessCostSummary {
  groupBy: 'day' | 'run' | 'role' | 'model';
  from: string;
  to: string;
  totalUsd: number;
  agentRuns: number;
  groups: HarnessCostGroup[];
}

export interface HarnessRolePolicy {
  engine: HarnessEngineId;
  model: string;
  /** Builder only: the model used when the plan rates the work hard. */
  hardModel?: string;
  maxTurns: number;
}

export interface HarnessSourceSettings {
  workspaceId: string;
  kind: HarnessSourceKind;
  enabled: boolean;
  /** Jira search query, GitHub `owner/repo`, Bitbucket `workspace/repo`, or a local task folder. */
  query: string;
  labels?: string[];
  tokenEnvVar?: string;
  lastCheckedAt?: string;
  lastFound?: number;
  lastError?: string;
}

export interface HarnessSettings {
  revision: number;
  enabled: boolean;
  autoStart: boolean;
  /** A run whose tests passed waits at Review, where a person tries it before hand-off. */
  requireReview: boolean;
  /** Agents may stop and ask a person when they cannot decide alone. */
  askQuestions: boolean;
  slots: Record<HarnessRoleId, number>;
  paused: Partial<Record<HarnessRoleId, boolean>>;
  sources: HarnessSourceSettings[];
  modelPolicy: Record<HarnessAgentRole, HarnessRolePolicy>;
  /** Each workspace's own choices over these settings. */
  workspaces: HarnessWorkspaceSettings[];
}

/** A role's CLI and model in one workspace; an empty field follows the defaults. Tool permissions stay the defaults' alone. */
export interface HarnessRoleModel {
  engine?: HarnessEngineId;
  model?: string;
  highDifficultyModel?: string;
  maxTurns?: number;
}

/** The skill of the repository, and the track of it, a role follows in one workspace. */
export interface HarnessProcedureStep {
  skill: string;
  track?: string;
}

/** One workspace's overrides. An absent or empty field follows the defaults. */
export interface HarnessWorkspaceSettings {
  workspaceId: string;
  enabled?: boolean | null;
  autoStart?: boolean | null;
  requireReview?: boolean | null;
  askQuestions?: boolean | null;
  verificationCommands: string[];
  scope: string[];
  models: Partial<Record<HarnessAgentRole, HarnessRoleModel>>;
  procedure: Partial<Record<HarnessAgentRole, HarnessProcedureStep>>;
}

/** The parts of `.kode-stream/settings.yaml` a person exports or imports one at a time. */
export type SettingsFilePart = 'provider' | 'pipeline' | 'tests' | 'scope' | 'models' | 'procedure' | 'sources';

export interface SettingsFileContent {
  version: number;
  ai?: { provider: string };
  harness?: {
    enabled?: boolean;
    autoStart?: boolean;
    review?: boolean;
    askQuestions?: boolean;
    tests?: string[];
    scope?: string[];
    models?: Partial<Record<HarnessAgentRole, { engine?: string; model?: string; hardModel?: string; maxTurns?: number }>>;
    procedure?: Partial<Record<HarnessAgentRole, HarnessProcedureStep>>;
    sources?: { kind: HarnessSourceKind; enabled: boolean; query: string; labels?: string[]; tokenEnvVar?: string }[];
  };
}

/** A workspace's shared settings file: whether its checkout has one, what it holds, and what an export would write now. */
export interface SettingsFileView {
  path: string;
  exists: boolean;
  error?: string;
  file?: SettingsFileContent;
  has: SettingsFilePart[];
  current: SettingsFileContent;
  parts: SettingsFilePart[];
}

export type HarnessEvent =
  | { seq: number; at: string; type: 'task.discovered' | 'task.updated' | 'task.ranked' | 'task.pinned'; task: HarnessTask }
  | { seq: number; at: string; type: 'task.removed'; taskId: string }
  | { seq: number; at: string; type: 'run.started' | 'run.blocked' | 'run.finished'; run: HarnessRun; /** The run's task as it stands after the change. */ task?: HarnessTask }
  | { seq: number; at: string; type: 'stage.started' | 'stage.round' | 'stage.finished'; stage: HarnessStage }
  | { seq: number; at: string; type: 'agent.started' | 'agent.progress' | 'agent.finished'; agentRun: HarnessAgentRun }
  | { seq: number; at: string; type: 'approval.requested' | 'approval.decided'; approval: HarnessApproval; /** The gate's task as it stands after the change. */ task?: HarnessTask }
  | { seq: number; at: string; type: 'role.paused' | 'role.resumed'; role: HarnessRoleHealth }
  | { seq: number; at: string; type: 'notification.failed'; message: string };

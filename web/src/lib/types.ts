export type ItemStatus = 'unsorted' | 'ideas' | 'draft' | 'in_progress' | 'review' | 'done';

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
  baselineBranch: string;
  lastSelectedBranch?: string;
  sources: string[];
  createdAt: string;
  lastScannedAt?: string;
}

export interface WorkspaceInput {
  name: string;
  path: string;
  baselineBranch: string;
  sources: string[];
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
  scope: string;
  identifier: string;
  title?: string;
  status?: string;
  owner?: string;
  tags?: string[];
}

export interface SourceSettingsResult {
  directory: string;
  exists: boolean;
  mode?: 'structured' | 'unstructured' | 'empty' | 'unknown';
  settings: SourceStructureSettings;
  warnings: { itemPath?: string; message: string }[];
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

export type FileKind = 'markdown' | 'html' | 'json' | 'yaml' | 'code' | 'text' | 'unsupported';

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

export interface BranchLoadResult {
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
  title: string;
  status?: ItemStatus;
  owner?: string;
  tags?: string[];
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
}

export type GitChangeStatus = 'modified' | 'added' | 'deleted' | 'renamed' | 'copied' | 'untracked' | 'conflicted';

export interface GitChange {
  path: string;
  oldPath?: string;
  status: GitChangeStatus;
  staged: boolean;
  conflict: boolean;
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

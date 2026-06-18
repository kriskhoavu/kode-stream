export type ItemStatus = 'unsorted' | 'ideas' | 'draft' | 'in_progress' | 'review' | 'done';

export interface WorkspaceConfig {
  id: string;
  name: string;
  path: string;
  baselineBranch: string;
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

export interface FileContent {
  id: string;
  path: string;
  content: string;
  language: string;
  hash: string;
}

export interface FileSaveInput {
  content: string;
  expectedHash?: string;
}

export interface ItemMetadataUpdateInput {
  title?: string;
  scope?: string;
  identifier?: string;
  status?: ItemStatus;
  owner?: string;
  tags?: string[];
}

export interface ItemStatusUpdateInput {
  status: ItemStatus;
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
  status: GitStatus;
}

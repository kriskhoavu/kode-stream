import { type ItemStatus } from './common';
import { type AgentConnection, type Capability, type CloudRole, type CloudUser, type RuntimeMode } from './workspace';

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

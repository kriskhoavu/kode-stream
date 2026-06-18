export type PlanStatus = 'unsorted' | 'ideas' | 'draft' | 'in_progress' | 'review' | 'done';

export interface RepositoryConfig {
  id: string;
  name: string;
  path: string;
  baselineBranch: string;
  planDirectories: string[];
  createdAt: string;
  lastScannedAt?: string;
}

export interface RepositoryInput {
  name: string;
  path: string;
  baselineBranch: string;
  planDirectories: string[];
}

export interface RepositorySettings {
  version: number;
  cards: RepositorySettingsCard[];
}

export interface RepositorySettingsCard {
  pathPattern: string;
  fields: RepositorySettingsFields;
}

export interface RepositorySettingsFields {
  service: string;
  ticket: string;
  title?: string;
  status?: string;
  owner?: string;
  tags?: string[];
}

export interface SourceSettingsResult {
  directory: string;
  exists: boolean;
  mode?: 'structured' | 'unstructured' | 'empty' | 'unknown';
  settings: RepositorySettings;
  warnings: { planPath?: string; message: string }[];
  scan?: ScanResult;
}

export interface PlanSummary {
  id: string;
  repositoryId: string;
  repositoryName: string;
  branch: string;
  service: string;
  ticket: string;
  title: string;
  status: PlanStatus;
  owner?: string;
  author?: string;
  tags: string[];
  updatedAt?: string;
  description?: string;
  metadataSource: string;
  planRoot?: string;
}

export interface PlanDocument {
  id: string;
  role: string;
  track?: string;
  path: string;
  label: string;
}

export interface PlanDetail extends PlanSummary {
  documents: PlanDocument[];
  metadata: Record<string, unknown>;
  warnings?: { planPath?: string; message: string }[];
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

export interface PlanMetadataUpdateInput {
  title?: string;
  service?: string;
  ticket?: string;
  status?: PlanStatus;
  owner?: string;
  tags?: string[];
}

export interface PlanStatusUpdateInput {
  status: PlanStatus;
}

export interface NewPlanInput {
  repositoryId: string;
  planDirectory: string;
  service: string;
  ticket: string;
  title: string;
  status?: PlanStatus;
  owner?: string;
  tags?: string[];
}

export interface WriteResult {
  plan: PlanDetail;
  scannedAt: string;
}

export interface ScanResult {
  repositoryId: string;
  scannedAt: string;
  planCount: number;
  warnings: { planPath?: string; message: string }[];
}

export interface PathSelection {
  path: string;
}

export interface AppState {
  version: string;
  repositoryCount: number;
  planCount: number;
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
  repositoryId: string;
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

export type PlanStatus = 'ideas' | 'draft' | 'in_progress' | 'review' | 'done';

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

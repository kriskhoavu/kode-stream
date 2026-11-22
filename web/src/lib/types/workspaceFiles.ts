import { type GitChangeStatus } from './git';
import { type FileContent, type FileKind } from './item';

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

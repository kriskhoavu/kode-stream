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

import type { BranchCreateInput, BranchSwitchInput, GitCommitInput, GitActivityEntry, GitOperationInput, GitOperationResult, GitStatus, GitStashEntry, WorkspaceBranches } from '../../lib/types';
import { request } from './http';
import { normalizeGitStatus, normalizeGitActivityEntry, normalizeGitResult } from './normalize';

export const gitApi = {
  gitStatus: (workspaceId: string, signal?: AbortSignal) => request<GitStatus>(`/api/workspaces/${workspaceId}/git/status`, { signal }, !signal).then(normalizeGitStatus),
  gitActivity: async (workspaceId: string, params: { path?: string; limit?: number; signal?: AbortSignal } = {}) => {
    const query = new URLSearchParams();
    if (params.path) query.set('path', params.path);
    if (params.limit) query.set('limit', String(params.limit));
    const suffix = query.size ? `?${query.toString()}` : '';
    const payload = await request<GitActivityEntry[] | null>(`/api/workspaces/${encodeURIComponent(workspaceId)}/git/activity${suffix}`, { signal: params.signal }, !params.signal);
    return (Array.isArray(payload) ? payload : []).map(normalizeGitActivityEntry);
  },
  workspaceBranches: async (workspaceId: string, signal?: AbortSignal) => {
    const response = await request<WorkspaceBranches>(`/api/workspaces/${encodeURIComponent(workspaceId)}/git/branches`, { signal }, !signal);
    return { ...response, current: response.current ?? '', branches: Array.isArray(response.branches) ? response.branches : [] };
  },
  gitFetch: (workspaceId: string, input: GitOperationInput = {}) =>
    request<GitOperationResult>(`/api/workspaces/${workspaceId}/git/fetch`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult),
  gitPull: (workspaceId: string, input: GitOperationInput = {}) =>
    request<GitOperationResult>(`/api/workspaces/${workspaceId}/git/pull`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult),
  gitPush: (workspaceId: string, input: GitOperationInput = {}) =>
    request<GitOperationResult>(`/api/workspaces/${workspaceId}/git/push`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult),
  gitCommit: (workspaceId: string, input: GitCommitInput) =>
    request<GitOperationResult>(`/api/workspaces/${workspaceId}/git/commit`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult),
  createBranch: (workspaceId: string, input: BranchCreateInput) =>
    request<GitOperationResult>(`/api/workspaces/${workspaceId}/git/branches`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult),
  switchBranch: (workspaceId: string, input: BranchSwitchInput) =>
    request<GitOperationResult>(`/api/workspaces/${workspaceId}/git/switch`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult),
  stashes: (workspaceId: string, signal?: AbortSignal) => request<GitStashEntry[]>(`/api/workspaces/${encodeURIComponent(workspaceId)}/git/stashes`, { signal }, !signal),
  applyStash: (workspaceId: string, ref: string) => request<GitOperationResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/git/stashes/${encodeURIComponent(ref)}/apply`, { method: 'POST' }).then(normalizeGitResult)
};

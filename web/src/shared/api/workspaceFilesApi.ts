import type { FileContent, WorkspaceDirectoryListing, ExplorerTreeMode, WorkspaceDirectoryCreateInput, WorkspaceFileCreateInput, WorkspaceFileRevertInput, WorkspaceFileSaveInput, WorkspaceFileWriteResult, WorkspacePathGitState, WorkspacePathMutationResult, WorkspacePathRenameInput, WorkspacePathSearchResponse, WorkspaceContentSearchResponse } from '../../lib/types';
import { request } from './http';
import { contentSearchQuery, normalizeContentSearchResponse, normalizeWorkspaceDirectoryListing, normalizeWorkspacePathGitState, normalizeWorkspacePathMutationResult } from './normalize';

export const workspaceFilesApi = {
  workspaceTree: async (workspaceId: string, path = '', includeIgnored = false, force = false, signal?: AbortSignal) => {
    const query = new URLSearchParams({ path });
    if (includeIgnored) query.set('includeIgnored', 'true');
		const listing = await request<WorkspaceDirectoryListing>(`/api/workspaces/${encodeURIComponent(workspaceId)}/tree?${query.toString()}`, { signal }, !force && !signal);
    return normalizeWorkspaceDirectoryListing(listing);
  },
  searchWorkspacePaths: async (params: { q: string; workspaceId?: string; includeIgnored?: boolean; signal?: AbortSignal }) => {
    const query = new URLSearchParams({ q: params.q });
    if (params.workspaceId) query.set('workspaceId', params.workspaceId);
    if (params.includeIgnored) query.set('includeIgnored', 'true');
    const response = await request<WorkspacePathSearchResponse>(`/api/workspaces/files/search?${query.toString()}`, params.signal ? { signal: params.signal } : undefined, !params.signal);
    return { ...response, results: Array.isArray(response.results) ? response.results : [], truncated: Boolean(response.truncated) };
  },
	searchWorkspaceContent: async (params: { q: string; mode: ExplorerTreeMode; workspaceId?: string; includeIgnored?: boolean; caseSensitive?: boolean; signal?: AbortSignal }) => {
		const query = contentSearchQuery(params);
		query.set('mode', params.mode);
		if (params.workspaceId) query.set('workspaceId', params.workspaceId);
		if (params.includeIgnored) query.set('includeIgnored', 'true');
		return normalizeContentSearchResponse(await request<WorkspaceContentSearchResponse>(`/api/workspaces/files/content-search?${query.toString()}`, params.signal ? { signal: params.signal } : undefined, !params.signal));
	},
  workspacePathGitStates: async (workspaceId: string, signal?: AbortSignal) =>
    (await request<WorkspacePathGitState[] | null>(`/api/workspaces/${encodeURIComponent(workspaceId)}/git/path-status`, { signal }, !signal) ?? []).map(normalizeWorkspacePathGitState),
  createWorkspaceFile: (workspaceId: string, input: WorkspaceFileCreateInput) =>
    request<WorkspacePathMutationResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/files`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeWorkspacePathMutationResult),
  createWorkspaceDirectory: (workspaceId: string, input: WorkspaceDirectoryCreateInput) =>
    request<WorkspacePathMutationResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/directories`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeWorkspacePathMutationResult),
  renameWorkspacePath: (workspaceId: string, input: WorkspacePathRenameInput) =>
    request<WorkspacePathMutationResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/paths/rename`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeWorkspacePathMutationResult),
  workspaceFile: (workspaceId: string, path: string) =>
    request<FileContent>(`/api/workspaces/${encodeURIComponent(workspaceId)}/files?path=${encodeURIComponent(path)}`),
  saveWorkspaceFile: (workspaceId: string, input: WorkspaceFileSaveInput) =>
    request<WorkspaceFileWriteResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/files`, { method: 'PUT', body: JSON.stringify(input) }),
  workspaceFileDiff: (workspaceId: string, path: string) =>
    request<{ diff: string }>(`/api/workspaces/${encodeURIComponent(workspaceId)}/files/diff?path=${encodeURIComponent(path)}`),
  revertWorkspaceFile: (workspaceId: string, input: WorkspaceFileRevertInput) =>
    request<WorkspaceFileWriteResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/files/revert`, { method: 'POST', body: JSON.stringify(input) })
};

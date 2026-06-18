import type {
  AppState,
  BranchCreateInput,
  BranchSwitchInput,
  FileContent,
  FileNode,
  FileSaveInput,
  GitCommitInput,
  GitOperationInput,
  GitOperationResult,
  GitStatus,
  NewItemInput,
  PathSelection,
  ItemDetail,
  ItemMetadataUpdateInput,
  ItemStatusUpdateInput,
  ItemSummary,
  WorkspaceConfig,
  WorkspaceInput,
  SourceStructureSettings,
  ScanResult,
  SourceSettingsResult,
  WriteResult
} from './types';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(options?.headers ?? {})
    }
  });
  const payload = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(payload.error ?? `Request failed: ${res.status}`);
  }
  return payload as T;
}

export const api = {
  state: () => request<AppState>('/api/state'),
  workspaces: async () => ((await request<WorkspaceConfig[] | null>('/api/workspaces')) ?? []).map(normalizeWorkspace),
  createWorkspace: (input: WorkspaceInput) => request<WorkspaceConfig>('/api/workspaces', { method: 'POST', body: JSON.stringify(input) }),
  updateWorkspace: (id: string, input: WorkspaceInput) => request<WorkspaceConfig>(`/api/workspaces/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
  deleteWorkspace: (id: string) => request<{ ok: boolean }>(`/api/workspaces/${id}`, { method: 'DELETE' }),
  scan: (workspaceId: string) => request<ScanResult>(`/api/workspaces/${workspaceId}/scan`, { method: 'POST' }),
  sourceStructure: (workspaceId: string, directory: string) =>
    request<SourceSettingsResult>(`/api/workspaces/${workspaceId}/source-structure?directory=${encodeURIComponent(directory)}`),
  saveSourceStructure: (workspaceId: string, directory: string, settings: SourceStructureSettings) =>
    request<SourceSettingsResult>(`/api/workspaces/${workspaceId}/source-structure?directory=${encodeURIComponent(directory)}`, {
      method: 'PUT',
      body: JSON.stringify(settings)
    }),
  selectDirectory: () => request<PathSelection>('/api/system/select-directory', { method: 'POST' }),
  openPath: (path: string) => request<{ ok: boolean }>('/api/system/open-path', { method: 'POST', body: JSON.stringify({ path }) }),
  items: async (params: URLSearchParams) => ((await request<ItemSummary[] | null>(`/api/items?${params.toString()}`)) ?? []).map(normalizeItem),
  item: async (id: string) => normalizeItemDetail(await request<ItemDetail>(`/api/items/${id}`)),
  files: async (id: string) => (await request<FileNode[] | null>(`/api/items/${id}/files`)) ?? [],
  file: (id: string, fileId: string) => request<FileContent>(`/api/items/${id}/files/${fileId}`),
  saveFile: (id: string, fileId: string, input: FileSaveInput) =>
    request<FileContent>(`/api/items/${id}/files/${fileId}`, { method: 'POST', body: JSON.stringify(input) }),
  revertFile: (id: string, fileId: string) =>
    request<ScanResult>(`/api/items/${id}/files/${fileId}/revert`, { method: 'POST' }),
  saveMetadata: (id: string, input: ItemMetadataUpdateInput) =>
    request<WriteResult>(`/api/items/${id}/metadata`, { method: 'PATCH', body: JSON.stringify(input) }),
  updateStatus: (id: string, input: ItemStatusUpdateInput) =>
    request<WriteResult>(`/api/items/${id}/status`, { method: 'PATCH', body: JSON.stringify(input) }),
  createItem: (input: NewItemInput) => request<WriteResult>('/api/items', { method: 'POST', body: JSON.stringify(input) }),
  diff: (id: string) => request<{ diff: string }>(`/api/items/${id}/diff`),
  gitStatus: (workspaceId: string) => request<GitStatus>(`/api/workspaces/${workspaceId}/git/status`).then(normalizeGitStatus),
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
    request<GitOperationResult>(`/api/workspaces/${workspaceId}/git/switch`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult)
};

function normalizeWorkspace(workspace: WorkspaceConfig): WorkspaceConfig {
  return {
    ...workspace,
    sources: Array.isArray(workspace.sources) ? workspace.sources : []
  };
}

function normalizeItem(item: ItemSummary): ItemSummary {
  return {
    ...item,
    tags: Array.isArray(item.tags) ? item.tags : []
  };
}

function normalizeItemDetail(item: ItemDetail): ItemDetail {
  return {
    ...normalizeItem(item),
    documents: Array.isArray(item.documents) ? item.documents : [],
    metadata: item.metadata ?? {},
    counts: item.counts ?? { files: 0 }
  };
}

function normalizeGitStatus(status: GitStatus): GitStatus {
  return {
    ...status,
    ahead: status.ahead ?? 0,
    behind: status.behind ?? 0,
    dirty: Boolean(status.dirty),
    conflicted: Boolean(status.conflicted),
    changes: Array.isArray(status.changes) ? status.changes : []
  };
}

function normalizeGitResult(result: GitOperationResult): GitOperationResult {
  return {
    ...result,
    ok: Boolean(result.ok),
    status: normalizeGitStatus(result.status ?? { workspaceId: '', branch: '', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] })
  };
}

export const statusLabels = {
  unsorted: 'Unsorted',
  ideas: 'Ideas',
  draft: 'Draft',
  in_progress: 'In Progress',
  review: 'Review',
  done: 'Done'
} as const;

export const statusOrder = Object.keys(statusLabels) as Array<keyof typeof statusLabels>;
export const editableStatusOrder = statusOrder.filter((status) => status !== 'unsorted');

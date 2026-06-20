import type {
  AppState,
  AuditEvent,
  BranchCreateInput,
  BranchSwitchInput,
  FileContent,
  FileNode,
  FileSaveInput,
  GitCommitInput,
  GitOperationInput,
  GitOperationResult,
  GitStatus,
  HealthCheck,
  NewItemInput,
  PathSelection,
  RecentItem,
  SavedFilter,
  SearchResult,
  ItemDetail,
  ItemMetadataUpdateInput,
  ItemStatusUpdateInput,
  ItemSummary,
  WorkspaceConfig,
  WorkspaceInput,
  WorkspaceHealth,
  WorkspaceDirectoryListing,
  WorkspaceDirectoryCreateInput,
  WorkspaceFileCreateInput,
  WorkspaceFileRevertInput,
  WorkspaceFileSaveInput,
  WorkspaceFileWriteResult,
  WorkspacePathGitState,
  WorkspacePathMutationResult,
  WorkspacePathRenameInput,
  WorkspacePathSearchResponse,
  WorkspaceTreeEntry,
  SourceStructureSettings,
  ScanResult,
  SourceSettingsResult,
  WriteResult
} from '../../lib/types';

export class ApiError extends Error {
  recoveryHint?: string;

  constructor(message: string, recoveryHint?: string) {
    super(message);
    this.name = 'ApiError';
    this.recoveryHint = recoveryHint;
  }
}

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
    throw new ApiError(payload.error ?? payload.message ?? `Request failed: ${res.status}`, payload.recoveryHint);
  }
  return payload as T;
}

export const api = {
  state: () => request<AppState>('/api/state'),
  search: async (params: { q: string; workspaceId?: string; types?: string[]; limit?: number }) => {
    const query = new URLSearchParams({ q: params.q });
    if (params.workspaceId) query.set('workspaceId', params.workspaceId);
    if (params.types?.length) query.set('types', params.types.join(','));
    if (params.limit) query.set('limit', String(params.limit));
    return ((await request<SearchResult[] | null>(`/api/search?${query.toString()}`)) ?? []).map(normalizeSearchResult);
  },
  savedFilters: async () => ((await request<SavedFilter[] | null>('/api/saved-filters')) ?? []).map(normalizeSavedFilter),
  saveFilter: (filter: Pick<SavedFilter, 'name' | 'route' | 'filters'> & Partial<Pick<SavedFilter, 'id' | 'workspaceId'>>) =>
    request<SavedFilter>('/api/saved-filters', { method: 'POST', body: JSON.stringify(filter) }).then(normalizeSavedFilter),
  deleteFilter: (id: string) => request<{ ok: boolean }>(`/api/saved-filters/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  recentItems: async (limit = 10) => ((await request<RecentItem[] | null>(`/api/recent-items?limit=${limit}`)) ?? []).map(normalizeRecentItem),
  recordRecentItem: (itemId: string) => request<{ ok: boolean }>('/api/recent-items', { method: 'POST', body: JSON.stringify({ itemId }) }),
  auditEvents: async (params: { workspaceId?: string; limit?: number } = {}) => {
    const query = new URLSearchParams();
    if (params.workspaceId) query.set('workspaceId', params.workspaceId);
    if (params.limit) query.set('limit', String(params.limit));
    const suffix = query.size ? `?${query.toString()}` : '';
    return ((await request<AuditEvent[] | null>(`/api/audit-events${suffix}`)) ?? []).map(normalizeAuditEvent);
  },
  workspaces: async () => ((await request<WorkspaceConfig[] | null>('/api/workspaces')) ?? []).map(normalizeWorkspace),
  createWorkspace: (input: WorkspaceInput) => request<WorkspaceConfig>('/api/workspaces', { method: 'POST', body: JSON.stringify(input) }),
  updateWorkspace: (id: string, input: WorkspaceInput) => request<WorkspaceConfig>(`/api/workspaces/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
  deleteWorkspace: (id: string) => request<{ ok: boolean }>(`/api/workspaces/${id}`, { method: 'DELETE' }),
  scan: (workspaceId: string) => request<ScanResult>(`/api/workspaces/${workspaceId}/scan`, { method: 'POST' }),
  workspaceHealth: (workspaceId: string) => request<WorkspaceHealth>(`/api/workspaces/${workspaceId}/health`).then(normalizeWorkspaceHealth),
  sourceStructure: (workspaceId: string, directory: string) =>
    request<SourceSettingsResult>(`/api/workspaces/${workspaceId}/source-structure?directory=${encodeURIComponent(directory)}`),
  saveSourceStructure: (workspaceId: string, directory: string, settings: SourceStructureSettings) =>
    request<SourceSettingsResult>(`/api/workspaces/${workspaceId}/source-structure?directory=${encodeURIComponent(directory)}`, {
      method: 'PUT',
      body: JSON.stringify(settings)
    }),
  workspaceTree: async (workspaceId: string, path = '', includeIgnored = false) => {
    const query = new URLSearchParams({ path });
    if (includeIgnored) query.set('includeIgnored', 'true');
    const listing = await request<WorkspaceDirectoryListing>(`/api/workspaces/${encodeURIComponent(workspaceId)}/tree?${query.toString()}`);
    return normalizeWorkspaceDirectoryListing(listing);
  },
  searchWorkspacePaths: async (params: { q: string; workspaceId?: string; includeIgnored?: boolean }) => {
    const query = new URLSearchParams({ q: params.q });
    if (params.workspaceId) query.set('workspaceId', params.workspaceId);
    if (params.includeIgnored) query.set('includeIgnored', 'true');
    const response = await request<WorkspacePathSearchResponse>(`/api/workspaces/files/search?${query.toString()}`);
    return { ...response, results: Array.isArray(response.results) ? response.results : [], truncated: Boolean(response.truncated) };
  },
  workspacePathGitStates: async (workspaceId: string) =>
    (await request<WorkspacePathGitState[] | null>(`/api/workspaces/${encodeURIComponent(workspaceId)}/git/path-status`) ?? []).map(normalizeWorkspacePathGitState),
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
    request<WorkspaceFileWriteResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/files/revert`, { method: 'POST', body: JSON.stringify(input) }),
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

function normalizeWorkspaceDirectoryListing(listing: WorkspaceDirectoryListing): WorkspaceDirectoryListing {
  return {
    ...listing,
    path: listing.path ?? '',
    hiddenCount: listing.hiddenCount ?? 0,
    entries: (Array.isArray(listing.entries) ? listing.entries : []).map(normalizeWorkspaceTreeEntry)
  };
}

function normalizeWorkspaceTreeEntry(entry: WorkspaceTreeEntry): WorkspaceTreeEntry {
  return {
    ...entry,
    hasChildren: Boolean(entry.hasChildren),
    ignored: Boolean(entry.ignored),
    hidden: Boolean(entry.hidden),
    editable: Boolean(entry.editable)
  };
}

function normalizeWorkspacePathGitState(state: WorkspacePathGitState): WorkspacePathGitState {
  return { ...state, staged: Boolean(state.staged), conflict: Boolean(state.conflict) };
}

function normalizeWorkspacePathMutationResult(result: WorkspacePathMutationResult): WorkspacePathMutationResult {
  return { ...result, invalidatedPaths: Array.isArray(result.invalidatedPaths) ? result.invalidatedPaths : [], refreshed: Boolean(result.refreshed) };
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

function normalizeAuditEvent(event: AuditEvent): AuditEvent {
  return {
    ...event,
    status: normalizeAuditStatus(event.status),
    paths: Array.isArray(event.paths) ? event.paths : [],
    durationMs: event.durationMs ?? 0
  };
}

function normalizeWorkspaceHealth(health: WorkspaceHealth): WorkspaceHealth {
  return {
    ...health,
    summary: normalizeHealthStatus(health.summary),
    checks: (Array.isArray(health.checks) ? health.checks : []).map(normalizeHealthCheck)
  };
}

function normalizeSearchResult(result: SearchResult): SearchResult {
  return {
    ...result,
    type: ['workspace', 'branch', 'savedFilter'].includes(result.type) ? result.type : 'item',
    subtitle: result.subtitle ?? '',
    context: result.context ?? '',
    score: result.score ?? 0
  };
}

function normalizeSavedFilter(filter: SavedFilter): SavedFilter {
  return { ...filter, filters: filter.filters ?? {} };
}

function normalizeRecentItem(item: RecentItem): RecentItem {
  return { ...item, subtitle: item.subtitle ?? '', route: item.route || `/items/${encodeURIComponent(item.itemId)}` };
}

function normalizeHealthCheck(check: HealthCheck): HealthCheck {
  return { ...check, status: normalizeHealthStatus(check.status) };
}

function normalizeAuditStatus(status: AuditEvent['status']): AuditEvent['status'] {
  return status === 'blocked' || status === 'failed' ? status : 'success';
}

function normalizeHealthStatus(status: HealthCheck['status']): HealthCheck['status'] {
  return status === 'warning' || status === 'failed' ? status : 'ok';
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

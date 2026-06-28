import type {
  AppState,
  AuditEvent,
  BranchLoadResult,
  BranchCreateInput,
  BranchSwitchInput,
  FileContent,
  FileNode,
  FileSaveInput,
  GitCommitInput,
  GitActivityEntry,
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
  WorkspaceCreateResult,
  WorkspaceBranches,
  WorkspaceInput,
  WorkspaceHealth,
  WorkspaceDirectoryListing,
	ExplorerTreeMode,
  WorkspaceDirectoryCreateInput,
  WorkspaceFileCreateInput,
  WorkspaceFileRevertInput,
  WorkspaceFileSaveInput,
  WorkspaceFileWriteResult,
  WorkspacePathGitState,
  WorkspacePathMutationResult,
  WorkspacePathRenameInput,
  WorkspacePathSearchResponse,
  WorkspaceContentSearchResponse,
  WorkspaceTreeEntry,
  SystemConfigPaths,
  SourceStructureSettings,
  ScanResult,
  SourceSettingsResult,
  WriteResult
} from '../../lib/types';

export class ApiError extends Error {
  recoveryHint?: string;
  operationLog?: string;

  constructor(message: string, recoveryHint?: string, operationLog?: string) {
    super(message);
    this.name = 'ApiError';
    this.recoveryHint = recoveryHint;
    this.operationLog = operationLog;
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
    throw new ApiError(payload.error ?? payload.message ?? `Request failed: ${res.status}`, payload.recoveryHint, payload.operationLog);
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
  createWorkspace: async (input: WorkspaceInput) => {
    const payload = await request<WorkspaceConfig | WorkspaceCreateResult>('/api/workspaces', { method: 'POST', body: JSON.stringify(input) });
    if (isWorkspaceCreateResult(payload)) {
      return {
        workspace: normalizeWorkspace(payload.workspace),
        operationLog: payload.operationLog ?? ''
      };
    }
    return {
      workspace: normalizeWorkspace(payload),
      operationLog: ''
    };
  },
  createWorkspaceStream: async (input: WorkspaceInput, onLog: (chunk: string) => void) => {
    const res = await fetch('/api/workspaces/stream-create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input)
    });
    if (!res.ok || !res.body) {
      const payload = await res.json().catch(() => ({}));
      throw new ApiError(payload.error ?? `Request failed: ${res.status}`, payload.recoveryHint, payload.operationLog);
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let result: { workspace?: WorkspaceConfig; operationLog?: string } = {};
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const messages = buffer.split('\n\n');
      buffer = messages.pop() ?? '';
      for (const raw of messages) {
        const parsed = parseSSEMessage(raw);
        if (!parsed) continue;
        if (parsed.event === 'log' && typeof parsed.data?.chunk === 'string') {
          onLog(parsed.data.chunk);
        }
        if (parsed.event === 'error') {
          throw new ApiError(parsed.data?.error ?? 'Workspace registration failed', undefined, parsed.data?.operationLog ?? '');
        }
        if (parsed.event === 'result') {
          result = parsed.data ?? {};
        }
      }
    }
    if (!result.workspace) {
      throw new ApiError('Workspace registration failed');
    }
    return {
      workspace: normalizeWorkspace(result.workspace),
      operationLog: result.operationLog ?? ''
    };
  },
  updateWorkspace: (id: string, input: WorkspaceInput) => request<WorkspaceConfig>(`/api/workspaces/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
  deleteWorkspace: (id: string) => request<{ ok: boolean }>(`/api/workspaces/${id}`, { method: 'DELETE' }),
  scan: (workspaceId: string) => request<ScanResult>(`/api/workspaces/${workspaceId}/scan`, { method: 'POST' }),
  loadKanbanBranch: (workspaceId: string, input: { branch?: string; force?: boolean } = {}) =>
    request<BranchLoadResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/kanban/branch`, {
      method: 'POST',
      body: JSON.stringify(input)
    }).then(normalizeBranchLoadResult),
  workspaceHealth: (workspaceId: string) => request<WorkspaceHealth>(`/api/workspaces/${workspaceId}/health`).then(normalizeWorkspaceHealth),
  sourceStructure: (workspaceId: string, directory: string) =>
    request<SourceSettingsResult>(`/api/workspaces/${workspaceId}/source-structure?directory=${encodeURIComponent(directory)}`),
  saveSourceStructure: (workspaceId: string, directory: string, settings: SourceStructureSettings) =>
    request<SourceSettingsResult>(`/api/workspaces/${workspaceId}/source-structure?directory=${encodeURIComponent(directory)}`, {
      method: 'PUT',
      body: JSON.stringify(settings)
    }),
  resetSourceStructure: (workspaceId: string, directory: string) =>
    request<SourceSettingsResult>(`/api/workspaces/${workspaceId}/source-structure?directory=${encodeURIComponent(directory)}`, { method: 'DELETE' }),
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
	searchWorkspaceContent: async (params: { q: string; mode: ExplorerTreeMode; workspaceId?: string; includeIgnored?: boolean; caseSensitive?: boolean }) => {
		const query = contentSearchQuery(params);
		query.set('mode', params.mode);
		if (params.workspaceId) query.set('workspaceId', params.workspaceId);
		if (params.includeIgnored) query.set('includeIgnored', 'true');
		return normalizeContentSearchResponse(await request<WorkspaceContentSearchResponse>(`/api/workspaces/files/content-search?${query.toString()}`));
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
  systemConfigPaths: () => request<SystemConfigPaths>('/api/system/config-paths').then(normalizeSystemConfigPaths),
  updateSystemConfigPaths: (input: { dataDir: string }) =>
    request<SystemConfigPaths>('/api/system/config-paths', { method: 'PUT', body: JSON.stringify(input) }).then(normalizeSystemConfigPaths),
  items: async (params: URLSearchParams) => ((await request<ItemSummary[] | null>(`/api/items?${params.toString()}`)) ?? []).map(normalizeItem),
  item: async (id: string) => normalizeItemDetail(await request<ItemDetail>(`/api/items/${id}`)),
  files: async (id: string) => (await request<FileNode[] | null>(`/api/items/${id}/files`)) ?? [],
	searchItemContent: async (id: string, params: { q: string; caseSensitive?: boolean }) =>
		normalizeContentSearchResponse(await request<WorkspaceContentSearchResponse>(`/api/items/${encodeURIComponent(id)}/content-search?${contentSearchQuery(params).toString()}`)),
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
  gitActivity: async (workspaceId: string, params: { path?: string; limit?: number } = {}) => {
    const query = new URLSearchParams();
    if (params.path) query.set('path', params.path);
    if (params.limit) query.set('limit', String(params.limit));
    const suffix = query.size ? `?${query.toString()}` : '';
    const payload = await request<GitActivityEntry[] | null>(`/api/workspaces/${encodeURIComponent(workspaceId)}/git/activity${suffix}`);
    return (Array.isArray(payload) ? payload : []).map(normalizeGitActivityEntry);
  },
  workspaceBranches: async (workspaceId: string) => {
    const response = await request<WorkspaceBranches>(`/api/workspaces/${encodeURIComponent(workspaceId)}/git/branches`);
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
    request<GitOperationResult>(`/api/workspaces/${workspaceId}/git/switch`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult)
};

function contentSearchQuery(params: { q: string; caseSensitive?: boolean }): URLSearchParams {
	const query = new URLSearchParams({ q: params.q });
	if (params.caseSensitive) query.set('caseSensitive', 'true');
	return query;
}

function normalizeContentSearchResponse(response: WorkspaceContentSearchResponse): WorkspaceContentSearchResponse {
	return {
		...response,
		results: Array.isArray(response.results) ? response.results : [],
		truncated: Boolean(response.truncated),
		filesVisited: Number(response.filesVisited) || 0,
		bytesRead: Number(response.bytesRead) || 0,
		skippedFiles: Number(response.skippedFiles) || 0
	};
}

function normalizeWorkspace(workspace: WorkspaceConfig): WorkspaceConfig {
  return {
    ...workspace,
    registrationMode: workspace.registrationMode === 'remote_clone' ? 'remote_clone' : 'local_path',
    remoteUrl: workspace.remoteUrl ?? '',
    clonePathManaged: Boolean(workspace.clonePathManaged),
    sources: Array.isArray(workspace.sources) ? workspace.sources : []
  };
}

function normalizeSystemConfigPaths(input: SystemConfigPaths): SystemConfigPaths {
  return {
    dataDir: input.dataDir ?? '',
    defaultDataDir: input.defaultDataDir ?? '',
    cloneRootDir: input.cloneRootDir ?? '',
    restartRequired: Boolean(input.restartRequired)
  };
}

function isWorkspaceCreateResult(input: WorkspaceConfig | WorkspaceCreateResult): input is WorkspaceCreateResult {
  return typeof (input as WorkspaceCreateResult).workspace === 'object' && (input as WorkspaceCreateResult).workspace !== null;
}

function parseSSEMessage(raw: string): { event: string; data: any } | null {
  const lines = raw.split('\n').map((line) => line.trimEnd());
  const eventLine = lines.find((line) => line.startsWith('event: '));
  const dataLines = lines.filter((line) => line.startsWith('data: ')).map((line) => line.slice(6));
  if (!eventLine || dataLines.length === 0) return null;
  const event = eventLine.slice(7).trim();
  const dataText = dataLines.join('\n');
  try {
    return { event, data: JSON.parse(dataText) };
  } catch {
    return { event, data: { chunk: dataText } };
  }
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
    sourceMode: item.sourceMode === 'snapshot' ? 'snapshot' : 'working_tree',
    editable: Boolean(item.editable),
    tags: Array.isArray(item.tags) ? item.tags : []
  };
}

function normalizeBranchLoadResult(result: BranchLoadResult): BranchLoadResult {
  if (Array.isArray(result)) {
    const items = result.map(normalizeItem);
    const branch = items[0]?.branch ?? '';
    return {
      workspaceId: items[0]?.workspaceId ?? '',
      branch,
      selectedBranch: branch,
      branchRef: '',
      commit: '',
      currentCheckoutBranch: branch,
      sourceMode: 'working_tree',
      mode: 'working_tree',
      editable: true,
      scannedAt: '',
      itemCount: items.length,
      warnings: [],
      items
    };
  }
  const sourceMode = result.sourceMode === 'snapshot' || result.mode === 'snapshot' ? 'snapshot' : 'working_tree';
  const items = (Array.isArray(result.items) ? result.items : []).map(normalizeItem);
  return {
    ...result,
    branch: result.branch || result.selectedBranch,
    selectedBranch: result.selectedBranch || result.branch,
    currentCheckoutBranch: result.currentCheckoutBranch ?? '',
    sourceMode,
    mode: sourceMode,
    editable: Boolean(result.editable),
    itemCount: Number(result.itemCount) || items.length,
    warnings: Array.isArray(result.warnings) ? result.warnings : [],
    items
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

function normalizeGitActivityEntry(entry: GitActivityEntry): GitActivityEntry {
  return {
    ...entry,
    committedAt: entry.committedAt ?? '',
    author: entry.author ?? '',
    message: entry.message ?? '',
    paths: Array.isArray(entry.paths) ? entry.paths : []
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
  draft: 'Draft',
  in_progress: 'In Progress',
  review: 'Review',
  done: 'Done'
} as const;

export const statusOrder = Object.keys(statusLabels) as Array<keyof typeof statusLabels>;
export const editableStatusOrder = statusOrder.filter((status) => status !== 'unsorted');

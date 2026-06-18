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
  NewPlanInput,
  PathSelection,
  PlanDetail,
  PlanMetadataUpdateInput,
  PlanStatusUpdateInput,
  PlanSummary,
  RepositoryConfig,
  RepositoryInput,
  RepositorySettings,
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
  repositories: async () => ((await request<RepositoryConfig[] | null>('/api/repositories')) ?? []).map(normalizeRepository),
  createRepository: (input: RepositoryInput) => request<RepositoryConfig>('/api/repositories', { method: 'POST', body: JSON.stringify(input) }),
  updateRepository: (id: string, input: RepositoryInput) => request<RepositoryConfig>(`/api/repositories/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
  deleteRepository: (id: string) => request<{ ok: boolean }>(`/api/repositories/${id}`, { method: 'DELETE' }),
  scan: (repositoryId: string) => request<ScanResult>(`/api/repositories/${repositoryId}/scan`, { method: 'POST' }),
  sourceSettings: (repositoryId: string, directory: string) =>
    request<SourceSettingsResult>(`/api/repositories/${repositoryId}/source-settings?directory=${encodeURIComponent(directory)}`),
  saveSourceSettings: (repositoryId: string, directory: string, settings: RepositorySettings) =>
    request<SourceSettingsResult>(`/api/repositories/${repositoryId}/source-settings?directory=${encodeURIComponent(directory)}`, {
      method: 'PUT',
      body: JSON.stringify(settings)
    }),
  selectDirectory: () => request<PathSelection>('/api/system/select-directory', { method: 'POST' }),
  openPath: (path: string) => request<{ ok: boolean }>('/api/system/open-path', { method: 'POST', body: JSON.stringify({ path }) }),
  plans: async (params: URLSearchParams) => ((await request<PlanSummary[] | null>(`/api/plans?${params.toString()}`)) ?? []).map(normalizePlan),
  plan: async (id: string) => normalizePlanDetail(await request<PlanDetail>(`/api/plans/${id}`)),
  files: async (id: string) => (await request<FileNode[] | null>(`/api/plans/${id}/files`)) ?? [],
  file: (id: string, fileId: string) => request<FileContent>(`/api/plans/${id}/files/${fileId}`),
  saveFile: (id: string, fileId: string, input: FileSaveInput) =>
    request<FileContent>(`/api/plans/${id}/files/${fileId}`, { method: 'POST', body: JSON.stringify(input) }),
  revertFile: (id: string, fileId: string) =>
    request<ScanResult>(`/api/plans/${id}/files/${fileId}/revert`, { method: 'POST' }),
  saveMetadata: (id: string, input: PlanMetadataUpdateInput) =>
    request<WriteResult>(`/api/plans/${id}/metadata`, { method: 'PATCH', body: JSON.stringify(input) }),
  updateStatus: (id: string, input: PlanStatusUpdateInput) =>
    request<WriteResult>(`/api/plans/${id}/status`, { method: 'PATCH', body: JSON.stringify(input) }),
  createPlan: (input: NewPlanInput) => request<WriteResult>('/api/plans', { method: 'POST', body: JSON.stringify(input) }),
  diff: (id: string) => request<{ diff: string }>(`/api/plans/${id}/diff`),
  gitStatus: (repositoryId: string) => request<GitStatus>(`/api/repositories/${repositoryId}/git/status`).then(normalizeGitStatus),
  gitFetch: (repositoryId: string, input: GitOperationInput = {}) =>
    request<GitOperationResult>(`/api/repositories/${repositoryId}/git/fetch`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult),
  gitPull: (repositoryId: string, input: GitOperationInput = {}) =>
    request<GitOperationResult>(`/api/repositories/${repositoryId}/git/pull`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult),
  gitPush: (repositoryId: string, input: GitOperationInput = {}) =>
    request<GitOperationResult>(`/api/repositories/${repositoryId}/git/push`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult),
  gitCommit: (repositoryId: string, input: GitCommitInput) =>
    request<GitOperationResult>(`/api/repositories/${repositoryId}/git/commit`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult),
  createBranch: (repositoryId: string, input: BranchCreateInput) =>
    request<GitOperationResult>(`/api/repositories/${repositoryId}/git/branches`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult),
  switchBranch: (repositoryId: string, input: BranchSwitchInput) =>
    request<GitOperationResult>(`/api/repositories/${repositoryId}/git/switch`, { method: 'POST', body: JSON.stringify(input) }).then(normalizeGitResult)
};

function normalizeRepository(repo: RepositoryConfig): RepositoryConfig {
  return {
    ...repo,
    planDirectories: Array.isArray(repo.planDirectories) ? repo.planDirectories : []
  };
}

function normalizePlan(plan: PlanSummary): PlanSummary {
  return {
    ...plan,
    tags: Array.isArray(plan.tags) ? plan.tags : []
  };
}

function normalizePlanDetail(plan: PlanDetail): PlanDetail {
  return {
    ...normalizePlan(plan),
    documents: Array.isArray(plan.documents) ? plan.documents : [],
    metadata: plan.metadata ?? {},
    counts: plan.counts ?? { files: 0 }
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
    status: normalizeGitStatus(result.status ?? { repositoryId: '', branch: '', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] })
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

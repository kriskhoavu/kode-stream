import type { FileContent, FileNode, PlanDetail, PlanSummary, RepositoryConfig, RepositoryInput, ScanResult } from './types';

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
  repositories: async () => ((await request<RepositoryConfig[] | null>('/api/repositories')) ?? []).map(normalizeRepository),
  createRepository: (input: RepositoryInput) => request<RepositoryConfig>('/api/repositories', { method: 'POST', body: JSON.stringify(input) }),
  scan: (repositoryId: string) => request<ScanResult>(`/api/repositories/${repositoryId}/scan`, { method: 'POST' }),
  plans: async (params: URLSearchParams) => ((await request<PlanSummary[] | null>(`/api/plans?${params.toString()}`)) ?? []).map(normalizePlan),
  plan: async (id: string) => normalizePlanDetail(await request<PlanDetail>(`/api/plans/${id}`)),
  files: async (id: string) => (await request<FileNode[] | null>(`/api/plans/${id}/files`)) ?? [],
  file: (id: string, fileId: string) => request<FileContent>(`/api/plans/${id}/files/${fileId}`),
  diff: (id: string) => request<{ diff: string }>(`/api/plans/${id}/diff`)
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

export const statusLabels = {
  ideas: 'Ideas',
  draft: 'Draft',
  in_progress: 'In Progress',
  review: 'Review',
  done: 'Done'
} as const;

export const statusOrder = Object.keys(statusLabels) as Array<keyof typeof statusLabels>;

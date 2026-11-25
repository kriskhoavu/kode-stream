import type { EmbeddedAISessionResult, SafeSessionRecord, SessionChanges, HarnessAgentRun, HarnessApproval, HarnessCostSummary, HarnessDecision, HarnessRoleHealth, HarnessRoleId, HarnessRun, HarnessRunDetail, HarnessSettings, SettingsFilePart, SettingsFileView, HarnessStatus, HarnessCreateTaskInput, HarnessRunFileDiff, HarnessRunPlan, HarnessSourcePoll, HarnessTask, HarnessTranscript } from '../../lib/types';
import { ApiError, request } from './http';
import { normalizeHarnessTask, normalizeHarnessRunDetail, normalizeHarnessSettings } from './normalize';

export const harnessApi = {
  harnessStatus: (signal?: AbortSignal) => request<HarnessStatus>('/api/harness/status', { signal }, !signal),
  harnessTasks: async (signal?: AbortSignal) => (await request<{ tasks?: HarnessTask[] | null }>('/api/harness/tasks', { signal }, !signal)).tasks?.map(normalizeHarnessTask) ?? [],
  harnessQueue: async (signal?: AbortSignal) => (await request<{ tasks?: HarnessTask[] | null }>('/api/harness/queue', { signal }, !signal)).tasks?.map(normalizeHarnessTask) ?? [],
  harnessRuns: async (signal?: AbortSignal) => (await request<{ runs?: HarnessRun[] | null }>('/api/harness/runs', { signal }, !signal)).runs ?? [],
  harnessRun: async (runId: string, signal?: AbortSignal) => normalizeHarnessRunDetail(await request<HarnessRunDetail>(`/api/harness/runs/${encodeURIComponent(runId)}`, { signal }, !signal)),
  harnessRunPlan: async (runId: string, signal?: AbortSignal) => { const plan = await request<HarnessRunPlan>(`/api/harness/runs/${encodeURIComponent(runId)}/plan`, { signal }, !signal); return { ...plan, files: plan.files ?? [] }; },
  harnessRunFileDiff: (runId: string, path: string, signal?: AbortSignal) => request<HarnessRunFileDiff>(`/api/harness/runs/${encodeURIComponent(runId)}/diff?path=${encodeURIComponent(path)}`, { signal }, !signal),
  harnessRunChanges: (runId: string, signal?: AbortSignal) => request<SessionChanges>(`/api/harness/runs/${encodeURIComponent(runId)}/changes`, { signal }, !signal),
  harnessAgentRunTranscript: (agentRunId: string, limit = 50, signal?: AbortSignal) => request<HarnessTranscript>(`/api/harness/agent-runs/${encodeURIComponent(agentRunId)}/transcript?limit=${limit}`, { signal }, false),
  harnessAgentRun: (agentRunId: string, signal?: AbortSignal) => request<{ agentRun: HarnessAgentRun; session?: SafeSessionRecord }>(`/api/harness/agent-runs/${encodeURIComponent(agentRunId)}`, { signal }, !signal),
  cancelHarnessRun: (runId: string, expectedRevision: number, reason?: string) => request<HarnessRun>(`/api/harness/runs/${encodeURIComponent(runId)}/cancel`, { method: 'POST', body: JSON.stringify({ expectedRevision, reason }) }, false),
  harnessApprovals: async (signal?: AbortSignal) => (await request<{ approvals?: HarnessApproval[] | null }>('/api/harness/approvals', { signal }, !signal)).approvals ?? [],
  /** Resolves to undefined when someone else decided first (409 `already_decided`). */
  approveHarnessGate: (approval: Pick<HarnessApproval, 'id' | 'revision'>, idempotencyKey: string, reason?: string) => decideHarnessGate(approval, 'approve', idempotencyKey, reason),
  rejectHarnessGate: (approval: Pick<HarnessApproval, 'id' | 'revision'>, idempotencyKey: string, reason: string, decision: 'reject' | 'replan' = 'reject') => decideHarnessGate(approval, decision, idempotencyKey, reason),
  startHarnessReviewSession: (runId: string, input: { provider: string; idempotencyKey: string; columns: number; rows: number }) => request<EmbeddedAISessionResult>(`/api/harness/runs/${encodeURIComponent(runId)}/review-session`, { method: 'POST', body: JSON.stringify(input) }, false),
  pinHarnessTask: (task: Pick<HarnessTask, 'id' | 'revision'>, pinned: boolean) => request<HarnessTask>(`/api/harness/tasks/${encodeURIComponent(task.id)}/pin`, { method: 'POST', body: JSON.stringify({ pinned, expectedRevision: task.revision }) }, false).then(normalizeHarnessTask),
  /** `before` names the task this one sits above. Omitted means last. */
  setHarnessPriority: (task: Pick<HarnessTask, 'id' | 'revision'>, input: { priorityOverride?: number; before?: string }) => request<HarnessTask>(`/api/harness/tasks/${encodeURIComponent(task.id)}/priority`, { method: 'POST', body: JSON.stringify({ ...input, expectedRevision: task.revision }) }, false).then(normalizeHarnessTask),
  setHarnessTaskQuestions: (task: Pick<HarnessTask, 'id' | 'revision'>, askQuestions: boolean | null) => request<HarnessTask>(`/api/harness/tasks/${encodeURIComponent(task.id)}/questions`, { method: 'POST', body: JSON.stringify({ askQuestions, expectedRevision: task.revision }) }, false).then(normalizeHarnessTask),
  createHarnessTask: (input: HarnessCreateTaskInput) => request<HarnessTask>('/api/harness/tasks', { method: 'POST', body: JSON.stringify(input) }, false).then(normalizeHarnessTask),
  startHarnessTask: (task: Pick<HarnessTask, 'id' | 'revision'>, idempotencyKey: string) => request<HarnessTask>(`/api/harness/tasks/${encodeURIComponent(task.id)}/start`, { method: 'POST', body: JSON.stringify({ idempotencyKey, expectedRevision: task.revision }) }, false).then(normalizeHarnessTask),
  removeHarnessTask: (task: Pick<HarnessTask, 'id' | 'revision'>) => request<HarnessTask>(`/api/harness/tasks/${encodeURIComponent(task.id)}/remove`, { method: 'POST', body: JSON.stringify({ expectedRevision: task.revision }) }, false),
  resetHarnessTask: (task: Pick<HarnessTask, 'id' | 'revision'>) => request<HarnessTask>(`/api/harness/tasks/${encodeURIComponent(task.id)}/reset`, { method: 'POST', body: JSON.stringify({ expectedRevision: task.revision }) }, false).then(normalizeHarnessTask),
  dismissHarnessTask: (task: Pick<HarnessTask, 'id' | 'revision'>, reason?: string) => request<HarnessTask>(`/api/harness/tasks/${encodeURIComponent(task.id)}/dismiss`, { method: 'POST', body: JSON.stringify({ reason, expectedRevision: task.revision }) }, false).then(normalizeHarnessTask),
  clearHarnessQueue: (workspaceId?: string) => request<{ removed: number }>('/api/harness/queue/clear', { method: 'POST', body: JSON.stringify({ workspaceId }) }, false),
  pollHarnessScouts: (workspaceId?: string) => request<{ started: boolean; sources?: HarnessSourcePoll[] }>('/api/harness/scout/poll', { method: 'POST', ...(workspaceId ? { body: JSON.stringify({ workspaceId }) } : {}) }, false),
  pauseHarnessRole: (role: HarnessRoleId) => request<HarnessRoleHealth>(`/api/harness/roles/${encodeURIComponent(role)}/pause`, { method: 'POST' }, false),
  resumeHarnessRole: (role: HarnessRoleId) => request<HarnessRoleHealth>(`/api/harness/roles/${encodeURIComponent(role)}/resume`, { method: 'POST' }, false),
  harnessCostSummary: (groupBy: HarnessCostSummary['groupBy'], range: { from?: string; to?: string } = {}, signal?: AbortSignal) => {
    const query = new URLSearchParams({ groupBy });
    if (range.from) query.set('from', range.from);
    if (range.to) query.set('to', range.to);
    return request<HarnessCostSummary>(`/api/harness/cost?${query.toString()}`, { signal }, !signal).then((summary) => ({ ...summary, groups: summary.groups ?? [] }));
  },
  harnessSettings: async (signal?: AbortSignal) => normalizeHarnessSettings(await request<HarnessSettings>('/api/harness/settings', { signal }, !signal)),
  saveHarnessSettings: async (settings: HarnessSettings) => normalizeHarnessSettings(await request<HarnessSettings>('/api/harness/settings', { method: 'PUT', body: JSON.stringify({ ...settings, expectedRevision: settings.revision }) }, false)),
  settingsFile: (workspaceId: string, signal?: AbortSignal) => request<SettingsFileView>(`/api/workspaces/${encodeURIComponent(workspaceId)}/settings-file`, { signal }, !signal),
  exportSettingsFile: (workspaceId: string, parts: SettingsFilePart[]) => request<{ path: string; has: SettingsFilePart[] }>(`/api/workspaces/${encodeURIComponent(workspaceId)}/settings-file/export`, { method: 'POST', body: JSON.stringify({ parts }) }, false),
  importSettingsFile: (workspaceId: string, parts: SettingsFilePart[]) => request<{ imported: SettingsFilePart[] }>(`/api/workspaces/${encodeURIComponent(workspaceId)}/settings-file/import`, { method: 'POST', body: JSON.stringify({ parts }) }, false),
  harnessEventsURL: (lastEventId?: number) => lastEventId ? `/api/harness/events?after=${lastEventId}` : '/api/harness/events'
};

async function decideHarnessGate(approval: Pick<HarnessApproval, 'id' | 'revision'>, decision: HarnessDecision, idempotencyKey: string, reason?: string): Promise<HarnessApproval | undefined> {
  try {
    return await request<HarnessApproval>(`/api/harness/approvals/${encodeURIComponent(approval.id)}/decide`, { method: 'POST', body: JSON.stringify({ decision, reason: reason?.trim() || undefined, idempotencyKey, expectedRevision: approval.revision }) }, false);
  } catch (error) {
    if (error instanceof ApiError && error.status === 409 && error.code === 'already_decided') return undefined;
    throw error;
  }
}

import type { WorkstreamBranchLoadResult, ReviewedPlanImportInput, JiraConnection, JiraConnectionTest, JiraIssueState, WorkspaceConfig, WorkspaceCreateResult, WorkspaceInput, WorkspaceHealth, WorkspaceImportPreview, WorkspaceImportRequest, WorkspaceImportResult, SourceStructureSettings, ScanResult, WorkspaceRuntimeConfig, SourceSettingsResult, WriteResult } from '../../lib/types';
import { ApiError, maxSSEBufferBytes, maxSSETotalBytes, ApiResponseError, apiURL, request, stringField, readJSONResponse, parseSSEMessage } from './http';
import { normalizeWorkspace, normalizeWorkspaceImportPreview, normalizeWorkspaceImportResult, isWorkspaceCreateResult, normalizeWorkstreamBranchLoadResult, normalizeWorkspaceHealth } from './normalize';

export const workspaceApi = {
  workspaces: async (signal?: AbortSignal) => ((await request<WorkspaceConfig[] | null>('/api/workspaces', { signal }, !signal)) ?? []).map(normalizeWorkspace),
	previewWorkspaceImport: (sourcePath: string) => request<WorkspaceImportPreview>('/api/workspaces/import-preview', { method: 'POST', body: JSON.stringify({ sourcePath }) }).then(normalizeWorkspaceImportPreview),
	importWorkspaces: (input: WorkspaceImportRequest) => request<WorkspaceImportResult[] | null>('/api/workspaces/import', { method: 'POST', body: JSON.stringify(input) }).then((results) => (Array.isArray(results) ? results : []).map(normalizeWorkspaceImportResult)),
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
  createWorkspaceStream: async (input: WorkspaceInput, onLog: (chunk: string) => void, signal?: AbortSignal) => {
    const res = await fetch(apiURL('/api/workspaces/stream-create'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input), signal
    });
    if (!res.ok || !res.body) {
      const payload: Record<string, unknown> = await readJSONResponse(res);
      throw new ApiError(stringField(payload, 'error') || `Request failed: ${res.status}`, stringField(payload, 'recoveryHint'), stringField(payload, 'operationLog'));
    }
    const contentType = res.headers?.get?.('content-type')?.toLowerCase() ?? '';
    if (!contentType.includes('text/event-stream')) {
      await res.body.cancel();
      throw new ApiResponseError('The server returned an unexpected workspace progress format.');
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let total = 0;
    let result: { workspace?: WorkspaceConfig; operationLog?: string } = {};
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
			if (signal?.aborted) {
				await reader.cancel();
				throw new DOMException('The workspace registration was cancelled.', 'AbortError');
			}
      total += value.byteLength;
      if (total > maxSSETotalBytes) {
        await reader.cancel();
        throw new ApiResponseError('Workspace progress response is too large.');
      }
      buffer += decoder.decode(value, { stream: true });
      if (buffer.length > maxSSEBufferBytes) {
        await reader.cancel();
        throw new ApiResponseError('Workspace progress response is too large.');
      }
      const messages = buffer.split('\n\n');
      buffer = messages.pop() ?? '';
      for (const raw of messages) {
        const parsed = parseSSEMessage(raw);
        if (!parsed) continue;
			if (signal?.aborted) {
				await reader.cancel();
				throw new DOMException('The workspace registration was cancelled.', 'AbortError');
			}
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
  testJiraConnection: (workspaceId: string, connection: JiraConnection) => request<JiraConnectionTest>(`/api/workspaces/${encodeURIComponent(workspaceId)}/jira/test`, { method: 'POST', body: JSON.stringify(connection) }),
  workspaceJiraIssue: (workspaceId: string, issueKey: string, signal?: AbortSignal) => request<JiraIssueState>(`/api/workspaces/${encodeURIComponent(workspaceId)}/jira/issues/${encodeURIComponent(issueKey)}`, { signal }, !signal),
  refreshWorkspaceJiraIssue: (workspaceId: string, issueKey: string, signal?: AbortSignal) => request<JiraIssueState>(`/api/workspaces/${encodeURIComponent(workspaceId)}/jira/issues/${encodeURIComponent(issueKey)}?refresh=1`, { signal }, false),
  workspaceJiraAttachmentURL: (workspaceId: string, issueKey: string, attachmentId: string) => apiURL(`/api/workspaces/${encodeURIComponent(workspaceId)}/jira/issues/${encodeURIComponent(issueKey)}/attachments/${encodeURIComponent(attachmentId)}`),
  workspaceRuntime: (workspaceId: string) => request<WorkspaceRuntimeConfig | { runtime: null }>(`/api/workspaces/${encodeURIComponent(workspaceId)}/runtime`).then((payload) => {
    if ('runtime' in payload) return payload.runtime;
    return payload;
  }),
  saveWorkspaceRuntime: (workspaceId: string, input: WorkspaceRuntimeConfig | null) => request<WorkspaceRuntimeConfig | null>(`/api/workspaces/${encodeURIComponent(workspaceId)}/runtime`, { method: 'PUT', body: JSON.stringify(input) }),
  deleteWorkspace: (id: string) => request<{ ok: boolean }>(`/api/workspaces/${id}`, { method: 'DELETE' }),
  scan: (workspaceId: string) => request<ScanResult>(`/api/workspaces/${workspaceId}/scan`, { method: 'POST' }),
  loadWorkstreamBranch: (workspaceId: string, input: { branch?: string; force?: boolean } = {}) =>
    request<WorkstreamBranchLoadResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/workstream/branch`, {
      method: 'POST',
      body: JSON.stringify(input)
    }, true).then(normalizeWorkstreamBranchLoadResult),
	loadWorkstreamCheckout: (workspaceId: string, input: { force?: boolean } = {}, signal?: AbortSignal) =>
		request<WorkstreamBranchLoadResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/workstream/checkout`, {
			method: 'POST', body: JSON.stringify(input), signal
		}, !signal).then(normalizeWorkstreamBranchLoadResult),
	loadBranchReview: (workspaceId: string, input: { branch: string; force?: boolean }, signal?: AbortSignal) =>
		request<WorkstreamBranchLoadResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/reviews/branch`, {
			method: 'POST', body: JSON.stringify(input), signal
		}, !signal).then(normalizeWorkstreamBranchLoadResult),
	importReviewedPlan: (workspaceId: string, input: ReviewedPlanImportInput) =>
		request<WriteResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/reviews/import`, {
			method: 'POST', body: JSON.stringify(input)
		}),
  workspaceHealth: (workspaceId: string, signal?: AbortSignal) => request<WorkspaceHealth>(`/api/workspaces/${workspaceId}/health`, { signal }, !signal).then(normalizeWorkspaceHealth),
  sourceStructure: (workspaceId: string, directory: string) =>
    request<SourceSettingsResult>(`/api/workspaces/${workspaceId}/source-structure?directory=${encodeURIComponent(directory)}`),
  saveSourceStructure: (workspaceId: string, directory: string, settings: SourceStructureSettings) =>
    request<SourceSettingsResult>(`/api/workspaces/${workspaceId}/source-structure?directory=${encodeURIComponent(directory)}`, {
      method: 'PUT',
      body: JSON.stringify(settings)
    }),
  resetSourceStructure: (workspaceId: string, directory: string) =>
    request<SourceSettingsResult>(`/api/workspaces/${workspaceId}/source-structure?directory=${encodeURIComponent(directory)}`, { method: 'DELETE' })
};

import type { AICapability, AIProviderCapabilityCatalog, AISettings, AIPlanPreset, AISessionEligibility, AISessionLaunchInput, AISessionLaunchResult, EmbeddedAISession, EmbeddedAISessionLaunchInput, EmbeddedAISessionResult, SafeSessionRecord, SessionChanges, ClaudeTranscript, ClaudeTurn } from '../../lib/types';
import { request } from './http';
import { normalizeAISettings } from './normalize';

export const aiApi = {
  aiCapabilities: (signal?: AbortSignal) => request<AICapability[]>('/api/ai/capabilities', { signal }, !signal),
  aiProviderCapabilities: (providerId: string, context?: { itemId?: string; workspaceId?: string; signal?: AbortSignal }) => {
    const query = new URLSearchParams();
    if (context?.itemId) query.set('itemId', context.itemId);
    if (context?.workspaceId) query.set('workspaceId', context.workspaceId);
    const suffix = query.size > 0 ? `?${query.toString()}` : '';
    return request<AIProviderCapabilityCatalog>(`/api/ai/providers/${encodeURIComponent(providerId)}/capabilities${suffix}`, { signal: context?.signal }, !context?.signal);
  },
  aiPresets: (signal?: AbortSignal) => request<AIPlanPreset[]>('/api/ai/presets', { signal }, !signal),
  aiSettings: (signal?: AbortSignal) => request<AISettings>('/api/ai/settings', { signal }, !signal).then(normalizeAISettings),
  saveAISettings: (settings: AISettings) => request<AISettings>('/api/ai/settings', { method: 'PUT', body: JSON.stringify(normalizeAISettings(settings)) }).then(normalizeAISettings),
  aiSessionEligibility: (itemId: string, signal?: AbortSignal) => request<AISessionEligibility>(`/api/items/${encodeURIComponent(itemId)}/ai-session-eligibility`, { signal }, !signal),
  launchAISession: (itemId: string, input: AISessionLaunchInput) => request<AISessionLaunchResult>(`/api/items/${encodeURIComponent(itemId)}/ai-sessions`, { method: 'POST', body: JSON.stringify(input) }),
	launchWorkspaceAISession: (workspaceId: string, input: AISessionLaunchInput & { contextPath: string }) => request<AISessionLaunchResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/ai-sessions`, { method: 'POST', body: JSON.stringify(input) }),
	startEmbeddedAISession: (itemId: string, input: EmbeddedAISessionLaunchInput) => request<EmbeddedAISessionResult>(`/api/items/${encodeURIComponent(itemId)}/ai-sessions/embedded`, { method: 'POST', body: JSON.stringify(input) }, false),
	startEmbeddedWorkspaceAISession: (workspaceId: string, input: EmbeddedAISessionLaunchInput & { contextPath: string }) => request<EmbeddedAISessionResult>(`/api/workspaces/${encodeURIComponent(workspaceId)}/ai-sessions/embedded`, { method: 'POST', body: JSON.stringify(input) }, false),
	embeddedAISession: (sessionId: string) => request<EmbeddedAISession>(`/api/ai/sessions/${encodeURIComponent(sessionId)}`),
	embeddedAISessionGrant: (sessionId: string) => request<EmbeddedAISessionResult['grant']>(`/api/ai/sessions/${encodeURIComponent(sessionId)}/grant`, { method: 'POST' }, false),
	aiSessionRecords: async (workspaceId: string, branch?: string, signal?: AbortSignal) => {
		const query = new URLSearchParams({ workspaceId });
		if (branch) query.set('branch', branch);
		return (await request<SafeSessionRecord[] | null>(`/api/ai/session-records?${query.toString()}`, { signal }, false)) ?? [];
	},
  cancelEmbeddedAISession: (sessionId: string) => request<EmbeddedAISession>(`/api/ai/sessions/${encodeURIComponent(sessionId)}`, { method: 'DELETE' }),
	claudeTranscripts: async (workspaceId: string, branch?: string, signal?: AbortSignal) => {
		const query = branch ? `?${new URLSearchParams({ branch }).toString()}` : '';
		return (await request<ClaudeTranscript[] | null>(`/api/workspaces/${encodeURIComponent(workspaceId)}/claude-transcripts${query}`, { signal }, false)) ?? [];
	},
	claudeTranscriptTail: (workspaceId: string, sessionId: string, limit = 20, signal?: AbortSignal) => request<ClaudeTurn[]>(`/api/workspaces/${encodeURIComponent(workspaceId)}/claude-transcripts/${encodeURIComponent(sessionId)}/tail?limit=${limit}`, { signal }, false),
	deleteSession: (sessionId: string) => request<void>(`/api/sessions/${encodeURIComponent(sessionId)}`, { method: 'DELETE' }),
	sessionChanges: (sessionId: string, signal?: AbortSignal) => request<SessionChanges>(`/api/sessions/${encodeURIComponent(sessionId)}/changes`, { signal }, !signal),
	sessionFileDiff: (sessionId: string, path: string, signal?: AbortSignal) => request<{ path: string; diff: string; truncated: boolean }>(`/api/sessions/${encodeURIComponent(sessionId)}/diff?path=${encodeURIComponent(path)}`, { signal }, !signal)
};

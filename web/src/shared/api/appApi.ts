import type { AppState, AgentConnectToken, CloudAgent, AuditEvent, RecentItem, SavedFilter, SearchResult } from '../../lib/types';
import { apiURL, request } from './http';
import { normalizeAuditEvent, normalizeSearchResult, normalizeSavedFilter, normalizeRecentItem } from './normalize';

export const appApi = {
  health: () => request<{ status?: string }>('/api/health', undefined, false),
  localServerReachable: async () => {
    try {
      await fetch(apiURL('/api/health'), { headers: { 'Content-Type': 'application/json' } });
      return true;
    } catch {
      return false;
    }
  },
  state: (signal?: AbortSignal) => request<AppState>('/api/state', { signal }, !signal),
  logout: () => request<{ ok: boolean }>('/api/auth/logout', { method: 'POST' }),
  cloudAgents: async () => ((await request<CloudAgent[] | null>('/api/agents')) ?? []),
  createAgentConnectToken: (input: { name?: string; platform?: string } = {}) => request<AgentConnectToken>('/api/agents/connect-token', { method: 'POST', body: JSON.stringify(input) }),
  search: async (params: { q: string; workspaceId?: string; types?: string[]; limit?: number; signal?: AbortSignal }) => {
    const query = new URLSearchParams({ q: params.q });
    if (params.workspaceId) query.set('workspaceId', params.workspaceId);
    if (params.types?.length) query.set('types', params.types.join(','));
    if (params.limit) query.set('limit', String(params.limit));
    return ((await request<SearchResult[] | null>(`/api/search?${query.toString()}`, params.signal ? { signal: params.signal } : undefined, !params.signal)) ?? []).map(normalizeSearchResult);
  },
  savedFilters: async (signal?: AbortSignal) => ((await request<SavedFilter[] | null>('/api/saved-filters', { signal }, !signal)) ?? []).map(normalizeSavedFilter),
  saveFilter: (filter: Pick<SavedFilter, 'name' | 'route' | 'filters'> & Partial<Pick<SavedFilter, 'id' | 'workspaceId'>>) =>
    request<SavedFilter>('/api/saved-filters', { method: 'POST', body: JSON.stringify(filter) }).then(normalizeSavedFilter),
  deleteFilter: (id: string) => request<{ ok: boolean }>(`/api/saved-filters/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  recentItems: async (limit = 10) => ((await request<RecentItem[] | null>(`/api/recent-items?limit=${limit}`)) ?? []).map(normalizeRecentItem),
  recordRecentItem: (itemId: string) => request<{ ok: boolean }>('/api/recent-items', { method: 'POST', body: JSON.stringify({ itemId }) }),
  auditEvents: async (params: { workspaceId?: string; limit?: number; signal?: AbortSignal } = {}) => {
    const query = new URLSearchParams();
    if (params.workspaceId) query.set('workspaceId', params.workspaceId);
    if (params.limit) query.set('limit', String(params.limit));
    const suffix = query.size ? `?${query.toString()}` : '';
    return ((await request<AuditEvent[] | null>(`/api/audit-events${suffix}`, { signal: params.signal }, !params.signal)) ?? []).map(normalizeAuditEvent);
  }
};

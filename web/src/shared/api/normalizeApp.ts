import type { AuditEvent, RecentItem, SavedFilter, SearchResult } from '../../lib/types';

export function normalizeAuditEvent(event: AuditEvent): AuditEvent {
  return {
    ...event,
    status: normalizeAuditStatus(event.status),
    paths: Array.isArray(event.paths) ? event.paths : [],
    durationMs: event.durationMs ?? 0
  };
}

export function normalizeSearchResult(result: SearchResult): SearchResult {
  return {
    ...result,
    type: ['workspace', 'branch', 'savedFilter'].includes(result.type) ? result.type : 'item',
    subtitle: result.subtitle ?? '',
    context: result.context ?? '',
    score: result.score ?? 0
  };
}

export function normalizeSavedFilter(filter: SavedFilter): SavedFilter {
  return { ...filter, filters: filter.filters ?? {} };
}

export function normalizeRecentItem(item: RecentItem): RecentItem {
  return { ...item, subtitle: item.subtitle ?? '', route: item.route || `/items/${encodeURIComponent(item.itemId)}` };
}

export function normalizeAuditStatus(status: AuditEvent['status']): AuditEvent['status'] {
  return status === 'blocked' || status === 'failed' ? status : 'success';
}

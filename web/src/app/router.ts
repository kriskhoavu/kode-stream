export type KnowledgeView = 'browse' | 'read' | 'graph';
export interface KnowledgeLocation { workspaceId?: string; slug?: string; view?: KnowledgeView; }
export type WorkbenchView = 'canvas' | 'sessions';
export interface CanvasLocation { workspaceId?: string; selectedNodeId?: string; view?: WorkbenchView; }
export interface ReviewLocation { workspaceId?: string; branch?: string; }
export type ItemView = 'run';
export type HarnessRunTab = 'ticket' | 'plan' | 'stages' | 'findings' | 'changes' | 'live' | 'links';

export type Route =
  | { name: 'workstream'; focusedItemId?: string; focusedSessionId?: string }
  | { name: 'workspaces' }
  | { name: 'settings' }
  | { name: 'knowledge'; location?: KnowledgeLocation }
  | { name: 'canvas'; location?: CanvasLocation }
  | { name: 'review'; location?: ReviewLocation }
  | { name: 'item'; itemId: string; /** `run` shows the item's harness run whole, full width. */ view?: ItemView }
  /** Redirects to the card that holds the run. */
  | { name: 'harness-run'; runId: string }
  | { name: 'not-found'; path: string };

export const routeDescriptors: Record<Route['name'], { title: string; mainId: string }> = {
  workstream: { title: 'Workstream · Kode Stream', mainId: 'app-main' },
  workspaces: { title: 'Workspaces · Kode Stream', mainId: 'app-main' },
  settings: { title: 'Settings · Kode Stream', mainId: 'app-main' },
  knowledge: { title: 'Knowledge · Kode Stream', mainId: 'app-main' },
  canvas: { title: 'Agent Space · Kode Stream', mainId: 'app-main' },
  review: { title: 'Branch Review · Kode Stream', mainId: 'app-main' },
  item: { title: 'Item · Kode Stream', mainId: 'app-main' },
  'harness-run': { title: 'Workstream · Kode Stream', mainId: 'app-main' },
  'not-found': { title: 'Page not found · Kode Stream', mainId: 'app-main' }
};

export function routeFromLocation(): Route {
  return routeFromPath(window.location.pathname, window.location.search);
}

export function routeFromPath(path: string, search = ''): Route {
  if (path.startsWith('/items/')) {
    const itemId = decodeURIComponent(path.split('/')[2] ?? '');
    if (!itemId) return { name: 'not-found', path };
    const view = new URLSearchParams(search).get('view') === 'run' ? 'run' : undefined;
    return view ? { name: 'item', itemId, view } : { name: 'item', itemId };
  }
  if (path === '/harness' || path.startsWith('/harness/')) {
    const segments = path.replace(/\/+$/, '').split('/').slice(2);
    const runId = segments.length === 2 && segments[0] === 'runs' ? decodeURIComponent(segments[1]) : '';
    return runId ? { name: 'harness-run', runId } : { name: 'workstream' };
  }
  if (path === '/workspaces') {
    return { name: 'workspaces' };
  }
  if (path === '/settings') {
    return { name: 'settings' };
  }
  if (path === '/knowledge') {
    return { name: 'knowledge', location: knowledgeLocationFromSearch(search) };
  }
	if (path === '/canvas') {
    return { name: 'canvas', location: canvasLocationFromSearch(search) };
	}
  if (path === '/sessions') {
    return { name: 'canvas', location: { view: 'sessions' } };
  }
  if (path === '/review') {
    return { name: 'review', location: reviewLocationFromSearch(search) };
  }
  if (path === '/workstream' || path === '/') {
    return { name: 'workstream', ...workstreamFocusFromSearch(search) };
  }
  return { name: 'not-found', path };
}

export function pathForRoute(route: Route): string {
	if (route.name === 'knowledge') return knowledgePath(route.location);
	if (route.name === 'canvas') return canvasPath(route.location);
  if (route.name === 'review') return reviewPath(route.location);
  if (route.name === 'harness-run') return `/harness/runs/${encodeURIComponent(route.runId)}`;
  return route.name === 'not-found' ? route.path
    : route.name === 'item'
    ? `/items/${encodeURIComponent(route.itemId)}${route.view ? `?view=${route.view}` : ''}`
    : route.name === 'workspaces'
      ? '/workspaces'
      : route.name === 'settings'
        ? '/settings'
        : workstreamPath(route.focusedItemId, route.focusedSessionId);
}

export function canvasLocationFromSearch(search: string): CanvasLocation | undefined {
	const query = new URLSearchParams(search);
	const workspaceId = query.get('workspaceId')?.trim() || undefined;
	const selectedNodeId = query.get('selectedNodeId')?.trim() || undefined;
	const asked = query.get('view');
	const view: WorkbenchView | undefined = asked === 'canvas' || asked === 'sessions' ? asked : selectedNodeId ? 'canvas' : undefined;
	return workspaceId || selectedNodeId || view ? { workspaceId, selectedNodeId, view } : undefined;
}

export function canvasPath(location?: CanvasLocation): string {
	const query = new URLSearchParams();
	if (location?.workspaceId) query.set('workspaceId', location.workspaceId);
	if (location?.selectedNodeId) query.set('selectedNodeId', location.selectedNodeId);
	if (location?.view === 'canvas') query.set('view', 'canvas');
	return query.size ? `/canvas?${query.toString()}` : '/canvas';
}

export function reviewLocationFromSearch(search: string): ReviewLocation | undefined {
  const query = new URLSearchParams(search);
  const workspaceId = query.get('workspaceId')?.trim() || undefined;
  const branch = query.get('branch')?.trim() || undefined;
  return workspaceId || branch ? { workspaceId, branch } : undefined;
}

export function reviewPath(location?: ReviewLocation): string {
  const query = new URLSearchParams();
  if (location?.workspaceId) query.set('workspaceId', location.workspaceId);
  if (location?.branch) query.set('branch', location.branch);
  return query.size ? `/review?${query.toString()}` : '/review';
}

export function knowledgeLocationFromSearch(search: string): KnowledgeLocation | undefined {
	const query = new URLSearchParams(search);
	const workspaceId = query.get('workspaceId')?.trim() || undefined;
	const slug = query.get('slug')?.trim() || undefined;
	const rawView = query.get('view');
	const view = rawView === 'browse' || rawView === 'read' || rawView === 'graph' ? rawView : undefined;
	return workspaceId || slug || view ? { workspaceId, slug, view } : undefined;
}

export function knowledgePath(location?: KnowledgeLocation): string {
	const query = new URLSearchParams();
	if (location?.workspaceId) query.set('workspaceId', location.workspaceId);
	if (location?.slug) query.set('slug', location.slug);
	if (location?.view) query.set('view', location.view);
	return query.size ? `/knowledge?${query.toString()}` : '/knowledge';
}

/** `agent` means nothing without `itemId`. */
function workstreamFocusFromSearch(search: string): { focusedItemId?: string; focusedSessionId?: string } {
  const query = new URLSearchParams(search);
  const focusedItemId = query.get('itemId')?.trim() || undefined;
  const focusedSessionId = focusedItemId ? query.get('agent')?.trim() || undefined : undefined;
  return focusedSessionId ? { focusedItemId, focusedSessionId } : { focusedItemId };
}

function workstreamPath(focusedItemId?: string, focusedSessionId?: string): string {
  if (!focusedItemId) return '/workstream';
  const query = new URLSearchParams({ itemId: focusedItemId });
  if (focusedSessionId) query.set('agent', focusedSessionId);
  return `/workstream?${query.toString()}`;
}

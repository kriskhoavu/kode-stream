export type KnowledgeView = 'browse' | 'read' | 'graph';
export interface KnowledgeLocation { workspaceId?: string; root?: string; slug?: string; view?: KnowledgeView; }
export interface CanvasLocation { workspaceId?: string; }
export interface ReviewLocation { workspaceId?: string; branch?: string; }

export type Route =
  | { name: 'workstream'; focusedItemId?: string }
  | { name: 'workspaces' }
  | { name: 'settings' }
  | { name: 'knowledge'; location?: KnowledgeLocation }
  | { name: 'canvas'; location?: CanvasLocation }
  | { name: 'review'; location?: ReviewLocation }
  | { name: 'item'; itemId: string }
  | { name: 'not-found'; path: string };

export const routeDescriptors: Record<Route['name'], { title: string; mainId: string }> = {
  workstream: { title: 'Workstream · Kode Stream', mainId: 'app-main' },
  workspaces: { title: 'Workspaces · Kode Stream', mainId: 'app-main' },
  settings: { title: 'Settings · Kode Stream', mainId: 'app-main' },
  knowledge: { title: 'Knowledge · Kode Stream', mainId: 'app-main' },
  canvas: { title: 'Workbench · Kode Stream', mainId: 'app-main' },
  review: { title: 'Branch Review · Kode Stream', mainId: 'app-main' },
  item: { title: 'Item · Kode Stream', mainId: 'app-main' },
  'not-found': { title: 'Page not found · Kode Stream', mainId: 'app-main' }
};

export function routeFromLocation(): Route {
  return routeFromPath(window.location.pathname, window.location.search);
}

export function routeFromPath(path: string, search = ''): Route {
  if (path.startsWith('/items/')) {
    const itemId = decodeURIComponent(path.split('/')[2] ?? '');
    return itemId ? { name: 'item', itemId } : { name: 'not-found', path };
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
  if (path === '/review') {
    return { name: 'review', location: reviewLocationFromSearch(search) };
  }
  if (path === '/workstream' || path === '/') {
    return { name: 'workstream', focusedItemId: workstreamFocusedItemFromSearch(search) };
  }
  return { name: 'not-found', path };
}

export function pathForRoute(route: Route): string {
	if (route.name === 'knowledge') return knowledgePath(route.location);
	if (route.name === 'canvas') return canvasPath(route.location);
  if (route.name === 'review') return reviewPath(route.location);
  return route.name === 'not-found' ? route.path
    : route.name === 'item'
    ? `/items/${encodeURIComponent(route.itemId)}`
    : route.name === 'workspaces'
      ? '/workspaces'
      : route.name === 'settings'
        ? '/settings'
        : workstreamPath(route.focusedItemId);
}

export function canvasLocationFromSearch(search: string): CanvasLocation | undefined {
	const query = new URLSearchParams(search);
	const workspaceId = query.get('workspaceId')?.trim() || undefined;
	return workspaceId ? { workspaceId } : undefined;
}

export function canvasPath(location?: CanvasLocation): string {
	const query = new URLSearchParams();
	if (location?.workspaceId) query.set('workspaceId', location.workspaceId);
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
	const root = query.get('root')?.trim() || undefined;
	const slug = query.get('slug')?.trim() || undefined;
	const rawView = query.get('view');
	const view = rawView === 'browse' || rawView === 'read' || rawView === 'graph' ? rawView : undefined;
	return workspaceId || root || slug || view ? { workspaceId, root, slug, view } : undefined;
}

export function knowledgePath(location?: KnowledgeLocation): string {
	const query = new URLSearchParams();
	if (location?.workspaceId) query.set('workspaceId', location.workspaceId);
	if (location?.root) query.set('root', location.root);
	if (location?.slug) query.set('slug', location.slug);
	if (location?.view) query.set('view', location.view);
	return query.size ? `/knowledge?${query.toString()}` : '/knowledge';
}

function workstreamFocusedItemFromSearch(search: string): string | undefined {
  return new URLSearchParams(search).get('itemId')?.trim() || undefined;
}

function workstreamPath(focusedItemId?: string): string {
  if (!focusedItemId) return '/workstream';
  return `/workstream?${new URLSearchParams({ itemId: focusedItemId }).toString()}`;
}

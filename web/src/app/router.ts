export type KnowledgeView = 'browse' | 'read' | 'graph';
export interface KnowledgeLocation { workspaceId?: string; root?: string; slug?: string; view?: KnowledgeView; }

export type Route =
  | { name: 'workstream'; focusedItemId?: string }
  | { name: 'workspaces' }
  | { name: 'settings' }
  | { name: 'knowledge'; location?: KnowledgeLocation }
  | { name: 'item'; itemId: string };

export function routeFromLocation(): Route {
  const path = window.location.pathname;
  if (path.startsWith('/items/')) {
    return { name: 'item', itemId: decodeURIComponent(path.split('/')[2] ?? '') };
  }
  if (path.startsWith('/workspaces')) {
    return { name: 'workspaces' };
  }
  if (path.startsWith('/settings')) {
    return { name: 'settings' };
  }
  if (path === '/knowledge') {
	return { name: 'knowledge', location: knowledgeLocationFromSearch(window.location.search) };
  }
  if (path === '/workstream' || path === '/') {
    return { name: 'workstream', focusedItemId: workstreamFocusedItemFromSearch(window.location.search) };
  }
  return { name: 'workstream' };
}

export function pathForRoute(route: Route): string {
	if (route.name === 'knowledge') return knowledgePath(route.location);
  return route.name === 'item'
    ? `/items/${encodeURIComponent(route.itemId)}`
    : route.name === 'workspaces'
      ? '/workspaces'
      : route.name === 'settings'
        ? '/settings'
        : workstreamPath(route.focusedItemId);
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

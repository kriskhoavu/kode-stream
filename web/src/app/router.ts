import type { ExplorerTreeMode } from '../lib/types';

export interface ExplorerLocation {
  workspaceId?: string;
  path?: string;
	mode?: ExplorerTreeMode;
}

export type KnowledgeView = 'browse' | 'read' | 'graph';
export interface KnowledgeLocation { workspaceId?: string; root?: string; slug?: string; view?: KnowledgeView; }

export type Route =
  | { name: 'kanban'; focusedItemId?: string }
  | { name: 'workspaces' }
  | { name: 'settings' }
  | { name: 'explorer'; location?: ExplorerLocation }
  | { name: 'knowledge'; location?: KnowledgeLocation }
  | { name: 'workspace'; itemId: string };

export function routeFromLocation(): Route {
  const path = window.location.pathname;
  if (path.startsWith('/items/')) {
    return { name: 'workspace', itemId: decodeURIComponent(path.split('/')[2] ?? '') };
  }
  if (path.startsWith('/workspaces')) {
    return { name: 'workspaces' };
  }
  if (path.startsWith('/settings')) {
    return { name: 'settings' };
  }
  if (path === '/explorer') {
    return { name: 'explorer', location: explorerLocationFromSearch(window.location.search) };
  }
  if (path === '/knowledge') {
	return { name: 'knowledge', location: knowledgeLocationFromSearch(window.location.search) };
  }
  return { name: 'kanban', focusedItemId: kanbanFocusedItemFromSearch(window.location.search) };
}

export function pathForRoute(route: Route): string {
	if (route.name === 'explorer') {
		return explorerPath(route.location);
	}
	if (route.name === 'knowledge') return knowledgePath(route.location);
  return route.name === 'workspace'
    ? `/items/${encodeURIComponent(route.itemId)}`
    : route.name === 'workspaces'
      ? '/workspaces'
      : route.name === 'settings'
        ? '/settings'
        : kanbanPath(route.focusedItemId);
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

export function explorerLocationFromSearch(search: string): ExplorerLocation | undefined {
  const query = new URLSearchParams(search);
  const workspaceId = query.get('workspaceId')?.trim() || undefined;
  const path = query.get('path')?.trim() || undefined;
	const rawMode = query.get('mode');
	const mode = rawMode === 'all' || rawMode === 'sources' ? rawMode : undefined;
	return workspaceId || path || mode ? { workspaceId, path, mode } : undefined;
}

export function explorerPath(location?: ExplorerLocation): string {
  const query = new URLSearchParams();
  if (location?.workspaceId) query.set('workspaceId', location.workspaceId);
  if (location?.path) query.set('path', location.path);
	if (location?.mode) query.set('mode', location.mode);
  return query.size ? `/explorer?${query.toString()}` : '/explorer';
}

function kanbanFocusedItemFromSearch(search: string): string | undefined {
  return new URLSearchParams(search).get('itemId')?.trim() || undefined;
}

function kanbanPath(focusedItemId?: string): string {
  if (!focusedItemId) return '/kanban';
  return `/kanban?${new URLSearchParams({ itemId: focusedItemId }).toString()}`;
}

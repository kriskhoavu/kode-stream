import type { ExplorerTreeMode } from '../lib/types';

export interface ExplorerLocation {
  workspaceId?: string;
  path?: string;
	mode?: ExplorerTreeMode;
}

export type Route =
  | { name: 'kanban' }
  | { name: 'items' }
  | { name: 'branches' }
  | { name: 'workspaces' }
  | { name: 'explorer'; location?: ExplorerLocation }
  | { name: 'workspace'; itemId: string };

export function routeFromLocation(): Route {
  const path = window.location.pathname;
  if (path.startsWith('/items/')) {
    return { name: 'workspace', itemId: decodeURIComponent(path.split('/')[2] ?? '') };
  }
  if (path === '/items') {
    return { name: 'items' };
  }
  if (path.startsWith('/branches')) {
    return { name: 'branches' };
  }
  if (path.startsWith('/workspaces')) {
    return { name: 'workspaces' };
  }
  if (path === '/explorer') {
    return { name: 'explorer', location: explorerLocationFromSearch(window.location.search) };
  }
  return { name: 'kanban' };
}

export function pathForRoute(route: Route): string {
	if (route.name === 'explorer') {
		return explorerPath(route.location);
	}
  return route.name === 'workspace'
    ? `/items/${encodeURIComponent(route.itemId)}`
    : route.name === 'workspaces'
      ? '/workspaces'
      : route.name === 'items'
        ? '/items'
        : route.name === 'branches'
          ? '/branches'
          : '/kanban';
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

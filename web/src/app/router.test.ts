import { describe, expect, it } from 'vitest';
import { explorerLocationFromSearch, explorerPath, pathForRoute, routeFromLocation } from './router';

describe('router', () => {
  it('parses item workspace routes', () => {
    window.history.pushState(null, '', '/items/PM-003%20Architecture');

    expect(routeFromLocation()).toEqual({ name: 'workspace', itemId: 'PM-003 Architecture' });
  });

  it('builds paths for routes', () => {
    expect(pathForRoute({ name: 'kanban' })).toBe('/kanban');
    expect(pathForRoute({ name: 'items' })).toBe('/items');
    expect(pathForRoute({ name: 'branches' })).toBe('/branches');
    expect(pathForRoute({ name: 'workspaces' })).toBe('/workspaces');
    expect(pathForRoute({ name: 'workspace', itemId: 'PM-003 Architecture' })).toBe('/items/PM-003%20Architecture');
    expect(pathForRoute({ name: 'explorer', location: { workspaceId: 'workspace one', path: 'plans/PM-007' } }))
      .toBe('/explorer?workspaceId=workspace+one&path=plans%2FPM-007');
  });

  it('parses and builds explorer selections', () => {
    window.history.pushState(null, '', '/explorer?workspaceId=ws-1&path=docs%2Fguide.md');
    expect(routeFromLocation()).toEqual({ name: 'explorer', location: { workspaceId: 'ws-1', path: 'docs/guide.md' } });
    expect(explorerLocationFromSearch('?path=README.md')).toEqual({ path: 'README.md' });
    expect(explorerPath()).toBe('/explorer');
		expect(explorerLocationFromSearch('?mode=all')).toEqual({ mode: 'all' });
		expect(explorerPath({ workspaceId: 'ws', mode: 'sources' })).toBe('/explorer?workspaceId=ws&mode=sources');
  });
});

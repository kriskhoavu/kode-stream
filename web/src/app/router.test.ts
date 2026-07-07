import { describe, expect, it } from 'vitest';
import { explorerLocationFromSearch, explorerPath, knowledgeLocationFromSearch, knowledgePath, pathForRoute, routeFromLocation } from './router';

describe('router', () => {
  it('parses item workspace routes', () => {
    window.history.pushState(null, '', '/items/PM-003%20Architecture');

    expect(routeFromLocation()).toEqual({ name: 'item', itemId: 'PM-003 Architecture' });
  });

  it('builds paths for routes', () => {
    expect(pathForRoute({ name: 'workspace' })).toBe('/workspace');
    expect(pathForRoute({ name: 'workspace', focusedItemId: 'item 1' })).toBe('/workspace?itemId=item+1');
    expect(pathForRoute({ name: 'workspaces' })).toBe('/workspaces');
    expect(pathForRoute({ name: 'settings' })).toBe('/settings');
    expect(pathForRoute({ name: 'item', itemId: 'PM-003 Architecture' })).toBe('/items/PM-003%20Architecture');
    expect(pathForRoute({ name: 'explorer', location: { workspaceId: 'workspace one', path: 'plans/PM-007' } }))
      .toBe('/explorer?workspaceId=workspace+one&path=plans%2FPM-007');
	expect(pathForRoute({ name: 'knowledge', location: { workspaceId: 'workspace one', root: 'master-data/article', slug: 'article overview', view: 'read' } }))
		.toBe('/knowledge?workspaceId=workspace+one&root=master-data%2Farticle&slug=article+overview&view=read');
  });

	it('parses and builds Knowledge selections', () => {
		window.history.pushState(null, '', '/knowledge?workspaceId=ws&root=docs%2Fwiki&slug=overview&view=read');
		expect(routeFromLocation()).toEqual({ name: 'knowledge', location: { workspaceId: 'ws', root: 'docs/wiki', slug: 'overview', view: 'read' } });
		expect(knowledgeLocationFromSearch('?view=invalid&slug=page')).toEqual({ slug: 'page' });
		expect(knowledgePath()).toBe('/knowledge');
	});

  it('falls removed top-level list routes back to Workspace', () => {
    window.history.pushState(null, '', '/items');
    expect(routeFromLocation()).toEqual({ name: 'workspace' });
    window.history.pushState(null, '', '/branches');
    expect(routeFromLocation()).toEqual({ name: 'workspace' });
    window.history.pushState(null, '', '/workspace?itemId=item-1');
    expect(routeFromLocation()).toEqual({ name: 'workspace', focusedItemId: 'item-1' });
    window.history.pushState(null, '', '/kanban?itemId=item-1');
    expect(routeFromLocation()).toEqual({ name: 'workspace' });
    window.history.pushState(null, '', '/settings');
    expect(routeFromLocation()).toEqual({ name: 'settings' });
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

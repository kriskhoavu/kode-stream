import { describe, expect, it } from 'vitest';
import { canvasLocationFromSearch, canvasPath, knowledgeLocationFromSearch, knowledgePath, pathForRoute, routeFromLocation } from './router';

describe('router', () => {
  it('parses item workspace routes', () => {
    window.history.pushState(null, '', '/items/PM-003%20Architecture');

    expect(routeFromLocation()).toEqual({ name: 'item', itemId: 'PM-003 Architecture' });
  });

  it('builds paths for routes', () => {
    expect(pathForRoute({ name: 'workstream' })).toBe('/workstream');
    expect(pathForRoute({ name: 'workstream', focusedItemId: 'item 1' })).toBe('/workstream?itemId=item+1');
    expect(pathForRoute({ name: 'workspaces' })).toBe('/workspaces');
    expect(pathForRoute({ name: 'settings' })).toBe('/settings');
    expect(pathForRoute({ name: 'item', itemId: 'PM-003 Architecture' })).toBe('/items/PM-003%20Architecture');
	expect(pathForRoute({ name: 'knowledge', location: { workspaceId: 'workspace one', root: 'master-data/article', slug: 'article overview', view: 'read' } }))
		.toBe('/knowledge?workspaceId=workspace+one&root=master-data%2Farticle&slug=article+overview&view=read');
  });

	it('parses and builds Knowledge selections', () => {
		window.history.pushState(null, '', '/knowledge?workspaceId=ws&root=docs%2Fwiki&slug=overview&view=read');
		expect(routeFromLocation()).toEqual({ name: 'knowledge', location: { workspaceId: 'ws', root: 'docs/wiki', slug: 'overview', view: 'read' } });
		expect(knowledgeLocationFromSearch('?view=invalid&slug=page')).toEqual({ slug: 'page' });
		expect(knowledgePath()).toBe('/knowledge');
	});

	it('parses and builds Canvas branch context', () => {
		window.history.pushState(null, '', '/canvas?workspaceId=ws+one&branch=feature%2FPM-037');
		expect(routeFromLocation()).toEqual({ name: 'canvas', location: { workspaceId: 'ws one', branch: 'feature/PM-037' } });
		expect(pathForRoute({ name: 'canvas', location: { workspaceId: 'ws one', branch: 'main' } })).toBe('/canvas?workspaceId=ws+one&branch=main');
		expect(canvasLocationFromSearch('?branch=main')).toEqual({ branch: 'main' });
		expect(canvasPath()).toBe('/canvas');
	});

  it('falls removed top-level list routes back to Workstream', () => {
    window.history.pushState(null, '', '/items');
    expect(routeFromLocation()).toEqual({ name: 'workstream' });
    window.history.pushState(null, '', '/branches');
    expect(routeFromLocation()).toEqual({ name: 'workstream' });
    window.history.pushState(null, '', '/workstream?itemId=item-1');
    expect(routeFromLocation()).toEqual({ name: 'workstream', focusedItemId: 'item-1' });
    window.history.pushState(null, '', '/kanban?itemId=item-1');
    expect(routeFromLocation()).toEqual({ name: 'workstream' });
    window.history.pushState(null, '', '/explorer?workspaceId=ws-1&path=docs%2Fguide.md');
    expect(routeFromLocation()).toEqual({ name: 'workstream' });
    window.history.pushState(null, '', '/settings');
    expect(routeFromLocation()).toEqual({ name: 'settings' });
  });
});

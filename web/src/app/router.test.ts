import { describe, expect, it } from 'vitest';
import { canvasLocationFromSearch, canvasPath, knowledgeLocationFromSearch, knowledgePath, pathForRoute, reviewLocationFromSearch, reviewPath, routeFromLocation } from './router';

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

	it('parses and builds checkout-derived Canvas context', () => {
		window.history.pushState(null, '', '/canvas?workspaceId=ws+one&branch=feature%2FPM-037');
		expect(routeFromLocation()).toEqual({ name: 'canvas', location: { workspaceId: 'ws one' } });
		expect(pathForRoute({ name: 'canvas', location: { workspaceId: 'ws one' } })).toBe('/canvas?workspaceId=ws+one');
		expect(canvasLocationFromSearch('?branch=main')).toBeUndefined();
		expect(canvasPath()).toBe('/canvas');
	});

  it('parses and builds Branch Review selections', () => {
    window.history.pushState(null, '', '/review?workspaceId=ws+one&branch=feature%2Fone');
    expect(routeFromLocation()).toEqual({ name: 'review', location: { workspaceId: 'ws one', branch: 'feature/one' } });
    expect(pathForRoute({ name: 'review', location: { workspaceId: 'ws one', branch: 'feature/one' } })).toBe('/review?workspaceId=ws+one&branch=feature%2Fone');
    expect(reviewLocationFromSearch('')).toBeUndefined();
    expect(reviewPath()).toBe('/review');
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

import { describe, expect, it } from 'vitest';
import { canvasLocationFromSearch, canvasPath, knowledgeLocationFromSearch, knowledgePath, pathForRoute, reviewLocationFromSearch, reviewPath, routeDescriptors, routeFromLocation } from './router';

describe('router', () => {
  it('parses item workspace routes', () => {
    window.history.pushState(null, '', '/items/PM-003%20Architecture');

    expect(routeFromLocation()).toEqual({ name: 'item', itemId: 'PM-003 Architecture' });
  });

  it('opens a card session from the Workstream agent parameter only with an item', () => {
    window.history.pushState(null, '', '/workstream?itemId=p1&agent=embedded-1');
    expect(routeFromLocation()).toEqual({ name: 'workstream', focusedItemId: 'p1', focusedSessionId: 'embedded-1' });
    window.history.pushState(null, '', '/workstream?agent=embedded-1');
    expect(routeFromLocation()).toEqual({ name: 'workstream' });
    expect(pathForRoute({ name: 'workstream', focusedItemId: 'p1', focusedSessionId: 'embedded-1' })).toBe('/workstream?itemId=p1&agent=embedded-1');
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

	it('parses and builds checkout-derived Workbench context', () => {
		window.history.pushState(null, '', '/canvas?workspaceId=ws+one&branch=feature%2FPM-037');
		expect(routeFromLocation()).toEqual({ name: 'canvas', location: { workspaceId: 'ws one' } });
		expect(pathForRoute({ name: 'canvas', location: { workspaceId: 'ws one' } })).toBe('/canvas?workspaceId=ws+one');
		expect(canvasLocationFromSearch('?branch=main')).toBeUndefined();
		expect(canvasPath()).toBe('/canvas');
	});

	it('parses and builds the Sessions Workbench view', () => {
		window.history.pushState(null, '', '/canvas?workspaceId=ws+one&view=sessions');
		expect(routeFromLocation()).toEqual({ name: 'canvas', location: { workspaceId: 'ws one', view: 'sessions' } });
		expect(pathForRoute({ name: 'canvas', location: { workspaceId: 'ws one', view: 'sessions' } })).toBe('/canvas?workspaceId=ws+one&view=sessions');
	});

	it('parses and builds a selected Canvas node for a session handoff', () => {
		window.history.pushState(null, '', '/canvas?workspaceId=ws+one&selectedNodeId=session%3Aembedded-1');
		expect(routeFromLocation()).toEqual({ name: 'canvas', location: { workspaceId: 'ws one', selectedNodeId: 'session:embedded-1' } });
		expect(pathForRoute({ name: 'canvas', location: { workspaceId: 'ws one', selectedNodeId: 'session:embedded-1' } })).toBe('/canvas?workspaceId=ws+one&selectedNodeId=session%3Aembedded-1');
	});

	it('redirects the legacy Sessions route into the Workbench Sessions view', () => {
		window.history.pushState(null, '', '/sessions');
		expect(routeFromLocation()).toEqual({ name: 'canvas', location: { view: 'sessions' } });
	});

  it('parses and builds Branch Review selections', () => {
    window.history.pushState(null, '', '/review?workspaceId=ws+one&branch=feature%2Fone');
    expect(routeFromLocation()).toEqual({ name: 'review', location: { workspaceId: 'ws one', branch: 'feature/one' } });
    expect(pathForRoute({ name: 'review', location: { workspaceId: 'ws one', branch: 'feature/one' } })).toBe('/review?workspaceId=ws+one&branch=feature%2Fone');
    expect(reviewLocationFromSearch('')).toBeUndefined();
    expect(reviewPath()).toBe('/review');
  });

  it('keeps valid legacy top-level routes and presents unknown paths as accessible not-found routes', () => {
    window.history.pushState(null, '', '/items');
    expect(routeFromLocation()).toEqual({ name: 'not-found', path: '/items' });
    window.history.pushState(null, '', '/branches');
    expect(routeFromLocation()).toEqual({ name: 'not-found', path: '/branches' });
    window.history.pushState(null, '', '/workstream?itemId=item-1');
    expect(routeFromLocation()).toEqual({ name: 'workstream', focusedItemId: 'item-1' });
    window.history.pushState(null, '', '/kanban?itemId=item-1');
    expect(routeFromLocation()).toEqual({ name: 'not-found', path: '/kanban' });
    window.history.pushState(null, '', '/explorer?workspaceId=ws-1&path=docs%2Fguide.md');
    expect(routeFromLocation()).toEqual({ name: 'not-found', path: '/explorer' });
    window.history.pushState(null, '', '/settings');
    expect(routeFromLocation()).toEqual({ name: 'settings' });
  });

  it('matches route segments exactly', () => {
    window.history.pushState(null, '', '/workspaces-anything');
    expect(routeFromLocation()).toEqual({ name: 'not-found', path: '/workspaces-anything' });
    window.history.pushState(null, '', '/settings-extra');
    expect(routeFromLocation()).toEqual({ name: 'not-found', path: '/settings-extra' });
  });

  it('declares an accessible title and focus target for every route', () => {
    expect(Object.values(routeDescriptors).every((descriptor) => descriptor.title.includes('Kode Stream') && descriptor.mainId === 'app-main')).toBe(true);
  });

  it('parses and builds harness views', () => {
    window.history.pushState(null, '', '/harness');
    expect(routeFromLocation()).toEqual({ name: 'harness', location: { view: 'inbox' } });
    window.history.pushState(null, '', '/harness/queue');
    expect(routeFromLocation()).toEqual({ name: 'harness', location: { view: 'queue' } });
    window.history.pushState(null, '', '/harness/runs/run%201?tab=live');
    expect(routeFromLocation()).toEqual({ name: 'harness', location: { view: 'run', runId: 'run 1', tab: 'live' } });
    window.history.pushState(null, '', '/harness/runs/r1?tab=live&agent=agent%202');
    expect(routeFromLocation()).toEqual({ name: 'harness', location: { view: 'run', runId: 'r1', tab: 'live', agentRunId: 'agent 2' } });
    window.history.pushState(null, '', '/harness/runs/r1?tab=bogus');
    expect(routeFromLocation()).toEqual({ name: 'harness', location: { view: 'run', runId: 'r1' } });
    window.history.pushState(null, '', '/harness/nowhere');
    expect(routeFromLocation()).toEqual({ name: 'not-found', path: '/harness/nowhere' });
    expect(pathForRoute({ name: 'harness', location: { view: 'approvals' } })).toBe('/harness/approvals');
    expect(pathForRoute({ name: 'harness', location: { view: 'settings' } })).toBe('/harness/settings');
    expect(pathForRoute({ name: 'harness', location: { view: 'run', runId: 'run 1', tab: 'changes' } })).toBe('/harness/runs/run%201?tab=changes');
    expect(pathForRoute({ name: 'harness', location: { view: 'inbox' } })).toBe('/harness');
  });
});

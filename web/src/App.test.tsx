import { describe, expect, it } from 'vitest';
import { routeFromLocation } from './app/router';

describe('routeFromLocation', () => {
  it('parses item workspace routes', () => {
    window.history.pushState(null, '', '/items/PM-003%20Architecture');

    expect(routeFromLocation()).toEqual({ name: 'item', itemId: 'PM-003 Architecture' });
  });

  it('falls removed list routes back to Workstream', () => {
    window.history.pushState(null, '', '/items');
    expect(routeFromLocation()).toEqual({ name: 'workstream' });

    window.history.pushState(null, '', '/branches');
    expect(routeFromLocation()).toEqual({ name: 'workstream' });
  });

  it('parses retained top-level routes', () => {
    window.history.pushState(null, '', '/workspaces');
    expect(routeFromLocation()).toEqual({ name: 'workspaces' });
    window.history.pushState(null, '', '/settings');
    expect(routeFromLocation()).toEqual({ name: 'settings' });
    window.history.pushState(null, '', '/knowledge?view=graph');
    expect(routeFromLocation()).toEqual({ name: 'knowledge', location: { view: 'graph' } });
  });

  it('defaults unknown paths to Workstream', () => {
    window.history.pushState(null, '', '/unknown');

    expect(routeFromLocation()).toEqual({ name: 'workstream' });
  });
});

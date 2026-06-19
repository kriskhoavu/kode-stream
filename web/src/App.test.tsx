import { describe, expect, it } from 'vitest';
import { routeFromLocation } from './App';

describe('routeFromLocation', () => {
  it('parses item workspace routes', () => {
    window.history.pushState(null, '', '/items/PM-003%20Architecture');

    expect(routeFromLocation()).toEqual({ name: 'workspace', itemId: 'PM-003 Architecture' });
  });

  it('parses top-level routes', () => {
    window.history.pushState(null, '', '/items');
    expect(routeFromLocation()).toEqual({ name: 'items' });

    window.history.pushState(null, '', '/branches');
    expect(routeFromLocation()).toEqual({ name: 'branches' });

    window.history.pushState(null, '', '/workspaces');
    expect(routeFromLocation()).toEqual({ name: 'workspaces' });
  });

  it('defaults unknown paths to kanban', () => {
    window.history.pushState(null, '', '/unknown');

    expect(routeFromLocation()).toEqual({ name: 'kanban' });
  });
});

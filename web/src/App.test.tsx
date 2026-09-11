import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { routeFromLocation } from './app/router';
import { App, LocalServerUnavailable } from './App';
import { api } from './shared/api';
import type { WorkspaceConfig } from './lib/types';

vi.mock('./pages/CanvasPage', () => ({
  CanvasPage: ({ workspace }: { workspace?: WorkspaceConfig }) => <div data-testid="canvas-page">Canvas for {workspace?.name ?? 'none'}</div>
}));

vi.mock('./pages/SessionsPage', () => ({
  SessionsPage: ({ onOpenSession }: { onOpenSession: (workspaceId: string, sessionNodeId: string) => void }) => (
    <button type="button" onClick={() => onOpenSession('workspace-b', 'session:session-1')}>Open session in Workspace B</button>
  )
}));

function workspace(id: string, name: string): WorkspaceConfig {
  return { id, name, path: `/${id}`, baselineBranch: 'main', sources: [], createdAt: '2026-01-01T00:00:00.000Z' };
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
  window.history.replaceState(null, '', '/workstream');
});

describe('routeFromLocation', () => {
  it('parses item workspace routes', () => {
    window.history.pushState(null, '', '/items/PM-003%20Architecture');

    expect(routeFromLocation()).toEqual({ name: 'item', itemId: 'PM-003 Architecture' });
  });

  it('presents removed list routes as not found', () => {
    window.history.pushState(null, '', '/items');
    expect(routeFromLocation()).toEqual({ name: 'not-found', path: '/items' });

    window.history.pushState(null, '', '/branches');
    expect(routeFromLocation()).toEqual({ name: 'not-found', path: '/branches' });
  });

  it('parses retained top-level routes', () => {
    window.history.pushState(null, '', '/workspaces');
    expect(routeFromLocation()).toEqual({ name: 'workspaces' });
    window.history.pushState(null, '', '/settings');
    expect(routeFromLocation()).toEqual({ name: 'settings' });
    window.history.pushState(null, '', '/knowledge?view=graph');
    expect(routeFromLocation()).toEqual({ name: 'knowledge', location: { view: 'graph' } });
  });

  it('presents unknown paths as not found', () => {
    window.history.pushState(null, '', '/unknown');

    expect(routeFromLocation()).toEqual({ name: 'not-found', path: '/unknown' });
  });
});

describe('LocalServerUnavailable', () => {
  it('shows the configured local API origin and retries on demand', () => {
    const retry = vi.fn();
    render(<LocalServerUnavailable status="unavailable" apiOrigin="http://127.0.0.1:9999" onRetry={retry} />);

    expect(screen.getByRole('alert')).toHaveTextContent('http://127.0.0.1:9999');
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

    expect(retry).toHaveBeenCalledTimes(1);
  });
});

describe('App route transitions', () => {
  it('navigates from quick search and announces browser back/forward transitions', async () => {
    vi.stubGlobal('fetch', vi.fn((path: string) => {
      const payload = path.includes('/api/state')
        ? { mode: 'local' }
        : path.includes('/api/navigation/recent')
          ? [{ itemId: 'settings', route: '/settings', title: 'Settings', subtitle: 'Configure Kode Stream' }]
          : [];
      return Promise.resolve(new Response(JSON.stringify(payload), { headers: { 'content-type': 'application/json' } }));
    }));
    window.history.replaceState(null, '', '/workstream');
    render(<App />);

    fireEvent.click(screen.getByRole('button', { name: 'Search' }));
    fireEvent.click(await screen.findByRole('button', { name: /Settings/ }));
    await waitFor(() => expect(document.title).toBe('Settings · Kode Stream'));
    expect(screen.getByText('Navigated to settings')).toBeInTheDocument();
    expect(document.getElementById('app-main')).toHaveFocus();

    window.history.back();
    await waitFor(() => expect(window.location.pathname).toBe('/workstream'));
    await waitFor(() => expect(document.title).toBe('Workstream · Kode Stream'));
    expect(screen.getByText('Navigated to workstream')).toBeInTheDocument();
    expect(document.getElementById('app-main')).toHaveFocus();

    window.history.forward();
    await waitFor(() => expect(window.location.pathname).toBe('/settings'));
    await waitFor(() => expect(document.title).toBe('Settings · Kode Stream'));
    expect(screen.getByText('Navigated to settings')).toBeInTheDocument();
    expect(document.getElementById('app-main')).toHaveFocus();
  });
});

describe('App workspace routing', () => {
  it('opening a session from a non-active workspace navigates to its canvas and selects that workspace', async () => {
    vi.spyOn(api, 'state').mockResolvedValue({ mode: 'local' } as never);
    vi.spyOn(api, 'workspaces').mockResolvedValue([workspace('workspace-a', 'Workspace A'), workspace('workspace-b', 'Workspace B')]);
    localStorage.setItem('kodeStream.activeWorkspaceId', 'workspace-a');
    window.history.replaceState(null, '', '/sessions');

    render(<App />);

    expect(await screen.findByRole('button', { name: 'Workspace A', expanded: false })).toBeInTheDocument();

    fireEvent.click(await screen.findByRole('button', { name: 'Open session in Workspace B' }));

    await waitFor(() => expect(window.location.pathname).toBe('/canvas'));
    expect(await screen.findByTestId('canvas-page')).toHaveTextContent('Canvas for Workspace B');
    expect(screen.getByRole('button', { name: 'Workspace B', expanded: false })).toBeInTheDocument();
    expect(localStorage.getItem('kodeStream.activeWorkspaceId')).toBe('workspace-b');
  });
});

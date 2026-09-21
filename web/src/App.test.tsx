import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { routeFromLocation } from './app/router';
import { App, LocalServerUnavailable } from './App';
import { api } from './shared/api';
import type { WorkspaceConfig } from './lib/types';

vi.mock('./pages/CanvasPage', () => ({
  CanvasPage: ({ workspace, location }: { workspace?: WorkspaceConfig; location?: { view?: string } }) => <div data-testid="canvas-page">{location?.view === 'sessions' ? 'Sessions' : 'Canvas'} for {workspace?.name ?? 'none'}</div>
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
  it('navigates from quick search, focuses the page, and omits redundant route announcements', async () => {
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
    expect(screen.queryByText('Navigated to settings')).not.toBeInTheDocument();
    expect(document.getElementById('app-main')).toHaveFocus();

    window.history.back();
    await waitFor(() => expect(window.location.pathname).toBe('/workstream'));
    await waitFor(() => expect(document.title).toBe('Workstream · Kode Stream'));
    expect(screen.queryByText('Navigated to workstream')).not.toBeInTheDocument();
    expect(document.getElementById('app-main')).toHaveFocus();

    window.history.forward();
    await waitFor(() => expect(window.location.pathname).toBe('/settings'));
    await waitFor(() => expect(document.title).toBe('Settings · Kode Stream'));
    expect(screen.queryByText('Navigated to settings')).not.toBeInTheDocument();
    expect(document.getElementById('app-main')).toHaveFocus();
  });
});

describe('App workspace routing', () => {
	it('restores the URL-selected workspace in the page, sidebar, and header after reload', async () => {
		vi.spyOn(api, 'state').mockResolvedValue({ mode: 'local' } as never);
		vi.spyOn(api, 'workspaces').mockResolvedValue([workspace('workspace-a', 'Workspace A'), workspace('workspace-b', 'Workspace B')]);
		localStorage.setItem('kodeStream.activeWorkspaceId', 'workspace-a');
		window.history.replaceState(null, '', '/canvas?workspaceId=workspace-b');

		render(<App />);

		expect(await screen.findByTestId('canvas-page')).toHaveTextContent('Canvas for Workspace B');
		expect(await screen.findByRole('button', { name: 'Workspace B', expanded: false })).toBeInTheDocument();
		await waitFor(() => expect(screen.getAllByRole('button', { name: 'Workspace B' }).find((button) => button.classList.contains('workspace-button'))).toHaveClass('active'));
		expect(localStorage.getItem('kodeStream.activeWorkspaceId')).toBe('workspace-b');
	});

  it('opens the legacy Sessions URL as the active workspace Sessions view', async () => {
    vi.spyOn(api, 'state').mockResolvedValue({ mode: 'local' } as never);
    vi.spyOn(api, 'workspaces').mockResolvedValue([workspace('workspace-a', 'Workspace A'), workspace('workspace-b', 'Workspace B')]);
    localStorage.setItem('kodeStream.activeWorkspaceId', 'workspace-a');
    window.history.replaceState(null, '', '/sessions');

    render(<App />);

    expect(await screen.findByRole('button', { name: 'Workspace A', expanded: false })).toBeInTheDocument();

    expect(await screen.findByTestId('canvas-page')).toHaveTextContent('Sessions for Workspace A');
    expect(screen.queryByRole('button', { name: 'Sessions' })).not.toBeInTheDocument();
  });
});

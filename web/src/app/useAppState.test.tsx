import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useAppState } from './useAppState';

describe('useAppState runtime context', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
		window.history.replaceState(null, '', '/workstream');
  });

  it('normalizes Cloud runtime state from the API', async () => {
    vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve({
      ok: true,
      json: async () => path === '/api/state'
        ? { mode: 'cloud', role: 'editor', user: { id: 'u1', role: 'editor' }, capabilities: { terminal: true, system: false }, agent: { available: false, status: 'offline' } }
        : []
    })));

    const { result } = renderHook(() => useAppState());

    await waitFor(() => expect(result.current.dataStatus).toBe('ready'));
    expect(result.current.runtimeContext).toMatchObject({
      role: 'editor',
      user: { id: 'u1' },
      capabilities: { terminal: true, system: false },
      agent: { available: false, status: 'offline' }
    });
  });

  it('fails closed instead of granting a Local administrator context when state fetch fails', async () => {
    vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve(path === '/api/state'
      ? { ok: false, status: 500, json: async () => ({ error: 'failed' }) }
      : { ok: true, json: async () => [] }
    )));

    const { result } = renderHook(() => useAppState());

    await waitFor(() => expect(result.current.dataStatus).toBe('unavailable'));
    expect(result.current.runtimeContext).toMatchObject({ mode: 'cloud', role: 'viewer', capabilities: { write: false, workspace_registration: false } });
  });

  it('keeps confirmed Local mode and refreshes only accepted snapshots', async () => {
    vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve({ ok: true, json: async () => path === '/api/state' ? { mode: 'local' } : [{ id: 'first', name: 'First', path: '/repo', baselineBranch: 'main', createdAt: '' }] })));
    const { result } = renderHook(() => useAppState());
    await waitFor(() => expect(result.current.dataStatus).toBe('ready'));
    expect(result.current.runtimeContext).toMatchObject({ mode: 'local', role: 'admin', capabilities: { write: true } });
    const before = result.current.contentRefreshKey;
    await act(async () => { await result.current.refreshAppStateOnly(); });
    expect(result.current.contentRefreshKey).toBe(before);
    expect(result.current.activeRepo?.id).toBe('first');
  });

	it('keeps the saved workspace while data loads and lets a valid workspace URL take precedence', async () => {
		localStorage.setItem('kodeStream.activeWorkspaceId', 'first');
		window.history.replaceState(null, '', '/canvas?workspaceId=second');
		vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve({ ok: true, json: async () => path === '/api/state' ? { mode: 'local' } : [
			{ id: 'first', name: 'First', path: '/first', baselineBranch: 'main', createdAt: '' },
			{ id: 'second', name: 'Second', path: '/second', baselineBranch: 'main', createdAt: '' }
		] })));

		const { result } = renderHook(() => useAppState());
		expect(localStorage.getItem('kodeStream.activeWorkspaceId')).toBe('first');
		await waitFor(() => expect(result.current.activeRepo?.id).toBe('second'));
		expect(localStorage.getItem('kodeStream.activeWorkspaceId')).toBe('second');
	});

  it('ignores an obsolete refresh success after a newer Cloud snapshot is accepted', async () => {
    const pending: Array<(response: unknown) => void> = [];
    vi.stubGlobal('fetch', vi.fn(() => new Promise((resolve) => {
      pending.push((payload) => resolve({ ok: true, json: async () => payload }));
    })));
    const { result } = renderHook(() => useAppState());
    await act(async () => { pending.shift()?.({ mode: 'cloud', role: 'viewer', capabilities: {} }); pending.shift()?.([]); });
    await waitFor(() => expect(result.current.dataStatus).toBe('ready'));
    let older: Promise<boolean> = Promise.resolve(false);
    let newer: Promise<boolean> = Promise.resolve(false);
    await act(async () => { older = result.current.refreshAppData(); newer = result.current.refreshAppData(); });
    const olderResolvers = pending.splice(0, 2);
    const newerResolvers = pending.splice(0, 2);
    await act(async () => {
      newerResolvers[0]({ mode: 'cloud', role: 'editor', capabilities: { write: true } });
      newerResolvers[1]([{ id: 'new', name: 'New', path: '/new', baselineBranch: 'main', createdAt: '' }]);
    });
    await newer;
    const acceptedKey = result.current.contentRefreshKey;
    expect(result.current).toMatchObject({ runtimeContext: { role: 'editor' }, activeRepo: { id: 'new' }, dataStatus: 'ready' });
    await act(async () => {
      olderResolvers[0]({ mode: 'cloud', role: 'admin', capabilities: { write: false } });
      olderResolvers[1]([{ id: 'old', name: 'Old', path: '/old', baselineBranch: 'main', createdAt: '' }]);
      await older;
    });
    expect(result.current).toMatchObject({ runtimeContext: { role: 'editor' }, activeRepo: { id: 'new' }, dataStatus: 'ready' });
    expect(result.current.contentRefreshKey).toBe(acceptedKey);
  });

  it('ignores an obsolete refresh failure after a newer Cloud snapshot is accepted', async () => {
    const pending: Array<{ resolve: (payload: unknown) => void; reject: (error: Error) => void }> = [];
    vi.stubGlobal('fetch', vi.fn(() => new Promise((resolve, reject) => pending.push({ resolve: (payload) => resolve({ ok: true, json: async () => payload }), reject }))));
    const { result } = renderHook(() => useAppState());
    await act(async () => { pending.shift()?.resolve({ mode: 'cloud', role: 'viewer', capabilities: {} }); pending.shift()?.resolve([]); });
    await waitFor(() => expect(result.current.dataStatus).toBe('ready'));
    let old: Promise<boolean> = Promise.resolve(false); let current: Promise<boolean> = Promise.resolve(false);
    await act(async () => { old = result.current.refreshAppData(); current = result.current.refreshAppData(); });
    const oldPair = pending.splice(0, 2); const currentPair = pending.splice(0, 2);
    await act(async () => { currentPair[0].resolve({ mode: 'cloud', role: 'editor', capabilities: { write: true } }); currentPair[1].resolve([{ id: 'new', name: 'New', path: '/new', baselineBranch: 'main', createdAt: '' }]); });
    await current;
    const acceptedKey = result.current.contentRefreshKey;
    await act(async () => { oldPair[0].reject(new Error('old state failed')); oldPair[1].reject(new Error('old workspaces failed')); await old; });
    expect(result.current).toMatchObject({ runtimeContext: { role: 'editor' }, activeRepo: { id: 'new' }, dataStatus: 'ready' });
    expect(result.current.contentRefreshKey).toBe(acceptedKey);
  });

  it.each(['focus', 'visibilitychange'])('refreshes a visible application on %s', async (event) => {
    vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve({ ok: true, json: async () => path === '/api/state' ? { mode: 'local' } : [{ id: 'ws', name: 'Workspace', path: '/repo', baselineBranch: 'main', createdAt: '' }] })));
    const { result } = renderHook(() => useAppState());
    await waitFor(() => expect(result.current.dataStatus).toBe('ready'));
    const key = result.current.contentRefreshKey;
    const descriptor = Object.getOwnPropertyDescriptor(document, 'visibilityState');
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
    act(() => { (event === 'focus' ? window : document).dispatchEvent(new Event(event)); });
    await waitFor(() => expect(result.current.contentRefreshKey).toBeGreaterThan(key));
    expect(result.current).toMatchObject({ dataStatus: 'ready', activeRepo: { id: 'ws' } });
    if (descriptor) Object.defineProperty(document, 'visibilityState', descriptor);
  });

  it('continues in memory when preference storage throws', async () => {
    const broken = { getItem: () => { throw new Error('blocked'); }, setItem: () => { throw new Error('blocked'); }, removeItem: () => { throw new Error('blocked'); } };
    vi.stubGlobal('localStorage', broken);
    vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve({ ok: true, json: async () => path === '/api/state' ? { mode: 'local' } : [] })));
    const { result } = renderHook(() => useAppState());
    await waitFor(() => expect(result.current.dataStatus).toBe('ready'));
    act(() => result.current.setTheme('dark'));
    expect(result.current.theme).toBe('dark');
  });

  it('publishes route title, focus, and a typed navigation transition', async () => {
    const main = document.createElement('main'); main.id = 'app-main'; main.tabIndex = -1; document.body.append(main);
    vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve({ ok: true, json: async () => path === '/api/state' ? { mode: 'local' } : [] })));
    const { result } = renderHook(() => useAppState());
    await waitFor(() => expect(result.current.dataStatus).toBe('ready'));
    act(() => result.current.navigate({ name: 'settings' }));
    await waitFor(() => expect(document.title).toBe('Settings · Kode Stream'));
    expect(main).toHaveFocus();
    main.remove();
  });

	it('sends an old harness link to the board and keeps the board when a workspace is chosen', async () => {
		window.history.replaceState(null, '', '/harness/queue');
		vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve({ ok: true, json: async () => path === '/api/state' ? { mode: 'local' } : [
			{ id: 'first', name: 'First', path: '/first', baselineBranch: 'main', createdAt: '' }
		] })));
		const { result } = renderHook(() => useAppState());
		await waitFor(() => expect(result.current.dataStatus).toBe('ready'));
		act(() => result.current.selectWorkspace(result.current.workspaces[0]));
		expect(result.current.route).toEqual({ name: 'workstream' });
		expect(window.location.pathname).toBe('/workstream');
	});
});


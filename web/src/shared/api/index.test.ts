import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, api } from '.';

describe('shared api facade', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('normalizes workspace sources', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [{ id: 'w1', name: 'Workspace', path: '/repo', baselineBranch: 'main', createdAt: '2026-06-20T00:00:00Z' }]
    }));

    await expect(api.workspaces()).resolves.toEqual([
      { id: 'w1', name: 'Workspace', path: '/repo', baselineBranch: 'main', createdAt: '2026-06-20T00:00:00Z', sources: [] }
    ]);
  });

  it('normalizes Git status defaults', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ workspaceId: 'w1', branch: 'main' })
    }));

    await expect(api.gitStatus('w1')).resolves.toEqual({
      workspaceId: 'w1',
      branch: 'main',
      ahead: 0,
      behind: 0,
      dirty: false,
      conflicted: false,
      changes: []
    });
  });

  it('normalizes audit and workspace health responses', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: async () => [{ id: 'event-1', time: '2026-06-20T00:00:00Z', operation: 'scan', status: 'unknown', message: 'done' }]
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ workspaceId: 'w1', checkedAt: '2026-06-20T00:00:00Z', summary: 'unknown' })
      });
    vi.stubGlobal('fetch', fetchMock);

    await expect(api.auditEvents({ workspaceId: 'w1', limit: 5 })).resolves.toEqual([
      { id: 'event-1', time: '2026-06-20T00:00:00Z', operation: 'scan', status: 'success', message: 'done', paths: [], durationMs: 0 }
    ]);
    await expect(api.workspaceHealth('w1')).resolves.toEqual({
      workspaceId: 'w1', checkedAt: '2026-06-20T00:00:00Z', summary: 'ok', checks: []
    });
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/audit-events?workspaceId=w1&limit=5', expect.any(Object));
  });

  it('preserves recovery hints on API errors', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      json: async () => ({ error: 'File changed', recoveryHint: 'Reload the file.' })
    }));

    const error = await api.saveFile('item-1', 'README_md', { content: 'new' }).catch((caught) => caught);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ message: 'File changed', recoveryHint: 'Reload the file.' });
  });

  it('normalizes search, saved filter, and recent item responses', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => [{ id: 'one', type: 'unknown', title: 'One', route: '/items/one' }] })
      .mockResolvedValueOnce({ ok: true, json: async () => [{ id: 'filter', name: 'Drafts', route: '/kanban' }] })
      .mockResolvedValueOnce({ ok: true, json: async () => [{ itemId: 'one', workspaceId: 'w1', title: 'One', openedAt: '2026-06-20T00:00:00Z' }] });
    vi.stubGlobal('fetch', fetchMock);

    await expect(api.search({ q: 'one', workspaceId: 'w1', limit: 5 })).resolves.toEqual([
      { id: 'one', type: 'item', title: 'One', subtitle: '', context: '', route: '/items/one', score: 0 }
    ]);
    await expect(api.savedFilters()).resolves.toEqual([{ id: 'filter', name: 'Drafts', route: '/kanban', filters: {} }]);
    await expect(api.recentItems()).resolves.toEqual([
      { itemId: 'one', workspaceId: 'w1', title: 'One', subtitle: '', route: '/items/one', openedAt: '2026-06-20T00:00:00Z' }
    ]);
  });
});

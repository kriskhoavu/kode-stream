import { act, renderHook } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { directoryCacheKey } from './tree';
import type { ExplorerLocation } from './types';
import { useWorkstreamExplorer } from './useWorkstreamExplorer';

const apiMock = vi.hoisted(() => ({
  items: vi.fn(), workspacePathGitStates: vi.fn(), workspaceTree: vi.fn()
}));

vi.mock('../../shared/api', () => ({ api: apiMock }));

beforeEach(() => {
  localStorage.clear();
  apiMock.items.mockReset().mockResolvedValue([]);
  apiMock.workspacePathGitStates.mockReset().mockResolvedValue([]);
  apiMock.workspaceTree.mockReset();
});

it('cancels directory reads when the explorer unmounts', async () => {
  let signal: AbortSignal | undefined;
  apiMock.workspaceTree.mockImplementation((_workspaceId: string, _path: string, _ignored: boolean, _force: boolean, requestSignal: AbortSignal) => {
    signal = requestSignal;
    return new Promise(() => undefined);
  });
  const { result, unmount } = renderHook(() => useWorkstreamExplorer([], { mode: 'all' }));

  act(() => { void result.current.loadDirectory('ws', 'docs', true); });
  expect(signal?.aborted).toBe(false);
  unmount();

  expect(signal?.aborted).toBe(true);
});

it('keeps the refreshed directory when an older response arrives later', async () => {
  const listing = (name: string) => ({ hiddenCount: 0, entries: [{ id: name, name, path: `docs/${name}`, type: 'file' }] });
  let resolveOld: (value: ReturnType<typeof listing>) => void = () => {};
  let resolveFresh: (value: ReturnType<typeof listing>) => void = () => {};
  apiMock.workspaceTree
    .mockReturnValueOnce(new Promise<ReturnType<typeof listing>>((resolve) => { resolveOld = resolve; }))
    .mockReturnValueOnce(new Promise<ReturnType<typeof listing>>((resolve) => { resolveFresh = resolve; }));
  const { result } = renderHook(() => useWorkstreamExplorer([], { mode: 'all' }));

  let oldLoad!: Promise<void>;
  let freshLoad!: Promise<void>;
  act(() => {
    oldLoad = result.current.loadDirectory('ws', 'docs', true);
    freshLoad = result.current.loadDirectory('ws', 'docs', true);
  });
  expect(apiMock.workspaceTree).toHaveBeenCalledTimes(2);
  expect(apiMock.workspaceTree).toHaveBeenCalledWith('ws', 'docs', false, true, expect.anything());

  await act(async () => {
    resolveFresh(listing('fresh.md'));
    await freshLoad;
  });
  await act(async () => {
    resolveOld(listing('old.md'));
    await oldLoad;
  });
  expect(result.current.cache.get(directoryCacheKey('ws', 'docs', false, 'all'))?.entries.map((entry) => entry.name)).toEqual(['fresh.md']);
});

it('classifies a selected path from its own workspace directory', async () => {
  apiMock.workspaceTree.mockImplementation(async (workspaceId: string) => ({
    hiddenCount: 0,
    entries: [{ id: workspaceId, name: 'shared', path: 'docs/shared', type: workspaceId === 'first' ? 'file' : 'directory' }]
  }));
  const { result, rerender } = renderHook(({ location }: { location: ExplorerLocation }) => useWorkstreamExplorer([], location), {
    initialProps: { location: { mode: 'all' } as ExplorerLocation }
  });

  await act(async () => {
    await result.current.loadDirectory('first', 'docs', true);
    await result.current.loadDirectory('second', 'docs', true);
  });
  await act(async () => {
    rerender({ location: { workspaceId: 'second', path: 'docs/shared', mode: 'all' } });
  });
  expect(result.current.selection).toMatchObject({ workspaceId: 'second', path: 'docs/shared', kind: 'directory' });
});

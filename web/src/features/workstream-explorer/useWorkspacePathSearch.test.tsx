import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { useWorkspacePathSearch } from './useWorkspacePathSearch';

describe('useWorkspacePathSearch', () => {
  afterEach(() => vi.restoreAllMocks());

  it('debounces queries and ignores stale responses', async () => {
    vi.useFakeTimers();
    const requests: Array<(value: { results: never[]; truncated: boolean }) => void> = [];
    vi.spyOn(api, 'searchWorkspacePaths').mockImplementation(() => new Promise((resolve) => requests.push(resolve)));
    const { result } = renderHook(() => useWorkspacePathSearch({ workspaceId: 'ws', includeIgnored: false, debounceMs: 10 }));
    act(() => result.current.setQuery('first'));
    await act(async () => vi.advanceTimersByTimeAsync(10));
    act(() => result.current.setQuery('second'));
    await act(async () => vi.advanceTimersByTimeAsync(10));
    await act(async () => requests[0]({ results: [], truncated: false }));
    expect(result.current.loading).toBe(true);
    await act(async () => requests[1]({ results: [], truncated: true }));
    expect(result.current.truncated).toBe(true);
    expect(result.current.loading).toBe(false);
    vi.useRealTimers();
  });
});

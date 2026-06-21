import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { useContentSearch } from './useContentSearch';

describe('useContentSearch', () => {
	afterEach(() => {
		vi.useRealTimers();
		vi.restoreAllMocks();
	});

	it('debounces queries, resets short queries, and ignores stale responses', async () => {
		vi.useFakeTimers();
		const requests: Array<(value: { results: never[]; truncated: boolean; filesVisited: number; bytesRead: number; skippedFiles: number }) => void> = [];
		vi.spyOn(api, 'searchWorkspaceContent').mockImplementation(() => new Promise((resolve) => requests.push(resolve)));
		const { result } = renderHook(() => useContentSearch({ kind: 'explorer', mode: 'sources', includeIgnored: false }, 10));

		act(() => result.current.setQuery('first'));
		await act(async () => vi.advanceTimersByTimeAsync(10));
		act(() => result.current.setQuery('second'));
		await act(async () => vi.advanceTimersByTimeAsync(10));
		await act(async () => requests[0]({ results: [], truncated: true, filesVisited: 1, bytesRead: 1, skippedFiles: 0 }));
		expect(result.current.loading).toBe(true);
		await act(async () => requests[1]({ results: [], truncated: false, filesVisited: 1, bytesRead: 1, skippedFiles: 0 }));
		expect(result.current.loading).toBe(false);
		expect(result.current.truncated).toBe(false);

		act(() => result.current.setQuery('x'));
		expect(result.current.results).toEqual([]);
		expect(result.current.loading).toBe(false);
	});

	it('uses the item endpoint for item scope', async () => {
		vi.useFakeTimers();
		vi.spyOn(api, 'searchItemContent').mockResolvedValue({ results: [], truncated: false, filesVisited: 0, bytesRead: 0, skippedFiles: 0 });
		const { result } = renderHook(() => useContentSearch({ kind: 'item', itemId: 'item-1' }, 10));
		act(() => result.current.setQuery('needle'));
		await act(async () => vi.advanceTimersByTimeAsync(10));
		expect(api.searchItemContent).toHaveBeenCalledWith('item-1', { q: 'needle', caseSensitive: false });
	});
});

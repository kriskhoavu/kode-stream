import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import { useGlobalSearch, useQuickSwitcher } from './hooks';

vi.mock('../../shared/api', () => ({
  api: { search: vi.fn(), recordRecentItem: vi.fn() }
}));

describe('search hooks', () => {
  afterEach(() => {
    vi.clearAllMocks();
    vi.useRealTimers();
  });

  it('opens and closes the quick switcher from the keyboard', () => {
    const { result } = renderHook(useQuickSwitcher);
    act(() => window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true })));
    expect(result.current.open).toBe(true);
    act(() => window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })));
    expect(result.current.open).toBe(false);
  });

  it('searches the active workspace and navigates the selected result', async () => {
    const resultItem = { id: 'one', type: 'item' as const, title: 'One', subtitle: '', context: '', workspaceId: 'w1', itemId: 'one', route: '/items/one', score: 100 };
    vi.mocked(api.search).mockResolvedValue([resultItem]);
    vi.mocked(api.recordRecentItem).mockResolvedValue({ ok: true });
    const navigate = vi.fn();
    const { result } = renderHook(() => useGlobalSearch({ workspaceId: 'w1', allWorkspaces: false, onNavigate: navigate }));

    act(() => result.current.setQuery('One'));
    await waitFor(() => expect(result.current.results).toHaveLength(1));
    act(() => result.current.onKeyDown({ key: 'Enter', preventDefault: vi.fn() } as unknown as React.KeyboardEvent));
    expect(api.search).toHaveBeenCalledWith(expect.objectContaining({ q: 'One', workspaceId: 'w1', limit: 30, signal: expect.any(AbortSignal) }));
    expect(navigate).toHaveBeenCalledWith('/items/one', resultItem);
    expect(api.recordRecentItem).toHaveBeenCalledWith('one');
  });

	it('aborts an issued request when the query is replaced or unmounted', async () => {
		vi.useFakeTimers();
		vi.mocked(api.search).mockImplementation(() => new Promise(() => undefined));
		const { result, unmount } = renderHook(() => useGlobalSearch({ allWorkspaces: true, onNavigate: vi.fn() }));
		act(() => result.current.setQuery('first'));
		await act(async () => vi.advanceTimersByTimeAsync(180));
		const first = vi.mocked(api.search).mock.calls[0][0].signal;
		act(() => result.current.setQuery('second'));
		expect(first?.aborted).toBe(true);
		await act(async () => vi.advanceTimersByTimeAsync(180));
		const second = vi.mocked(api.search).mock.calls[1][0].signal;
		unmount();
		expect(second?.aborted).toBe(true);
	});
});

import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { useGlobalSearch, useQuickSwitcher } from './hooks';

vi.mock('../../lib/api', () => ({
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
    expect(api.search).toHaveBeenCalledWith({ q: 'One', workspaceId: 'w1', limit: 30 });
    expect(navigate).toHaveBeenCalledWith('/items/one', resultItem);
    expect(api.recordRecentItem).toHaveBeenCalledWith('one');
  });
});

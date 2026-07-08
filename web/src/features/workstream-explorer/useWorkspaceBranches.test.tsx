import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError } from '../../lib/api';
import type { WorkspaceConfig } from '../../lib/types';
import { useWorkspaceBranches } from './useWorkspaceBranches';

const workspace = { id: 'ws', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: [], createdAt: '' } satisfies WorkspaceConfig;

describe('useWorkspaceBranches', () => {
  afterEach(() => vi.restoreAllMocks());

  it('loads and switches one workspace branch', async () => {
    vi.spyOn(api, 'workspaceBranches')
      .mockResolvedValueOnce({ workspaceId: 'ws', current: 'main', branches: ['feature/a', 'main'] })
      .mockResolvedValueOnce({ workspaceId: 'ws', current: 'feature/a', branches: ['feature/a', 'main'] });
    vi.spyOn(api, 'switchBranch').mockResolvedValue({
      ok: true,
      status: { workspaceId: 'ws', branch: 'feature/a', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }
    });
    const onSwitched = vi.fn();
    const { result } = renderHook(() => useWorkspaceBranches([workspace], onSwitched));
    await waitFor(() => expect(result.current.states.ws?.current).toBe('main'));

    await act(async () => expect(await result.current.switchBranch(workspace, 'feature/a')).toBe(true));

    expect(api.switchBranch).toHaveBeenCalledWith('ws', { name: 'feature/a', confirm: false });
    expect(onSwitched).toHaveBeenCalledWith('ws', 'feature/a');
    await waitFor(() => expect(result.current.states.ws?.current).toBe('feature/a'));
  });

  it('keeps the current branch and exposes guarded switch errors', async () => {
    vi.spyOn(api, 'workspaceBranches').mockResolvedValue({ workspaceId: 'ws', current: 'main', branches: ['feature/a', 'main'] });
    vi.spyOn(api, 'switchBranch').mockRejectedValue(new ApiError('working tree has local changes', 'Commit or revert changes.'));
    const { result } = renderHook(() => useWorkspaceBranches([workspace]));
    await waitFor(() => expect(result.current.states.ws?.loading).toBe(false));

    await act(async () => expect(await result.current.switchBranch(workspace, 'feature/a')).toBe(false));

    expect(result.current.states.ws).toMatchObject({ current: 'main', switching: false, error: 'working tree has local changes', recoveryHint: 'Commit or revert changes.' });
  });
});

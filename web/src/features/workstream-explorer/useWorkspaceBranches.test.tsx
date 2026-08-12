import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError } from '../../shared/api';
import type { WorkspaceConfig } from '../../lib/types';
import { useWorkspaceBranches } from './useWorkspaceBranches';

const workspace = { id: 'ws', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: [], createdAt: '' } satisfies WorkspaceConfig;

describe('useWorkspaceBranches', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

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

	expect(api.switchBranch).toHaveBeenCalledWith('ws', { name: 'feature/a' });
    expect(onSwitched).toHaveBeenCalledWith('ws', 'feature/a');
    await waitFor(() => expect(result.current.states.ws?.current).toBe('feature/a'));
  });

  it('keeps the current branch and exposes guarded switch errors', async () => {
    vi.spyOn(api, 'workspaceBranches').mockResolvedValue({ workspaceId: 'ws', current: 'main', branches: ['feature/a', 'main'] });
    vi.spyOn(api, 'switchBranch').mockRejectedValue(new ApiError('working tree has local changes', 'Commit or revert changes.'));
    vi.stubGlobal('confirm', vi.fn(() => false));
    const { result } = renderHook(() => useWorkspaceBranches([workspace]));
    await waitFor(() => expect(result.current.states.ws?.loading).toBe(false));

    await act(async () => expect(await result.current.switchBranch(workspace, 'feature/a')).toBe(false));

    expect(result.current.states.ws).toMatchObject({ current: 'main', switching: false, error: 'working tree has local changes', recoveryHint: 'Commit or revert changes.' });
  });

  it('offers structured carry, stash, and cancel decisions without an obsolete retry', async () => {
    vi.spyOn(api, 'workspaceBranches')
      .mockResolvedValueOnce({ workspaceId: 'ws', current: 'main', branches: ['feature/a', 'main'] })
      .mockResolvedValueOnce({ workspaceId: 'ws', current: 'feature/a', branches: ['feature/a', 'main'] });
    vi.spyOn(api, 'switchBranch')
		.mockRejectedValueOnce(new ApiError('branch switch decision required', 'Choose carry or stash in the checkout picker.', undefined, { code: 'branch_switch_decision_required', details: { canCarryChanges: 'false' } }));
    const { result } = renderHook(() => useWorkspaceBranches([workspace]));
    await waitFor(() => expect(result.current.states.ws?.loading).toBe(false));

	await act(async () => expect(await result.current.switchBranch(workspace, 'feature/a')).toBe(false));

	expect(api.switchBranch).toHaveBeenCalledOnce();
	expect(api.switchBranch).toHaveBeenCalledWith('ws', { name: 'feature/a' });
	expect(result.current.states.ws?.current).toBe('main');
	expect(result.current.decision).toMatchObject({ target: 'feature/a', canCarry: false });
	act(() => result.current.cancelDecision());
	expect(result.current.decision).toBeNull();
  });

  it('executes carry and stash decisions and keeps recovery errors visible', async () => {
    vi.spyOn(api, 'workspaceBranches').mockResolvedValue({ workspaceId: 'ws', current: 'main', branches: ['feature/a', 'main'] });
    const switcher = vi.spyOn(api, 'switchBranch')
      .mockRejectedValueOnce(new ApiError('decision', undefined, undefined, { code: 'branch_switch_decision_required', details: { canCarryChanges: 'true' } }))
      .mockResolvedValueOnce({ ok: true, status: { workspaceId: 'ws', branch: 'feature/a', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] } })
      .mockRejectedValueOnce(new ApiError('decision', undefined, undefined, { code: 'branch_switch_decision_required', details: { canCarryChanges: 'false' } }))
      .mockRejectedValueOnce(new ApiError('stash failed', 'Recover the stash manually.'));
    const { result } = renderHook(() => useWorkspaceBranches([workspace]));
    await waitFor(() => expect(result.current.states.ws?.loading).toBe(false));
	await act(async () => { await result.current.switchBranch(workspace, 'feature/a'); });
	await act(async () => expect(await result.current.resolveDecision('carry', '')).toBe(true));
    expect(switcher).toHaveBeenNthCalledWith(2, 'ws', { name: 'feature/a', strategy: 'carry', stashMessage: undefined });
	await act(async () => { await result.current.switchBranch(workspace, 'feature/b'); });
	await act(async () => expect(await result.current.resolveDecision('stash', 'protect')).toBe(false));
	expect(switcher).toHaveBeenLastCalledWith('ws', { name: 'feature/b', strategy: 'stash', stashMessage: 'protect' });
    expect(result.current.states.ws).toMatchObject({ error: 'stash failed', recoveryHint: 'Recover the stash manually.' });
  });

  it('refreshes checkout state when the application regains focus', async () => {
    vi.spyOn(api, 'workspaceBranches')
      .mockResolvedValueOnce({ workspaceId: 'ws', current: 'main', branches: ['feature/a', 'main'] })
      .mockResolvedValueOnce({ workspaceId: 'ws', current: 'feature/a', branches: ['feature/a', 'main'] });
    const { result } = renderHook(() => useWorkspaceBranches([workspace]));
    await waitFor(() => expect(result.current.states.ws?.current).toBe('main'));

    act(() => window.dispatchEvent(new FocusEvent('focus')));

    await waitFor(() => expect(result.current.states.ws?.current).toBe('feature/a'));
    expect(api.workspaceBranches).toHaveBeenCalledTimes(2);
  });
});

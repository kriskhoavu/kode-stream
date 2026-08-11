import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import { notifyReliabilityChanged, useAuditEvents, useWorkspaceHealth } from './hooks';

vi.mock('../../shared/api', () => ({
  api: {
    workspaceHealth: vi.fn(),
    auditEvents: vi.fn()
  }
}));

describe('reliability hooks', () => {
  afterEach(() => vi.clearAllMocks());

  it('loads workspace health and refreshes after operations', async () => {
    vi.mocked(api.workspaceHealth).mockResolvedValue({ workspaceId: 'w1', checkedAt: '2026-06-20T00:00:00Z', summary: 'ok', checks: [] });
    const { result } = renderHook(() => useWorkspaceHealth('w1'));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.health?.summary).toBe('ok');
    act(() => notifyReliabilityChanged());
    await waitFor(() => expect(api.workspaceHealth).toHaveBeenCalledTimes(2));
  });

  it('returns an empty activity state', async () => {
    vi.mocked(api.auditEvents).mockResolvedValue([]);
    const { result } = renderHook(() => useAuditEvents('w1'));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.events).toEqual([]);
    expect(result.current.error).toBe('');
  });

  it('returns request errors without stale data', async () => {
    vi.mocked(api.workspaceHealth).mockRejectedValue(new Error('Health unavailable'));
    const { result } = renderHook(() => useWorkspaceHealth('w1'));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.health).toBeNull();
    expect(result.current.error).toBe('Health unavailable');
  });
});

import { renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useAppState } from './useAppState';

describe('useAppState runtime context', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  it('normalizes Cloud runtime state from the API', async () => {
    vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve({
      ok: true,
      json: async () => path === '/api/state'
        ? { mode: 'cloud', role: 'editor', user: { id: 'u1', role: 'editor' }, capabilities: { terminal: true, system: false }, agent: { available: false, status: 'offline' } }
        : []
    })));

    const { result } = renderHook(() => useAppState());

    await waitFor(() => expect(result.current.runtimeContext.mode).toBe('cloud'));
    expect(result.current.runtimeContext).toMatchObject({
      role: 'editor',
      user: { id: 'u1' },
      capabilities: { terminal: true, system: false },
      agent: { available: false, status: 'offline' }
    });
  });

  it('falls back to local runtime state when state fetch fails', async () => {
    vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve(path === '/api/state'
      ? { ok: false, status: 500, json: async () => ({ error: 'failed' }) }
      : { ok: true, json: async () => [] }
    )));

    const { result } = renderHook(() => useAppState());

    await waitFor(() => expect(result.current.runtimeContext.mode).toBe('local'));
    expect(result.current.runtimeContext.role).toBe('admin');
    expect(result.current.runtimeContext.agent).toEqual({ available: true, status: 'local' });
  });
});

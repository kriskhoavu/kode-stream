import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import type { AISettings } from '../../lib/types';
import { useAISettings } from './useAISettings';

vi.mock('../../shared/api', () => ({
  api: { aiSettings: vi.fn(), aiCapabilities: vi.fn(), saveAISettings: vi.fn() }
}));

const settings: AISettings = {
  defaultProvider: 'codex', defaultTerminal: 'terminal',
  providers: { codex: { enabled: true, executable: 'codex', args: [] } },
  terminals: { terminal: { enabled: true, executable: '/Terminal.app', args: [] } }
};

describe('useAISettings', () => {
  afterEach(() => vi.clearAllMocks());

  it('loads settings and capabilities together', async () => {
    vi.mocked(api.aiSettings).mockResolvedValue(settings);
    vi.mocked(api.aiCapabilities).mockResolvedValue([{ id: 'codex', kind: 'provider', detected: true, configured: true, executable: '/bin/codex' }]);
    const { result } = renderHook(() => useAISettings());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.settings?.defaultProvider).toBe('codex');
    expect(result.current.capabilities[0].detected).toBe(true);
  });

  it('saves the edited settings and refreshes detection', async () => {
    vi.mocked(api.aiSettings).mockResolvedValue(settings);
    vi.mocked(api.aiCapabilities).mockResolvedValue([]);
    vi.mocked(api.saveAISettings).mockResolvedValue({ ...settings, defaultProvider: 'claude' });
    const { result } = renderHook(() => useAISettings());
    await waitFor(() => expect(result.current.loading).toBe(false));
    act(() => result.current.setSettings({ ...settings, defaultProvider: 'claude' }));
    await act(async () => result.current.save());
    expect(api.saveAISettings).toHaveBeenCalledWith(expect.objectContaining({ defaultProvider: 'claude' }));
    expect(result.current.saved).toBe(true);
  });

  it('reports load errors', async () => {
    vi.mocked(api.aiSettings).mockRejectedValue(new Error('Detection unavailable'));
    vi.mocked(api.aiCapabilities).mockResolvedValue([]);
    const { result } = renderHook(() => useAISettings());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('Detection unavailable');
  });
});

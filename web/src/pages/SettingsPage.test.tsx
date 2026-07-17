import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../lib/api';
import { defaultAppSettings } from '../features/settings/appSettings';
import { SettingsPage } from './SettingsPage';

vi.mock('../lib/api', async (importOriginal) => {
  const original = await importOriginal<typeof import('../lib/api')>();
  return { ...original, api: { ...original.api, aiSettings: vi.fn(), aiCapabilities: vi.fn(), saveAISettings: vi.fn() } };
});

describe('SettingsPage AI settings', () => {
  afterEach(() => vi.clearAllMocks());

  it('shows detection and saves executable overrides', async () => {
    const settings = {
      defaultProvider: 'codex', defaultTerminal: 'terminal',
      providers: { codex: { enabled: true, executable: 'codex', args: ['Read {contextFile}'] } },
      terminals: { terminal: { enabled: true, executable: '/Terminal.app', args: [] } }
    };
    vi.mocked(api.aiSettings).mockResolvedValue(settings);
    vi.mocked(api.aiCapabilities).mockResolvedValue([
      { id: 'codex', kind: 'provider', detected: true, configured: true, executable: '/bin/codex' },
      { id: 'terminal', kind: 'terminal', detected: false, configured: true, executable: '/Terminal.app', reason: 'configured path was not found' }
    ]);
    vi.mocked(api.saveAISettings).mockImplementation(async (value) => value);
    render(<SettingsPage settings={defaultAppSettings} onChange={vi.fn()} />);
    expect(await screen.findByText('Detected')).toBeInTheDocument();
    const executable = screen.getAllByLabelText('Executable')[0];
    fireEvent.change(executable, { target: { value: '/custom/codex' } });
    fireEvent.click(screen.getByRole('button', { name: /save ai settings/i }));
    await waitFor(() => expect(api.saveAISettings).toHaveBeenCalledWith(expect.objectContaining({
      providers: { codex: expect.objectContaining({ executable: '/custom/codex' }) }
    })));
  });

  it('renders AI templates when persisted args are null', async () => {
    vi.mocked(api.aiSettings).mockResolvedValue({
      defaultProvider: 'codex', defaultTerminal: 'terminal',
      providers: { codex: { enabled: true, executable: 'codex', args: null as unknown as string[] } },
      terminals: { terminal: { enabled: true, executable: '/Terminal.app', args: null as unknown as string[] } }
    });
    vi.mocked(api.aiCapabilities).mockResolvedValue([]);

    render(<SettingsPage settings={defaultAppSettings} onChange={vi.fn()} />);

    expect(await screen.findByText('Codex')).toBeInTheDocument();
    expect(screen.getAllByLabelText('Arguments, one per line')[0]).toHaveValue('');
  });
});

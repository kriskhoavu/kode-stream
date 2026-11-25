import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../shared/api';
import { defaultAppSettings } from '../features/settings/appSettings';
import { SettingsPage } from './SettingsPage';

vi.mock('../features/workstream/HarnessSettingsSection', () => ({ HarnessSettingsSection: () => <section aria-label="Harness settings" /> }));

vi.mock('../shared/api', async (importOriginal) => {
  const original = await importOriginal<typeof import('../shared/api')>();
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
    fireEvent.click(within(screen.getByRole('navigation', { name: 'Settings areas' })).getByRole('button', { name: /AI tools/ }));
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
    fireEvent.click(within(screen.getByRole('navigation', { name: 'Settings areas' })).getByRole('button', { name: /AI tools/ }));

    expect(await screen.findByText('Codex')).toBeInTheDocument();
    expect(screen.getAllByLabelText('Arguments, one per line')[0]).toHaveValue('');
  });
});

describe('SettingsPage header backdrop', () => {
  afterEach(() => vi.clearAllMocks());

  it('offers every variant with the current one selected', () => {
    vi.mocked(api.aiSettings).mockResolvedValue(null as never);
    vi.mocked(api.aiCapabilities).mockResolvedValue([]);
    render(<SettingsPage settings={{ ...defaultAppSettings, headerBackdrop: 'neon' }} onChange={vi.fn()} />);

    const group = screen.getByRole('radiogroup', { name: 'Header backdrop' });
    const options = within(group).getAllByRole('radio');

    expect(options).toHaveLength(3);
    expect(within(group).getByRole('radio', { name: /neon/i })).toBeChecked();
    expect(within(group).getByRole('radio', { name: /lattice/i })).not.toBeChecked();
  });

  it('reports the chosen variant without disturbing the other settings', () => {
    vi.mocked(api.aiSettings).mockResolvedValue(null as never);
    vi.mocked(api.aiCapabilities).mockResolvedValue([]);
    const onChange = vi.fn();
    const settings = { ...defaultAppSettings, headerBackdrop: 'lattice' as const, visibleWorkstreamStatuses: ['draft' as const] };
    render(<SettingsPage settings={settings} onChange={onChange} />);

    fireEvent.click(screen.getByRole('radio', { name: /none/i }));

    expect(onChange).toHaveBeenCalledWith({ ...settings, headerBackdrop: 'none' });
  });
});

describe('SettingsPage areas', () => {
  afterEach(() => vi.clearAllMocks());

  it('shows one area at a time, chosen from the menu or, on a phone, the select', () => {
    vi.mocked(api.aiSettings).mockResolvedValue(null as never);
    vi.mocked(api.aiCapabilities).mockResolvedValue([]);
    render(<SettingsPage settings={defaultAppSettings} onChange={vi.fn()} />);
    const menu = screen.getByRole('navigation', { name: 'Settings areas' });

    expect(within(menu).getByRole('button', { name: /Appearance/ })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('radiogroup', { name: 'Header backdrop' })).toBeInTheDocument();
    expect(screen.queryByRole('region', { name: 'Harness settings' })).not.toBeInTheDocument();

    fireEvent.click(within(menu).getByRole('button', { name: /Harness/ }));
    expect(screen.getByRole('region', { name: 'Harness settings' })).toBeInTheDocument();
    expect(screen.queryByRole('radiogroup', { name: 'Header backdrop' })).not.toBeInTheDocument();

    fireEvent.change(screen.getByRole('combobox', { name: 'Settings area' }), { target: { value: 'appearance' } });
    expect(screen.getByRole('radiogroup', { name: 'Header backdrop' })).toBeInTheDocument();
  });
});

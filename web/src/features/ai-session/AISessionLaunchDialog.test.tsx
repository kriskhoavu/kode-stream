import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { AISessionLaunchDialog } from './AISessionLaunchDialog';

vi.mock('../../lib/api', () => ({ api: {
  aiSettings: vi.fn(), aiCapabilities: vi.fn(), aiSessionEligibility: vi.fn(), launchAISession: vi.fn()
} }));

function mockOptions(cardContextAvailable = true) {
  vi.mocked(api.aiSettings).mockResolvedValue({
    defaultProvider: 'codex', defaultTerminal: 'terminal',
    providers: { codex: { enabled: true, executable: 'codex', args: [] } },
    terminals: { terminal: { enabled: true, executable: '/Terminal.app', args: [] } }
  });
  vi.mocked(api.aiCapabilities).mockResolvedValue([
    { id: 'codex', kind: 'provider', detected: true, configured: true, executable: '/bin/codex' },
    { id: 'terminal', kind: 'terminal', detected: true, configured: true, executable: '/Terminal.app' }
  ]);
  vi.mocked(api.aiSessionEligibility).mockResolvedValue({ editable: cardContextAvailable, cardContextAvailable, missing: cardContextAvailable ? [] : ['editable working-tree item'] });
}

describe('AISessionLaunchDialog', () => {
  afterEach(() => vi.clearAllMocks());

  it('provides card context without implementation readiness', async () => {
    mockOptions();
    vi.mocked(api.launchAISession).mockResolvedValue({ accepted: true, provider: 'codex', terminal: 'terminal', contextMode: 'card_context', startedAt: '2026-07-02T00:00:00Z' });
    const onClose = vi.fn();
    render(<AISessionLaunchDialog itemId="item-1" onClose={onClose} onLaunched={vi.fn()} />);
    expect(await screen.findByText(/selected card path will be provided/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Open session' }));
    await waitFor(() => expect(api.launchAISession).toHaveBeenCalledWith('item-1', { provider: 'codex', terminal: 'terminal', contextMode: 'card_context' }));
    expect(onClose).toHaveBeenCalled();
  });

  it('keeps the dialog open and reports launch errors', async () => {
    mockOptions();
    vi.mocked(api.launchAISession).mockRejectedValue(new Error('Terminal missing'));
    render(<AISessionLaunchDialog itemId="item-1" onClose={vi.fn()} onLaunched={vi.fn()} />);
    await screen.findByText(/selected card path/i);
    fireEvent.click(screen.getByRole('button', { name: 'Open session' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Terminal missing');
  });

  it('prevents duplicate launch submissions', async () => {
    mockOptions();
    let resolveLaunch!: (value: { accepted: true; provider: string; terminal: string; contextMode: 'card_context'; startedAt: string }) => void;
    vi.mocked(api.launchAISession).mockReturnValue(new Promise((resolve) => { resolveLaunch = resolve; }));
    render(<AISessionLaunchDialog itemId="item-1" onClose={vi.fn()} onLaunched={vi.fn()} />);
    await screen.findByText(/selected card path/i);
    fireEvent.click(screen.getByRole('button', { name: 'Open session' }));
    expect(screen.getByRole('button', { name: 'Opening...' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Opening...' }));
    expect(api.launchAISession).toHaveBeenCalledTimes(1);
    await act(async () => resolveLaunch({ accepted: true, provider: 'codex', terminal: 'terminal', contextMode: 'card_context', startedAt: '2026-07-02T00:00:00Z' }));
  });

  it('allows workspace-only sessions when card context is unavailable', async () => {
    mockOptions(false);
    vi.mocked(api.launchAISession).mockResolvedValue({ accepted: true, provider: 'codex', terminal: 'terminal', contextMode: 'workspace_only', startedAt: '2026-07-02T00:00:00Z' });
    render(<AISessionLaunchDialog itemId="snapshot" onClose={vi.fn()} onLaunched={vi.fn()} />);
    fireEvent.click(await screen.findByLabelText(/workspace only/i));
    expect(screen.getByText(/no card context will be injected/i)).toBeInTheDocument();
    expect(screen.queryByText(/selected card path will be provided/i)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Open session' }));
    await waitFor(() => expect(api.launchAISession).toHaveBeenCalledWith('snapshot', { provider: 'codex', terminal: 'terminal', contextMode: 'workspace_only' }));
  });
});

import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { AISessionLaunchDialog } from './AISessionLaunchDialog';

vi.mock('../../lib/api', () => ({ api: {
  aiSettings: vi.fn(), aiCapabilities: vi.fn(), aiPresets: vi.fn(), aiSessionEligibility: vi.fn(), launchAISession: vi.fn(), startEmbeddedAISession: vi.fn()
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
  vi.mocked(api.aiPresets).mockResolvedValue([
    { id: 'implementation-plan', name: 'Create implementation plan', prompt: 'Create a plan', contextMode: 'card_context' },
    { id: 'technical-design', name: 'Create technical design', prompt: 'Create a design', contextMode: 'card_context' }
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
    await waitFor(() => expect(api.launchAISession).toHaveBeenCalledWith('item-1', { provider: 'codex', terminal: 'terminal', contextMode: 'card_context', presetId: 'implementation-plan', customPrompt: undefined }));
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
    await waitFor(() => expect(api.launchAISession).toHaveBeenCalledWith('snapshot', { provider: 'codex', terminal: 'terminal', contextMode: 'workspace_only', presetId: 'implementation-plan', customPrompt: undefined }));
  });

  it('passes a free prompt when selected', async () => {
    mockOptions();
    vi.mocked(api.launchAISession).mockResolvedValue({ accepted: true, provider: 'codex', terminal: 'terminal', contextMode: 'card_context', startedAt: '2026-07-02T00:00:00Z' });
    render(<AISessionLaunchDialog itemId="item-1" onClose={vi.fn()} onLaunched={vi.fn()} />);
    await screen.findByText(/selected card path/i);
    fireEvent.change(screen.getByLabelText('AI prompt'), { target: { value: '' } });
    fireEvent.change(screen.getByLabelText('Free prompt'), { target: { value: 'Use the Jira context first.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Open session' }));
    await waitFor(() => expect(api.launchAISession).toHaveBeenCalledWith('item-1', { provider: 'codex', terminal: 'terminal', contextMode: 'card_context', presetId: undefined, customPrompt: 'Use the Jira context first.' }));
  });

	it('starts an embedded session without requiring an external terminal', async () => {
		mockOptions();
		vi.mocked(api.startEmbeddedAISession).mockResolvedValue({ session: { id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'running', startedAt: '2026-07-03T00:00:00Z' }, grant: { sessionId: 'session-1', token: 'secret', expiresAt: '2026-07-03T00:01:00Z' } });
		const launched = vi.fn();
		render(<AISessionLaunchDialog itemId="item-1" onClose={vi.fn()} onLaunched={launched} />);
		fireEvent.click(await screen.findByLabelText('Embedded terminal'));
		expect(screen.queryByLabelText('Terminal')).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Open session' }));
		await waitFor(() => expect(api.startEmbeddedAISession).toHaveBeenCalledWith('item-1', { provider: 'codex', contextMode: 'card_context', presetId: 'implementation-plan', customPrompt: undefined, columns: 80, rows: 24 }));
		expect(launched).toHaveBeenCalledWith(expect.objectContaining({ session: expect.objectContaining({ id: 'session-1' }) }), expect.objectContaining({ surface: 'embedded' }));
	});
});

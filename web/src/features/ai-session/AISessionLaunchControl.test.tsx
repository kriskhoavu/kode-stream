import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { AISessionLaunchResult } from '../../lib/types';
import { api } from '../../lib/api';
import { AISessionLaunchControl } from './AISessionLaunchControl';

vi.mock('../../lib/api', () => ({ api: { launchAISession: vi.fn(), startEmbeddedAISession: vi.fn(), cancelEmbeddedAISession: vi.fn() } }));
vi.mock('./AISessionLaunchDialog', () => ({
	AISessionLaunchDialog: ({ onLaunched }: { onLaunched: (result: AISessionLaunchResult, input: AISessionLaunchResult) => void }) => <button onClick={() => onLaunched({ accepted: true, provider: 'codex', terminal: 'iterm2', contextMode: 'workspace_only', startedAt: '2026-07-02T00:00:00Z' }, { accepted: true, provider: 'codex', terminal: 'iterm2', contextMode: 'workspace_only', surface: 'external', startedAt: '2026-07-02T00:00:00Z' })}>Save test choice</button>
}));

describe('AISessionLaunchControl', () => {
  afterEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
  });

  it('opens configuration first, then uses the remembered choice for one-click launch', async () => {
    vi.mocked(api.launchAISession).mockResolvedValue({ accepted: true, provider: 'codex', terminal: 'iterm2', contextMode: 'workspace_only', startedAt: '2026-07-02T00:00:00Z' });
    render(<AISessionLaunchControl itemId="item-1" onLaunched={vi.fn()} onError={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: 'Open AI session' }));
    fireEvent.click(screen.getByRole('button', { name: 'Save test choice' }));
		expect(JSON.parse(localStorage.getItem('aiSession.lastLaunch') ?? 'null')).toEqual({ provider: 'codex', terminal: 'iterm2', contextMode: 'workspace_only', surface: 'external' });
    expect(screen.getByRole('button', { name: /using saved choice: Codex · iTerm2 · workspace only/i })).toHaveAttribute('title', 'Saved choice: Codex · iTerm2 · workspace only');
    fireEvent.click(screen.getByRole('button', { name: /using saved choice/i }));

    await waitFor(() => expect(api.launchAISession).toHaveBeenCalledWith('item-1', { provider: 'codex', terminal: 'iterm2', contextMode: 'workspace_only', presetId: undefined, promptDraft: undefined, customPrompt: undefined, selectedSkills: undefined, selectedAgents: undefined }));
  });

  it('always opens configuration from the settings segment', () => {
    localStorage.setItem('aiSession.lastLaunch', JSON.stringify({ provider: 'claude', terminal: 'wezterm', contextMode: 'card_context' }));
    render(<AISessionLaunchControl itemId="item-1" onLaunched={vi.fn()} onError={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: 'Configure AI session' }));

    expect(screen.getByRole('button', { name: 'Save test choice' })).toBeInTheDocument();
    expect(api.launchAISession).not.toHaveBeenCalled();
  });

  it('reopens configuration when a remembered launch fails', async () => {
    localStorage.setItem('aiSession.lastLaunch', JSON.stringify({ provider: 'claude', terminal: 'wezterm', contextMode: 'card_context' }));
    vi.mocked(api.launchAISession).mockRejectedValue(new Error('Provider unavailable'));
    const onError = vi.fn();
    render(<AISessionLaunchControl itemId="item-1" onLaunched={vi.fn()} onError={onError} />);

    fireEvent.click(screen.getByRole('button', { name: /using saved choice/i }));

    expect(await screen.findByRole('button', { name: 'Save test choice' })).toBeInTheDocument();
    expect(onError).toHaveBeenCalledWith(expect.objectContaining({ message: 'Provider unavailable' }));
  });

  it('opens configuration instead of using a remembered embedded launch when embedded sessions are disabled', async () => {
    localStorage.setItem('aiSession.lastLaunch', JSON.stringify({ provider: 'codex', terminal: 'terminal', contextMode: 'card_context', surface: 'embedded' }));
    render(<AISessionLaunchControl itemId="item-1" allowEmbedded={false} onLaunched={vi.fn()} onError={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: /using saved choice/i }));

    expect(screen.getByRole('button', { name: 'Save test choice' })).toBeInTheDocument();
    expect(api.startEmbeddedAISession).not.toHaveBeenCalled();
  });
});

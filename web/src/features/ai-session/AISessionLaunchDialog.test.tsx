import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { AISessionLaunchDialog } from './AISessionLaunchDialog';

vi.mock('../../lib/api', () => ({ api: {
  aiSettings: vi.fn(), aiCapabilities: vi.fn(), aiProviderCapabilities: vi.fn(), aiPresets: vi.fn(), aiSessionEligibility: vi.fn(), launchAISession: vi.fn(), startEmbeddedAISession: vi.fn()
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
  vi.mocked(api.aiProviderCapabilities).mockResolvedValue({
    provider: 'codex',
    skills: [{ id: 'implementation-planning', name: 'Implementation planning', kind: 'skill', description: 'Focus on concrete implementation steps.', provider: 'codex', scope: 'workspace', sourcePath: '.codex/skills/implementation-planning.md' }],
    agents: [{ id: 'reviewer', name: 'Reviewer', kind: 'agent', description: 'Focus on review.', provider: 'codex', scope: 'global', sourcePath: '.codex/agents/reviewer.md' }],
    supportsNativeSelection: false,
    supportsPromptFallback: true
  });
  vi.mocked(api.aiSessionEligibility).mockResolvedValue({ editable: cardContextAvailable, cardContextAvailable, missing: cardContextAvailable ? [] : ['editable working-tree item'] });
}

function capability(id: string, name: string, scope: 'workspace' | 'global' = 'workspace') {
  return { id, name, kind: 'skill' as const, description: '', provider: 'codex', scope, sourcePath: `.codex/${scope === 'workspace' ? 'skills' : 'agents'}/${id}.md` };
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
    await waitFor(() => expect(api.launchAISession).toHaveBeenCalledWith('item-1', { provider: 'codex', terminal: 'terminal', contextMode: 'card_context', surface: 'external', presetId: 'implementation-plan', promptDraft: 'Create a plan', selectedSkills: undefined, selectedAgents: undefined }));
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
    await waitFor(() => expect(api.launchAISession).toHaveBeenCalledWith('snapshot', { provider: 'codex', terminal: 'terminal', contextMode: 'workspace_only', surface: 'external', presetId: 'implementation-plan', promptDraft: 'Create a plan', selectedSkills: undefined, selectedAgents: undefined }));
  });

  it('passes a free prompt when selected', async () => {
    mockOptions();
    vi.mocked(api.launchAISession).mockResolvedValue({ accepted: true, provider: 'codex', terminal: 'terminal', contextMode: 'card_context', startedAt: '2026-07-02T00:00:00Z' });
    render(<AISessionLaunchDialog itemId="item-1" onClose={vi.fn()} onLaunched={vi.fn()} />);
    await screen.findByText(/selected card path/i);
    fireEvent.change(screen.getByLabelText('AI prompt'), { target: { value: '' } });
    fireEvent.change(screen.getByLabelText('Prompt'), { target: { value: 'Use the Jira context first.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Open session' }));
    await waitFor(() => expect(api.launchAISession).toHaveBeenCalledWith('item-1', { provider: 'codex', terminal: 'terminal', contextMode: 'card_context', surface: 'external', presetId: undefined, promptDraft: 'Use the Jira context first.', selectedSkills: undefined, selectedAgents: undefined }));
  });

	it('starts an embedded session without requiring an external terminal', async () => {
		mockOptions();
		vi.mocked(api.startEmbeddedAISession).mockResolvedValue({ session: { id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'running', startedAt: '2026-07-03T00:00:00Z' }, grant: { sessionId: 'session-1', token: 'secret', expiresAt: '2026-07-03T00:01:00Z' } });
		const launched = vi.fn();
		render(<AISessionLaunchDialog itemId="item-1" onClose={vi.fn()} onLaunched={launched} />);
		expect(await screen.findByLabelText('Integrated terminal')).toBeChecked();
		fireEvent.click(screen.getByLabelText('Embedded terminal'));
		expect(screen.queryByLabelText('Terminal')).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Open session' }));
		await waitFor(() => expect(api.startEmbeddedAISession).toHaveBeenCalledWith('item-1', { provider: 'codex', contextMode: 'card_context', presetId: 'implementation-plan', promptDraft: 'Create a plan', selectedSkills: undefined, selectedAgents: undefined, columns: 80, rows: 24 }));
		expect(launched).toHaveBeenCalledWith(expect.objectContaining({ session: expect.objectContaining({ id: 'session-1' }) }), expect.objectContaining({ surface: 'embedded' }));
	});

	it('includes selected provider capabilities in the launch payload', async () => {
		mockOptions();
		vi.mocked(api.launchAISession).mockResolvedValue({ accepted: true, provider: 'codex', terminal: 'terminal', contextMode: 'card_context', startedAt: '2026-07-02T00:00:00Z' });
		render(<AISessionLaunchDialog itemId="item-1" onClose={vi.fn()} onLaunched={vi.fn()} />);
		await screen.findByText(/selected card path/i);
		fireEvent.click(await screen.findByRole('button', { name: /skills/i }));
		await screen.findByText('.codex/skills/implementation-planning.md');
		fireEvent.click(screen.getByRole('checkbox', { name: /Implementation planning/i }));
		fireEvent.click(screen.getByRole('button', { name: /agents/i }));
		fireEvent.click(screen.getAllByRole('tab', { name: /global/i })[1]);
		fireEvent.click(screen.getByRole('checkbox', { name: /Reviewer/i }));
		expect(screen.getByLabelText('Selected skills')).toHaveTextContent('Implementation planning');
		expect(screen.getByLabelText('Selected agents')).toHaveTextContent('Reviewer');
		expect(screen.getByText('.codex/skills/implementation-planning.md')).toBeInTheDocument();
		expect(screen.getByText('.codex/agents/reviewer.md')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Open session' }));
		await waitFor(() => expect(api.launchAISession).toHaveBeenCalledWith('item-1', { provider: 'codex', terminal: 'terminal', contextMode: 'card_context', surface: 'external', presetId: 'implementation-plan', promptDraft: 'Create a plan', selectedSkills: ['implementation-planning'], selectedAgents: ['reviewer'] }));
	});

	it('does not preselect provider capabilities from saved preferences', async () => {
		mockOptions();
		render(<AISessionLaunchDialog itemId="item-1" preference={{ provider: 'codex', terminal: 'terminal', contextMode: 'card_context', selectedSkills: ['implementation-planning'], selectedAgents: ['reviewer'] }} onClose={vi.fn()} onLaunched={vi.fn()} />);
		fireEvent.click(await screen.findByRole('button', { name: /skills/i }));
		expect(await screen.findByRole('checkbox', { name: /Implementation planning/i })).not.toBeChecked();
		fireEvent.click(screen.getByRole('button', { name: /agents/i }));
		fireEvent.click(screen.getAllByRole('tab', { name: /global/i })[1]);
		expect(screen.getByRole('checkbox', { name: /Reviewer/i })).not.toBeChecked();
	});

	it('allows deselecting chosen capabilities from summary badges', async () => {
		mockOptions();
		render(<AISessionLaunchDialog itemId="item-1" onClose={vi.fn()} onLaunched={vi.fn()} />);
		fireEvent.click(await screen.findByRole('button', { name: /skills/i }));
		await screen.findByRole('checkbox', { name: /Implementation planning/i });
		fireEvent.click(screen.getByRole('checkbox', { name: /Implementation planning/i }));
		expect(screen.getByLabelText('Selected skills')).toHaveTextContent('Implementation planning');
		fireEvent.click(screen.getByRole('button', { name: 'Remove Implementation planning' }));
		expect(screen.queryByLabelText('Selected skills')).not.toBeInTheDocument();
		expect(screen.getByRole('checkbox', { name: /Implementation planning/i })).not.toBeChecked();
	});

	it('filters capability rows and supports bulk selection', async () => {
		mockOptions();
		vi.mocked(api.aiProviderCapabilities).mockResolvedValue({
			provider: 'codex',
			skills: [
				capability('implementation-planning', 'Implementation planning'),
				capability('technical-design', 'Technical design'),
				capability('test-scenarios', 'Test scenarios', 'global')
			],
			agents: [],
			supportsNativeSelection: false,
			supportsPromptFallback: true
		});
		render(<AISessionLaunchDialog itemId="item-1" onClose={vi.fn()} onLaunched={vi.fn()} />);
		fireEvent.click(await screen.findByRole('button', { name: /skills/i }));
		await screen.findByText('Technical design');
		fireEvent.change(screen.getByLabelText(/filter skills/i), { target: { value: 'design' } });
		expect(screen.getByText('Technical design')).toBeInTheDocument();
		expect(screen.queryByText('Implementation planning')).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Select all' }));
		expect(screen.getByRole('checkbox', { name: /Technical design/i })).toBeChecked();
		fireEvent.click(screen.getByRole('button', { name: 'Clear' }));
		expect(screen.getByRole('checkbox', { name: /Technical design/i })).not.toBeChecked();
		fireEvent.change(screen.getByLabelText(/filter skills/i), { target: { value: '' } });
		fireEvent.click(screen.getAllByRole('tab', { name: /global/i })[0]);
		expect(screen.getByText('Test scenarios')).toBeInTheDocument();
	});

	it('selects all matching capabilities in the active scope at once', async () => {
		mockOptions();
		vi.mocked(api.aiProviderCapabilities).mockResolvedValue({
			provider: 'codex',
			skills: [
				capability('implementation-planning', 'Implementation planning'),
				capability('technical-design', 'Technical design'),
				capability('test-scenarios', 'Test scenarios', 'global')
			],
			agents: [],
			supportsNativeSelection: false,
			supportsPromptFallback: true
		});
		render(<AISessionLaunchDialog itemId="item-1" onClose={vi.fn()} onLaunched={vi.fn()} />);
		fireEvent.click(await screen.findByRole('button', { name: /skills/i }));
		await screen.findByText('Technical design');
		fireEvent.click(screen.getByRole('button', { name: 'Select all' }));
		expect(screen.getByRole('checkbox', { name: /Implementation planning/i })).toBeChecked();
		expect(screen.getByRole('checkbox', { name: /Technical design/i })).toBeChecked();
		expect(screen.getByLabelText('Selected skills')).toHaveTextContent('Implementation planning');
		expect(screen.getByLabelText('Selected skills')).toHaveTextContent('Technical design');
	});

	it('collapses long capability lists until expanded', async () => {
		mockOptions();
		vi.mocked(api.aiProviderCapabilities).mockResolvedValue({
			provider: 'codex',
			skills: [
				capability('skill-1', 'Skill 1'),
				capability('skill-2', 'Skill 2'),
				capability('skill-3', 'Skill 3'),
				capability('skill-4', 'Skill 4'),
				capability('skill-5', 'Skill 5'),
				capability('skill-6', 'Skill 6'),
				capability('skill-7', 'Skill 7')
			],
			agents: [],
			supportsNativeSelection: false,
			supportsPromptFallback: true
		});
		render(<AISessionLaunchDialog itemId="item-1" onClose={vi.fn()} onLaunched={vi.fn()} />);
		fireEvent.click(await screen.findByRole('button', { name: /skills/i }));
		await screen.findByText('Skill 6');
		expect(screen.queryByText('Skill 7')).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Show 1 more' }));
		expect(screen.getByText('Skill 7')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Show less' }));
		expect(screen.queryByText('Skill 7')).not.toBeInTheDocument();
	});

	it('collapses and expands capability sections', async () => {
		mockOptions();
		render(<AISessionLaunchDialog itemId="item-1" onClose={vi.fn()} onLaunched={vi.fn()} />);
		expect(screen.queryByLabelText(/filter skills/i)).not.toBeInTheDocument();
		fireEvent.click(await screen.findByRole('button', { name: /skills/i }));
		expect(screen.getByLabelText(/filter skills/i)).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: /skills/i }));
		expect(screen.queryByLabelText(/filter skills/i)).not.toBeInTheDocument();
	});
});

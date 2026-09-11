import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import type { GlobalSessionRow, GlobalSessionsResponse } from '../../lib/types';
import { SessionsPage } from './SessionsPage';

vi.mock('../ai-session/EmbeddedTerminal', () => ({
	EmbeddedTerminal: () => <section data-testid="mock-page-terminal" />
}));

function row(overrides: Partial<GlobalSessionRow['record']> = {}, rowOverrides: Partial<GlobalSessionRow> = {}): GlobalSessionRow {
	return {
		workspaceName: 'Workspace One',
		checkoutBranch: 'main',
		planTitle: 'Ship the thing',
		changesCount: 2,
		record: {
			id: 'session-1',
			workspaceId: 'workspace-1',
			provider: 'codex',
			intent: 'card_context',
			requestedBranch: 'main',
			state: 'running',
			startedAt: '2026-01-01T00:00:00.000Z',
			lastKnownAt: '2026-01-01T00:00:00.000Z',
			live: true,
			...overrides
		},
		...rowOverrides
	};
}

function response(rows: GlobalSessionRow[]): GlobalSessionsResponse {
	return { rows, truncated: false };
}

describe('SessionsPage', () => {
	beforeEach(() => {
		localStorage.clear();
		vi.spyOn(api, 'embeddedAISession').mockResolvedValue({ id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'running', startedAt: '' });
		vi.spyOn(api, 'embeddedAISessionGrant').mockResolvedValue({ sessionId: 'session-1', token: 'grant', expiresAt: '' });
	});
	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('toggles the scope, re-queries, and persists the choice', async () => {
		const listSessions = vi.spyOn(api, 'listSessions').mockResolvedValue(response([row()]));
		render(<SessionsPage onOpenSession={vi.fn()} onOpenWorkbench={vi.fn()} />);
		await waitFor(() => expect(listSessions).toHaveBeenCalledWith('active'));

		fireEvent.click(screen.getByRole('button', { name: 'All' }));
		await waitFor(() => expect(listSessions).toHaveBeenCalledWith('all'));
		expect(screen.getByRole('button', { name: 'All' })).toHaveAttribute('aria-pressed', 'true');
		expect(localStorage.getItem('kodeStream.sessions.scope')).toBe('all');
	});

	it('renders the Session Browser grouped by workspace and plan', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([row()]));
		render(<SessionsPage onOpenSession={vi.fn()} onOpenWorkbench={vi.fn()} />);
		expect(await screen.findByText('Workspace One · Ship the thing')).toBeInTheDocument();
		await screen.findByTestId('mock-page-terminal');
	});

	it('opens the Workbench for a row with its session node selected', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([row()]));
		const onOpenSession = vi.fn();
		render(<SessionsPage onOpenSession={onOpenSession} onOpenWorkbench={vi.fn()} />);
		await screen.findByTestId('mock-page-terminal');
		fireEvent.click(screen.getByRole('button', { name: 'Open on board' }));
		expect(onOpenSession).toHaveBeenCalledWith('workspace-1', 'session:session-1');
	});

	it('confirms before stopping a live session', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([row()]));
		const cancel = vi.spyOn(api, 'cancelEmbeddedAISession').mockResolvedValue({} as never);
		render(<SessionsPage onOpenSession={vi.fn()} onOpenWorkbench={vi.fn()} />);
		await screen.findByTestId('mock-page-terminal');
		fireEvent.click(screen.getByRole('button', { name: 'Stop Ship the thing session' }));
		expect(await screen.findByRole('dialog', { name: 'Stop session?' })).toBeInTheDocument();
		expect(cancel).not.toHaveBeenCalled();

		fireEvent.click(screen.getByRole('button', { name: 'Stop session' }));
		await waitFor(() => expect(cancel).toHaveBeenCalledWith('session-1'));
		await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Stop session?' })).not.toBeInTheDocument());
	});

	it('cancels a stop confirmation without cancelling the session', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([row()]));
		const cancel = vi.spyOn(api, 'cancelEmbeddedAISession').mockResolvedValue({} as never);
		render(<SessionsPage onOpenSession={vi.fn()} onOpenWorkbench={vi.fn()} />);
		await screen.findByTestId('mock-page-terminal');
		fireEvent.click(screen.getByRole('button', { name: 'Stop Ship the thing session' }));
		const dialog = await screen.findByRole('dialog');
		fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		expect(cancel).not.toHaveBeenCalled();
	});

	it('shows a stop error and keeps the dialog open when stopping fails', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([row()]));
		const cancel = vi.spyOn(api, 'cancelEmbeddedAISession').mockRejectedValue(new Error('Could not stop the process.'));
		render(<SessionsPage onOpenSession={vi.fn()} onOpenWorkbench={vi.fn()} />);
		await screen.findByTestId('mock-page-terminal');
		fireEvent.click(screen.getByRole('button', { name: 'Stop Ship the thing session' }));
		const dialog = await screen.findByRole('dialog');
		fireEvent.click(within(dialog).getByRole('button', { name: 'Stop session' }));

		expect(await within(dialog).findByRole('alert')).toHaveTextContent('Could not stop the process.');
		expect(screen.getByRole('dialog')).toBeInTheDocument();
		expect(cancel).toHaveBeenCalledWith('session-1');
	});

	it('renders the empty state with a link to the Workbench', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([]));
		const onOpenWorkbench = vi.fn();
		render(<SessionsPage onOpenSession={vi.fn()} onOpenWorkbench={onOpenWorkbench} />);
		expect(await screen.findByText('No sessions running. Start one from a plan on the Workbench.')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: /Go to Workbench/ }));
		expect(onOpenWorkbench).toHaveBeenCalled();
	});
});

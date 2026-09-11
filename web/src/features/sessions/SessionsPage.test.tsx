import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import type { GlobalSessionRow, GlobalSessionsResponse } from '../../lib/types';
import { SessionsPage } from './SessionsPage';

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

function workspaceRow(workspaceName: string, id: string, planTitle: string): GlobalSessionRow {
	return row({ id, workspaceId: id }, { workspaceName, planTitle });
}

describe('SessionsPage', () => {
	beforeEach(() => {
		localStorage.clear();
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

	it('opens the Workbench for a row with its session node selected', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([row()]));
		const onOpenSession = vi.fn();
		render(<SessionsPage onOpenSession={onOpenSession} onOpenWorkbench={vi.fn()} />);
		fireEvent.click(await screen.findByRole('button', { name: 'Open Ship the thing in the Workbench' }));
		expect(onOpenSession).toHaveBeenCalledWith('workspace-1', 'session:session-1');
	});

	it('confirms before stopping a live session', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([row()]));
		const cancel = vi.spyOn(api, 'cancelEmbeddedAISession').mockResolvedValue({} as never);
		render(<SessionsPage onOpenSession={vi.fn()} onOpenWorkbench={vi.fn()} />);
		fireEvent.click(await screen.findByRole('button', { name: 'Stop Ship the thing' }));
		expect(await screen.findByRole('dialog', { name: 'Stop this session?' })).toBeInTheDocument();
		expect(cancel).not.toHaveBeenCalled();

		fireEvent.click(screen.getByRole('button', { name: 'Stop session' }));
		await waitFor(() => expect(cancel).toHaveBeenCalledWith('session-1'));
		await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Stop this session?' })).not.toBeInTheDocument());
	});

	it('cancels a stop confirmation without cancelling the session', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([row()]));
		const cancel = vi.spyOn(api, 'cancelEmbeddedAISession').mockResolvedValue({} as never);
		render(<SessionsPage onOpenSession={vi.fn()} onOpenWorkbench={vi.fn()} />);
		fireEvent.click(await screen.findByRole('button', { name: 'Stop Ship the thing' }));
		fireEvent.click(await screen.findByRole('button', { name: 'Cancel' }));
		await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Stop this session?' })).not.toBeInTheDocument());
		expect(cancel).not.toHaveBeenCalled();
	});

	it('renders the empty state with a link to the Workbench', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([]));
		const onOpenWorkbench = vi.fn();
		render(<SessionsPage onOpenSession={vi.fn()} onOpenWorkbench={onOpenWorkbench} />);
		expect(await screen.findByText('No sessions running. Start one from a plan on the Workbench.')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: /Go to Workbench/ }));
		expect(onOpenWorkbench).toHaveBeenCalled();
	});

	it('shows a stop error and keeps the dialog open when stopping fails', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([row()]));
		const cancel = vi.spyOn(api, 'cancelEmbeddedAISession').mockRejectedValue(new Error('Could not stop the process.'));
		render(<SessionsPage onOpenSession={vi.fn()} onOpenWorkbench={vi.fn()} />);
		fireEvent.click(await screen.findByRole('button', { name: 'Stop Ship the thing' }));
		fireEvent.click(await screen.findByRole('button', { name: 'Stop session' }));

		expect(await screen.findByRole('alert')).toHaveTextContent('Could not stop the process.');
		expect(screen.getByRole('dialog', { name: 'Stop this session?' })).toBeInTheDocument();
		expect(cancel).toHaveBeenCalledWith('session-1');
	});

	it('prunes a workspace filter once its row disappears instead of leaving an invisible filter', async () => {
		const listSessions = vi.spyOn(api, 'listSessions')
			.mockResolvedValueOnce(response([workspaceRow('Workspace One', 'session-1', 'Plan One'), workspaceRow('Workspace Two', 'session-2', 'Plan Two')]))
			.mockResolvedValueOnce(response([workspaceRow('Workspace One', 'session-1', 'Plan One')]));
		render(<SessionsPage onOpenSession={vi.fn()} onOpenWorkbench={vi.fn()} />);
		await waitFor(() => expect(listSessions).toHaveBeenCalledTimes(1));

		fireEvent.click(await screen.findByRole('button', { name: 'Workspace Two' }));
		expect(screen.getByRole('button', { name: 'Workspace Two' })).toHaveAttribute('aria-pressed', 'true');
		expect(screen.queryByText('Plan One')).not.toBeInTheDocument();

		// Re-query (a scope toggle triggers a reload) with Workspace Two's row gone.
		fireEvent.click(screen.getByRole('button', { name: 'All' }));
		await waitFor(() => expect(listSessions).toHaveBeenCalledTimes(2));

		// The stale selection is pruned rather than leaving the table stuck empty.
		await waitFor(() => expect(screen.queryByRole('button', { name: 'Workspace Two' })).not.toBeInTheDocument());
		expect(await screen.findByText('Plan One')).toBeInTheDocument();
	});

	it('offers Clear filters when a filter empties the table, and restores the rows', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([workspaceRow('Workspace One', 'session-1', 'Plan One')]));
		render(<SessionsPage onOpenSession={vi.fn()} onOpenWorkbench={vi.fn()} />);
		await screen.findByText('Plan One');

		fireEvent.change(screen.getByRole('searchbox', { name: 'Search sessions' }), { target: { value: 'nonexistent' } });
		expect(await screen.findByText('No sessions match the current filters.')).toBeInTheDocument();

		fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));
		expect(await screen.findByText('Plan One')).toBeInTheDocument();
	});
});

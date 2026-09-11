import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import type { GlobalSessionRow } from '../../lib/types';
import { SessionsTable } from './SessionsTable';

vi.mock('../../shared/api', async () => {
	const actual = await vi.importActual<typeof import('../../shared/api')>('../../shared/api');
	return { ...actual, api: { cancelEmbeddedAISession: vi.fn() } };
});

describe('SessionsTable', () => {
	beforeEach(() => { vi.clearAllMocks(); });

	it('shows an empty message when there are no rows', () => {
		render(<SessionsTable rows={[]} onOpen={vi.fn()} onChanges={vi.fn()} emptyMessage="No sessions for this workspace." />);
		expect(screen.getByText('No sessions for this workspace.')).toBeInTheDocument();
	});

	it('renders columns and only offers Stop for a live session', () => {
		render(<SessionsTable rows={[liveRow(), exitedRow()]} onOpen={vi.fn()} onChanges={vi.fn()} />);
		expect(screen.getByRole('columnheader', { name: 'State' })).toBeInTheDocument();
		expect(screen.getByRole('columnheader', { name: 'Changes' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Open Canvas terminal support session' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Stop Canvas terminal support session' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'View changes for Canvas terminal support session' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Open codex session' })).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Stop codex session' })).not.toBeInTheDocument();
		expect(screen.getByText('3')).toBeInTheDocument();
	});

	it('calls onOpen and onChanges with the row', () => {
		const onOpen = vi.fn();
		const onChanges = vi.fn();
		const row = liveRow();
		render(<SessionsTable rows={[row]} onOpen={onOpen} onChanges={onChanges} />);
		fireEvent.click(screen.getByRole('button', { name: 'Open Canvas terminal support session' }));
		expect(onOpen).toHaveBeenCalledWith(row);
		fireEvent.click(screen.getByRole('button', { name: 'View changes for Canvas terminal support session' }));
		expect(onChanges).toHaveBeenCalledWith(row);
	});

	it('confirms before cancelling, defaulting focus to Cancel', async () => {
		vi.mocked(api.cancelEmbeddedAISession).mockResolvedValue({} as never);
		const onStopped = vi.fn();
		render(<SessionsTable rows={[liveRow()]} onOpen={vi.fn()} onChanges={vi.fn()} onStopped={onStopped} />);
		fireEvent.click(screen.getByRole('button', { name: 'Stop Canvas terminal support session' }));
		const dialog = await screen.findByRole('dialog');
		expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus();
		expect(api.cancelEmbeddedAISession).not.toHaveBeenCalled();
		fireEvent.click(within(dialog).getByRole('button', { name: 'Stop session' }));
		await waitFor(() => expect(api.cancelEmbeddedAISession).toHaveBeenCalledWith('session-live'));
		await waitFor(() => expect(onStopped).toHaveBeenCalled());
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('cancelling the dialog does not stop the session', async () => {
		render(<SessionsTable rows={[liveRow()]} onOpen={vi.fn()} onChanges={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Stop Canvas terminal support session' }));
		const dialog = await screen.findByRole('dialog');
		fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		expect(api.cancelEmbeddedAISession).not.toHaveBeenCalled();
	});

	it('shows the stop error inside the dialog when cancelling fails', async () => {
		vi.mocked(api.cancelEmbeddedAISession).mockRejectedValue(new Error('The session could not be stopped.'));
		render(<SessionsTable rows={[liveRow()]} onOpen={vi.fn()} onChanges={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Stop Canvas terminal support session' }));
		const dialog = await screen.findByRole('dialog');
		fireEvent.click(within(dialog).getByRole('button', { name: 'Stop session' }));
		expect(await within(dialog).findByRole('alert')).toHaveTextContent('The session could not be stopped.');
		expect(screen.getByRole('dialog')).toBeInTheDocument();
	});

	it('advances a live row duration on an interval tick without a prop change', () => {
		vi.useFakeTimers();
		try {
			const row = liveRow();
			row.record.startedAt = new Date(Date.now() - 1000).toISOString();
			render(<SessionsTable rows={[row]} onOpen={vi.fn()} onChanges={vi.fn()} />);
			const before = screen.getByRole('row', { name: /running/ }).textContent;
			act(() => { vi.advanceTimersByTime(3000); });
			const after = screen.getByRole('row', { name: /running/ }).textContent;
			expect(after).not.toBe(before);
		} finally {
			vi.useRealTimers();
		}
	});

	it('does not tick when no row is live', () => {
		vi.useFakeTimers();
		try {
			const setIntervalSpy = vi.spyOn(window, 'setInterval');
			render(<SessionsTable rows={[exitedRow()]} onOpen={vi.fn()} onChanges={vi.fn()} />);
			expect(setIntervalSpy).not.toHaveBeenCalled();
		} finally {
			vi.useRealTimers();
		}
	});
});

function liveRow(): GlobalSessionRow {
	return { record: { id: 'session-live', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', requestedBranch: 'main', state: 'running', startedAt: new Date().toISOString(), lastKnownAt: '', live: true }, workspaceName: 'Workspace', checkoutBranch: 'main', planTitle: 'Canvas terminal support', changesCount: 3 };
}

function exitedRow(): GlobalSessionRow {
	return { record: { id: 'session-exited', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', requestedBranch: 'main', state: 'exited', startedAt: '2026-01-01T00:00:00Z', endedAt: '2026-01-01T00:05:00Z', lastKnownAt: '', live: false, exitCode: 0 }, workspaceName: 'Workspace', checkoutBranch: 'main' };
}

import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CanvasProjection, GlobalSessionRow } from '../../lib/types';
import { api } from '../../shared/api';
import { CanvasSessionsView } from './CanvasSessionsView';

vi.mock('../../shared/api', () => ({ api: { aiSessionRecords: vi.fn(), cancelEmbeddedAISession: vi.fn(), deleteSession: vi.fn(), harnessRuns: vi.fn(), harnessTasks: vi.fn() } }));
vi.mock('../sessions/SessionBrowser', () => ({
	SessionBrowser: ({ rows, grouping, onOpenOnBoard, onStop, onRemove }: { rows: GlobalSessionRow[]; grouping: string; onOpenOnBoard: (row: GlobalSessionRow) => void; onStop: (row: GlobalSessionRow) => void; onRemove: (row: GlobalSessionRow) => void }) => <div data-testid="session-browser" data-grouping={grouping}>
		{rows.map((row) => <div key={row.record.id}><span>{row.planTitle}</span><button type="button" onClick={() => onOpenOnBoard(row)}>Open {row.record.id}</button><button type="button" onClick={() => onStop(row)}>Stop {row.record.id}</button><button type="button" onClick={() => onRemove(row)}>Remove {row.record.id}</button></div>)}
	</div>
}));

describe('CanvasSessionsView', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.aiSessionRecords).mockResolvedValue([{
			id: 'session-1', workspaceId: 'workspace-1', planRef: { itemId: 'item-1', itemPath: 'plans/platform/PM-042', branchKey: 'main' }, provider: 'codex', intent: 'card_context', requestedBranch: 'main', state: 'running', startedAt: '', lastKnownAt: '', live: true
		}]);
		vi.mocked(api.cancelEmbeddedAISession).mockResolvedValue({ id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'cancelled', startedAt: '' });
		vi.mocked(api.deleteSession).mockResolvedValue(undefined);
	});

	it('shows Sessions as the active Workbench view and groups sessions by plan', async () => {
		const onViewChange = vi.fn();
		render(<CanvasSessionsView workspaceId="workspace-1" workspaceName="Workspace" checkoutBranch="main" projection={projection()} onViewChange={onViewChange} onOpenOnBoard={vi.fn()} onReload={vi.fn()} />);
		expect(screen.getByRole('button', { name: 'Sessions' })).toHaveAttribute('aria-pressed', 'true');
		fireEvent.click(screen.getByRole('button', { name: 'Canvas' }));
		expect(onViewChange).toHaveBeenCalledWith('canvas');
		expect(await screen.findByText('Session focus')).toBeInTheDocument();
		expect(screen.getByTestId('session-browser')).toHaveAttribute('data-grouping', 'plan');
	});

	it('lists the harness runs of this workspace, working first, and opens one live', async () => {
		const onOpenHarnessRun = vi.fn();
		vi.mocked(api.harnessRuns).mockResolvedValue([
			{ id: 'run-done', taskId: 'task-a', workspaceId: 'workspace-1', state: 'passed', startedAt: '2026-01-02T00:00:00Z' },
			{ id: 'run-live', taskId: 'task-b', workspaceId: 'workspace-1', state: 'running', startedAt: '2026-01-01T00:00:00Z' },
			{ id: 'run-other', taskId: 'task-c', workspaceId: 'workspace-2', state: 'running', startedAt: '2026-01-01T00:00:00Z' }
		] as never);
		vi.mocked(api.harnessTasks).mockResolvedValue([
			{ id: 'task-a', source: { key: 'DI-1' }, title: 'Done one' },
			{ id: 'task-b', source: { key: 'DI-2' }, title: 'Live one' }
		] as never);
		render(<CanvasSessionsView workspaceId="workspace-1" checkoutBranch="main" projection={projection()} onViewChange={vi.fn()} onOpenOnBoard={vi.fn()} onOpenHarnessRun={onOpenHarnessRun} onReload={vi.fn()} />);
		const section = await screen.findByRole('region', { name: 'Harness · 1 working' });
		const items = section.querySelectorAll('li');
		expect(items[0]).toHaveTextContent('Working');
		expect(items[0]).toHaveTextContent('DI-2 · Live one');
		expect(items[1]).toHaveTextContent('DI-1 · Done one');
		expect(section).not.toHaveTextContent('run-other');
		fireEvent.click(screen.getByRole('button', { name: 'Watch' }));
		expect(onOpenHarnessRun).toHaveBeenCalledWith('run-live');
	});

	it('returns a listed session to its node on the Canvas and can stop it', async () => {
		const onOpenOnBoard = vi.fn();
		const onReload = vi.fn();
		render(<CanvasSessionsView workspaceId="workspace-1" checkoutBranch="main" projection={projection()} onViewChange={vi.fn()} onOpenOnBoard={onOpenOnBoard} onReload={onReload} />);
		fireEvent.click(await screen.findByRole('button', { name: 'Open session-1' }));
		expect(onOpenOnBoard).toHaveBeenCalledWith('session-1');
		fireEvent.click(screen.getByRole('button', { name: 'Stop session-1' }));
		await waitFor(() => expect(api.cancelEmbeddedAISession).toHaveBeenCalledWith('session-1'));
		await waitFor(() => expect(onReload).toHaveBeenCalled());
	});

	it('removes a finished session from the list and refreshes the Canvas', async () => {
		vi.mocked(api.aiSessionRecords).mockResolvedValueOnce([{
			id: 'session-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', requestedBranch: 'main', state: 'cancelled', startedAt: '', lastKnownAt: '', live: false
		}]);
		const onReload = vi.fn();
		render(<CanvasSessionsView workspaceId="workspace-1" checkoutBranch="main" projection={projection()} onViewChange={vi.fn()} onOpenOnBoard={vi.fn()} onReload={onReload} />);
		fireEvent.click(await screen.findByRole('button', { name: 'Remove session-1' }));
		await waitFor(() => expect(api.deleteSession).toHaveBeenCalledWith('session-1'));
		await waitFor(() => expect(screen.queryByRole('button', { name: 'Remove session-1' })).not.toBeInTheDocument());
		expect(onReload).toHaveBeenCalled();
	});
});

function projection(): CanvasProjection {
	return {
		layout: { id: 'layout', workspaceId: 'workspace-1', branchKey: 'main', viewport: { x: 0, y: 0, zoom: 1 }, version: 1, createdAt: '', updatedAt: '' },
		nodes: [{ id: 'plan:item-1', kind: 'plan', state: 'resolved', entityRef: { kind: 'plan', workspaceId: 'workspace-1', itemId: 'item-1' }, position: { x: 0, y: 0 }, collapsed: false, revision: 1, plan: { itemId: 'item-1', title: 'Session focus', status: 'done', editable: true, actions: {} } }],
		connections: [],
		unplaced: []
	};
}

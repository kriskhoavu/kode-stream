import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CanvasProjection, GlobalSessionRow } from '../../lib/types';
import type { HarnessSessionRow } from '../sessions/agentSessions';
import { api } from '../../shared/api';
import { CanvasSessionsView } from './CanvasSessionsView';

vi.mock('../../shared/api', () => ({ api: { aiSessionRecords: vi.fn(), cancelEmbeddedAISession: vi.fn(), deleteSession: vi.fn() } }));
vi.mock('../sessions/SessionBrowser', () => ({
	SessionBrowser: ({ rows, harness, grouping, onOpenOnBoard, onStop, onRemove }: { rows: GlobalSessionRow[]; harness?: HarnessSessionRow[]; grouping: string; onOpenOnBoard: (row: GlobalSessionRow) => void; onStop: (row: GlobalSessionRow) => void; onRemove: (row: GlobalSessionRow) => void }) => <div data-testid="session-browser" data-grouping={grouping} data-harness={harness === undefined ? 'off' : 'on'}>
		{rows.map((row) => <div key={row.record.id}><span>{row.planTitle}</span><button type="button" onClick={() => onOpenOnBoard(row)}>Open {row.record.id}</button><button type="button" onClick={() => onStop(row)}>Stop {row.record.id}</button><button type="button" onClick={() => onRemove(row)}>Remove {row.record.id}</button></div>)}
		{harness?.map((row) => <p key={row.key} data-testid="harness-row">{`${row.key} ${row.runs.map((detail) => `${detail.run.id}:${detail.agentRuns.length}`).join(' ')}`}</p>)}
	</div>
}));
const harnessState = vi.hoisted(() => ({ current: { status: { enabled: false, storageSupported: true }, runs: {}, details: {}, tasks: {} } as Record<string, unknown> }));
const loadRun = vi.hoisted(() => vi.fn());
vi.mock('../workstream/useBoardHarness', () => ({ useBoardHarness: () => ({ state: harnessState.current, loadRun, cancelRun: vi.fn(), client: {} }) }));

describe('CanvasSessionsView', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.aiSessionRecords).mockResolvedValue([{
			id: 'session-1', workspaceId: 'workspace-1', planRef: { itemId: 'item-1', itemPath: 'plans/platform/PM-042', branchKey: 'main' }, provider: 'codex', intent: 'card_context', requestedBranch: 'main', state: 'running', startedAt: '', lastKnownAt: '', live: true
		}]);
		vi.mocked(api.cancelEmbeddedAISession).mockResolvedValue({ id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'cancelled', startedAt: '' });
		vi.mocked(api.deleteSession).mockResolvedValue(undefined);
	});

	it('groups sessions by plan', async () => {
		render(<CanvasSessionsView workspaceId="workspace-1" workspaceName="Workspace" checkoutBranch="main" projection={projection()} onOpenOnBoard={vi.fn()} onReload={vi.fn()} />);
		expect(await screen.findByText('Session focus')).toBeInTheDocument();
		expect(screen.getByTestId('session-browser')).toHaveAttribute('data-grouping', 'plan');
	});

	it('lists one harness row per ticket of this workspace, holding its runs, beside the interactive sessions', async () => {
		const run = (id: string, workspaceId: string, taskId: string, startedAt = '2026-01-01T00:00:00Z') => ({ id, taskId, workspaceId, state: 'passed', startedAt, revision: 2, branch: `harness/${id}` });
		const stage = { id: 'stage-plan', kind: 'plan', ordinal: 1, state: 'passed' };
		const agentRun = (id: string, runId: string, round: number) => ({ id, runId, stageId: 'stage-plan', role: 'planner', round, state: 'succeeded', startedAt: `2026-01-01T00:0${round}:00Z` });
		harnessState.current = {
			status: { enabled: true, storageSupported: true },
			runs: { 'run-a': run('run-a', 'workspace-1', 'task-a'), 'run-b': run('run-b', 'workspace-2', 'task-b'), 'run-c': run('run-c', 'workspace-1', 'task-c'), 'run-d': run('run-d', 'workspace-1', 'task-a', '2026-01-02T00:00:00Z') },
			details: {
				'run-a': { run: run('run-a', 'workspace-1', 'task-a'), stages: [stage], agentRuns: [agentRun('agent-1', 'run-a', 1), agentRun('agent-2', 'run-a', 2)] },
				'run-d': { run: run('run-d', 'workspace-1', 'task-a', '2026-01-02T00:00:00Z'), stages: [stage], agentRuns: [agentRun('agent-3', 'run-d', 1)] }
			},
			tasks: { 'task-a': { id: 'task-a', source: { key: 'DI-1' }, title: 'Plan one' } }
		};
		render(<CanvasSessionsView workspaceId="workspace-1" checkoutBranch="main" projection={projection()} onOpenOnBoard={vi.fn()} onOpenHarnessRun={vi.fn()} onReload={vi.fn()} />);
		expect(await screen.findByTestId('session-browser')).toHaveAttribute('data-harness', 'on');
		expect(screen.getAllByTestId('harness-row').map((row) => row.textContent)).toEqual(['DI-1 run-d:1 run-a:2']);
		expect(loadRun).toHaveBeenCalledWith('run-c');
		expect(loadRun).not.toHaveBeenCalledWith('run-b');
		expect(loadRun).not.toHaveBeenCalledWith('run-a');
		harnessState.current = { status: { enabled: false, storageSupported: true }, runs: {}, details: {}, tasks: {} };
	});

	it('lists interactive sessions alone while the harness is off', async () => {
		render(<CanvasSessionsView workspaceId="workspace-1" checkoutBranch="main" projection={projection()} onOpenOnBoard={vi.fn()} onOpenHarnessRun={vi.fn()} onReload={vi.fn()} />);
		expect(await screen.findByTestId('session-browser')).toHaveAttribute('data-harness', 'off');
	});

	it('returns a listed session to its node on the Canvas and can stop it', async () => {
		const onOpenOnBoard = vi.fn();
		const onReload = vi.fn();
		render(<CanvasSessionsView workspaceId="workspace-1" checkoutBranch="main" projection={projection()} onOpenOnBoard={onOpenOnBoard} onReload={onReload} />);
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
		render(<CanvasSessionsView workspaceId="workspace-1" checkoutBranch="main" projection={projection()} onOpenOnBoard={vi.fn()} onReload={onReload} />);
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

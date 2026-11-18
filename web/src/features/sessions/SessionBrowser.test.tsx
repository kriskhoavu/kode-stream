import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { GlobalSessionRow, HarnessAgentRun, HarnessRun, HarnessRunDetail, HarnessTask, SafeSessionRecord } from '../../lib/types';
import type { HarnessSessionRow } from './agentSessions';
import { writePreference } from '../../shared/preferences/store';
import { SessionBrowser } from './SessionBrowser';

vi.mock('./SessionBrowserTerminal', () => ({
	SessionBrowserTerminal: vi.fn(({ record, visible }: { record: SafeSessionRecord; visible: boolean }) => (
		<div data-testid="mock-terminal" data-session={record.id} data-visible={String(visible)} />
	))
}));
vi.mock('./SessionChangesPanel', () => ({
	SessionChangesPanel: ({ session }: { session: SafeSessionRecord }) => <div data-testid="mock-changes" data-session={session.id} />
}));

import { SessionBrowserTerminal } from './SessionBrowserTerminal';

function record(overrides: Partial<SafeSessionRecord> = {}): SafeSessionRecord {
	return { id: 'session-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'Investigate flaky test', requestedBranch: 'main', state: 'running', startedAt: new Date(Date.now() - 60_000).toISOString(), lastKnownAt: '', live: true, ...overrides };
}

function row(overrides: Partial<SafeSessionRecord> = {}, rowOverrides: Partial<GlobalSessionRow> = {}): GlobalSessionRow {
	return { record: record(overrides), workspaceName: 'Workspace One', checkoutBranch: 'main', ...rowOverrides };
}

describe('SessionBrowser', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		localStorage.clear();
	});

	it('groups rows by plan and shows a state glyph and label for each state', () => {
		const rows = [
			row({ id: 'a', state: 'running', live: true }, { planTitle: 'Plan A' }),
			row({ id: 'b', state: 'exited', live: false, exitCode: 0 }, { planTitle: 'Plan B' }),
			row({ id: 'c', state: 'failed', live: false, exitCode: 1 }, { planTitle: 'Plan C' }),
			row({ id: 'd', state: 'cancelled', live: false }, { planTitle: 'Plan D' })
		];
		render(<SessionBrowser rows={rows} grouping="plan" persistKey="test" onStop={vi.fn()} onReload={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Live only' }));
		const groupTitles = [...document.querySelectorAll('.session-browser-group-title')].map((element) => element.textContent);
		expect(groupTitles).toEqual(['Plan A', 'Plan B', 'Plan C', 'Plan D']);
		expect(screen.getByRole('button', { name: 'Plan A, Running' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Plan B, Exited' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Plan C, Failed' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Plan D, Cancelled' })).toBeInTheDocument();
	});

	it('groups by workspace then plan when grouping is workspace-plan', () => {
		const rows = [
			row({ id: 'a' }, { planTitle: 'Plan A', workspaceName: 'Workspace One' }),
			row({ id: 'b', live: false, state: 'exited', exitCode: 0 }, { planTitle: 'Plan B', workspaceName: 'Workspace Two' })
		];
		render(<SessionBrowser rows={rows} grouping="workspace-plan" persistKey="test" onStop={vi.fn()} onReload={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Live only' }));
		const groupTitles = [...document.querySelectorAll('.session-browser-group-title')].map((element) => element.textContent);
		expect(groupTitles).toEqual(['Workspace One · Plan A', 'Workspace Two · Plan B']);
	});

	it('filters by title and provider text', () => {
		const rows = [
			row({ id: 'a', provider: 'codex' }, { planTitle: 'Ship the thing' }),
			row({ id: 'b', provider: 'claude', live: false, state: 'exited', exitCode: 0 }, { planTitle: 'Fix the bug' })
		];
		render(<SessionBrowser rows={rows} grouping="plan" persistKey="test" onStop={vi.fn()} onReload={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Live only' })); // turn Live only off so both are candidates
		fireEvent.change(screen.getByRole('searchbox', { name: 'Filter sessions' }), { target: { value: 'bug' } });
		expect(screen.queryByRole('button', { name: 'Ship the thing, Running' })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Fix the bug, Exited' })).toBeInTheDocument();
	});

	it('defaults Live only to on when a session is live, and hides finished sessions until toggled off', () => {
		const rows = [
			row({ id: 'a', live: true, state: 'running' }, { planTitle: 'Live plan' }),
			row({ id: 'b', live: false, state: 'exited', exitCode: 0 }, { planTitle: 'Finished plan' })
		];
		render(<SessionBrowser rows={rows} grouping="plan" persistKey="test" onStop={vi.fn()} onReload={vi.fn()} />);
		expect(screen.getByRole('button', { name: 'Live only' })).toHaveAttribute('aria-pressed', 'true');
		expect(screen.getByRole('button', { name: 'Live plan, Running' })).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Finished plan, Exited' })).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Live only' }));
		expect(screen.getByRole('button', { name: 'Finished plan, Exited' })).toBeInTheDocument();
	});

	it('selects a session on click, marks it aria-current, and renders its terminal as visible', () => {
		const rows = [row({ id: 'a' }, { planTitle: 'Plan A' })];
		render(<SessionBrowser rows={rows} grouping="plan" persistKey="test-select" onStop={vi.fn()} onReload={vi.fn()} />);
		const card = screen.getByRole('button', { name: 'Plan A, Running' });
		fireEvent.click(card);
		expect(card).toHaveAttribute('aria-current', 'true');
		expect(screen.getByTestId('mock-terminal')).toHaveAttribute('data-session', 'a');
		expect(screen.getByTestId('mock-terminal')).toHaveAttribute('data-visible', 'true');
		expect(vi.mocked(SessionBrowserTerminal)).toHaveBeenCalled();
	});

	it('moves the selection with arrow keys and opens it with Enter', () => {
		const rows = [
			row({ id: 'a', startedAt: new Date(Date.now() - 5000).toISOString() }, { planTitle: 'Plan A' }),
			row({ id: 'b', startedAt: new Date(Date.now() - 1000).toISOString() }, { planTitle: 'Plan B' })
		];
		render(<SessionBrowser rows={rows} grouping="plan" persistKey="test-keys" onStop={vi.fn()} onReload={vi.fn()} />);
		const first = screen.getByRole('button', { name: 'Plan B, Running' });
		const second = screen.getByRole('button', { name: 'Plan A, Running' });
		expect(first).toHaveAttribute('tabIndex', '0');
		fireEvent.keyDown(first, { key: 'ArrowDown' });
		expect(second).toHaveAttribute('aria-current', 'true');
		fireEvent.keyDown(second, { key: 'Enter' });
		expect(screen.getByTestId('mock-terminal')).toHaveAttribute('data-session', 'a');
	});

	it('only renders one terminal, for the currently selected session', () => {
		const rows = [
			row({ id: 'a' }, { planTitle: 'Plan A' }),
			row({ id: 'b', live: false, state: 'exited', exitCode: 0 }, { planTitle: 'Plan B' })
		];
		render(<SessionBrowser rows={rows} grouping="plan" persistKey="test-single" onStop={vi.fn()} onReload={vi.fn()} />);
		expect(screen.getAllByTestId('mock-terminal')).toHaveLength(1);
		fireEvent.click(screen.getByRole('button', { name: 'Live only' }));
		fireEvent.click(screen.getByRole('button', { name: 'Plan B, Exited' }));
		expect(screen.getAllByTestId('mock-terminal')).toHaveLength(1);
		expect(screen.getByTestId('mock-terminal')).toHaveAttribute('data-session', 'b');
	});

	it('shows the Changes sub-tab using the shared SessionChangesPanel', () => {
		const rows = [row({ id: 'a' }, { planTitle: 'Plan A' })];
		render(<SessionBrowser rows={rows} grouping="plan" persistKey="test-changes" onStop={vi.fn()} onReload={vi.fn()} />);
		expect(screen.getByRole('tablist', { name: 'Session detail tabs' })).toHaveClass('session-browser-detail-tabs');
		fireEvent.click(screen.getByRole('tab', { name: 'Changes' }));
		expect(screen.getByTestId('mock-changes')).toHaveAttribute('data-session', 'a');
	});

	it('confirms before stopping a live session from the detail pane', async () => {
		const onStop = vi.fn().mockResolvedValue(undefined);
		const rows = [row({ id: 'a' }, { planTitle: 'Plan A' })];
		render(<SessionBrowser rows={rows} grouping="plan" persistKey="test-stop" onStop={onStop} onReload={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Stop Plan A session' }));
		const dialog = await screen.findByRole('dialog');
		expect(onStop).not.toHaveBeenCalled();
		fireEvent.click(within(dialog).getByRole('button', { name: 'Stop session' }));
		await waitFor(() => expect(onStop).toHaveBeenCalledWith(rows[0]));
		await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
	});

	it('shows a stop error inside the dialog when stopping fails', async () => {
		const onStop = vi.fn().mockRejectedValue(new Error('Could not stop.'));
		const rows = [row({ id: 'a' }, { planTitle: 'Plan A' })];
		render(<SessionBrowser rows={rows} grouping="plan" persistKey="test-stop-error" onStop={onStop} onReload={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Stop Plan A session' }));
		const dialog = await screen.findByRole('dialog');
		fireEvent.click(within(dialog).getByRole('button', { name: 'Stop session' }));
		expect(await within(dialog).findByRole('alert')).toHaveTextContent('Could not stop.');
	});

	it('confirms before removing a finished session and never offers removal for a live one', async () => {
		const onRemove = vi.fn().mockResolvedValue(undefined);
		const rows = [
			row({ id: 'live', state: 'running', live: true }, { planTitle: 'Live plan' }),
			row({ id: 'finished', state: 'cancelled', live: false }, { planTitle: 'Finished plan' })
		];
		render(<SessionBrowser rows={rows} grouping="plan" persistKey="test-remove" onStop={vi.fn()} onRemove={onRemove} onReload={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Live only' }));
		expect(screen.queryByRole('button', { name: 'Remove Live plan session' })).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Remove Finished plan session' }));
		const dialog = screen.getByRole('dialog');
		expect(onRemove).not.toHaveBeenCalled();
		fireEvent.click(within(dialog).getByRole('button', { name: 'Remove session' }));
		await waitFor(() => expect(onRemove).toHaveBeenCalledWith(rows[1]));
		await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
	});

	it('offers Open on board only when the callback is given', () => {
		const rows = [row({ id: 'a' }, { planTitle: 'Plan A' })];
		const { rerender } = render(<SessionBrowser rows={rows} grouping="plan" persistKey="test-board" onStop={vi.fn()} onReload={vi.fn()} />);
		expect(screen.queryByRole('button', { name: 'Open on board' })).not.toBeInTheDocument();
		const onOpenOnBoard = vi.fn();
		rerender(<SessionBrowser rows={rows} grouping="plan" persistKey="test-board" onStop={vi.fn()} onReload={vi.fn()} onOpenOnBoard={onOpenOnBoard} />);
		fireEvent.click(screen.getByRole('button', { name: 'Open on board' }));
		expect(onOpenOnBoard).toHaveBeenCalledWith(rows[0]);
	});

	it('shows the empty message when there are no rows', () => {
		render(<SessionBrowser rows={[]} grouping="plan" persistKey="test-empty" onStop={vi.fn()} onReload={vi.fn()} emptyMessage="No sessions yet." />);
		expect(screen.getByText('No sessions yet.')).toBeInTheDocument();
	});

	it('keeps the card list reachable by falling the roving tabindex back to the first visible row when the selection is filtered out', () => {
		const rows = [
			row({ id: 'a', intent: 'Ship the thing' }, { planTitle: undefined }),
			row({ id: 'b', intent: 'Fix the bug' }, { planTitle: undefined })
		];
		render(<SessionBrowser rows={rows} grouping="plan" persistKey="test-roving" onStop={vi.fn()} onReload={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Ship the thing, Running' }));
		fireEvent.change(screen.getByRole('searchbox', { name: 'Filter sessions' }), { target: { value: 'bug' } });
		expect(screen.queryByRole('button', { name: 'Ship the thing, Running' })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Fix the bug, Running' })).toHaveAttribute('tabIndex', '0');
	});

	it('turns Live only off instead of dropping a remembered selection that Live-only would hide', () => {
		const rows = [
			row({ id: 'a', live: true, state: 'running' }, { planTitle: 'Live plan' }),
			row({ id: 'b', live: false, state: 'exited', exitCode: 0 }, { planTitle: 'Finished plan' })
		];
		writePreference('sessionBrowser.selection.test-remembered-hidden', 'b');
		render(<SessionBrowser rows={rows} grouping="plan" persistKey="test-remembered-hidden" onStop={vi.fn()} onReload={vi.fn()} />);
		expect(screen.getByRole('button', { name: 'Live only' })).toHaveAttribute('aria-pressed', 'false');
		expect(screen.getByRole('button', { name: 'Finished plan, Exited' })).toHaveAttribute('aria-current', 'true');
		expect(screen.getByRole('button', { name: 'Finished plan, Exited' })).toHaveAttribute('tabIndex', '0');
	});

	it('remembers the selected session per persistKey across remounts', () => {
		const rows = [
			row({ id: 'a', startedAt: new Date(Date.now() - 5000).toISOString() }, { planTitle: 'Plan A' }),
			row({ id: 'b', startedAt: new Date(Date.now() - 1000).toISOString() }, { planTitle: 'Plan B' })
		];
		const { unmount } = render(<SessionBrowser rows={rows} grouping="plan" persistKey="test-remember" onStop={vi.fn()} onReload={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Plan A, Running' }));
		unmount();
		render(<SessionBrowser rows={rows} grouping="plan" persistKey="test-remember" onStop={vi.fn()} onReload={vi.fn()} />);
		expect(screen.getByRole('button', { name: 'Plan A, Running' })).toHaveAttribute('aria-current', 'true');
	});

	describe('with harness sessions', () => {
		const run = { id: 'run-1', taskId: 'task-1', workspaceId: 'workspace-1', branch: 'harness/DI-7-1', state: 'running', startedAt: new Date(Date.now() - 3_600_000).toISOString(), revision: 1, totals: { costUsd: 0, durationMs: 0, turns: 0 } } as unknown as HarnessRun;
		const task = { id: 'task-1', source: { kind: 'jira', key: 'DI-7', labels: [] }, title: 'Fix the price' } as unknown as HarnessTask;
		const agent = (id: string, round: number, state: HarnessAgentRun['state']): HarnessAgentRun => ({ id, runId: 'run-1', stageId: 's', role: 'planner', round, engine: 'claudecode', model: 'opus', state, startedAt: new Date(Date.now() - (10 - round) * 60_000).toISOString(), turns: 3, durationMs: 60_000, costUsd: 0.1, blocking: 0, advisory: 0 });
		const detail = { run, task, stages: [], approvals: [], agentRuns: [agent('agent-1', 1, 'succeeded'), agent('agent-2', 2, 'running'), agent('agent-3', 3, 'queued')] } as unknown as HarnessRunDetail;
		const harness: HarnessSessionRow[] = [{ key: 'DI-7', task, runs: [detail] }];

		it('lists one harness card per ticket, under the ticket it shares with interactive sessions', () => {
			const rows = [row({ id: 'i-1', state: 'exited', live: false, exitCode: 0 }, { planTitle: 'Fix the price', planKey: 'DI-7' })];
			render(<SessionBrowser rows={rows} harness={harness} renderHarnessDetail={(selected) => <div data-testid="harness-detail">{selected.key}</div>} grouping="plan" persistKey="test-mixed" onStop={vi.fn()} onReload={vi.fn()} />);
			const groups = [...document.querySelectorAll('.session-browser-group')];
			expect(groups).toHaveLength(1);
			expect(groups[0].querySelector('.session-browser-group-title')).toHaveTextContent('DI-7 · Fix the priceWorking');
			// Two started rounds and a queued one make one card, naming the agent at work.
			const card = screen.getByRole('button', { name: 'DI-7 harness run, Working' });
			expect(card).toHaveTextContent('Planner · round 2');
			expect(card).toHaveTextContent('2 rounds');
			expect(screen.queryByRole('button', { name: /round 1/ })).not.toBeInTheDocument();
			fireEvent.click(screen.getByRole('button', { name: 'Live only' }));
			expect(within(groups[0] as HTMLElement).getByText('Interactive')).toBeInTheDocument();
		});

		it('filters by kind and opens a harness card in its own detail, not a terminal', () => {
			const rows = [row({ id: 'i-1', state: 'running', live: true }, { planTitle: 'Other plan' })];
			render(<SessionBrowser rows={rows} harness={harness} renderHarnessDetail={(selected) => <div data-testid="harness-detail">{selected.key}</div>} grouping="plan" persistKey="test-kinds" onStop={vi.fn()} onReload={vi.fn()} />);
			fireEvent.click(screen.getByRole('tab', { name: 'Harness' }));
			expect(screen.queryByRole('button', { name: /Other plan/ })).not.toBeInTheDocument();
			fireEvent.click(screen.getByRole('button', { name: 'DI-7 harness run, Working' }));
			expect(screen.getByTestId('harness-detail')).toHaveTextContent('DI-7');
			expect(screen.queryByTestId('mock-terminal')).not.toBeInTheDocument();
			fireEvent.click(screen.getByRole('tab', { name: 'Interactive' }));
			expect(screen.queryByRole('button', { name: /harness run/ })).not.toBeInTheDocument();
		});

		it('shows no kind filter when the harness is off', () => {
			render(<SessionBrowser rows={[row()]} grouping="plan" persistKey="test-plain" onStop={vi.fn()} onReload={vi.fn()} />);
			expect(screen.queryByRole('tablist', { name: 'Session kind' })).not.toBeInTheDocument();
		});
	});
});

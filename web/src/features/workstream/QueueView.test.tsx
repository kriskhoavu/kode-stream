import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { QueueView } from './QueueView';
import { initialHarnessState } from '../harness/harnessState';
import type { HarnessState } from '../harness/harnessState';
import { approval, run, status, task } from '../harness/testData';
import type { HarnessController } from '../harness/useHarness';
import { apiHarnessClient } from '../harness/harnessClient';

afterEach(cleanup);

function controller(state: HarnessState, pollScouts?: () => Promise<{ started: boolean }>): HarnessController {
	return {
		state, client: { ...apiHarnessClient, pollScouts }, dispatch: vi.fn(), refresh: vi.fn().mockResolvedValue(undefined), loadRun: vi.fn().mockResolvedValue(undefined),
		approve: vi.fn().mockResolvedValue(undefined), reject: vi.fn().mockResolvedValue(undefined), pin: vi.fn().mockResolvedValue(undefined), setPriority: vi.fn().mockResolvedValue(undefined), start: vi.fn().mockResolvedValue(undefined), setQuestions: vi.fn(),
		cancelRun: vi.fn(), pauseRole: vi.fn(), resumeRole: vi.fn(), createTask: vi.fn()
	};
}

const ready = task('PM-1', { score: 42, scoreReasons: ['Blocks two other tasks', 'High priority in Jira'] });
const stuck = task('PM-2', { state: 'blocked', runId: 'run-2', needsYou: true, score: 30 });
const waits = task('PM-3', { dependsOn: [ready.id], score: 20 });
const state: HarnessState = {
	...initialHarnessState, loaded: true,
	status: status({ autoStart: false, roles: [{ role: 'scout', paused: false, working: 0, slots: 1, queued: 0, lastRunAt: new Date(Date.now() - 5 * 60_000).toISOString(), found: 2 }] }),
	tasks: { [ready.id]: ready, [stuck.id]: stuck, [waits.id]: waits },
	runs: { 'run-2': run('run-2', stuck.id, { state: 'blocked' }) },
	approvals: { a1: approval('a1', stuck.id, { runId: 'run-2', gate: 'resume' }) }
};

describe('QueueView', () => {
	it('ranks the rows with their scores, explains the chosen one and says what starts next', () => {
		render(<QueueView tasks={Object.values(state.tasks)} harness={controller(state)} onOpenTask={vi.fn()} />);
		const rows = within(screen.getByRole('list', { name: 'Queue in order' })).getAllByRole('listitem');
		expect(rows.map((row) => row.textContent)).toEqual([expect.stringContaining('PM-2'), expect.stringContaining('PM-1'), expect.stringContaining('PM-3')]);
		expect(within(rows[1]).getByText('42')).toBeInTheDocument();
		fireEvent.click(rows[1]);
		const side = screen.getByRole('complementary', { name: 'About the queue' });
		expect(within(side).getByText('Blocks two other tasks')).toBeInTheDocument();
		expect(within(side).getByText('Starts when you press Start.')).toBeInTheDocument();
		expect(within(side).getByText(/Last looked 5 min ago · 2 new tasks/)).toBeInTheDocument();
	});

	it('starts a ready task, retries a stuck one through its gate, and asks the Finder to look now', async () => {
		const pollScouts = vi.fn().mockResolvedValue({ started: true });
		const harness = controller(state, pollScouts);
		render(<QueueView tasks={Object.values(state.tasks)} harness={harness} onOpenTask={vi.fn()} />);
		const rows = within(screen.getByRole('list', { name: 'Queue in order' })).getAllByRole('listitem');
		fireEvent.click(within(rows[0]).getByRole('button', { name: 'Retry' }));
		expect(harness.approve).toHaveBeenCalledWith(expect.objectContaining({ id: 'a1' }));
		fireEvent.click(within(rows[1]).getByRole('button', { name: 'Start' }));
		expect(harness.start).toHaveBeenCalledWith(expect.objectContaining({ id: ready.id }));
		expect(within(rows[2]).queryByRole('button', { name: 'Retry' })).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Look now' }));
		expect(pollScouts).toHaveBeenCalled();
		expect(await screen.findByText('The Finder is looking now. New tasks appear here as it finds them.')).toBeInTheDocument();
		expect(harness.refresh).toHaveBeenCalled();
	});
});

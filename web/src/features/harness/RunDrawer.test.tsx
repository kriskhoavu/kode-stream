import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useState } from 'react';
import type { HarnessRunTab } from '../../app/router';
import type { HarnessTask } from '../../lib/types';
import { RunDrawer } from './RunDrawer';
import { createFixtureClient } from './fixture';
import { compareTasks, harnessReducer, initialHarnessState } from './harnessState';
import type { HarnessState } from './harnessState';

afterEach(cleanup);

async function setup(taskId: string, runId?: string) {
	const client = createFixtureClient({ tickMs: 0, arrivalMs: 0 });
	let state = harnessReducer(initialHarnessState, { type: 'snapshot', status: await client.status(), tasks: await client.tasks(), runs: await client.runs(), approvals: await client.approvals() });
	if (runId) state = harnessReducer(state, { type: 'detail', detail: await client.run(runId) });
	return { client, state };
}

function Harnessed({ state, client, taskId, wave, onMove = vi.fn(), onClose = vi.fn(), onStart, initialTab = 'stages' }: { state: HarnessState; client: ReturnType<typeof createFixtureClient>; taskId: string; wave: HarnessTask[]; onMove?: () => void; onClose?: () => void; onStart?: (task: HarnessTask) => void; initialTab?: HarnessRunTab }) {
	const [tab, setTab] = useState<HarnessRunTab>(initialTab);
	return <RunDrawer state={state} client={client} taskId={taskId} tab={tab} wave={wave} onTabChange={setTab} onClose={onClose} loadRun={vi.fn().mockResolvedValue(undefined)} onApprove={vi.fn()} onReject={vi.fn()} onPin={vi.fn()} onMove={onMove} onStart={onStart} />;
}

describe('RunDrawer', () => {
	it('opens on the task with its stages, and moves between tabs with the arrow keys', async () => {
		const { client, state } = await setup('task-pm-02', 'run-pm-02');
		render(<Harnessed state={state} client={client} taskId="task-pm-02" wave={[]} />);
		expect(screen.getByRole('heading', { name: 'Add login form' })).toHaveFocus();
		expect(screen.getByRole('list', { name: 'Progress for PM-02' })).toBeInTheDocument();
		const stages = screen.getByRole('tab', { name: 'Stages' });
		expect(stages).toHaveAttribute('aria-selected', 'true');
		fireEvent.keyDown(stages, { key: 'ArrowRight' });
		expect(screen.getByRole('tab', { name: 'Findings' })).toHaveAttribute('aria-selected', 'true');
		expect(screen.getByRole('region', { name: 'Plan check, round 1' })).toHaveTextContent('The plan writes the session cookie without the Secure flag.');
	});

	it('lists changed files and the live agent without a terminal when no session is exposed', async () => {
		const { client, state } = await setup('task-pm-02', 'run-pm-02');
		render(<Harnessed state={state} client={client} taskId="task-pm-02" wave={[]} initialTab="changes" />);
		expect(await screen.findByText('web/src/features/auth/LoginForm.tsx')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('tab', { name: 'Live' }));
		expect(await screen.findByText('Writing LoginForm.tsx and its test')).toBeInTheDocument();
		expect(screen.getByText(/Builder/)).toBeInTheDocument();
	});

	it('says what a task waits for and moves it within its wave from the keyboard', async () => {
		const { client, state } = await setup('task-srv-02');
		const onMove = vi.fn();
		const wave = ['task-pm-03', 'task-data-01', 'task-srv-02'].map((id) => state.tasks[id]).sort(compareTasks);
		render(<Harnessed state={state} client={client} taskId="task-srv-02" wave={wave} onMove={onMove} initialTab="links" />);
		expect(screen.getByText('SRV-01')).toBeInTheDocument();
		const index = wave.findIndex((task) => task.id === 'task-srv-02');
		fireEvent.click(screen.getByRole('button', { name: 'Move up' }));
		expect(onMove).toHaveBeenLastCalledWith(state.tasks['task-srv-02'], wave[index - 1]);
		fireEvent.click(screen.getByRole('tab', { name: 'Stages' }));
		expect(screen.getByText('This task has not started, so there is nothing here yet.')).toBeInTheDocument();
	});

	it('shows the waiting gate at the top and closes on Escape', async () => {
		const { client, state } = await setup('task-ux-11', 'run-ux-11');
		const onClose = vi.fn();
		render(<Harnessed state={state} client={client} taskId="task-ux-11" wave={[]} onClose={onClose} />);
		expect(screen.getByRole('heading', { name: /approve the plan for UX-11/ })).toBeInTheDocument();
		fireEvent.keyDown(window, { key: 'Escape' });
		await waitFor(() => expect(onClose).toHaveBeenCalled());
	});

	it('offers Start on a task that is up next, and says it is starting once pressed', async () => {
		const { client, state } = await setup('task-pm-04');
		const onStart = vi.fn();
		const { rerender } = render(<Harnessed state={state} client={client} taskId="task-pm-04" wave={[]} onStart={onStart} />);
		const start = screen.getByRole('button', { name: 'Start' });
		expect(screen.getByRole('region', { name: 'Start this task' })).toHaveTextContent('you approve it before any code is written');
		fireEvent.click(start);
		expect(onStart).toHaveBeenCalledWith(expect.objectContaining({ id: 'task-pm-04' }));
		const started = harnessReducer(state, { type: 'task.saved', task: { ...state.tasks['task-pm-04'], startRequested: true } });
		rerender(<Harnessed state={started} client={client} taskId="task-pm-04" wave={[]} onStart={onStart} />);
		expect(screen.getByRole('button', { name: 'Starting…' })).toBeDisabled();
		expect(screen.getByText('Starting soon.')).toBeInTheDocument();
	});

	it('opens an unstarted task on its ticket, with what the tracker said about it', async () => {
		const { client, state } = await setup('task-pm-04');
		render(<Harnessed state={state} client={client} taskId="task-pm-04" wave={[]} initialTab="ticket" />);
		expect(screen.getByRole('tab', { name: 'Ticket' })).toHaveAttribute('aria-selected', 'true');
		const panel = screen.getByRole('tabpanel');
		expect(panel).toHaveTextContent(/Status/);
		expect(within(panel).getByRole('link', { name: /Open PM-04 in Jira/ })).toBeInTheDocument();
		expect(panel).toHaveTextContent('The sample data has no tracker behind it');
	});

	it('hides Start on a task that already has a run', async () => {
		const { client, state } = await setup('task-pm-02', 'run-pm-02');
		render(<Harnessed state={state} client={client} taskId="task-pm-02" wave={[]} onStart={vi.fn()} />);
		expect(screen.queryByRole('button', { name: 'Start' })).not.toBeInTheDocument();
	});
});

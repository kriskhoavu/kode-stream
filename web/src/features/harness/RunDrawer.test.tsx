import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
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

function Harnessed({ state, client, taskId, queue, onMove = vi.fn(), onClose = vi.fn(), onStart, initialTab = 'stages', agentRunId }: { state: HarnessState; client: ReturnType<typeof createFixtureClient>; taskId: string; queue: HarnessTask[]; onMove?: () => void; onClose?: () => void; onStart?: (task: HarnessTask) => void; initialTab?: HarnessRunTab; agentRunId?: string }) {
	const [tab, setTab] = useState<HarnessRunTab>(initialTab);
	return <RunDrawer state={state} client={client} taskId={taskId} tab={tab} agentRunId={agentRunId} queue={queue} onTabChange={setTab} onClose={onClose} loadRun={vi.fn().mockResolvedValue(undefined)} onApprove={vi.fn()} onReject={vi.fn()} onPin={vi.fn()} onMove={onMove} onStart={onStart} />;
}

describe('RunDrawer', () => {
	it('opens on the task with its stages, and moves between tabs with the arrow keys', async () => {
		const { client, state } = await setup('task-pm-02', 'run-pm-02');
		render(<Harnessed state={state} client={client} taskId="task-pm-02" queue={[]} />);
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
		render(<Harnessed state={state} client={client} taskId="task-pm-02" queue={[]} initialTab="changes" />);
		expect(await screen.findByText('web/src/features/auth/LoginForm.tsx')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('tab', { name: 'Live' }));
		expect(await screen.findByText('Writing LoginForm.tsx and its test')).toBeInTheDocument();
		expect(screen.getAllByText(/Builder/).length).toBeGreaterThan(0);
		const conversation = await screen.findByRole('list', { name: 'Builder conversation' });
		await waitFor(() => expect(conversation).toHaveTextContent('Harness'));
		expect(conversation).toHaveTextContent('Writing LoginForm.tsx and its test');
	});

	it('lists every agent round of the run on the Live tab and switches the conversation', async () => {
		const { client, state } = await setup('task-pm-02', 'run-pm-02');
		render(<Harnessed state={state} client={client} taskId="task-pm-02" queue={[]} initialTab="live" />);
		const rounds = await screen.findByRole('tablist', { name: 'Agent rounds' });
		expect(within(rounds).getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['5Builder · round 1working']);
		const plan = within(rounds).getByRole('button', { name: /^Plan/ });
		expect(plan).toHaveAttribute('aria-expanded', 'false');
		expect(plan).toHaveTextContent('steps 1–4');
		expect(within(rounds).queryByRole('tab', { name: /Checker · round 1/ })).toBeNull();
		expect(within(rounds).getByRole('tab', { name: /Builder · round 1/ })).toHaveAttribute('aria-selected', 'true');
		fireEvent.click(plan);
		expect(within(rounds).getAllByRole('tab').map((tab) => tab.textContent?.match(/^\d+/)?.[0])).toEqual(['1', '2', '3', '4', '5']);
		const checker = within(rounds).getByRole('tab', { name: /Checker · round 1/ });
		fireEvent.click(checker);
		expect(checker).toHaveAttribute('aria-selected', 'true');
		await waitFor(() => expect(screen.getByRole('list', { name: 'Checker conversation' })).toHaveTextContent('Brief for the plan reviewer, round 1.'));
	});

	it('opens the Live tab on the conversation it was pointed at, with its stage unfolded', async () => {
		const { client, state } = await setup('task-pm-02', 'run-pm-02');
		const checker = state.details['run-pm-02'].agentRuns.find((agentRun) => agentRun.role === 'plan_reviewer' && agentRun.round === 2);
		render(<Harnessed state={state} client={client} taskId="task-pm-02" queue={[]} initialTab="live" agentRunId={checker?.id} />);
		const rounds = await screen.findByRole('tablist', { name: 'Agent rounds' });
		expect(within(rounds).getByRole('button', { name: /^Plan/ })).toHaveAttribute('aria-expanded', 'true');
		expect(within(rounds).getByRole('tab', { name: /Checker · round 2/ })).toHaveAttribute('aria-selected', 'true');
	});

	it('says what a task waits for and moves it within the Task Queue', async () => {
		const { client, state } = await setup('task-srv-02');
		const onMove = vi.fn();
		const queue = ['task-pm-03', 'task-data-01', 'task-srv-02'].map((id) => state.tasks[id]).sort(compareTasks);
		render(<Harnessed state={state} client={client} taskId="task-srv-02" queue={queue} onMove={onMove} initialTab="links" />);
		expect(screen.getByText('SRV-01')).toBeInTheDocument();
		const index = queue.findIndex((task) => task.id === 'task-srv-02');
		fireEvent.click(screen.getByRole('button', { name: 'Move up' }));
		expect(onMove).toHaveBeenLastCalledWith(state.tasks['task-srv-02'], queue[index - 1]);
		fireEvent.click(screen.getByRole('tab', { name: 'Stages' }));
		expect(screen.getByText('This task has not started, so there is nothing here yet.')).toBeInTheDocument();
	});

	it('shows the waiting gate at the top and closes on Escape', async () => {
		const { client, state } = await setup('task-ux-11', 'run-ux-11');
		const onClose = vi.fn();
		render(<Harnessed state={state} client={client} taskId="task-ux-11" queue={[]} onClose={onClose} />);
		expect(screen.getByRole('heading', { name: /approve the plan for UX-11/ })).toBeInTheDocument();
		fireEvent.keyDown(window, { key: 'Escape' });
		await waitFor(() => expect(onClose).toHaveBeenCalled());
	});

	it('offers Start on a task that is up next, and says it is starting once pressed', async () => {
		const { client, state } = await setup('task-pm-04');
		const onStart = vi.fn();
		const { rerender } = render(<Harnessed state={state} client={client} taskId="task-pm-04" queue={[]} onStart={onStart} />);
		const start = screen.getByRole('button', { name: 'Start' });
		expect(screen.getByRole('region', { name: 'Start this task' })).toHaveTextContent('you approve it before any code is written');
		fireEvent.click(start);
		expect(onStart).toHaveBeenCalledWith(expect.objectContaining({ id: 'task-pm-04' }));
		const started = harnessReducer(state, { type: 'task.saved', task: { ...state.tasks['task-pm-04'], startRequested: true } });
		rerender(<Harnessed state={started} client={client} taskId="task-pm-04" queue={[]} onStart={onStart} />);
		expect(screen.getByRole('button', { name: 'Starting…' })).toBeDisabled();
		expect(screen.getByText('Starting soon.')).toBeInTheDocument();
	});

	it('offers Start again on a stopped task, whose spent start request no longer counts', async () => {
		const base = await setup('task-pm-02', 'run-pm-02');
		const state = harnessReducer(base.state, { type: 'task.saved', task: { ...base.state.tasks['task-pm-02'], state: 'cancelled', startRequested: true, note: 'Stopped by Anna: wrong approach.' } });
		const onStart = vi.fn();
		render(<Harnessed state={state} client={base.client} taskId="task-pm-02" queue={[]} onStart={onStart} />);
		const region = screen.getByRole('region', { name: 'Start this task' });
		expect(region).toHaveTextContent('Stopped. Stopped by Anna: wrong approach. Start again begins a new run');
		fireEvent.click(within(region).getByRole('button', { name: 'Start again' }));
		expect(onStart).toHaveBeenCalledWith(expect.objectContaining({ id: 'task-pm-02' }));
	});

	it('opens a run whole from Open details, and offers it only once there is a run', async () => {
		const { client, state } = await setup('task-pm-02', 'run-pm-02');
		const onOpenDetails = vi.fn();
		render(<RunDrawer state={state} client={client} taskId="task-pm-02" tab="stages" queue={[]} onTabChange={vi.fn()} onClose={vi.fn()} onOpenDetails={onOpenDetails} loadRun={vi.fn().mockResolvedValue(undefined)} onApprove={vi.fn()} onReject={vi.fn()} onPin={vi.fn()} onMove={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Open details' }));
		expect(onOpenDetails).toHaveBeenCalledWith('run-pm-02');
		cleanup();
		const unstarted = await setup('task-pm-04');
		render(<RunDrawer state={unstarted.state} client={unstarted.client} taskId="task-pm-04" tab="ticket" queue={[]} onTabChange={vi.fn()} onClose={vi.fn()} onOpenDetails={onOpenDetails} loadRun={vi.fn().mockResolvedValue(undefined)} onApprove={vi.fn()} onReject={vi.fn()} onPin={vi.fn()} onMove={vi.fn()} />);
		expect(screen.queryByRole('button', { name: 'Open details' })).not.toBeInTheDocument();
	});

	it('stops a running task from its panel, after a confirmation', async () => {
		const { client, state } = await setup('task-pm-02', 'run-pm-02');
		const onCancel = vi.fn().mockResolvedValue(undefined);
		render(<RunDrawer state={state} client={client} taskId="task-pm-02" tab="stages" queue={[]} onTabChange={vi.fn()} onClose={vi.fn()} onCancel={onCancel} loadRun={vi.fn().mockResolvedValue(undefined)} onApprove={vi.fn()} onReject={vi.fn()} onPin={vi.fn()} onMove={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Stop' }));
		fireEvent.click(within(screen.getByRole('dialog', { name: 'Stop PM-02?' })).getByRole('button', { name: 'Stop this task' }));
		await waitFor(() => expect(onCancel).toHaveBeenCalledWith(expect.objectContaining({ id: 'run-pm-02' })));
	});

	it('opens an unstarted task on its ticket, with what the tracker said about it', async () => {
		const { client, state } = await setup('task-pm-04');
		render(<Harnessed state={state} client={client} taskId="task-pm-04" queue={[]} initialTab="ticket" />);
		expect(screen.getByRole('tab', { name: 'Ticket' })).toHaveAttribute('aria-selected', 'true');
		const panel = screen.getByRole('tabpanel');
		expect(panel).toHaveTextContent(/Status/);
		expect(within(panel).getByRole('link', { name: /Open PM-04 in Jira/ })).toBeInTheDocument();
		expect(panel).toHaveTextContent('The sample data has no tracker behind it');
	});

	it('lets a task decide on its own whether an agent may stop and ask', async () => {
		const { client, state } = await setup('task-pm-04');
		const onSetQuestions = vi.fn();
		render(<RunDrawer state={state} client={client} taskId="task-pm-04" tab="ticket" queue={[]} onTabChange={vi.fn()} onClose={vi.fn()} loadRun={vi.fn().mockResolvedValue(undefined)} onApprove={vi.fn()} onReject={vi.fn()} onPin={vi.fn()} onMove={vi.fn()} onSetQuestions={onSetQuestions} askDefault={false} />);
		const select = screen.getByRole('combobox', { name: 'May an agent stop and ask on this task' });
		expect(select).toHaveValue('default');
		expect(screen.getByText('An agent decides alone and its open questions show on the plan approval.')).toBeInTheDocument();
		fireEvent.change(select, { target: { value: 'yes' } });
		expect(onSetQuestions).toHaveBeenCalledWith(expect.objectContaining({ id: 'task-pm-04' }), true);
		fireEvent.change(select, { target: { value: 'default' } });
		expect(onSetQuestions).toHaveBeenLastCalledWith(expect.objectContaining({ id: 'task-pm-04' }), null);
	});

	it('hides Start on a task that already has a run', async () => {
		const { client, state } = await setup('task-pm-02', 'run-pm-02');
		render(<Harnessed state={state} client={client} taskId="task-pm-02" queue={[]} onStart={vi.fn()} />);
		expect(screen.queryByRole('button', { name: 'Start' })).not.toBeInTheDocument();
	});
	it('widens from the grip on its left edge, as the item panel does, within its limits', async () => {
		const { client, state } = await setup('task-pm-02', 'run-pm-02');
		render(<Harnessed state={state} client={client} taskId="task-pm-02" queue={[]} />);
		const drawer = screen.getByRole('complementary');
		expect(drawer.style.getPropertyValue('--drawer-width')).toBe('560px');
		expect(screen.queryByRole('button', { name: 'Move up' })).not.toBeInTheDocument();
		const grip = screen.getByRole('button', { name: 'Resize details panel' });
		// jsdom's PointerEvent drops clientX, so the pointer is a MouseEvent.
		const pointer = (target: EventTarget, type: string, clientX: number) => act(() => { target.dispatchEvent(new MouseEvent(type, { clientX, bubbles: true })); });
		pointer(grip, 'pointerdown', 800);
		pointer(window, 'pointermove', 600);
		expect(drawer.style.getPropertyValue('--drawer-width')).toBe('760px');
		pointer(window, 'pointermove', 0);
		expect(drawer.style.getPropertyValue('--drawer-width')).toBe('1120px');
		pointer(window, 'pointerup', 0);
		pointer(window, 'pointermove', 900);
		expect(drawer.style.getPropertyValue('--drawer-width')).toBe('1120px');
	});
});

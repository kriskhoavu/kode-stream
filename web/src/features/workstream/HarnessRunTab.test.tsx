import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { HarnessRunTab } from './HarnessRunTab';
import { cardWork } from './harnessJoin';
import { createFixtureClient } from '../harness/fixture';
import { harnessReducer, initialHarnessState } from '../harness/harnessState';
import type { HarnessState } from '../harness/harnessState';
import type { HarnessController } from '../harness/useHarness';

afterEach(cleanup);

async function stateWith(runId: string) {
	const client = createFixtureClient({ tickMs: 0, arrivalMs: 0 });
	let state = harnessReducer(initialHarnessState, { type: 'snapshot', status: await client.status(), tasks: await client.tasks(), runs: await client.runs(), approvals: await client.approvals() });
	state = harnessReducer(state, { type: 'detail', detail: await client.run(runId) });
	return { client, state };
}

function controller(state: HarnessState, client: HarnessController['client']): HarnessController {
	return {
		state, client, dispatch: vi.fn(), refresh: vi.fn().mockResolvedValue(undefined), loadRun: vi.fn().mockResolvedValue(undefined),
		approve: vi.fn().mockResolvedValue(undefined), reject: vi.fn().mockResolvedValue(undefined), pin: vi.fn(), setPriority: vi.fn(), start: vi.fn(), setQuestions: vi.fn(),
		cancelRun: vi.fn(), pauseRole: vi.fn().mockResolvedValue(undefined), resumeRole: vi.fn(), createTask: vi.fn()
	};
}

describe('HarnessRunTab', () => {
	it('shows the stepper, who works, the findings and the ledger, and opens a conversation from a step', async () => {
		const { client, state } = await stateWith('run-pm-02');
		const harness = controller(state, client);
		const work = cardWork(state.tasks['tsk-PM-02'] ?? Object.values(state.tasks).find((task) => task.source.key === 'PM-02')!, state, false);
		const onOpenConversation = vi.fn();
		render(<HarnessRunTab work={work} harness={harness} onOpenConversation={onOpenConversation} onOpenFull={vi.fn()} onTakeOver={vi.fn()} />);
		expect(harness.loadRun).toHaveBeenCalledWith('run-pm-02');
		expect(screen.getByText('Working · Build · round 1 of 3')).toBeInTheDocument();
		expect(screen.getByRole('list', { name: 'Progress for PM-02' })).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'What the Checker said · plan, round 1' })).toBeInTheDocument();
		const ledger = screen.getByRole('region', { name: 'What each step took' });
		fireEvent.click(within(ledger).getAllByRole('button', { name: /^Round \d/ })[0]);
		expect(onOpenConversation).toHaveBeenCalledWith(expect.any(String));
		expect(screen.getByRole('button', { name: 'Open full width' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Add a note' })).toBeDisabled();
	});

	it('pauses the working role after this round, once confirmed', async () => {
		const { client, state } = await stateWith('run-pm-02');
		const harness = controller(state, client);
		const work = cardWork(Object.values(state.tasks).find((task) => task.source.key === 'PM-02')!, state, false);
		render(<HarnessRunTab work={work} harness={harness} onOpenConversation={vi.fn()} onTakeOver={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Pause after this round' }));
		expect(screen.getByRole('dialog', { name: 'Pause the Builder after this round?' })).toBeInTheDocument();
		fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Pause after this round' }));
		expect(harness.pauseRole).toHaveBeenCalledWith('coder');
	});

	it('hands the card over by hand', async () => {
		const { client, state } = await stateWith('run-pm-02');
		const harness = controller(state, client);
		const work = cardWork(Object.values(state.tasks).find((task) => task.source.key === 'PM-02')!, state, false);
		const onTakeOver = vi.fn();
		render(<HarnessRunTab work={work} harness={harness} onOpenConversation={vi.fn()} onTakeOver={onTakeOver} />);
		fireEvent.click(screen.getByRole('button', { name: 'Take over by hand' }));
		expect(onTakeOver).toHaveBeenCalled();
	});

	it('sends a note through the gate that waits, as a change asked on a plan', async () => {
		const client = createFixtureClient({ tickMs: 0, arrivalMs: 0 });
		const state = harnessReducer(initialHarnessState, { type: 'snapshot', status: await client.status(), tasks: await client.tasks(), runs: await client.runs(), approvals: await client.approvals() });
		const approval = Object.values(state.approvals).find((candidate) => candidate.state === 'requested' && candidate.gate === 'plan' && candidate.runId);
		expect(approval).toBeDefined();
		const detailed = harnessReducer(state, { type: 'detail', detail: await client.run(approval!.runId!) });
		const harness = controller(detailed, client);
		const work = cardWork(detailed.tasks[approval!.taskId], detailed, false);
		render(<HarnessRunTab work={work} harness={harness} onOpenConversation={vi.fn()} onTakeOver={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Add a note' }));
		fireEvent.change(screen.getByLabelText('What should change?'), { target: { value: 'Keep the cookie Secure.' } });
		fireEvent.click(screen.getByRole('button', { name: 'Send to Planner' }));
		expect(harness.reject).toHaveBeenCalledWith(expect.objectContaining({ id: approval!.id }), 'Keep the cookie Secure.');
	});
});

import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RunView } from './RunView';
import { createFixtureClient } from './fixture';
import { harnessReducer, initialHarnessState } from './harnessState';

afterEach(cleanup);

async function stateWith(runId: string) {
	const client = createFixtureClient({ tickMs: 0, arrivalMs: 0 });
	let state = harnessReducer(initialHarnessState, { type: 'snapshot', status: await client.status(), tasks: await client.tasks(), runs: await client.runs(), approvals: await client.approvals() });
	state = harnessReducer(state, { type: 'detail', detail: await client.run(runId) });
	return state;
}

describe('RunView', () => {
	it('shows the run header, stepper, ledger and what the Checker said', async () => {
		const state = await stateWith('run-pm-02');
		render(<RunView state={state} runId="run-pm-02" onSelectRun={vi.fn()} onRefresh={vi.fn()} loadRun={vi.fn().mockResolvedValue(undefined)} onApprove={vi.fn()} onReject={vi.fn()} />);
		expect(screen.getByRole('combobox', { name: 'Task' })).toHaveDisplayValue('PM-02 · Add login form · Working');
		expect(screen.getByText('Working · Build, round 1 of 3')).toBeInTheDocument();
		expect(screen.getByText(/So far: 5 agent runs/)).toBeInTheDocument();
		expect(screen.getByRole('list', { name: 'Progress for PM-02' })).toBeInTheDocument();
		expect(screen.getByRole('row', { name: /Total so far/ })).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'What the Checker said · plan, round 2' })).toBeInTheDocument();
		expect(screen.getByText('Round 1 asked for 1 must-fix item. Round 2 found them fixed.')).toBeInTheDocument();
		expect(screen.getByText('Suggestions · 4')).toBeInTheDocument();
		expect(screen.queryByText(/Must fix/)).not.toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'Jira PM-02' })).toBeInTheDocument();
	});

	it('folds a finished stage in the ledger and keeps the one in play open', async () => {
		const state = await stateWith('run-pm-02');
		render(<RunView state={state} runId="run-pm-02" onSelectRun={vi.fn()} onRefresh={vi.fn()} loadRun={vi.fn().mockResolvedValue(undefined)} onApprove={vi.fn()} onReject={vi.fn()} />);
		const plan = screen.getByRole('button', { name: 'Plan' });
		const build = screen.getByRole('button', { name: 'Build' });
		expect(plan).toHaveAttribute('aria-expanded', 'false');
		expect(build).toHaveAttribute('aria-expanded', 'true');
		expect(screen.queryByText('Round 1 · plan')).not.toBeInTheDocument();
		fireEvent.click(plan);
		expect(plan).toHaveAttribute('aria-expanded', 'true');
		expect(screen.getByText('Round 1 · plan')).toBeInTheDocument();
	});

	it('offers the conversations of a run that is no longer working', async () => {
		const state = await stateWith('run-srv-04');
		const onOpenTab = vi.fn();
		render(<RunView state={state} runId="run-srv-04" onSelectRun={vi.fn()} onRefresh={vi.fn()} loadRun={vi.fn().mockResolvedValue(undefined)} onApprove={vi.fn()} onReject={vi.fn()} onOpenTab={onOpenTab} />);
		fireEvent.click(screen.getByRole('button', { name: 'Read the conversations' }));
		expect(onOpenTab).toHaveBeenCalledWith('live');
	});

	it('opens the conversation of a ledger step from its row', async () => {
		const state = await stateWith('run-pm-02');
		const onOpenTab = vi.fn();
		render(<RunView state={state} runId="run-pm-02" onSelectRun={vi.fn()} onRefresh={vi.fn()} loadRun={vi.fn().mockResolvedValue(undefined)} onApprove={vi.fn()} onReject={vi.fn()} onOpenTab={onOpenTab} />);
		fireEvent.click(screen.getByRole('button', { name: 'Round 1 · build' }));
		const builder = state.details['run-pm-02'].agentRuns.find((agentRun) => agentRun.role === 'coder');
		expect(onOpenTab).toHaveBeenCalledWith('live', builder?.id);
	});

	it('stops a working run after a confirmation that names the agent it kills', async () => {
		const state = await stateWith('run-pm-02');
		const onCancel = vi.fn().mockResolvedValue(undefined);
		render(<RunView state={state} runId="run-pm-02" onSelectRun={vi.fn()} onRefresh={vi.fn()} loadRun={vi.fn().mockResolvedValue(undefined)} onApprove={vi.fn()} onReject={vi.fn()} onCancel={onCancel} />);
		fireEvent.click(screen.getByRole('button', { name: 'Stop' }));
		expect(screen.getByRole('dialog', { name: 'Stop PM-02?' })).toHaveTextContent(/The Builder is stopped mid-step and the run is cancelled. PM-02 stays in the Task Queue as Stopped/);
		fireEvent.click(screen.getByRole('button', { name: 'Stop this task' }));
		expect(onCancel).toHaveBeenCalledWith(expect.objectContaining({ id: 'run-pm-02' }));
	});

	it('puts the waiting gate above the stepper', async () => {
		const state = await stateWith('run-ux-11');
		render(<RunView state={state} runId="run-ux-11" onSelectRun={vi.fn()} onRefresh={vi.fn()} loadRun={vi.fn().mockResolvedValue(undefined)} onApprove={vi.fn()} onReject={vi.fn()} />);
		const gate = screen.getByRole('heading', { name: /approve the plan for UX-11/ });
		const stepper = screen.getByRole('list', { name: 'Progress for UX-11' });
		expect(gate.compareDocumentPosition(stepper) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});
});

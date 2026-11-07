import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RunPlanPanel, orderPlanFiles, planSourceLine } from './RunPlanPanel';
import { apiHarnessClient } from './harnessClient';
import { run } from './testData';
import type { HarnessRunPlan } from '../../lib/types';

afterEach(cleanup);

const plan: HarnessRunPlan = { branch: 'harness/PM-1-abc', from: 'worktree', files: [
	{ path: 'plans/PM-1/steps.md', status: 'added', content: 'Write the test first.' },
	{ path: 'plans/PM-1/README.md', status: 'modified', content: '# The plan\n\nThree steps.' },
	{ path: 'plans/PM-1/big.md', status: 'added', content: '', truncated: true }
] };

describe('RunPlanPanel', () => {
	it('reads the plan before it is merged, README first, and switches files', async () => {
		const runPlan = vi.fn().mockResolvedValue(plan);
		render(<RunPlanPanel client={{ ...apiHarnessClient, runPlan }} run={run('run-1', 'task-1')} />);
		expect(await screen.findByRole('heading', { name: 'The plan' })).toBeInTheDocument();
		expect(screen.getByText("Live from the harness's own copy. Not merged yet.")).toBeInTheDocument();
		expect(runPlan).toHaveBeenCalledWith('run-1', expect.anything());
		fireEvent.click(screen.getByRole('button', { name: /steps\.md/ }));
		expect(await screen.findByText('Write the test first.')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: /big\.md/ }));
		expect(screen.getByText('This file is too large to show here.')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
		await waitFor(() => expect(runPlan).toHaveBeenCalledTimes(2));
	});

	it('says when the Planner has written nothing, and when the copy and branch are gone', async () => {
		const { unmount } = render(<RunPlanPanel client={{ ...apiHarnessClient, runPlan: vi.fn().mockResolvedValue({ branch: 'harness/x', from: 'worktree', files: [] }) }} run={run('run-1', 'task-1')} />);
		expect(await screen.findByText('The Planner has not written a plan yet.')).toBeInTheDocument();
		unmount();
		render(<RunPlanPanel client={{ ...apiHarnessClient, runPlan: vi.fn().mockResolvedValue({ branch: 'harness/x', from: '', files: [] }) }} run={run('run-1', 'task-1')} />);
		expect(await screen.findByText(/copy and branch are gone/)).toBeInTheDocument();
	});

	it('orders README first and names the branch it read from', () => {
		expect(orderPlanFiles(plan.files).map((file) => file.path)).toEqual(['plans/PM-1/README.md', 'plans/PM-1/big.md', 'plans/PM-1/steps.md']);
		expect(planSourceLine({ ...plan, from: 'branch' })).toBe('From branch harness/PM-1-abc. Not merged yet.');
	});
});

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RunPlanPanel, commonFolder, orderPlanFiles, planSourceLine } from './RunPlanPanel';
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
		const list = screen.getByRole('list', { name: 'Plan files, in reading order' });
		fireEvent.click(within(list).getByRole('button', { name: /steps\.md/ }));
		expect(await screen.findByText('Write the test first.')).toBeInTheDocument();
		fireEvent.click(within(list).getByRole('button', { name: /big\.md/ }));
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

	it('lists a plan folder in reading order, says its folder once, and pages through it', async () => {
		const files = ['plan.yaml', 'automation/results/latest.md', 'solution/solution-01-backend.md', 'scenario/scenario-00-overview.md', 'automation/README.md', 'implementation-plan.md', 'README.md']
			.map((path) => ({ path: `plans/api/APP-562/${path}`, status: 'added' as const, content: `# ${path}` }));
		expect(orderPlanFiles(files).map((file) => file.path.slice('plans/api/APP-562/'.length))).toEqual(['README.md', 'implementation-plan.md', 'solution/solution-01-backend.md', 'scenario/scenario-00-overview.md', 'automation/README.md', 'automation/results/latest.md', 'plan.yaml']);
		expect(commonFolder(files.map((file) => file.path))).toBe('plans/api/APP-562/');
		render(<RunPlanPanel client={{ ...apiHarnessClient, runPlan: vi.fn().mockResolvedValue({ branch: 'harness/x', from: 'worktree', files }) }} run={run('run-1', 'task-1')} />);
		const list = await screen.findByRole('list', { name: 'Plan files, in reading order' });
		expect(screen.getByText('api/APP-562/')).toBeInTheDocument();
		expect(list.querySelectorAll('li')[2]).toHaveTextContent('3solution/solution-01-backend.mdA');
		const pager = screen.getByRole('navigation', { name: 'Read the plan in order' });
		expect(pager).toHaveTextContent('1 of 7');
		fireEvent.click(screen.getByRole('button', { name: /^Next/ }));
		expect(await screen.findByRole('heading', { name: 'implementation-plan.md' })).toBeInTheDocument();
		expect(pager).toHaveTextContent('2 of 7');
		fireEvent.click(screen.getByRole('button', { name: /^Previous/ }));
		expect(await screen.findByRole('heading', { name: 'README.md' })).toBeInTheDocument();
	});

	it('orders README first and names the branch it read from', () => {
		expect(orderPlanFiles(plan.files).map((file) => file.path)).toEqual(['plans/PM-1/README.md', 'plans/PM-1/big.md', 'plans/PM-1/steps.md']);
		expect(planSourceLine({ ...plan, from: 'branch' })).toBe('From branch harness/PM-1-abc. Not merged yet.');
	});
});

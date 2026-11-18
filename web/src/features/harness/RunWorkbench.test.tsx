import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { RunWorkbench, buildTree } from './RunWorkbench';
import { createFixtureClient } from './fixture';

afterEach(cleanup);

async function setup(runId: string) {
	const client = createFixtureClient({ tickMs: 0, arrivalMs: 0 });
	const detail = await client.run(runId);
	render(<RunWorkbench client={client} detail={detail} />);
	return { client, detail };
}

describe('buildTree', () => {
	it('merges a folder that holds only one folder into one row, as the item page does', () => {
		const tree = buildTree(['article/app/src/A.java', 'article/app/src/B.java', 'salesapi/x/C.java']);
		expect(tree.dirs.map((dir) => dir.name)).toEqual(['article/app/src', 'salesapi/x']);
		expect(tree.dirs[0].files).toEqual(['article/app/src/A.java', 'article/app/src/B.java']);
	});
});

describe('RunWorkbench', () => {
	it('shows Plan, Changes and Agents on the left and the task on the right', async () => {
		await setup('run-pm-02');
		const views = screen.getByRole('tablist', { name: 'Run view' });
		expect(within(views).getByRole('tab', { name: 'Plan' })).toHaveAttribute('aria-selected', 'true');
		expect(within(screen.getByRole('tablist', { name: 'Task panel' })).getByRole('tab', { name: /Info/ })).toHaveAttribute('aria-selected', 'true');
		fireEvent.click(within(views).getByRole('tab', { name: 'Agents' }));
		expect(await screen.findByRole('button', { name: /All steps/ })).toBeInTheDocument();
	});

	it('reads a whole section of conversations at once', async () => {
		await setup('run-pm-02');
		fireEvent.click(within(screen.getByRole('tablist', { name: 'Run view' })).getByRole('tab', { name: 'Agents' }));
		fireEvent.click(await screen.findByRole('button', { name: /^Plan.*read all/ }));
		expect(screen.getAllByRole('region', { name: /Planner, round 1/ }).length).toBeGreaterThan(0);
		expect(screen.getByRole('button', { name: 'Close Plan · all' })).toBeInTheDocument();
	});

	it('lists every review with its findings under Checker', async () => {
		await setup('run-pm-02');
		fireEvent.click(within(screen.getByRole('tablist', { name: 'Task panel' })).getByRole('tab', { name: /Checker/ }));
		expect(screen.getByRole('button', { name: /Plan check · round 2/ })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: /Plan check · round 1/ })).toBeInTheDocument();
	});
});

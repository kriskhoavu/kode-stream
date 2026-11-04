import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { HarnessPage } from './HarnessPage';
import { approval, run, stage, status, task } from '../features/harness/testData';

function json(body: unknown, statusCode = 200) {
	return Promise.resolve(new Response(JSON.stringify(body), { status: statusCode, headers: { 'content-type': 'application/json' } }));
}

function stubHarness(overrides: Record<string, unknown> = {}) {
	const routes: Record<string, unknown> = {
		'/api/harness/status': status({ waitingCount: 1, lastEventSeq: 3 }),
		'/api/harness/tasks': { tasks: [task('PM-02', { title: 'Add login form', needsYou: true, state: 'running', runId: 'run-1' }), task('PM-04', { title: 'Write conventions' })] },
		'/api/harness/runs': { runs: [run('run-1', 'task-PM-02', { currentStage: stage('run-1', 'plan_approval', { state: 'awaiting_approval' }) })] },
		'/api/harness/approvals': { approvals: [approval('ap-1', 'task-PM-02')] },
		...overrides
	};
	const fetchMock = vi.fn((path: string, init?: RequestInit) => {
		const route = path.split('?')[0];
		if (route === '/api/harness/cost') return json({ groupBy: 'role', from: '', to: '', totalUsd: 2.5, agentRuns: 3, groups: [] });
		if (route === '/api/harness/approvals/ap-1/decide' && init?.method === 'POST') return json(approval('ap-1', 'task-PM-02', { state: 'approved', channel: 'dashboard', revision: 2 }));
		return route in routes ? json(routes[route]) : json({ error: 'not found' }, 404);
	});
	vi.stubGlobal('fetch', fetchMock);
	return fetchMock;
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

beforeEach(() => {
	vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} unobserve() {} });
});

describe('HarnessPage', () => {
	it('opens on the Inbox, reports the waiting count and approves from the gate card', async () => {
		const fetchMock = stubHarness();
		const onWaitingCountChange = vi.fn();
		render(<HarnessPage location={{ view: 'inbox' }} onLocationChange={vi.fn()} onWaitingCountChange={onWaitingCountChange} />);
		expect(await screen.findByRole('heading', { name: /approve the plan for PM-02/ })).toBeInTheDocument();
		expect(screen.getByRole('status')).toHaveTextContent('1 item waiting on you');
		await waitFor(() => expect(onWaitingCountChange).toHaveBeenLastCalledWith(1));
		expect(screen.getByRole('button', { name: 'Inbox' })).toHaveAttribute('aria-current', 'page');

		fireEvent.click(screen.getByRole('button', { name: 'Approve' }));
		expect(await screen.findByText(/Approved by you/)).toBeInTheDocument();
		const decide = fetchMock.mock.calls.find(([path]) => path === '/api/harness/approvals/ap-1/decide');
		expect(JSON.parse(String(decide?.[1]?.body))).toMatchObject({ decision: 'approve', expectedRevision: 1 });
		await waitFor(() => expect(onWaitingCountChange).toHaveBeenLastCalledWith(0));
	});

	it('switches sections through the location', async () => {
		stubHarness();
		const onLocationChange = vi.fn();
		render(<HarnessPage location={{ view: 'queue' }} onLocationChange={onLocationChange} />);
		const wave = await screen.findByRole('region', { name: /^Wave 0,/ });
		expect(wave).toHaveTextContent('PM-02');
		expect(wave).toHaveTextContent('Waiting on you');
		expect(wave).toHaveTextContent('Up next');
		fireEvent.click(screen.getByRole('button', { name: 'Approvals' }));
		expect(onLocationChange).toHaveBeenCalledWith({ view: 'approvals' });
	});

	it('explains plainly when the storage cannot hold the harness', async () => {
		stubHarness({ '/api/harness/status': status({ storageSupported: false }) });
		render(<HarnessPage location={{ view: 'inbox' }} onLocationChange={vi.fn()} />);
		expect(await screen.findByRole('alert')).toHaveTextContent('Switch Storage to SQLite in Settings');
	});

	it('offers a retry when the harness cannot be reached', async () => {
		vi.stubGlobal('fetch', vi.fn(() => json({ error: 'Harness unavailable', code: 'unavailable', recoveryHint: 'Start Kode Stream in Local mode.' }, 503)));
		render(<HarnessPage location={{ view: 'inbox' }} onLocationChange={vi.fn()} />);
		expect(await screen.findByRole('alert')).toHaveTextContent('Harness unavailable Start Kode Stream in Local mode.');
		expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
	});

	it('opens the drawer from a queue card and the pipeline tab from the URL', async () => {
		stubHarness({ '/api/harness/runs/run-1': { run: run('run-1', 'task-PM-02'), task: task('PM-02', { title: 'Add login form', runId: 'run-1', state: 'running' }), stages: [], agentRuns: [], approvals: [] } });
		const onLocationChange = vi.fn();
		const { unmount } = render(<HarnessPage location={{ view: 'queue' }} onLocationChange={onLocationChange} />);
		fireEvent.click(await screen.findByRole('button', { name: /^PM-04, Write conventions/ }));
		expect(screen.getByRole('complementary', { name: 'Write conventions' })).toBeInTheDocument();
		fireEvent.click(screen.getAllByRole('button', { name: 'Close details' })[1]);
		expect(screen.queryByRole('complementary', { name: 'Write conventions' })).not.toBeInTheDocument();
		unmount();

		render(<HarnessPage location={{ view: 'run', runId: 'run-1', tab: 'links' }} onLocationChange={onLocationChange} />);
		expect(await screen.findByRole('complementary', { name: 'Add login form' })).toBeInTheDocument();
		fireEvent.click(screen.getByRole('tab', { name: 'Stages' }));
		expect(onLocationChange).toHaveBeenLastCalledWith({ view: 'run', runId: 'run-1', tab: 'stages' });
	});
});


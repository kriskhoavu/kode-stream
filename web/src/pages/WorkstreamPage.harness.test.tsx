import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { WorkstreamPage } from './WorkstreamPage';
import type { ItemSummary, WorkstreamBranchLoadResult } from '../lib/types';
import { approval, run, stage, status, task } from '../features/harness/testData';

const workspace = { id: 'ws', name: 'Sample', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '2026-10-01T00:00:00Z' };
const item: ItemSummary = { id: 'p1', workspaceId: 'ws', workspaceName: 'Sample', branch: 'main', scope: 'platform', identifier: 'PM-1', title: 'Built by the harness', status: 'in_progress', tags: [], metadataSource: 'plan.yaml', itemPath: 'plans/platform/PM-1' };
const quiet: ItemSummary = { ...item, id: 'p2', identifier: 'PM-2', title: 'Quiet card', itemPath: 'plans/platform/PM-2' };
const decision: ItemSummary = { ...item, id: 'p3', identifier: 'PM-3', title: 'Needs a decision', itemPath: 'plans/platform/PM-3' };

function branchResult(items: ItemSummary[]): WorkstreamBranchLoadResult {
	return { workspaceId: 'ws', branch: 'main', selectedBranch: 'main', branchRef: 'refs/heads/main', commit: '', currentCheckoutBranch: 'main', sourceMode: 'working_tree', mode: 'working_tree', editable: true, scannedAt: '2026-10-03T00:00:00Z', itemCount: items.length, warnings: [], items };
}

function response(body: unknown, ok = true, httpStatus = 200): Response {
	return { ok, status: httpStatus, json: async () => body } as Response;
}

const working = task('PM-1', { state: 'running', runId: 'run-1' });
const found = task('PM-9', { title: 'Found by the Finder' });
const stuck = task('PM-3', { needsYou: true, runId: 'run-3', title: 'Needs a decision' });

function stubBoard(options: { enabled?: boolean } = {}) {
	const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
		const url = String(input);
		if (url === '/api/workspaces/ws/workstream/checkout') return Promise.resolve(response(branchResult([item, quiet, decision])));
		if (url === '/api/harness/status') return Promise.resolve(response(status({ enabled: options.enabled ?? true })));
		if (url === '/api/harness/tasks') return Promise.resolve(response({ tasks: [working, found, stuck] }));
		if (url === '/api/harness/runs') return Promise.resolve(response({ runs: [
			run('run-1', working.id, { currentStage: stage('run-1', 'implement', { state: 'running', round: 2 }), activeAgentRun: { id: 'agent-1', runId: 'run-1', stageId: 'run-1-implement', role: 'coder', round: 2, engine: 'claudecode', model: 'claude-opus-5-5', state: 'running', turns: 3, durationMs: 1000, costUsd: 0.1, blocking: 0, advisory: 0 } }),
			run('run-3', stuck.id, { currentStage: stage('run-3', 'plan_approval', { state: 'awaiting_approval' }) })
		] }));
		if (url === '/api/harness/approvals') return Promise.resolve(response({ approvals: [approval('a-3', stuck.id, { runId: 'run-3', gate: 'plan', summary: 'Three steps, one file.' })] }));
		if (url === '/api/harness/tasks' && init?.method === 'POST') return Promise.resolve(response(task('PM-2', { workspaceId: 'ws', startRequested: true }), true, 201));
		if (url === '/api/harness/settings') return Promise.resolve(response({ revision: 1, enabled: true, autoStart: true, requirePrApproval: true, askQuestions: true, slots: {}, paused: {}, sources: [], modelPolicy: {} }));
		if (url === '/api/harness/approvals/a-3/decide' && init?.method === 'POST') return Promise.resolve(response({ ...approval('a-3', stuck.id, { runId: 'run-3', gate: 'plan' }), state: 'approved', decidedAt: '2026-11-04T10:00:00.000Z', channel: 'dashboard', revision: 2 }));
		if (url.startsWith('/api/ai/session-records')) return Promise.resolve(response([]));
		if (url.startsWith('/api/workspaces/ws/claude-transcripts')) return Promise.resolve(response([]));
		if (url === '/api/saved-filters') return Promise.resolve(response([]));
		if (url === '/api/workspaces/ws/git/branches') return Promise.resolve(response({ workspaceId: 'ws', current: 'main', branches: ['main'] }));
		return Promise.resolve(response({}));
	});
	vi.stubGlobal('fetch', fetchMock);
	return fetchMock;
}

function renderBoard() {
	return render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
}

// The board remembers its view in the browser's storage; each test starts on the status columns.
afterEach(() => { vi.unstubAllGlobals(); window.localStorage.clear(); });

describe('WorkstreamPage with the harness', () => {
	it('shows who works on a card, its stage and progress, and a card for a task no plan holds', async () => {
		stubBoard();
		renderBoard();
		const card = await screen.findByRole('group', { name: 'Harness: Working, Build · round 2 of 3' });
		expect(within(card).getByText('Claude · Opus 5.5')).toBeInTheDocument();
		expect(within(card).getByText('Build · round 2 of 3')).toBeInTheDocument();
		expect(within(card).getByRole('progressbar', { name: 'Run progress' })).toHaveAttribute('aria-valuenow', '42');
		expect(screen.getByText('Found by the Finder')).toBeInTheDocument();
		expect(screen.getByText('no plan yet')).toBeInTheDocument();
		expect(screen.getByText('By hand')).toBeInTheDocument();
		expect(screen.getByText('Needs a decision').closest('.plan-card')).toHaveClass('needs-you-card');
	});

	it('lists what needs you above the board and approves from there', async () => {
		const fetchMock = stubBoard();
		renderBoard();
		const strip = await screen.findByRole('region', { name: /Needs you/ });
		expect(within(strip).getByText('waits for you to approve the plan for it · Three steps, one file.')).toBeInTheDocument();
		expect(within(strip).getByRole('button', { name: 'Ask for changes' })).toBeInTheDocument();
		fireEvent.click(within(strip).getByRole('button', { name: 'Approve' }));
		await waitFor(() => expect(fetchMock.mock.calls.some(([url, init]) => String(url) === '/api/harness/approvals/a-3/decide' && init?.method === 'POST')).toBe(true));
		const body = JSON.parse(String(fetchMock.mock.calls.find(([url]) => String(url) === '/api/harness/approvals/a-3/decide')?.[1]?.body));
		expect(body.decision).toBe('approve');
	});

	it('asks for changes with a reason', async () => {
		const fetchMock = stubBoard();
		renderBoard();
		const strip = await screen.findByRole('region', { name: /Needs you/ });
		fireEvent.click(within(strip).getByRole('button', { name: 'Ask for changes' }));
		fireEvent.change(within(strip).getByLabelText('What should change?'), { target: { value: 'Split the second step.' } });
		fireEvent.click(within(strip).getByRole('button', { name: 'Send' }));
		await waitFor(() => expect(fetchMock.mock.calls.some(([url, init]) => String(url) === '/api/harness/approvals/a-3/decide' && init?.method === 'POST')).toBe(true));
		const body = JSON.parse(String(fetchMock.mock.calls.find(([url]) => String(url) === '/api/harness/approvals/a-3/decide')?.[1]?.body));
		expect(body).toMatchObject({ decision: 'reject', reason: 'Split the second step.' });
	});

	it('offers the Worked by facet and narrows the board with it', async () => {
		stubBoard();
		renderBoard();
		await screen.findByText('Claude · Opus 5.5');
		fireEvent.click(screen.getByRole('button', { name: /Worked by/ }));
		fireEvent.click(screen.getByRole('checkbox', { name: 'By hand' }));
		await waitFor(() => expect(screen.queryByText('Built by the harness')).not.toBeInTheDocument());
		expect(screen.getByText('Quiet card')).toBeInTheDocument();
	});

	it('keeps a quiet board when the harness is off', async () => {
		stubBoard({ enabled: false });
		renderBoard();
		await screen.findByText('Quiet card');
		expect(screen.queryByText('By hand')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /Worked by/ })).not.toBeInTheDocument();
		expect(screen.queryByRole('region', { name: /Needs you/ })).not.toBeInTheDocument();
	});
});

describe('WorkstreamPage board views', () => {
	class FakeResizeObserver { observe() {} disconnect() {} unobserve() {} }

	it('switches the board to the waves and to the queue, with the side panel and the Finder', async () => {
		vi.stubGlobal('ResizeObserver', FakeResizeObserver);
		const fetchMock = stubBoard();
		renderBoard();
		await screen.findByText('Claude · Opus 5.5');
		const switcher = screen.getByRole('navigation', { name: 'Board view' });
		fireEvent.click(within(switcher).getByRole('button', { name: 'Waves' }));
		expect(await screen.findByRole('region', { name: /Wave 0/ })).toBeInTheDocument();
		fireEvent.click(within(switcher).getByRole('button', { name: 'Queue' }));
		const queue = await screen.findByRole('list', { name: 'Queue in order' });
		expect(within(queue).getAllByRole('listitem')).toHaveLength(3);
		const side = screen.getByRole('complementary', { name: 'About the queue' });
		expect(within(side).getByRole('heading', { name: 'Why here' })).toBeInTheDocument();
		expect(within(side).getByRole('heading', { name: 'What starts next' })).toBeInTheDocument();
		const foundRow = within(queue).getAllByRole('listitem').find((row) => row.textContent?.includes('Found by the Finder'))!;
		fireEvent.click(within(foundRow).getByRole('button', { name: 'Start' }));
		await waitFor(() => expect(fetchMock.mock.calls.some(([url, init]) => String(url) === `/api/harness/tasks/${found.id}/start` && init?.method === 'POST')).toBe(true));
		fireEvent.click(within(side).getByRole('button', { name: 'Look now' }));
		await waitFor(() => expect(fetchMock.mock.calls.some(([url, init]) => String(url) === '/api/harness/scout/poll' && init?.method === 'POST')).toBe(true));
	});

	it('hides the view switcher while the harness is off', async () => {
		stubBoard({ enabled: false });
		renderBoard();
		await screen.findByText('Quiet card');
		expect(screen.queryByRole('navigation', { name: 'Board view' })).not.toBeInTheDocument();
	});
});

describe('WorkstreamPage runs a card with the harness', () => {
	it('offers Run with the harness on a card by hand and posts the ticket', async () => {
		const fetchMock = stubBoard();
		renderBoard();
		const quietCard = (await screen.findByText('Quiet card')).closest('.plan-card')!;
		fireEvent.click(within(quietCard as HTMLElement).getByRole('button', { name: 'Run with the harness' }));
		const dialog = await screen.findByRole('dialog', { name: 'Run PM-2 with the harness' });
		fireEvent.click(within(dialog).getByRole('button', { name: 'Run with the harness' }));
		await waitFor(() => expect(fetchMock.mock.calls.some(([url, init]) => String(url) === '/api/harness/tasks' && init?.method === 'POST')).toBe(true));
		const body = JSON.parse(String(fetchMock.mock.calls.find(([url, init]) => String(url) === '/api/harness/tasks' && init?.method === 'POST')?.[1]?.body));
		expect(body).toMatchObject({ workspaceId: 'ws', key: 'PM-2', title: 'Quiet card', startNow: true });
		expect(typeof body.idempotencyKey).toBe('string');
	});
});

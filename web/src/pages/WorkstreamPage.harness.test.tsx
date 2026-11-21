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
		if (url === `/api/harness/tasks/${found.id}/start` && init?.method === 'POST') return Promise.resolve(response({ ...found, startRequested: true, revision: 2 }));
		if (url === '/api/harness/scout/poll' && init?.method === 'POST') return Promise.resolve(response({ started: true, sources: [{ workspaceId: 'ws', kind: 'jira', found: 4, added: 1 }] }));
		if (url === '/api/harness/settings') return Promise.resolve(response({ revision: 1, enabled: true, autoStart: true, requireReview: true, askQuestions: true, slots: {}, paused: {}, sources: [], modelPolicy: {} }));
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

/** Tasks is the board's first view; the plans on the branch sit behind the other toggle. */
async function showPlans() {
	fireEvent.click(await screen.findByRole('button', { name: 'Plans' }));
}

function column(status: string): HTMLElement {
	return document.querySelector(`.workstream-column[data-status="${status}"]`) as HTMLElement;
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
		expect(card.closest('.plan-card')).toHaveClass('working-card');
		expect(screen.getByText('Found by the Finder').closest('.plan-card')).not.toHaveClass('working-card');
		expect(screen.getByText('Found by the Finder')).toBeInTheDocument();
		expect(screen.getByText('no plan yet')).toBeInTheDocument();
		expect(screen.getByText('Needs a decision').closest('.plan-card')).toHaveClass('needs-you-card');
		// The Tasks board holds tasks only: the plan nobody handed to the harness is not on it.
		expect(screen.queryByText('Quiet card')).not.toBeInTheDocument();
		expect(screen.queryByText('By hand')).not.toBeInTheDocument();
		await showPlans();
		expect(await screen.findByText('By hand')).toBeInTheDocument();
		expect(screen.queryByText('Found by the Finder')).not.toBeInTheDocument();
	});

	it('puts each task in the column its run has reached, and moves none of them by hand but a start', async () => {
		stubBoard();
		renderBoard();
		await screen.findByText('Found by the Finder');
		expect(within(column('unsorted')).getByRole('heading', { name: 'To do' })).toBeInTheDocument();
		expect(within(column('unsorted')).getByText('Found by the Finder')).toBeInTheDocument();
		// Started, its plan waiting for approval: Draft. Its plan approved and being built: In Progress.
		expect(within(column('draft')).getByText('Needs a decision')).toBeInTheDocument();
		expect(within(column('in_progress')).getByText('Built by the harness')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Move item status' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: '+ New item' })).not.toBeInTheDocument();
		expect(screen.queryByTitle('Configure source items')).not.toBeInTheDocument();
		// Only the task nobody started may be dragged, and only to start it.
		expect([...document.querySelectorAll('.plan-card.draggable')].map((card) => card.textContent)).toEqual([expect.stringContaining('Found by the Finder')]);
		await showPlans();
		// On the Plans board a plan keeps the status a person gave it.
		expect(within(column('in_progress')).getByText('Needs a decision')).toBeInTheDocument();
		expect(screen.getAllByRole('button', { name: 'Move item status' }).length).toBeGreaterThan(0);
	});

	it('starts a task dragged from To do to Draft, and from nowhere else', async () => {
		const fetchMock = stubBoard();
		renderBoard();
		const card = (await screen.findByText('Found by the Finder')).closest('.plan-card') as HTMLElement;
		const transfer = { data: '', effectAllowed: '', dropEffect: '', setData(_: string, value: string) { this.data = value; }, getData() { return this.data; } };
		const starts = () => fetchMock.mock.calls.filter(([url]) => String(url).endsWith('/start'));
		fireEvent.dragStart(card, { dataTransfer: transfer });
		fireEvent.dragOver(column('in_progress'), { dataTransfer: transfer });
		expect(column('in_progress')).not.toHaveClass('drop-target');
		fireEvent.drop(column('in_progress'), { dataTransfer: transfer });
		expect(starts()).toHaveLength(0);
		fireEvent.dragOver(column('draft'), { dataTransfer: transfer });
		expect(column('draft')).toHaveClass('drop-target');
		expect(within(column('draft')).getByRole('heading', { name: 'Draft' })).toBeInTheDocument();
		fireEvent.drop(column('draft'), { dataTransfer: transfer });
		fireEvent.dragEnd(card);
		await waitFor(() => expect(starts()).toHaveLength(1));
		expect(JSON.parse(String(starts()[0][1]?.body))).toMatchObject({ expectedRevision: 1 });
	});

	it('opens the workspace AI and harness settings beside To do, as Plans opens its source items', async () => {
		stubBoard();
		renderBoard();
		await screen.findByText('Found by the Finder');
		expect(screen.queryByTitle('Configure source items')).not.toBeInTheDocument();
		// The configure column takes a narrow track of its own, so the five status columns stay in one row.
		expect((document.querySelector('.task-board') as HTMLElement).style.gridTemplateColumns.split(' 44px')).toHaveLength(2);
		fireEvent.click(screen.getByTitle('Configure AI and harness'));
		const dialog = screen.getByRole('dialog', { name: 'AI and harness' });
		expect(within(dialog).getByText('Sample')).toBeInTheDocument();
		fireEvent.click(within(dialog).getByRole('button', { name: 'Close AI and harness' }));
		expect(screen.queryByRole('dialog', { name: 'AI and harness' })).not.toBeInTheDocument();
		await showPlans();
		expect(screen.queryByTitle('Configure AI and harness')).not.toBeInTheDocument();
		expect(await screen.findByTitle('Configure source items')).toBeInTheDocument();
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
		expect(screen.queryByRole('button', { name: /Worked by/ })).not.toBeInTheDocument();
		await showPlans();
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

	it('shows Tasks first, beside Plans, each as a Board or a List', async () => {
		stubBoard();
		renderBoard();
		await screen.findByText('Claude · Opus 5.5');
		const kinds = screen.getByRole('group', { name: 'Show on the board' });
		expect(within(kinds).getAllByRole('button').map((button) => button.textContent?.trim())).toEqual(['Tasks', 'Plans']);
		expect(within(kinds).getByRole('button', { name: 'Tasks' })).toHaveAttribute('aria-pressed', 'true');
		const layouts = screen.getByRole('group', { name: 'Board view' });
		expect(within(layouts).getAllByRole('button').map((button) => button.getAttribute('aria-label'))).toEqual(['Board', 'List']);
		expect(within(layouts).getByRole('button', { name: 'List' })).toHaveTextContent('');
		// The view switcher sits beside Save view, not in the title row.
		expect(layouts.closest('.saved-filter-bar')).not.toBeNull();
		expect(within(layouts).getByRole('button', { name: 'Board' })).toHaveAttribute('aria-pressed', 'true');
	});

	it('lists the plans in rows by status, and moves one from its row', async () => {
		const fetchMock = stubBoard();
		renderBoard();
		await showPlans();
		fireEvent.click(within(screen.getByRole('group', { name: 'Board view' })).getByRole('button', { name: 'List' }));
		const group = await screen.findByRole('list', { name: 'In Progress' });
		expect(within(group).getAllByRole('listitem').map((row) => row.getAttribute('data-item-id')).filter(Boolean).sort()).toEqual(['p1', 'p2', 'p3']);
		expect(document.querySelector('.workstream-board')).toBeNull();
		const quietRow = within(group).getAllByRole('listitem').find((row) => row.textContent?.includes('Quiet card'))!;
		expect(within(quietRow).getByText('By hand')).toBeInTheDocument();
		fireEvent.click(within(quietRow).getByRole('button', { name: 'Move Quiet card' }));
		fireEvent.click(screen.getByRole('menuitemradio', { name: 'Review' }));
		await waitFor(() => expect(fetchMock.mock.calls.some(([url, init]) => String(url).includes('/api/items/p2') && init?.method && init.method !== 'GET')).toBe(true));
		// Each toggle keeps its own view: Tasks is still a board.
		fireEvent.click(screen.getByRole('button', { name: 'Tasks' }));
		expect(await screen.findByText('Found by the Finder')).toBeInTheDocument();
		expect(within(screen.getByRole('group', { name: 'Board view' })).getByRole('button', { name: 'Board' })).toHaveAttribute('aria-pressed', 'true');
	});

	it('lists the tasks as the Task Queue, with the side panel and the Finder', async () => {
		vi.stubGlobal('ResizeObserver', FakeResizeObserver);
		const fetchMock = stubBoard();
		renderBoard();
		await screen.findByText('Claude · Opus 5.5');
		fireEvent.click(within(screen.getByRole('group', { name: 'Board view' })).getByRole('button', { name: 'List' }));
		const queue = await screen.findByRole('list', { name: 'Task Queue in order' });
		expect(within(queue).getAllByRole('listitem')).toHaveLength(3);
		const side = screen.getByRole('complementary', { name: 'About the Task Queue' });
		expect(within(side).getByRole('heading', { name: 'Why here' })).toBeInTheDocument();
		expect(within(side).getByRole('heading', { name: 'What starts next' })).toBeInTheDocument();
		const foundRow = within(queue).getAllByRole('listitem').find((row) => row.textContent?.includes('Found by the Finder'))!;
		fireEvent.click(within(foundRow).getByRole('button', { name: 'Start' }));
		await waitFor(() => expect(fetchMock.mock.calls.some(([url, init]) => String(url) === `/api/harness/tasks/${found.id}/start` && init?.method === 'POST')).toBe(true));
		fireEvent.click(within(side).getByRole('button', { name: 'Look now' }));
		await waitFor(() => expect(fetchMock.mock.calls.some(([url, init]) => String(url) === '/api/harness/scout/poll' && init?.method === 'POST')).toBe(true));
	});

	it('shows only the plans while the harness is off, still as a Board or a List', async () => {
		stubBoard({ enabled: false });
		renderBoard();
		await screen.findByText('Quiet card');
		expect(screen.queryByRole('group', { name: 'Show on the board' })).not.toBeInTheDocument();
		expect(screen.getByRole('group', { name: 'Board view' })).toBeInTheDocument();
	});
});

describe('WorkstreamPage runs a card with the harness', () => {
	it('offers Run with the harness on a card by hand and posts the ticket', async () => {
		const fetchMock = stubBoard();
		renderBoard();
		await showPlans();
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

describe('WorkstreamPage ticket cards', () => {
	it('opens a ticket card on its plan tab, and labels the stop with its status', async () => {
		stubBoard();
		renderBoard();
		const ticket = (await screen.findByText('Found by the Finder')).closest('.plan-card') as HTMLElement;
		expect(ticket).toHaveClass('ticket-card');
		expect(within(ticket).getByText('Jira ticket')).toBeInTheDocument();
		// A task a plan holds keeps the plan card's look on the Tasks board.
		const held = screen.getByText('Built by the harness').closest('.plan-card') as HTMLElement;
		expect(held).toHaveClass('plan-kind-card');
		expect(within(held).getByText('Plan')).toBeInTheDocument();
		expect(within(ticket).getByText('no plan yet')).toBeInTheDocument();
		expect(screen.getByText(/of 3 tasks/)).toBeInTheDocument();
		fireEvent.click(ticket);
		expect(await screen.findByRole('tab', { name: 'Ticket', selected: true })).toBeInTheDocument();
		expect(screen.getByRole('tab', { name: 'Plan' })).toBeInTheDocument();
	});

	it('asks Jira at once when Refresh is pressed on Tasks, and not on Plans', async () => {
		const fetchMock = stubBoard();
		renderBoard();
		await screen.findByText('Found by the Finder');
		const polls = () => fetchMock.mock.calls.filter(([url]) => String(url) === '/api/harness/scout/poll');
		fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
		expect(await screen.findByText('Jira checked: 4 found, 1 new')).toBeInTheDocument();
		expect(polls()).toHaveLength(1);
		expect(JSON.parse(String(polls()[0][1]?.body))).toEqual({ workspaceId: 'ws' });
		await showPlans();
		fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
		expect(await screen.findByText('3 items indexed')).toBeInTheDocument();
		expect(polls()).toHaveLength(1);
	});
});

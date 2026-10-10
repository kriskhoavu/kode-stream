import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { WorkstreamPage } from './WorkstreamPage';
import { readStringPreference } from '../shared/preferences/store';
import type { ClaudeTranscript, ItemSummary, SafeSessionRecord, WorkstreamBranchLoadResult } from '../lib/types';
import { embeddedSessionAttachEvent, notifyEmbeddedSessionEnded } from '../features/ai-session/terminalSessions';

vi.mock('../features/ai-session/SessionTerminal', () => ({
	SessionTerminal: ({ record, visible }: { record: SafeSessionRecord; visible: boolean }) => <section data-testid={`terminal-${record.id}`} hidden={!visible}><textarea aria-label="Agent terminal input" /></section>
}));

const workspace = { id: 'r1', name: 'Sample', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '2026-10-01T00:00:00Z' };
const item: ItemSummary = { id: 'p1', workspaceId: 'r1', workspaceName: 'Sample', branch: 'main', scope: 'platform', identifier: 'PM-043', title: 'Card agents', status: 'in_progress', tags: [], metadataSource: 'plan.yaml', itemPath: 'plans/platform/PM-043' };
const quiet: ItemSummary = { ...item, id: 'p2', identifier: 'PM-044', title: 'Quiet card', itemPath: 'plans/platform/PM-044' };

function record(id: string, provider: string, live: boolean, itemId = 'p1'): SafeSessionRecord {
	return { id, workspaceId: 'r1', planRef: { itemId, itemPath: 'plans/platform/PM-043', branchKey: 'main' }, provider, intent: 'card_context', requestedBranch: 'main', state: live ? 'running' : 'exited', startedAt: '2026-10-03T08:00:00Z', lastKnownAt: '2026-10-03T08:00:00Z', live };
}

function stubBoard(records: SafeSessionRecord[], transcripts: ClaudeTranscript[] = []) {
	const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
		const url = String(input);
		if (url === '/api/workspaces/r1/workstream/checkout') return Promise.resolve(response(branchResult([item, quiet])));
		if (url.startsWith('/api/ai/session-records')) return Promise.resolve(response(records));
		if (url.startsWith('/api/workspaces/r1/claude-transcripts')) return Promise.resolve(response(transcripts));
		if (url === '/api/ai/sessions/live-1' && !init?.method) return Promise.resolve(response({ id: 'live-1', itemId: 'p1', workspaceId: 'r1', provider: 'claude', intent: 'card_context', state: 'running', startedAt: '' }));
		if (url === '/api/ai/sessions/live-1/grant') return Promise.resolve(response({ sessionId: 'live-1', token: 'fresh', expiresAt: '' }));
		if (url.includes('/files')) return Promise.resolve(response([]));
		if (url === '/api/saved-filters') return Promise.resolve(response([]));
		if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main'] }));
		return Promise.resolve(response({}));
	});
	vi.stubGlobal('fetch', fetchMock);
	return fetchMock;
}

function renderBoard() {
	return render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
}

afterEach(() => { vi.unstubAllGlobals(); });

describe('WorkstreamPage card agents', () => {
	it('shows a chip per live session, +N past three, a hollow chip for an active terminal conversation, and nothing for ended ones', async () => {
		const active: ClaudeTranscript = { sessionId: '11111111-1111-4111-8111-111111111111', startedAt: new Date().toISOString(), lastActiveAt: new Date().toISOString(), messages: 4, itemIds: ['p1'] };
		stubBoard([record('live-1', 'claude', true), record('live-2', 'codex', true), record('live-3', 'copilot', true), record('live-4', 'opencode', true), record('ended-1', 'codex', false)], [active]);
		renderBoard();
		const cluster = await screen.findByRole('group', { name: 'Agents on Card agents' });
		expect(within(cluster).getAllByRole('button', { name: /Open the running/ })).toHaveLength(3);
		expect(within(cluster).getByRole('button', { name: 'Show all 5 agents' })).toHaveTextContent('+2');
		expect(screen.getByRole('button', { name: 'Start an agent session for Quiet card' })).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Start an agent session for Card agents' })).not.toBeInTheDocument();
	});

	it('opens a live session in the drawer from its chip without previewing the card first', async () => {
		stubBoard([record('live-1', 'claude', true)]);
		renderBoard();
		fireEvent.click(await screen.findByRole('button', { name: 'Open the running Claude session' }));
		const drawer = await screen.findByLabelText('Item preview');
		expect(within(drawer).getByRole('button', { name: /Agents/ })).toHaveClass('active');
		expect(within(drawer).getByRole('button', { name: 'Back to document' })).toBeInTheDocument();
		expect(within(drawer).getByTestId('terminal-live-1')).toBeVisible();
		expect(within(drawer).queryByText('Document')).not.toBeInTheDocument();
		fireEvent.click(within(drawer).getByRole('button', { name: 'Back to document' }));
		expect(within(drawer).getByText('Document')).toBeInTheDocument();
	});

	it('drops a session from the card as soon as it is reported ended, even while the server still lists it live', async () => {
		stubBoard([record('live-1', 'claude', true)]);
		renderBoard();
		await screen.findByRole('button', { name: 'Open the running Claude session' });
		act(() => { notifyEmbeddedSessionEnded('live-1'); });
		expect(screen.queryByRole('button', { name: 'Open the running Claude session' })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Start an agent session for Card agents' })).toBeInTheDocument();
		await act(async () => { await new Promise((resolve) => setTimeout(resolve, 50)); });
		expect(screen.queryByRole('button', { name: 'Open the running Claude session' })).not.toBeInTheDocument();
	});

	it('keeps a live session connected after Back to document, and hands it to the dock when the drawer closes', async () => {
		stubBoard([record('live-1', 'claude', true)]);
		const attached = vi.fn();
		window.addEventListener(embeddedSessionAttachEvent, attached);
		renderBoard();
		fireEvent.click(await screen.findByRole('button', { name: 'Open the running Claude session' }));
		const drawer = await screen.findByLabelText('Item preview');
		fireEvent.click(within(drawer).getByRole('button', { name: 'Back to document' }));
		expect(within(drawer).getByText('Document')).toBeInTheDocument();
		expect(within(drawer).getByTestId('terminal-live-1')).not.toBeVisible();
		expect(attached).not.toHaveBeenCalled();
		fireEvent.click(within(drawer).getAllByRole('button', { name: 'Close preview' })[0]);
		await waitFor(() => expect(attached).toHaveBeenCalledTimes(1));
		window.removeEventListener(embeddedSessionAttachEvent, attached);
		await act(async () => undefined);
	});

	it('collapses the Work Item panel to a rail, remembers it, and reopens on the tab picked from the rail', async () => {
		stubBoard([record('live-1', 'claude', true)]);
		renderBoard();
		fireEvent.click(await screen.findByRole('button', { name: 'Open the running Claude session' }));
		const drawer = await screen.findByLabelText('Item preview');
		fireEvent.click(within(drawer).getByRole('button', { name: 'Collapse work item panel' }));
		expect(within(drawer).queryByRole('tablist', { name: 'Work item side panel' })).not.toBeInTheDocument();
		expect(drawer.querySelector('.plan-drawer-body')).toHaveClass('work-item-collapsed');
		expect(within(drawer).getByTestId('terminal-live-1')).toBeVisible();
		expect(readStringPreference('workstream.drawer.workItemCollapsed')).toBe('1');
		fireEvent.click(within(drawer).getByRole('button', { name: 'Open Git' }));
		expect(within(drawer).getByRole('tablist', { name: 'Work item side panel' })).toBeInTheDocument();
		expect(within(drawer).getByRole('button', { name: /^Git/ })).toHaveClass('active');
		expect(readStringPreference('workstream.drawer.workItemCollapsed')).toBe('0');
	});

	// The app bumps refreshKey whenever the window regains focus. Treating that
	// as a new board swapped every card for a skeleton and back: a flicker.
	it('keeps the cards and an open drawer on screen when the app refreshes', async () => {
		const fetchMock = stubBoard([record('live-1', 'claude', true)]);
		const view = renderBoard();
		fireEvent.click(await screen.findByRole('button', { name: 'Open the running Claude session' }));
		await screen.findByLabelText('Item preview');
		const boardLoads = () => fetchMock.mock.calls.filter(([url]) => String(url) === '/api/workspaces/r1/workstream/checkout').length;
		const before = boardLoads();
		view.rerender(<WorkstreamPage workspace={workspace} refreshKey={1} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
		expect(document.querySelector('.plan-card.skeleton')).toBeNull();
		expect(screen.getByRole('button', { name: 'Start an agent session for Quiet card' })).toBeInTheDocument();
		expect(screen.getByLabelText('Item preview')).toBeInTheDocument();
		await waitFor(() => expect(boardLoads()).toBe(before + 1));
		expect(document.querySelector('.plan-card.skeleton')).toBeNull();
		expect(within(screen.getByLabelText('Item preview')).getByTestId('terminal-live-1')).toBeVisible();
	});

	it('opens the launcher on a card with no live session', async () => {
		stubBoard([]);
		renderBoard();
		fireEvent.click(await screen.findByRole('button', { name: 'Start an agent session for Quiet card' }));
		const drawer = await screen.findByLabelText('Item preview');
		expect(within(drawer).getByRole('button', { name: /New session|Open AI session/ })).toBeInTheDocument();
		expect(within(drawer).getByText(/Start a session from the Agents tab/)).toBeInTheDocument();
	});

	it('hands a live session to the terminal dock when the drawer closes, and keeps Escape typed into the terminal', async () => {
		stubBoard([record('live-1', 'claude', true)]);
		const attached = vi.fn();
		window.addEventListener(embeddedSessionAttachEvent, attached);
		renderBoard();
		fireEvent.click(await screen.findByRole('button', { name: 'Open the running Claude session' }));
		const drawer = await screen.findByLabelText('Item preview');
		fireEvent.keyDown(within(drawer).getByLabelText('Agent terminal input'), { key: 'Escape' });
		expect(screen.getByLabelText('Item preview')).toBeInTheDocument();
		fireEvent.click(within(drawer).getAllByRole('button', { name: 'Close preview' })[0]);
		await waitFor(() => expect(attached).toHaveBeenCalledTimes(1));
		expect((attached.mock.calls[0][0] as CustomEvent).detail.grant.token).toBe('fresh');
		await waitFor(() => expect(screen.queryByLabelText('Item preview')).not.toBeInTheDocument());
		window.removeEventListener(embeddedSessionAttachEvent, attached);
		await act(async () => undefined);
	});
});

function branchResult(items: ItemSummary[]): WorkstreamBranchLoadResult {
	return { workspaceId: 'r1', branch: 'main', selectedBranch: 'main', branchRef: 'refs/heads/main', commit: '', currentCheckoutBranch: 'main', sourceMode: 'working_tree', mode: 'working_tree', editable: true, scannedAt: '2026-10-03T00:00:00Z', itemCount: items.length, warnings: [], items };
}

function response(body: unknown, ok = true, status = 200): Response {
	return { ok, status, json: async () => body } as Response;
}

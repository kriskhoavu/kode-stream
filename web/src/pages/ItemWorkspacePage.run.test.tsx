import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ItemWorkspacePage } from './ItemWorkspacePage';
import { run, stage, status, task } from '../features/harness/testData';

vi.mock('./WorkstreamExplorer', () => ({
	WorkstreamExplorer: ({ rightPanel }: { rightPanel?: { content?: React.ReactNode } }) => <div data-testid="embedded-explorer">{rightPanel?.content}</div>
}));

const workspaces = [{ id: 'ws-1', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '2026-07-10T00:00:00Z' }];
const working = task('PM-012', { workspaceId: 'ws-1', state: 'running', runId: 'run-1' });
const building = stage('run-1', 'implement', { state: 'running', round: 1, ordinal: 2, startedAt: '2026-11-04T08:10:00.000Z' });
const theRun = run('run-1', working.id, { workspaceId: 'ws-1', currentStageId: building.id, currentStage: building });

function stub() {
	vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
		const url = String(input);
		if (url === '/api/items/item-1') return Promise.resolve(response({ id: 'item-1', workspaceId: 'ws-1', workspaceName: 'Workspace', scope: 'platform', branch: 'main', identifier: 'PM-012', title: 'Drag cards', status: 'in_progress', tags: [], metadataSource: 'plan.yaml', itemPath: 'plans/platform/PM-012', counts: { files: 1 }, warnings: [] }));
		if (url === '/api/items/item-1/files') return Promise.resolve(response([{ id: 'readme', name: 'README.md', path: 'README.md', type: 'file', editable: true, kind: 'markdown' }]));
		if (url === '/api/items/item-1/files/readme') return Promise.resolve(response({ id: 'readme', path: 'README.md', content: '# Drag cards', hash: 'hash', kind: 'markdown', sizeBytes: 12, editable: true, truncated: false }));
		if (url === '/api/items/item-1/diff') return Promise.resolve(response({ diff: '' }));
		if (url === '/api/workspaces/ws-1/git/status') return Promise.resolve(response({ workspaceId: 'ws-1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
		if (url === '/api/workspaces/ws-1/git/branches') return Promise.resolve(response({ workspaceId: 'ws-1', current: 'main', branches: ['main'] }));
		if (url === '/api/workspaces') return Promise.resolve(response(workspaces));
		if (url === '/api/harness/status') return Promise.resolve(response(status()));
		if (url === '/api/harness/tasks') return Promise.resolve(response({ tasks: [working] }));
		if (url === '/api/harness/runs') return Promise.resolve(response({ runs: [theRun] }));
		if (url === '/api/harness/approvals') return Promise.resolve(response({ approvals: [] }));
		if (url === '/api/harness/runs/run-1') return Promise.resolve(response({ run: theRun, task: working, stages: [building], agentRuns: [], approvals: [] }));
		if (url.startsWith('/api/ai/session-records')) return Promise.resolve(response([]));
		return Promise.resolve(response({}));
	}));
}

function response(payload: unknown) {
	return { ok: true, json: async () => payload };
}

afterEach(() => { vi.unstubAllGlobals(); });

describe('ItemWorkspacePage with a harness run', () => {
	it('offers the Harness run mode on the Agents tab', async () => {
		stub();
		render(<ItemWorkspacePage itemId="item-1" refreshKey={0} workspaces={workspaces} onBack={vi.fn()} onViewChange={vi.fn()} />);
		fireEvent.click(await screen.findByRole('button', { name: /Agents/ }));
		expect(await screen.findByRole('tab', { name: 'Harness run' })).toHaveAttribute('aria-selected', 'true');
		expect(await screen.findByText('Working · Build · round 1 of 3')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Open full width' })).toBeInTheDocument();
	});

	it('shows the run whole, full width, with Stop in its bar, on ?view=run', async () => {
		stub();
		const onViewChange = vi.fn();
		render(<ItemWorkspacePage itemId="item-1" view="run" refreshKey={0} workspaces={workspaces} onBack={vi.fn()} onViewChange={onViewChange} />);
		expect(await screen.findByRole('region', { name: 'Harness run, full width' })).toBeInTheDocument();
		await waitFor(() => expect(screen.getByRole('button', { name: 'Stop' })).toBeInTheDocument());
		expect(screen.getByRole('list', { name: 'Progress for PM-012' })).toBeInTheDocument();
		expect(screen.queryByTestId('embedded-explorer')).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Back to the card' }));
		expect(onViewChange).toHaveBeenCalledWith(undefined);
	});
});

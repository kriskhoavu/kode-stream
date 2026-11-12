import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { HarnessRunRedirect } from './HarnessRunRedirect';
import { run, task } from '../harness/testData';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

const theTask = task('PM-7', { workspaceId: 'ws', runId: 'run-7' });
const detail = { run: run('run-7', theTask.id, { workspaceId: 'ws' }), task: theTask, stages: [], agentRuns: [], approvals: [] };
const checkout = (identifiers: string[]) => ({ workspaceId: 'ws', branch: 'main', selectedBranch: 'main', branchRef: 'refs/heads/main', commit: '', currentCheckoutBranch: 'main', sourceMode: 'working_tree', mode: 'working_tree', editable: true, scannedAt: '', itemCount: identifiers.length, warnings: [], items: identifiers.map((identifier) => ({ id: `item-${identifier}`, workspaceId: 'ws', workspaceName: 'Sample', branch: 'main', scope: 'platform', identifier, title: identifier, status: 'draft', tags: [], metadataSource: 'plan.yaml' })) });

function stub(items: string[], runOk = true) {
	vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
		const url = String(input);
		if (url === '/api/harness/runs/run-7') return Promise.resolve({ ok: runOk, status: runOk ? 200 : 404, json: async () => (runOk ? detail : { error: 'no such run', code: 'not_found' }) });
		if (url === '/api/workspaces/ws/workstream/checkout') return Promise.resolve({ ok: true, json: async () => checkout(items) });
		return Promise.resolve({ ok: false, status: 404, json: async () => ({}) });
	}));
}

describe('HarnessRunRedirect', () => {
	it('opens the card that holds the run, by its Jira key whatever the case', async () => {
		stub(['PM-1', 'pm-7']);
		const onResolved = vi.fn();
		const onMissing = vi.fn();
		render(<HarnessRunRedirect runId="run-7" onResolved={onResolved} onMissing={onMissing} onBack={vi.fn()} />);
		await waitFor(() => expect(onResolved).toHaveBeenCalledWith('item-pm-7'));
		expect(onMissing).not.toHaveBeenCalled();
	});

	it('shows a Jira ticket\'s run whole when no card holds it, with a way back to the board', async () => {
		stub(['PM-1']);
		const onMissing = vi.fn();
		const onBack = vi.fn();
		render(<HarnessRunRedirect runId="run-7" onResolved={vi.fn()} onMissing={onMissing} onBack={onBack} />);
		expect(await screen.findByRole('region', { name: 'Harness run, full width' })).toHaveTextContent('Jira ticket/PM-7');
		fireEvent.click(screen.getByRole('button', { name: 'Back to the board' }));
		expect(onBack).toHaveBeenCalled();
		expect(onMissing).not.toHaveBeenCalled();
	});

	it('sends an unknown run to the board', async () => {
		stub([], false);
		const onMissing = vi.fn();
		render(<HarnessRunRedirect runId="run-7" onResolved={vi.fn()} onMissing={onMissing} onBack={vi.fn()} />);
		await waitFor(() => expect(onMissing).toHaveBeenCalled());
	});
});

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RunWithHarnessDialog, harnessBranchFor } from './RunWithHarnessDialog';
import { initialHarnessState } from '../harness/harnessState';
import { apiHarnessClient } from '../harness/harnessClient';
import { task } from '../harness/testData';
import type { HarnessController } from '../harness/useHarness';
import type { HarnessSettings, ItemSummary } from '../../lib/types';

afterEach(cleanup);

const settings: HarnessSettings = {
	revision: 1, enabled: true, autoStart: true, requirePrApproval: true, askQuestions: true,
	slots: { scout: 1, sorter: 1, planner: 1, coder: 1, reviewer: 1, qa: 1 }, paused: {}, sources: [],
	modelPolicy: {
		planner: { engine: 'claudecode', model: 'claude-opus-5-5', maxTurns: 20 },
		plan_reviewer: { engine: 'claudecode', model: 'claude-sonnet-5-5', maxTurns: 10 },
		coder: { engine: 'codex', model: 'gpt-5', hardModel: 'gpt-5-pro', maxTurns: 40 },
		code_reviewer: { engine: 'claudecode', model: 'claude-sonnet-5-5', maxTurns: 10 },
		qa_fixer: { engine: 'codex', model: 'gpt-5', maxTurns: 20 }
	}
};
const item: ItemSummary = { id: 'p2', workspaceId: 'ws', workspaceName: 'Sample', branch: 'main', scope: 'platform', identifier: 'PM-2', title: 'Quiet card', description: 'What the ticket says', status: 'draft', tags: [], metadataSource: 'plan.yaml', itemPath: 'plans/platform/PM-2' };

function controller(): HarnessController {
	return {
		state: { ...initialHarnessState, loaded: true }, client: { ...apiHarnessClient, settings: vi.fn().mockResolvedValue(settings) }, dispatch: vi.fn(), refresh: vi.fn(), loadRun: vi.fn(),
		approve: vi.fn(), reject: vi.fn(), pin: vi.fn(), setPriority: vi.fn(), start: vi.fn(), setQuestions: vi.fn().mockResolvedValue(undefined),
		createTask: vi.fn().mockImplementation(async (input: { key: string; startNow?: boolean }) => task(input.key, { startRequested: input.startNow })),
		cancelRun: vi.fn(), pauseRole: vi.fn(), resumeRole: vi.fn(), removeTask: vi.fn().mockResolvedValue(undefined)
	};
}

describe('RunWithHarnessDialog', () => {
	it('names the branch and the models, the approvals, and hands the card over to start now', async () => {
		const harness = controller();
		const onStarted = vi.fn();
		render(<RunWithHarnessDialog item={item} workspace={{ id: 'ws', name: 'Sample', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '' }} harness={harness} onClose={vi.fn()} onStarted={onStarted} />);
		expect(screen.getByRole('dialog', { name: 'Run PM-2 with the harness' })).toBeInTheDocument();
		expect(screen.getByText('harness/PM-2-…')).toBeInTheDocument();
		expect(await screen.findByText((_, element) => element?.tagName === 'LI' && element.textContent === 'Builder writes the code: Codex · gpt-5 (Codex · gpt-5-pro when the plan is hard)')).toBeInTheDocument();
		expect(screen.getByText(/The draft pull request: yours to approve/)).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Run with the harness' }));
		await waitFor(() => expect(onStarted).toHaveBeenCalled());
		expect(harness.createTask).toHaveBeenCalledWith({ workspaceId: 'ws', key: 'PM-2', title: 'Quiet card', body: 'What the ticket says', startNow: true });
		expect(harness.setQuestions).not.toHaveBeenCalled();
	});

	it('queues instead of starting and keeps the task from asking questions when unticked', async () => {
		const harness = controller();
		const onStarted = vi.fn();
		render(<RunWithHarnessDialog item={item} harness={harness} onClose={vi.fn()} onStarted={onStarted} />);
		await screen.findByText((_, element) => element?.tagName === 'LI' && (element.textContent?.startsWith('Planner writes the plan') ?? false));
		fireEvent.click(screen.getByRole('radio', { name: /Queue it/ }));
		fireEvent.click(screen.getByRole('checkbox', { name: /may stop and ask me/ }));
		fireEvent.click(screen.getByRole('button', { name: 'Add to the Task Queue' }));
		await waitFor(() => expect(onStarted).toHaveBeenCalled());
		expect(harness.createTask).toHaveBeenCalledWith(expect.objectContaining({ startNow: false }));
		expect(harness.setQuestions).toHaveBeenCalledWith(expect.objectContaining({ source: expect.objectContaining({ key: 'PM-2' }) }), false);
	});

	it('shows why the harness refused', async () => {
		const harness = controller();
		(harness.createTask as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('PM-2 already ran through the harness and does not reopen'));
		render(<RunWithHarnessDialog item={item} harness={harness} onClose={vi.fn()} onStarted={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Run with the harness' }));
		expect(await screen.findByRole('alert')).toHaveTextContent('The harness could not take this card.');
	});

	it('names the branch the way the harness does', () => {
		expect(harnessBranchFor('PM-2')).toBe('harness/PM-2-…');
		expect(harnessBranchFor('a key/with spaces')).toBe('harness/a-key-with-spaces-…');
	});
});

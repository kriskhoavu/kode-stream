import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { HarnessAgentRun, HarnessApproval, HarnessRun, HarnessRunDetail, HarnessStage, HarnessTask } from '../../lib/types';
import type { HarnessClient } from '../harness/harnessClient';
import type { HarnessSessionRow } from './agentSessions';
import { HarnessSessionDetail } from './HarnessSessionDetail';

vi.mock('./SessionChangesPanel', () => ({ SessionChangesPanel: () => <div data-testid="mock-changes" /> }));
vi.mock('./SessionBrowserTerminal', () => ({ SessionBrowserTerminal: ({ record }: { record: { id: string } }) => <div data-testid="mock-terminal">{record.id}</div> }));
const apiMock = vi.hoisted(() => ({
	aiSessionRecords: vi.fn(),
	aiSettings: vi.fn(),
	startHarnessReviewSession: vi.fn()
}));
vi.mock('../../shared/api', () => ({ api: apiMock }));

const minute = (n: number) => new Date(Date.parse('2026-01-01T00:00:00Z') + n * 60_000).toISOString();
const run = { id: 'run-1', taskId: 'task-1', workspaceId: 'workspace-1', branch: 'harness/DI-7-1', state: 'running', startedAt: minute(0), revision: 3 } as unknown as HarnessRun;
const task = { id: 'task-1', source: { kind: 'jira', key: 'DI-7', labels: [] }, title: 'Fix the price' } as unknown as HarnessTask;
const stage = (id: string, kind: HarnessStage['kind'], ordinal: number, state: HarnessStage['state']) => ({ id, runId: 'run-1', kind, ordinal, state, round: 1, maxRounds: 3, durationMs: 0, startedAt: minute(ordinal) }) as unknown as HarnessStage;
const agent = (id: string, stageId: string, role: HarnessAgentRun['role'], round: number, state: HarnessAgentRun['state'], at: number, verdict?: HarnessAgentRun['verdict']): HarnessAgentRun => ({ id, runId: 'run-1', stageId, role, round, engine: 'claudecode', model: 'opus', state, startedAt: minute(at), turns: 4, durationMs: 60_000, costUsd: 0.2, blocking: verdict === 'changes_requested' ? 1 : 0, advisory: 0, verdict });
const detail = {
	run, task, approvals: [],
	stages: [stage('s-plan', 'plan', 1, 'passed'), stage('s-build', 'implement', 3, 'running')],
	agentRuns: [
		agent('planner-1', 's-plan', 'planner', 1, 'succeeded', 1),
		agent('checker-1', 's-plan', 'plan_reviewer', 1, 'succeeded', 2, 'changes_requested'),
		agent('planner-2', 's-plan', 'planner', 2, 'succeeded', 3),
		agent('builder-1', 's-build', 'coder', 1, 'running', 5)
	]
} as unknown as HarnessRunDetail;
const row: HarnessSessionRow = { key: 'DI-7', task, runs: [detail] };

function stepButton(name: string): HTMLElement {
	return screen.getByText(name, { selector: '.hx-stage-name' }).closest('button') as HTMLElement;
}

describe('HarnessSessionDetail', () => {
	it('opens on the agent at work and walks the steps and rounds of one ticket', () => {
		const client = { agentRunTranscript: vi.fn().mockResolvedValue({ turns: [] }) } as unknown as HarnessClient;
		render(<HarnessSessionDetail row={row} client={client} onOpenRun={vi.fn()} />);

		expect(screen.getByText('Builder · round 1')).toBeInTheDocument();
		expect(stepButton('Build')).toHaveAttribute('aria-pressed', 'true');
		expect(client.agentRunTranscript).toHaveBeenLastCalledWith('builder-1', 50, expect.anything());
		expect(screen.queryByRole('group', { name: /Rounds of/ })).not.toBeInTheDocument();

		// A step with two rounds opens on its last, and its round chips pick the earlier one.
		fireEvent.click(stepButton('Plan'));
		expect(screen.getByText('Planner · round 2')).toBeInTheDocument();
		const rounds = screen.getByRole('group', { name: 'Rounds of Plan' });
		fireEvent.click(rounds.querySelectorAll('button')[0]);
		expect(screen.getByText('Planner · round 1')).toBeInTheDocument();
		expect(client.agentRunTranscript).toHaveBeenLastCalledWith('planner-1', 50, expect.anything());

		// A step no agent worked in says so instead of showing an empty conversation.
		fireEvent.click(stepButton('Test'));
		expect(screen.getByText(/Test has no agent conversation in this run/)).toBeInTheDocument();
		expect(screen.queryByRole('tab', { name: 'Conversation' })).not.toBeInTheDocument();
	});
});

describe('HarnessSessionDetail at Review', () => {
	it('opens on Review, starts a session in the run worktree, and sends the decisions', async () => {
		const reviewStage = { ...stage('s-review', 'review', 5, 'awaiting_approval') } as HarnessStage;
		const waiting = { id: 'ap-review', runId: 'run-1', taskId: 'task-1', stageId: 's-review', gate: 'review', state: 'requested', requestedAt: minute(9), revision: 1 } as HarnessApproval;
		const atReview = { ...detail, run: { ...run, currentStage: reviewStage }, stages: [...detail.stages, reviewStage], approvals: [waiting] } as unknown as HarnessRunDetail;
		const record = { id: 'review-session', workspaceId: 'workspace-1', provider: 'claude', intent: 'harness_review', requestedBranch: 'harness/DI-7-1', state: 'running', startedAt: minute(10), lastKnownAt: minute(10), live: true };
		apiMock.aiSessionRecords.mockResolvedValue([]);
		apiMock.aiSettings.mockResolvedValue({ defaultProvider: 'claude' });
		apiMock.startHarnessReviewSession.mockResolvedValue({ record });
		const decide = { approve: vi.fn(), reject: vi.fn(), decisions: {} };
		const client = { agentRunTranscript: vi.fn().mockResolvedValue({ turns: [] }) } as unknown as HarnessClient;
		render(<HarnessSessionDetail row={{ key: 'DI-7', task, runs: [atReview] }} client={client} decide={decide} />);

		expect(stepButton('Your review')).toHaveAttribute('aria-pressed', 'true');
		await waitFor(() => expect(apiMock.aiSessionRecords).toHaveBeenCalledWith('workspace-1', 'harness/DI-7-1', expect.anything()));
		fireEvent.click(screen.getByRole('button', { name: 'Try it in a session' }));
		await waitFor(() => expect(screen.getByTestId('mock-terminal')).toHaveTextContent('review-session'));
		expect(apiMock.startHarnessReviewSession).toHaveBeenCalledWith('run-1', expect.objectContaining({ provider: 'claude' }));
		expect(screen.queryByRole('button', { name: /in a session/ })).not.toBeInTheDocument();

		fireEvent.click(screen.getByRole('button', { name: 'Hand off' }));
		expect(decide.approve).toHaveBeenCalledWith(waiting, undefined);
	});
});

describe('readableTurns', () => {
	it('drops the run worktree folder from tool lines, spaces in its path included', async () => {
		const { readableTurns } = await import('./agentSessions');
		const turns = readableTurns([
			{ role: 'tool', text: 'Read · /Users/me/Library/Application Support/kode-stream/harness/worktrees/workspace-ff9/run-425/plans/api/plan.md' },
			{ role: 'tool', text: 'Grep · /Users/me/Library/Application Support/kode-stream/harness/worktrees/workspace-ff9/run-425' },
			{ role: 'assistant', text: 'See /Users/me/Library/Application Support/kode-stream/harness/worktrees/workspace-ff9/run-425/a.md' }
		]);
		expect(turns?.map((turn) => turn.text)).toEqual([
			'Read · ./plans/api/plan.md',
			'Grep · ./',
			'See /Users/me/Library/Application Support/kode-stream/harness/worktrees/workspace-ff9/run-425/a.md'
		]);
	});
});

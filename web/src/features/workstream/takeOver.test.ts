import { describe, expect, it, vi } from 'vitest';
import { takeOverByHand } from './takeOver';
import { cardWork } from './harnessJoin';
import { initialHarnessState } from '../harness/harnessState';
import type { HarnessState } from '../harness/harnessState';
import { run, stage, status, task } from '../harness/testData';
import type { HarnessController } from '../harness/useHarness';
import { apiHarnessClient } from '../harness/harnessClient';

function controller(state: HarnessState): HarnessController {
	return { state, client: apiHarnessClient, dispatch: vi.fn(), refresh: vi.fn(), loadRun: vi.fn(), approve: vi.fn(), reject: vi.fn(), pin: vi.fn(), setPriority: vi.fn(), start: vi.fn(), createTask: vi.fn(), setQuestions: vi.fn(), cancelRun: vi.fn(), pauseRole: vi.fn().mockResolvedValue(undefined), resumeRole: vi.fn(), removeTask: vi.fn().mockResolvedValue(undefined) };
}

describe('takeOverByHand', () => {
	const working = task('PM-1', { state: 'running', runId: 'run-1' });
	const agent = { id: 'agent-1', runId: 'run-1', stageId: 'run-1-implement', role: 'coder' as const, round: 1, engine: 'claudecode' as const, model: 'opus', state: 'running' as const, turns: 1, durationMs: 0, costUsd: 0, blocking: 0, advisory: 0 };

	it('pauses the working role and names its session', async () => {
		const state: HarnessState = { ...initialHarnessState, loaded: true, status: status(), tasks: { [working.id]: working }, runs: { 'run-1': run('run-1', working.id, { currentStage: stage('run-1', 'implement', { state: 'running' }), activeAgentRun: agent }) } };
		const harness = controller(state);
		expect(await takeOverByHand(cardWork(working, state, false), harness)).toEqual({ agentRunId: 'agent-1' });
		expect(harness.pauseRole).toHaveBeenCalledWith('coder');
	});

	it('does not pause a role twice, and names nothing while nobody works', async () => {
		const paused: HarnessState = { ...initialHarnessState, loaded: true, status: status({ roles: [{ role: 'coder', paused: true, working: 1, slots: 1, queued: 0 }] }), tasks: { [working.id]: working }, runs: { 'run-1': run('run-1', working.id, { activeAgentRun: agent }) } };
		const harness = controller(paused);
		expect(await takeOverByHand(cardWork(working, paused, false), harness)).toEqual({ agentRunId: 'agent-1' });
		expect(harness.pauseRole).not.toHaveBeenCalled();
		const idle: HarnessState = { ...initialHarnessState, loaded: true, status: status(), tasks: { [working.id]: working }, runs: { 'run-1': run('run-1', working.id) } };
		expect(await takeOverByHand(cardWork(working, idle, false), controller(idle))).toEqual({});
	});
});

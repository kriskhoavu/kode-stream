import { describe, expect, it } from 'vitest';
import type { ItemSummary } from '../../lib/types';
import { initialHarnessState } from '../harness/harnessState';
import type { HarnessState } from '../harness/harnessState';
import { approval, run, stage, status, task } from '../harness/testData';
import { joinHarness, runProgress, workedBy } from './harnessJoin';

const item = (id: string, identifier: string): ItemSummary => ({ id, workspaceId: 'ws', workspaceName: 'Sample', branch: 'main', scope: 'platform', identifier, title: `Item ${identifier}`, status: 'in_progress', tags: [], metadataSource: 'plan.yaml', itemPath: `plans/platform/${identifier}` });

function loaded(patch: Partial<HarnessState>): HarnessState {
	return { ...initialHarnessState, loaded: true, status: status(), ...patch };
}

describe('joinHarness', () => {
	it('joins a task to the item of its Jira key, whatever the case, and keeps the rest by hand', () => {
		const state = loaded({
			tasks: { 'task-PM-1': task('pm-1', { state: 'running', runId: 'run-1' }) },
			runs: { 'run-1': run('run-1', 'task-PM-1', { currentStage: stage('run-1', 'implement', { state: 'running', round: 2 }) }) }
		});
		const join = joinHarness([item('p1', 'PM-1'), item('p2', 'PM-2')], state, 'ws');
		const work = join.byItemId.get('p1');
		expect(work?.status).toBe('working');
		expect(work?.stageLabel).toBe('Build · round 2 of 3');
		expect(work?.progress).toBeCloseTo(2.5 / 6);
		expect(workedBy(work)).toBe('harness');
		expect(workedBy(join.byItemId.get('p2'))).toBe('hand');
		expect(join.synthetic).toEqual([]);
	});

	it('makes a card of its own for a task no item holds, in the Unsorted column', () => {
		const state = loaded({ tasks: { 'task-PM-9': task('PM-9', { title: 'Found by the Finder' }) } });
		const join = joinHarness([item('p1', 'PM-1')], state, 'ws');
		expect(join.synthetic).toHaveLength(1);
		expect(join.synthetic[0]).toMatchObject({ id: 'harness:task-PM-9', identifier: 'PM-9', title: 'Found by the Finder', status: 'unsorted', metadataSource: 'harness' });
		expect(join.byItemId.get('harness:task-PM-9')?.synthetic).toBe(true);
	});

	it('skips tasks of another workspace, dismissed tasks and an unloaded state', () => {
		const state = loaded({ tasks: { a: task('PM-1', { id: 'a', workspaceId: 'other' }), b: task('PM-2', { id: 'b', state: 'dismissed' }) } });
		expect(joinHarness([item('p1', 'PM-1')], state, 'ws').byItemId.size).toBe(0);
		expect(joinHarness([item('p1', 'PM-1')], initialHarnessState, 'ws').byItemId.size).toBe(0);
	});

	it('lists the approvals waiting on a card, newest first', () => {
		const state = loaded({
			tasks: { 'task-PM-1': task('PM-1', { needsYou: true, runId: 'run-1' }) },
			runs: { 'run-1': run('run-1', 'task-PM-1') },
			approvals: { a1: approval('a1', 'task-PM-1', { requestedAt: '2026-11-04T09:00:00.000Z' }), a2: approval('a2', 'task-PM-1', { requestedAt: '2026-11-04T10:00:00.000Z' }), a3: approval('a3', 'task-PM-1', { state: 'approved' }) }
		});
		const work = joinHarness([item('p1', 'PM-1')], state, 'ws').byItemId.get('p1');
		expect(work?.status).toBe('waiting');
		expect(work?.waiting.map((entry) => entry.id)).toEqual(['a2', 'a1']);
	});
});

describe('runProgress', () => {
	it('counts a stage under way as half and a passed run as whole', () => {
		expect(runProgress(undefined)).toBeUndefined();
		expect(runProgress(run('r', 't'))).toBe(0);
		expect(runProgress(run('r', 't', { currentStage: stage('r', 'plan', { state: 'running' }) }))).toBeCloseTo(0.5 / 6);
		expect(runProgress(run('r', 't', { currentStage: stage('r', 'draft_pr', { state: 'passed' }) }))).toBe(1);
		expect(runProgress(run('r', 't', { state: 'passed' }))).toBe(1);
	});
});

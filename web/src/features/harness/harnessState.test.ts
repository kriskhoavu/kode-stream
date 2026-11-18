import { describe, expect, it } from 'vitest';
import { decisionLine, doneToday, startOfDay, harnessReducer, initialHarnessState, queueTasks, waitingApprovals, waitsFor, workingRuns } from './harnessState';
import type { HarnessState } from './harnessState';
import { approval, run, stage, status, task } from './testData';

function loaded(): HarnessState {
	return harnessReducer(initialHarnessState, {
		type: 'snapshot',
		status: status({ lastEventSeq: 10 }),
		tasks: [task('A', { score: 5 }), task('B', { score: 9, dependsOn: ['task-A'] }), task('C', { pinned: true, score: 1 }), task('D', { needsYou: true, score: 0 }), task('E', { state: 'stale' })],
		runs: [run('run-1', 'task-D', { currentStage: stage('run-1', 'plan_approval', { state: 'awaiting_approval' }) }), run('run-2', 'task-A', { currentStageId: 'run-2-implement', currentStage: stage('run-2', 'implement', { state: 'running' }) })],
		approvals: [approval('ap-1', 'task-D')]
	});
}

describe('harness reducer', () => {
	it('loads a snapshot and orders the queue: waiting on you, pinned, then score', () => {
		const state = loaded();
		expect(state.loaded).toBe(true);
		expect(state.lastSeq).toBe(10);
		expect(queueTasks(state.tasks).map((item) => item.source.key)).toEqual(['D', 'C', 'B', 'A']);
		expect(waitsFor(state.tasks['task-B'], state.tasks)).toEqual(['A']);
		expect(workingRuns(state).map((item) => item.id)).toEqual(['run-2']);
	});

	it('ignores events it has already seen', () => {
		const state = loaded();
		const stale = harnessReducer(state, { type: 'event', event: { seq: 9, at: '', type: 'task.updated', task: task('A', { title: 'Old' }) } });
		expect(stale).toBe(state);
		const fresh = harnessReducer(state, { type: 'event', event: { seq: 11, at: '', type: 'task.updated', task: task('A', { title: 'New' }) } });
		expect(fresh.tasks['task-A'].title).toBe('New');
		expect(fresh.lastSeq).toBe(11);
	});

	it('takes the task a run or gate event carries, so a started task leaves Up next', () => {
		const state = loaded();
		const started = harnessReducer(state, { type: 'event', event: { seq: 11, at: '', type: 'run.started', run: run('run-9', 'task-C'), task: task('C', { pinned: true, state: 'running', runId: 'run-9', startRequested: true }) } });
		expect(started.tasks['task-C']).toMatchObject({ state: 'running', runId: 'run-9' });
		expect(started.runs['run-9']).toBeDefined();
		const asked = harnessReducer(started, { type: 'event', event: { seq: 12, at: '', type: 'approval.requested', approval: approval('ap-9', 'task-C', { runId: 'run-9', gate: 'question' }), task: task('C', { state: 'running', runId: 'run-9', needsYou: true }) } });
		expect(asked.tasks['task-C'].needsYou).toBe(true);
	});

	it('drops a task the harness removed, and ignores one it never held', () => {
		const state = loaded();
		const removed = harnessReducer(state, { type: 'event', event: { seq: 11, at: '', type: 'task.removed', taskId: 'task-A' } });
		expect(removed.tasks['task-A']).toBeUndefined();
		expect(removed.lastSeq).toBe(11);
		const unknown = harnessReducer(removed, { type: 'event', event: { seq: 12, at: '', type: 'task.removed', taskId: 'task-nobody' } });
		expect(unknown.tasks).toBe(removed.tasks);
	});

	it('announces a new approval assertively and a finished stage politely', () => {
		let state = loaded();
		state = harnessReducer(state, { type: 'event', event: { seq: 11, at: '', type: 'approval.requested', approval: approval('ap-2', 'task-A', { gate: 'draft_pr' }) } });
		expect(state.announcement).toMatchObject({ text: 'A is waiting on you', assertive: true });
		expect(waitingApprovals(state).map((item) => item.id)).toEqual(['ap-1', 'ap-2']);
		state = harnessReducer(state, { type: 'event', event: { seq: 12, at: '', type: 'stage.finished', stage: stage('run-2', 'implement', { state: 'passed' }) } });
		expect(state.announcement).toMatchObject({ text: 'A: Build finished', assertive: false });
	});

	it('reconciles an optimistic decision with the decided event and names the channel', () => {
		let state = harnessReducer(loaded(), { type: 'decision.sent', approvalId: 'ap-1', decision: 'approve' });
		expect(decisionLine(state.approvals['ap-1'], state.decisions['ap-1'])).toBe('Sending…');
		state = harnessReducer(state, { type: 'event', event: { seq: 11, at: '', type: 'approval.decided', approval: approval('ap-1', 'task-D', { state: 'approved', channel: 'dashboard', revision: 2, decidedBy: { actorId: 'me', displayName: 'Khoa' } }) } });
		expect(decisionLine(state.approvals['ap-1'], state.decisions['ap-1'])).toBe('Approved by you');
		const elsewhere = approval('ap-3', 'task-A', { state: 'approved', channel: 'webex', decidedBy: { actorId: 'anna', displayName: 'Anna' } });
		expect(decisionLine(elsewhere)).toBe('Approved via Webex by Anna');
	});

	it('keeps a failed decision so the card can say so', () => {
		let state = harnessReducer(loaded(), { type: 'decision.sent', approvalId: 'ap-1', decision: 'reject', reason: 'No' });
		state = harnessReducer(state, { type: 'decision.failed', approvalId: 'ap-1', error: 'Could not send. Try again.' });
		expect(decisionLine(state.approvals['ap-1'], state.decisions['ap-1'])).toBe('Could not send. Try again.');
	});

	it('does not let an older approval revision overwrite a newer one', () => {
		let state = harnessReducer(loaded(), { type: 'decision.done', approvalId: 'ap-1', approval: approval('ap-1', 'task-D', { state: 'approved', revision: 3 }) });
		state = harnessReducer(state, { type: 'event', event: { seq: 11, at: '', type: 'approval.requested', approval: approval('ap-1', 'task-D', { revision: 2 }) } });
		expect(state.approvals['ap-1'].state).toBe('approved');
	});

	it('lists runs and hand-finished tasks done today', () => {
		const now = Date.now();
		const today = (hours: number) => new Date(startOfDay(now) + hours * 3_600_000).toISOString();
		const state = harnessReducer(initialHarnessState, {
			type: 'snapshot', status: status(), approvals: [],
			tasks: [task('A'), task('M', { state: 'done', finishedOutside: { by: 'Anna', at: today(0.5) } })],
			runs: [run('r1', 'task-A', { state: 'passed', finishedAt: today(0.75) }), run('r0', 'task-A', { state: 'passed', finishedAt: today(-30) })]
		});
		expect(doneToday(state, now).map((entry) => entry.kind === 'run' ? entry.run.id : entry.task.id)).toEqual(['r1', 'task-M']);
	});
});

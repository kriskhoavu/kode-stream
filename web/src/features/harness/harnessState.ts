import type {
	HarnessAgentRun,
	HarnessApproval,
	HarnessCostSummary,
	HarnessDecision,
	HarnessEvent,
	HarnessRun,
	HarnessRunDetail,
	HarnessStage,
	HarnessStatus,
	HarnessTask
} from '../../lib/types';
import { channelLabels, stageKindLabels } from './vocabulary';

export type HarnessConnection = 'connecting' | 'live' | 'reconnecting' | 'polling' | 'offline';

/** A decision this browser sent, kept until the server's event reconciles it. */
export interface SentDecision {
	approvalId: string;
	decision: HarnessDecision;
	state: 'sending' | 'sent' | 'failed';
	reason?: string;
	error?: string;
}

export interface HarnessAnnouncement { id: number; text: string; assertive: boolean }

export interface HarnessState {
	loaded: boolean;
	error?: string;
	status?: HarnessStatus;
	tasks: Record<string, HarnessTask>;
	runs: Record<string, HarnessRun>;
	approvals: Record<string, HarnessApproval>;
	details: Record<string, HarnessRunDetail>;
	cost?: HarnessCostSummary;
	lastSeq: number;
	connection: HarnessConnection;
	decisions: Record<string, SentDecision>;
	announcement?: HarnessAnnouncement;
}

export type HarnessAction =
	| { type: 'snapshot'; status: HarnessStatus; tasks: HarnessTask[]; runs: HarnessRun[]; approvals: HarnessApproval[]; cost?: HarnessCostSummary }
	| { type: 'snapshot.failed'; error: string }
	| { type: 'detail'; detail: HarnessRunDetail }
	| { type: 'event'; event: HarnessEvent }
	| { type: 'decision.sent'; approvalId: string; decision: HarnessDecision; reason?: string }
	| { type: 'decision.done'; approvalId: string; approval?: HarnessApproval }
	| { type: 'decision.failed'; approvalId: string; error: string }
	| { type: 'task.saved'; task: HarnessTask }
	| { type: 'connection'; connection: HarnessConnection };

export const initialHarnessState: HarnessState = {
	loaded: false,
	tasks: {},
	runs: {},
	approvals: {},
	details: {},
	lastSeq: 0,
	connection: 'connecting',
	decisions: {}
};

export function harnessReducer(state: HarnessState, action: HarnessAction): HarnessState {
	switch (action.type) {
		case 'snapshot': {
			const details = { ...state.details };
			for (const run of action.runs) {
				if (details[run.id]) details[run.id] = { ...details[run.id], run: { ...details[run.id].run, ...run } };
			}
			return {
				...state,
				loaded: true,
				error: undefined,
				status: action.status,
				tasks: byId(action.tasks),
				runs: byId(action.runs),
				approvals: byId(action.approvals),
				details,
				cost: action.cost ?? state.cost,
				lastSeq: Math.max(state.lastSeq, action.status.lastEventSeq ?? 0)
			};
		}
		case 'snapshot.failed':
			return { ...state, error: action.error, loaded: state.loaded };
		case 'detail':
			return {
				...state,
				details: { ...state.details, [action.detail.run.id]: action.detail },
				runs: { ...state.runs, [action.detail.run.id]: { ...state.runs[action.detail.run.id], ...action.detail.run } },
				tasks: { ...state.tasks, [action.detail.task.id]: action.detail.task },
				approvals: { ...state.approvals, ...byId(action.detail.approvals) }
			};
		case 'event':
			return applyEvent(state, action.event);
		case 'decision.sent':
			return { ...state, decisions: { ...state.decisions, [action.approvalId]: { approvalId: action.approvalId, decision: action.decision, reason: action.reason, state: 'sending' } } };
		case 'decision.done': {
			const sent = state.decisions[action.approvalId];
			const decisions = sent ? { ...state.decisions, [action.approvalId]: { ...sent, state: 'sent' as const } } : state.decisions;
			return action.approval ? upsertApproval({ ...state, decisions }, action.approval) : { ...state, decisions };
		}
		case 'decision.failed': {
			const sent = state.decisions[action.approvalId];
			if (!sent) return state;
			return { ...state, decisions: { ...state.decisions, [action.approvalId]: { ...sent, state: 'failed', error: action.error } } };
		}
		case 'task.saved':
			return { ...state, tasks: { ...state.tasks, [action.task.id]: action.task } };
		case 'connection':
			return state.connection === action.connection ? state : { ...state, connection: action.connection };
	}
}

function applyEvent(state: HarnessState, event: HarnessEvent): HarnessState {
	if (event.seq <= state.lastSeq) return state;
	let next: HarnessState = { ...state, lastSeq: event.seq };
	// A run or gate event carries its task, whose state, run and needs-you change with it.
	if ('task' in event && event.task && event.type !== 'task.discovered' && event.type !== 'task.updated' && event.type !== 'task.ranked' && event.type !== 'task.pinned') {
		next.tasks = { ...next.tasks, [event.task.id]: event.task };
	}
	switch (event.type) {
		case 'task.discovered':
		case 'task.updated':
		case 'task.ranked':
		case 'task.pinned':
			next.tasks = { ...next.tasks, [event.task.id]: event.task };
			return next;
		case 'task.removed': {
			if (!(event.taskId in next.tasks)) return next;
			const { [event.taskId]: _gone, ...tasks } = next.tasks;
			next.tasks = tasks;
			return next;
		}
		case 'run.started':
		case 'run.blocked':
		case 'run.finished': {
			const run = { ...next.runs[event.run.id], ...event.run };
			next.runs = { ...next.runs, [run.id]: run };
			const detail = next.details[run.id];
			if (detail) next.details = { ...next.details, [run.id]: { ...detail, run: { ...detail.run, ...event.run } } };
			if (event.type === 'run.finished') next = announce(next, `${taskKey(next, run.taskId)}: ${run.state === 'passed' ? 'finished' : run.state === 'cancelled' ? 'stopped' : "didn't pass"}`, false);
			if (event.type === 'run.blocked') next = announce(next, `${taskKey(next, run.taskId)}: stopped and needs you`, false);
			return next;
		}
		case 'stage.started':
		case 'stage.round':
		case 'stage.finished': {
			next = upsertStage(next, event.stage);
			if (event.type === 'stage.finished') {
				const run = next.runs[event.stage.runId];
				const word = event.stage.state === 'passed' ? 'finished' : event.stage.state === 'failed' ? "didn't pass" : 'stopped';
				next = announce(next, `${run ? taskKey(next, run.taskId) : 'A task'}: ${stageKindLabels[event.stage.kind]} ${word}`, false);
			}
			return next;
		}
		case 'agent.started':
		case 'agent.progress':
		case 'agent.finished':
			return upsertAgentRun(next, event.agentRun);
		case 'approval.requested': {
			next = upsertApproval(next, event.approval);
			return announce(next, `${taskKey(next, event.approval.taskId)} is waiting on you`, true);
		}
		case 'approval.decided':
			return upsertApproval(next, event.approval);
		case 'role.paused':
		case 'role.resumed': {
			if (!next.status) return next;
			const roles = next.status.roles.some((role) => role.role === event.role.role)
				? next.status.roles.map((role) => role.role === event.role.role ? event.role : role)
				: [...next.status.roles, event.role];
			next.status = { ...next.status, roles };
			return next;
		}
		case 'notification.failed':
			return next;
	}
}

function upsertApproval(state: HarnessState, approval: HarnessApproval): HarnessState {
	const current = state.approvals[approval.id];
	if (current && current.revision > approval.revision) return state;
	const next = { ...state, approvals: { ...state.approvals, [approval.id]: approval } };
	const detail = approval.runId ? next.details[approval.runId] : undefined;
	if (detail) next.details = { ...next.details, [detail.run.id]: { ...detail, approvals: upsert(detail.approvals, approval) } };
	return next;
}

function upsertStage(state: HarnessState, stage: HarnessStage): HarnessState {
	const next = { ...state };
	const run = next.runs[stage.runId];
	if (run) {
		const current = run.currentStageId === stage.id || stage.state === 'running' || stage.state === 'awaiting_approval';
		next.runs = { ...next.runs, [run.id]: current ? { ...run, currentStageId: stage.id, currentStage: stage } : run };
	}
	const detail = next.details[stage.runId];
	if (detail) next.details = { ...next.details, [stage.runId]: { ...detail, stages: upsert(detail.stages, stage).sort((a, b) => a.ordinal - b.ordinal) } };
	return next;
}

function upsertAgentRun(state: HarnessState, agentRun: HarnessAgentRun): HarnessState {
	const next = { ...state };
	const run = next.runs[agentRun.runId];
	if (run) {
		const active = agentRun.state === 'running' || agentRun.state === 'queued';
		const activeAgentRun = active ? agentRun : run.activeAgentRun?.id === agentRun.id ? undefined : run.activeAgentRun;
		next.runs = { ...next.runs, [run.id]: { ...run, activeAgentRun } };
	}
	const detail = next.details[agentRun.runId];
	if (detail) next.details = { ...next.details, [agentRun.runId]: { ...detail, agentRuns: upsert(detail.agentRuns, agentRun) } };
	return next;
}

function announce(state: HarnessState, text: string, assertive: boolean): HarnessState {
	return { ...state, announcement: { id: (state.announcement?.id ?? 0) + 1, text, assertive } };
}

function taskKey(state: HarnessState, taskId: string): string {
	return state.tasks[taskId]?.source.key ?? 'A task';
}

function byId<T extends { id: string }>(items: T[]): Record<string, T> {
	return Object.fromEntries(items.map((item) => [item.id, item]));
}

function upsert<T extends { id: string }>(items: T[], item: T): T[] {
	const index = items.findIndex((candidate) => candidate.id === item.id);
	if (index < 0) return [...items, item];
	const copy = items.slice();
	copy[index] = { ...copy[index], ...item };
	return copy;
}

// ── Selectors ────────────────────────────────────────────────────────────────

const doneTaskStates = new Set(['done']);

export function waitingApprovals(state: HarnessState): HarnessApproval[] {
	return Object.values(state.approvals)
		.filter((approval) => approval.state === 'requested')
		.sort((a, b) => Date.parse(b.requestedAt) - Date.parse(a.requestedAt));
}

/** Approvals decided since local midnight, newest first. */
export function decidedToday(state: HarnessState, now: number): HarnessApproval[] {
	const midnight = startOfDay(now);
	return Object.values(state.approvals)
		.filter((approval) => approval.state !== 'requested' && approval.decidedAt && Date.parse(approval.decidedAt) >= midnight)
		.sort((a, b) => Date.parse(b.decidedAt ?? '') - Date.parse(a.decidedAt ?? ''));
}

export function workingRuns(state: HarnessState): HarnessRun[] {
	return Object.values(state.runs)
		.filter((run) => run.state === 'running' && run.currentStage?.state !== 'awaiting_approval')
		.sort((a, b) => Date.parse(a.startedAt) - Date.parse(b.startedAt));
}

export type DoneTodayEntry = { kind: 'run'; run: HarnessRun; task?: HarnessTask; at: string } | { kind: 'manual'; task: HarnessTask; at: string };

export function doneToday(state: HarnessState, now: number): DoneTodayEntry[] {
	const midnight = startOfDay(now);
	const runs: DoneTodayEntry[] = Object.values(state.runs)
		.filter((run) => run.state === 'passed' && run.finishedAt && Date.parse(run.finishedAt) >= midnight)
		.map((run) => ({ kind: 'run', run, task: state.tasks[run.taskId], at: run.finishedAt ?? '' }));
	const manual: DoneTodayEntry[] = Object.values(state.tasks)
		.filter((task) => task.finishedOutside?.at && Date.parse(task.finishedOutside.at) >= midnight)
		.map((task) => ({ kind: 'manual', task, at: task.finishedOutside?.at ?? '' }));
	return [...runs, ...manual].sort((a, b) => Date.parse(b.at) - Date.parse(a.at));
}

/** The queue as the Sorter ranked it: waiting on you first, then pinned, then by score. */
export function queueTasks(tasks: Record<string, HarnessTask>, options: { includeFinished?: boolean } = {}): HarnessTask[] {
	return Object.values(tasks)
		.filter((task) => options.includeFinished || !(task.state === 'dismissed' || task.state === 'stale'))
		.sort(compareTasks);
}

/** Whether a person may start the task: one waiting in the queue, or one whose last run was stopped. */
export function canStartTask(task: Pick<HarnessTask, 'state'>): boolean {
	return task.state === 'ranked' || task.state === 'discovered' || task.state === 'cancelled';
}

/** Whether a start was asked for and not yet taken. A stopped task's old request was spent on the run that stopped. */
export function startPending(task: Pick<HarnessTask, 'state' | 'startRequested'>): boolean {
	return Boolean(task.startRequested) && task.state !== 'cancelled';
}

export function compareTasks(a: HarnessTask, b: HarnessTask): number {
	if (a.needsYou !== b.needsYou) return a.needsYou ? -1 : 1;
	if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
	return b.score - a.score || a.source.key.localeCompare(b.source.key);
}

/** The dependencies of a task that are not done yet, by source key. */
export function waitsFor(task: HarnessTask, tasks: Record<string, HarnessTask>): string[] {
	return task.dependsOn
		.map((id) => tasks[id])
		.filter((dependency): dependency is HarnessTask => Boolean(dependency) && !doneTaskStates.has(dependency.state) && !dependency.finishedOutside)
		.map((dependency) => dependency.source.key);
}

/** How a decision reads once made: "Approved by you", "Approved via Webex by Anna". */
export function decisionLine(approval: HarnessApproval, sent?: SentDecision): string {
	if (approval.state === 'requested') {
		if (sent?.state === 'sending') return 'Sending…';
		if (sent?.state === 'failed') return sent.error ?? 'Could not send. Try again.';
		return '';
	}
	const verb = approval.state === 'approved' ? (approval.gate === 'question' ? 'Answered' : 'Approved') : approval.state === 'rejected' ? (approval.gate === 'question' || approval.gate === 'resume' ? 'Stopped' : 'Changes asked') : approval.state === 'expired' ? 'Expired' : 'Replaced by a newer request';
	if (approval.state === 'expired' || approval.state === 'superseded') return verb;
	const mine = sent && approval.channel === 'dashboard';
	if (mine) return `${verb} by you`;
	const who = approval.decidedBy?.displayName ?? 'someone';
	const where = approval.channel && approval.channel !== 'dashboard' ? `${channelLabels[approval.channel]} ` : '';
	return `${verb} ${where}by ${who}`;
}

export function startOfDay(now: number): number {
	const date = new Date(now);
	date.setHours(0, 0, 0, 0);
	return date.getTime();
}

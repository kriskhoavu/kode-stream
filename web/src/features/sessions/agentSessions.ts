import type { GlobalSessionRow, HarnessAgentRun, HarnessRun, HarnessTask } from '../../lib/types';
import { isSessionLive } from './sessionDisplay';

/**
 * Agent Space lists every agent conversation of a workspace as one kind of
 * row, a session, of two kinds:
 *
 * - An interactive session is a coding agent a person started in a terminal
 *   on the workspace's checkout. The person types into it and stops it.
 * - A harness session is one agent the harness started for a task (a
 *   Planner, Checker or Builder round), headless, in the run's own worktree.
 *   Nobody types into it: a person steers it through the run's questions and
 *   approvals, and reads its conversation.
 */
export type SessionKind = 'interactive' | 'harness';

export interface HarnessSessionRow {
	agentRun: HarnessAgentRun;
	run: HarnessRun;
	task?: HarnessTask;
	/** The run's step this agent worked in, as the run page names it ("Plan", "Build 2"). */
	step: string;
}

export type AgentSession =
	| { kind: 'interactive'; id: string; row: GlobalSessionRow }
	| { kind: 'harness'; id: string; harness: HarnessSessionRow };

export function interactiveSession(row: GlobalSessionRow): AgentSession {
	return { kind: 'interactive', id: row.record.id, row };
}

export function harnessSession(harness: HarnessSessionRow): AgentSession {
	return { kind: 'harness', id: `harness:${harness.agentRun.id}`, harness };
}

export function sessionIsLive(session: AgentSession): boolean {
	return session.kind === 'interactive' ? isSessionLive(session.row.record) : session.harness.agentRun.state === 'running';
}

export function sessionStartedAt(session: AgentSession): string {
	return session.kind === 'interactive' ? session.row.record.startedAt : session.harness.agentRun.startedAt ?? session.harness.run.startedAt;
}

/** The ticket or plan a session belongs to: its key, when it has one, joins a harness task to an interactive session on the same plan. */
export function sessionGroup(session: AgentSession): { key: string; title: string } {
	if (session.kind === 'harness') {
		const { task, run } = session.harness;
		if (!task) return { key: `run:${run.id}`, title: run.branch };
		return { key: `key:${task.source.key}`, title: `${task.source.key} · ${task.title}` };
	}
	const { row } = session;
	if (row.planKey) return { key: `key:${row.planKey}`, title: row.planTitle ? `${row.planKey} · ${row.planTitle}` : row.planKey };
	const label = row.planTitle || row.record.intent || row.record.provider;
	return { key: `label:${label}`, title: label };
}

export function byLiveThenNewest(a: AgentSession, b: AgentSession): number {
	const liveDiff = Number(sessionIsLive(b)) - Number(sessionIsLive(a));
	if (liveDiff !== 0) return liveDiff;
	return Date.parse(sessionStartedAt(b)) - Date.parse(sessionStartedAt(a));
}

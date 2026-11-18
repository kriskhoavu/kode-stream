import type { ClaudeTurn, GlobalSessionRow, HarnessRun, HarnessRunDetail, HarnessTask } from '../../lib/types';
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
 *
 * The list shows one harness row per ticket, never one per round: its detail
 * walks the run's steps and rounds, so a ticket's many agents stay one card.
 */
export type SessionKind = 'interactive' | 'harness';

/** One ticket's harness sessions: every agent of its runs, newest run first. */
export interface HarnessSessionRow {
	/** The ticket's key (`APP-664`), or its run's id while the task is unknown. */
	key: string;
	task?: HarnessTask;
	/** The ticket's runs with their agents, newest first; never empty. */
	runs: HarnessRunDetail[];
}

/** The run a ticket's card speaks for: its newest. */
export function harnessRun(row: HarnessSessionRow): HarnessRun {
	return row.runs[0].run;
}

export type AgentSession =
	| { kind: 'interactive'; id: string; row: GlobalSessionRow }
	| { kind: 'harness'; id: string; harness: HarnessSessionRow };

export function interactiveSession(row: GlobalSessionRow): AgentSession {
	return { kind: 'interactive', id: row.record.id, row };
}

export function harnessSession(harness: HarnessSessionRow): AgentSession {
	return { kind: 'harness', id: `harness:${harness.key}`, harness };
}

export function sessionIsLive(session: AgentSession): boolean {
	return session.kind === 'interactive' ? isSessionLive(session.row.record) : harnessRun(session.harness).state === 'running';
}

export function sessionStartedAt(session: AgentSession): string {
	return session.kind === 'interactive' ? session.row.record.startedAt : harnessRun(session.harness).startedAt;
}

/** The ticket or plan a session belongs to: its key, when it has one, joins a harness task to an interactive session on the same plan. */
export function sessionGroup(session: AgentSession): { key: string; title: string } {
	if (session.kind === 'harness') {
		const { task } = session.harness;
		if (!task) return { key: `run:${session.harness.key}`, title: harnessRun(session.harness).branch };
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

/**
 * A path inside a run's worktree, as an agent's tool call names it: from a
 * filesystem root, through folders whose names may hold spaces ("Application
 * Support"), to the run's own folder, which adds nothing a reader needs.
 */
const worktreePath = /\/(?:Users|home|private|var|tmp)\/[^\n]*?\/harness\/worktrees\/[^/\s]+\/[^/\s]+(?:\/|(?=\s|$))/g;

export function readableTurns(turns: ClaudeTurn[] | undefined): ClaudeTurn[] | undefined {
	return turns?.map((turn) => (turn.role === 'tool' ? { ...turn, text: turn.text.replace(worktreePath, './') } : turn));
}

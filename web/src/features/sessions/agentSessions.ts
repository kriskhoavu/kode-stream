import type { ClaudeTurn, GlobalSessionRow, HarnessRun, HarnessRunDetail, HarnessTask } from '../../lib/types';
import { isSessionLive } from './sessionDisplay';

// The list shows one harness row per ticket, never one per round.
export type SessionKind = 'interactive' | 'harness';

export interface HarnessSessionRow {
	key: string;
	task?: HarnessTask;
	runs: HarnessRunDetail[];
}

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

// Folder names in the path may hold spaces ("Application Support").
const worktreePath = /\/(?:Users|home|private|var|tmp)\/[^\n]*?\/harness\/worktrees\/[^/\s]+\/[^/\s]+(?:\/|(?=\s|$))/g;

export function readableTurns(turns: ClaudeTurn[] | undefined): ClaudeTurn[] | undefined {
	return turns?.map((turn) => (turn.role === 'tool' ? { ...turn, text: turn.text.replace(worktreePath, './') } : turn));
}

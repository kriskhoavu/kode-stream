import type { HarnessApproval, HarnessRun, HarnessStage, HarnessStatus, HarnessTask } from '../../lib/types';

/** Small builders for harness tests; each returns a complete object with sensible defaults. */
export function task(key: string, patch: Partial<HarnessTask> = {}): HarnessTask {
	return {
		id: `task-${key}`, workspaceId: 'ws', source: { kind: 'jira', key, labels: [] }, title: `Title ${key}`, state: 'ranked',
		score: 10, scoreReasons: [], priorityOverride: 0, pinned: false, needsYou: false, attempts: 0, revision: 1, dependsOn: [], ...patch
	};
}

export function stage(runId: string, kind: HarnessStage['kind'], patch: Partial<HarnessStage> = {}): HarnessStage {
	return { id: `${runId}-${kind}`, runId, kind, ordinal: 0, state: 'pending', round: 1, maxRounds: 3, attempt: 1, maxAttempts: 3, turns: 0, durationMs: 0, costUsd: 0, blocking: 0, advisory: 0, ...patch };
}

export function run(id: string, taskId: string, patch: Partial<HarnessRun> = {}): HarnessRun {
	return { id, taskId, workspaceId: 'ws', branch: `harness/${id}`, state: 'running', totals: { turns: 0, durationMs: 0, costUsd: 0, blocking: 0, advisory: 0 }, startedAt: '2026-11-04T08:00:00.000Z', revision: 1, ...patch };
}

export function approval(id: string, taskId: string, patch: Partial<HarnessApproval> = {}): HarnessApproval {
	return { id, taskId, runId: 'run-1', gate: 'plan', state: 'requested', requestedAt: '2026-11-04T09:00:00.000Z', revision: 1, ...patch };
}

export function status(patch: Partial<HarnessStatus> = {}): HarnessStatus {
	return { enabled: true, storageSupported: true, mode: 'local', running: true, waitingCount: 0, lastEventSeq: 0, roles: [], engines: [], ...patch };
}

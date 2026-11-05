import { useCallback, useEffect, useReducer, useRef } from 'react';
import { ApiError } from '../../shared/api';
import { newIdempotencyKey } from '../../shared/api/idempotency';
import type { HarnessApproval, HarnessRoleId, HarnessTask } from '../../lib/types';
import type { HarnessClient } from './harnessClient';
import { harnessReducer, initialHarnessState, startOfDay } from './harnessState';
import type { HarnessAction, HarnessState } from './harnessState';

export interface HarnessController {
	state: HarnessState;
	client: HarnessClient;
	dispatch: (action: HarnessAction) => void;
	refresh: () => Promise<void>;
	loadRun: (runId: string) => Promise<void>;
	approve: (approval: HarnessApproval, reason?: string) => Promise<void>;
	reject: (approval: HarnessApproval, reason: string) => Promise<void>;
	pin: (task: HarnessTask, pinned: boolean) => Promise<void>;
	setPriority: (task: HarnessTask, input: { priorityOverride?: number; before?: string }) => Promise<void>;
	start: (task: HarnessTask) => Promise<void>;
	setQuestions: (task: HarnessTask, askQuestions: boolean | null) => Promise<void>;
	pauseRole: (role: HarnessRoleId) => Promise<void>;
	resumeRole: (role: HarnessRoleId) => Promise<void>;
}

/**
 * Owned once by HarnessPage; every view reads from it through props. A
 * snapshot is fetched on mount and on every `refresh`; `useHarnessEvents`
 * keeps it current between snapshots, and polls when the feed is down.
 */
export function useHarness(client: HarnessClient): HarnessController {
	const [state, dispatch] = useReducer(harnessReducer, initialHarnessState);
	const controller = useRef<AbortController | undefined>(undefined);
	const keys = useRef(new Map<string, string>());

	const refresh = useCallback(async () => {
		controller.current?.abort();
		const active = new AbortController();
		controller.current = active;
		try {
			const [status, tasks, runs, approvals, cost] = await Promise.all([
				client.status(active.signal),
				client.tasks(active.signal),
				client.runs(active.signal),
				client.approvals(active.signal),
				client.cost('role', { from: new Date(startOfDay(Date.now())).toISOString() }, active.signal).catch(() => undefined)
			]);
			if (active.signal.aborted) return;
			dispatch({ type: 'snapshot', status, tasks, runs, approvals, cost });
		} catch (error) {
			if (active.signal.aborted) return;
			dispatch({ type: 'snapshot.failed', error: plainError(error, 'The harness could not be reached.') });
		}
	}, [client]);

	useEffect(() => {
		void refresh();
		return () => controller.current?.abort();
	}, [refresh]);

	const loadRun = useCallback(async (runId: string) => {
		try {
			dispatch({ type: 'detail', detail: await client.run(runId) });
		} catch (error) {
			dispatch({ type: 'snapshot.failed', error: plainError(error, 'This run could not be loaded.') });
		}
	}, [client]);

	const decide = useCallback(async (approval: HarnessApproval, decision: 'approve' | 'reject', reason?: string) => {
		// One key per approval and decision, so a retry after a dropped reply is a replay, not a second decision.
		const keyName = `${approval.id}:${decision}:${approval.revision}`;
		const key = keys.current.get(keyName) ?? newIdempotencyKey('harness');
		keys.current.set(keyName, key);
		dispatch({ type: 'decision.sent', approvalId: approval.id, decision, reason });
		try {
			const decided = decision === 'approve' ? await client.approve(approval, key, reason) : await client.reject(approval, key, reason ?? '');
			dispatch({ type: 'decision.done', approvalId: approval.id, approval: decided });
			if (!decided) void refresh();
		} catch (error) {
			dispatch({ type: 'decision.failed', approvalId: approval.id, error: plainError(error, 'Could not send. Try again.') });
		}
	}, [client, refresh]);

	const approve = useCallback((approval: HarnessApproval, reason?: string) => decide(approval, 'approve', reason), [decide]);
	const reject = useCallback((approval: HarnessApproval, reason: string) => decide(approval, 'reject', reason), [decide]);

	const saveTask = useCallback(async (save: () => Promise<HarnessTask>) => {
		try {
			dispatch({ type: 'task.saved', task: await save() });
		} catch (error) {
			dispatch({ type: 'snapshot.failed', error: plainError(error, 'The change was not saved.') });
			void refresh();
		}
	}, [refresh]);

	const pin = useCallback((task: HarnessTask, pinned: boolean) => saveTask(() => client.pin(task, pinned)), [client, saveTask]);
	const setPriority = useCallback((task: HarnessTask, input: { priorityOverride?: number; before?: string }) => saveTask(() => client.setPriority(task, input)), [client, saveTask]);
	const start = useCallback((task: HarnessTask) => saveTask(() => client.start(task, newIdempotencyKey())), [client, saveTask]);
	const setQuestions = useCallback((task: HarnessTask, askQuestions: boolean | null) => saveTask(() => client.setQuestions(task, askQuestions)), [client, saveTask]);

	const roleChange = useCallback(async (change: () => Promise<unknown>) => {
		try {
			await change();
		} catch (error) {
			dispatch({ type: 'snapshot.failed', error: plainError(error, 'The change was not saved.') });
		}
		await refresh();
	}, [refresh]);
	const pauseRole = useCallback((role: HarnessRoleId) => roleChange(() => client.pauseRole(role)), [client, roleChange]);
	const resumeRole = useCallback((role: HarnessRoleId) => roleChange(() => client.resumeRole(role)), [client, roleChange]);

	return { state, client, dispatch, refresh, loadRun, approve, reject, pin, setPriority, start, setQuestions, pauseRole, resumeRole };
}

export function plainError(error: unknown, fallback: string): string {
	if (error instanceof ApiError) {
		if (error.status === 503) return error.recoveryHint ? `${error.message} ${error.recoveryHint}` : error.message;
		if (error.status === 409) return 'Someone changed this first. The page now shows the latest.';
		return error.message || fallback;
	}
	return fallback;
}

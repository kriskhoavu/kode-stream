import { useCallback, useState } from 'react';
import type { HarnessTask } from '../../lib/types';
import type { HarnessController } from '../harness/useHarness';
import { retryApproval } from './harnessJoin';

/** The queue's actions on a task, each marking the task busy until the harness answers. */
export function useTaskActions(harness: HarnessController) {
	const [pendingIds, setPendingIds] = useState<Set<string>>(() => new Set());
	const track = useCallback(async (task: HarnessTask, change: () => Promise<void>) => {
		setPendingIds((current) => new Set(current).add(task.id));
		try {
			await change();
		} finally {
			setPendingIds((current) => { const next = new Set(current); next.delete(task.id); return next; });
		}
	}, []);
	const pin = useCallback((task: HarnessTask, pinned: boolean) => void track(task, () => harness.pin(task, pinned)), [harness, track]);
	const move = useCallback((task: HarnessTask, before?: HarnessTask) => void track(task, () => harness.setPriority(task, { before: before?.id })), [harness, track]);
	const start = useCallback((task: HarnessTask) => void track(task, () => harness.start(task)), [harness, track]);
	const retry = useCallback((task: HarnessTask) => {
		const approval = retryApproval(task, harness.state);
		if (approval) void track(task, () => harness.approve(approval));
	}, [harness, track]);
	const canRetry = useCallback((task: HarnessTask) => Boolean(retryApproval(task, harness.state)), [harness.state]);
	return { pendingIds, pin, move, start, retry, canRetry };
}

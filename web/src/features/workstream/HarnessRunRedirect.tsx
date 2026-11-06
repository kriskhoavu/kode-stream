import { useEffect } from 'react';
import { api } from '../../shared/api';
import { itemJoinKey, taskJoinKey } from './harnessJoin';

/**
 * An old link to a harness run: finds the card that holds the run, by the
 * task's Jira key in its workspace, and hands it on; with no such card the
 * board is the place to be.
 */
export function HarnessRunRedirect({ runId, onResolved, onMissing }: { runId: string; onResolved: (itemId: string) => void; onMissing: () => void }) {
	useEffect(() => {
		const controller = new AbortController();
		(async () => {
			try {
				const detail = await api.harnessRun(runId, controller.signal);
				const checkout = await api.loadWorkstreamCheckout(detail.task.workspaceId, {}, controller.signal);
				const key = taskJoinKey(detail.task);
				const item = checkout.items.find((candidate) => itemJoinKey(candidate) === key);
				if (controller.signal.aborted) return;
				if (item) onResolved(item.id); else onMissing();
			} catch {
				if (!controller.signal.aborted) onMissing();
			}
		})();
		return () => controller.abort();
	}, [runId, onResolved, onMissing]);
	return <section className="empty-state" role="status">Opening the card for this run…</section>;
}

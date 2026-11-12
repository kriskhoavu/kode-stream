import { useEffect, useRef, useState } from 'react';
import type { HarnessTask } from '../../lib/types';
import { api } from '../../shared/api';
import { itemJoinKey, taskJoinKey } from './harnessJoin';
import { HarnessRunPage } from './HarnessRunPage';

/**
 * A link to a harness run: finds the card that holds the run, by the task's
 * Jira key in its workspace, and hands it on. A run no plan card holds, a Jira
 * ticket's, shows here whole; an unknown run sends the reader to the board.
 */
export function HarnessRunRedirect({ runId, onResolved, onMissing, onBack }: { runId: string; onResolved: (itemId: string) => void; onMissing: () => void; onBack: () => void }) {
	const [ticket, setTicket] = useState<HarnessTask | undefined>();
	// The page stays open on a ticket's run, so the lookup runs once per run, not on every new callback.
	const handlers = useRef({ onResolved, onMissing });
	handlers.current = { onResolved, onMissing };
	useEffect(() => {
		const controller = new AbortController();
		setTicket(undefined);
		(async () => {
			try {
				const detail = await api.harnessRun(runId, controller.signal);
				const checkout = await api.loadWorkstreamCheckout(detail.task.workspaceId, {}, controller.signal);
				const key = taskJoinKey(detail.task);
				const item = checkout.items.find((candidate) => itemJoinKey(candidate) === key);
				if (controller.signal.aborted) return;
				if (item) handlers.current.onResolved(item.id); else setTicket(detail.task);
			} catch {
				if (!controller.signal.aborted) handlers.current.onMissing();
			}
		})();
		return () => controller.abort();
	}, [runId]);
	if (ticket) return <HarnessRunPage runId={runId} task={ticket} onBack={onBack} />;
	return <section className="empty-state" role="status">Opening this run…</section>;
}

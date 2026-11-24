import { useCallback, useEffect, useState } from 'react';
import type { HarnessApproval, HarnessRunDetail, HarnessTask, SafeSessionRecord } from '../../lib/types';
import { providerFor } from '../../lib/types';
import { api } from '../../shared/api';
import { newIdempotencyKey } from '../../shared/api/idempotency';
import { GateCard } from '../harness/GateCard';
import type { SentDecision } from '../harness/harnessState';
import { decisionLine } from '../harness/harnessState';
import { SessionBrowserTerminal } from './SessionBrowserTerminal';
import { SessionChangesPanel } from './SessionChangesPanel';
import type { ChangesSource } from './SessionChangesPanel';
import { formatSessionDate, isSessionLive } from './sessionDisplay';

export const reviewIntent = 'harness_review';

export type ReviewDecide = {
	approve: (approval: HarnessApproval, reason?: string) => void;
	reject: (approval: HarnessApproval, reason: string, decision?: 'reject' | 'replan') => void;
	decisions: Record<string, SentDecision>;
};

// The session runs on the run's branch, so the Session Browser's own list does not hold it.
export function HarnessReviewPanel({ detail, task, changes, decide }: {
	detail: HarnessRunDetail;
	task?: HarnessTask;
	changes: ChangesSource;
	decide?: ReviewDecide;
}) {
	const { run } = detail;
	const [sessions, setSessions] = useState<SafeSessionRecord[]>([]);
	const [starting, setStarting] = useState(false);
	const [error, setError] = useState('');
	const [tab, setTab] = useState<'session' | 'changes'>('session');

	const load = useCallback(async (signal?: AbortSignal) => {
		const records = await api.aiSessionRecords(run.workspaceId, run.branch, signal);
		if (!signal?.aborted) setSessions(records.filter((record) => record.intent === reviewIntent));
	}, [run.workspaceId, run.branch]);

	useEffect(() => {
		const controller = new AbortController();
		setError('');
		load(controller.signal).catch((caught) => { if (!controller.signal.aborted) setError(caught instanceof Error ? caught.message : 'The review sessions could not be loaded.'); });
		return () => controller.abort();
	}, [load]);

	const review = detail.stages.filter((stage) => stage.kind === 'review').at(-1);
	const approvals = detail.approvals.filter((approval) => approval.gate === 'review');
	const waiting = approvals.find((approval) => approval.state === 'requested');
	const decided = approvals.filter((approval) => approval.state !== 'requested').at(-1);
	const session = sessions[0];
	const live = session ? isSessionLive(session) : false;
	const canOpen = Boolean(waiting) && !live && !starting;

	const open = async () => {
		setStarting(true);
		setError('');
		try {
			const settings = await api.aiSettings();
			const result = await api.startHarnessReviewSession(run.id, { provider: providerFor(settings, run.workspaceId), idempotencyKey: newIdempotencyKey('review'), columns: 100, rows: 30 });
			if (result.record) setSessions((current) => [result.record as SafeSessionRecord, ...current.filter((record) => record.id !== result.record?.id)]);
			else await load();
			setTab('session');
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'The session could not be started.');
		} finally {
			setStarting(false);
		}
	};

	return <div className="session-harness-review">
		<div className="session-harness-review-head">
			{waiting && decide
				? <GateCard
					approval={waiting}
					task={task}
					sent={decide.decisions[waiting.id]}
					onApprove={(reason) => decide.approve(waiting, reason)}
					onReject={(reason, decision) => decide.reject(waiting, reason, decision)}
					onSession={canOpen ? () => void open() : undefined}
					sessionLabel={session ? 'Open a new session' : 'Try it in a session'}
				/>
				: <p className="session-harness-agent muted">{waiting
					? 'This run waits for your review. Decide on its run page.'
					: decided
						? `${decisionLine(decided)}${decided.reason ? `: “${decided.reason}”` : ''}.`
						: review
							? 'Review has not started.'
							: run.state === 'running'
								? 'Review comes once the tests pass. The run reaches it on its own.'
								: run.state === 'passed'
									? 'This run handed off without Review, which was turned off.'
									: 'This run stopped before Review.'}</p>}
			{starting && <p className="session-harness-agent muted" role="status">Starting a session in the run&apos;s worktree…</p>}
			{error && <p className="error" role="alert">{error}</p>}
		</div>
		{(session || waiting) && <>
			<div className="hx-tabs session-browser-detail-tabs" role="tablist" aria-label="Review tabs">
				<button type="button" role="tab" className={tab === 'session' ? 'on' : ''} aria-selected={tab === 'session'} onClick={() => setTab('session')}>Your session</button>
				<button type="button" role="tab" className={tab === 'changes' ? 'on' : ''} aria-selected={tab === 'changes'} onClick={() => setTab('changes')}>Changes</button>
			</div>
			{tab === 'session' && (session
				? <div className="session-harness-review-session">
					<p className="session-harness-review-note">{live ? `Running in the run's worktree on ${run.branch} since ${formatSessionDate(session.startedAt)}.` : `The session ended ${formatSessionDate(session.endedAt ?? session.lastKnownAt)}. What it changed stays in the worktree.`}</p>
					{live && <SessionBrowserTerminal record={session} visible onReload={() => load()} />}
				</div>
				: <p className="session-harness-review-note">No session yet. Try it in a session opens one in the run&apos;s worktree, on {run.branch}, with a brief about the change.</p>)}
			{tab === 'changes' && <SessionChangesPanel source={changes} />}
		</>}
	</div>;
}

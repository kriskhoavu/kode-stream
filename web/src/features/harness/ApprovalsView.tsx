import type { HarnessApproval } from '../../lib/types';
import type { HarnessState } from './harnessState';
import { decisionLine, waitingApprovals } from './harnessState';
import { GateCard } from './GateCard';
import { formatAgo, formatTime, gateTitles } from './vocabulary';
import { useNow } from './useNow';

/** Every gate waiting on a person, then the recent history of decisions from every channel. */
export function ApprovalsView({ state, onApprove, onReject, onOpenRun }: {
	state: HarnessState;
	onApprove: (approval: HarnessApproval, reason?: string) => void;
	onReject: (approval: HarnessApproval, reason: string) => void;
	onOpenRun: (runId: string) => void;
}) {
	const now = useNow(60_000);
	const waiting = waitingApprovals(state);
	const history = Object.values(state.approvals)
		.filter((approval) => approval.state !== 'requested')
		.sort((a, b) => Date.parse(b.decidedAt ?? b.requestedAt) - Date.parse(a.decidedAt ?? a.requestedAt));
	return (
		<div className="hx-inbox">
			<section className="hx-section" aria-labelledby="hx-approvals-open">
				<h2 className="hx-section-title" id="hx-approvals-open">Waiting on you <small>· {waiting.length}</small></h2>
				{waiting.map((approval) => (
					<GateCard key={approval.id} approval={approval} task={state.tasks[approval.taskId]} sent={state.decisions[approval.id]}
						onApprove={(reason) => onApprove(approval, reason)} onReject={(reason) => onReject(approval, reason)}
						onOpen={approval.runId ? () => onOpenRun(approval.runId as string) : undefined} />
				))}
				{waiting.length === 0 && <div className="hx-card hx-empty"><span>Nothing is waiting on you.</span></div>}
			</section>
			<section className="hx-section" aria-labelledby="hx-approvals-history">
				<h2 className="hx-section-title" id="hx-approvals-history">Decided</h2>
				{history.length === 0 ? <p className="hx-muted">No decisions yet.</p> : (
					<div className="hx-card hx-table-card">
						<table className="hx-table">
							<thead><tr><th scope="col">Task</th><th scope="col">Asked to</th><th scope="col">Outcome</th><th scope="col">When</th></tr></thead>
							<tbody>
								{history.map((approval) => {
									const task = state.tasks[approval.taskId];
									return (
										<tr key={approval.id}>
											<td><b>{task?.source.key ?? '—'}</b> {task?.title}</td>
											<td>{gateTitles[approval.gate]}</td>
											<td>{decisionLine(approval, state.decisions[approval.id])}{approval.reason ? <span className="hx-muted"> · “{approval.reason}”</span> : null}</td>
											<td className="hx-num" title={approval.decidedAt}>{formatTime(approval.decidedAt)} · {formatAgo(approval.decidedAt, now)}</td>
										</tr>
									);
								})}
							</tbody>
						</table>
					</div>
				)}
			</section>
		</div>
	);
}

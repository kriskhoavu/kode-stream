import { useState } from 'react';
import { ChevronRight } from 'lucide-react';
import type { HarnessAgentRun, HarnessApproval, HarnessRunDetail, HarnessStage } from '../../lib/types';
import { HxStatusBadge } from './HxStatusBadge';
import { LiveTimer } from './LiveTimer';
import { agentRunStatus, channelLabels, formatCost, formatMinutes, plural, stageStatus } from './vocabulary';

const stepWords: Record<HarnessAgentRun['role'], string> = {
	planner: 'plan',
	plan_reviewer: 'plan check',
	coder: 'build',
	code_reviewer: 'code check',
	qa_fixer: 'fix'
};

type LedgerRow = { id: string; step: string; agentRun?: HarnessAgentRun; stage?: HarnessStage };
type LedgerGroup = { id: string; title: string; note?: string; rows: LedgerRow[]; open: boolean };

function decidedNote(approval: HarnessApproval | undefined): string | undefined {
	if (!approval || approval.state === 'requested') return approval ? 'Waiting on you' : undefined;
	const who = approval.decidedBy?.displayName ?? 'Someone';
	const verb = approval.state === 'approved' ? 'approved' : approval.state === 'rejected' ? 'asked for changes' : approval.state;
	return `${who} ${verb}${approval.channel && approval.channel !== 'dashboard' ? ` ${channelLabels[approval.channel]}` : ''}`;
}

/** One table of every step the run took, grouped by stage, with a total. Numbers only: nothing here moves. */
export function ledgerGroups(detail: HarnessRunDetail): LedgerGroup[] {
	const approvalOf = (gate: HarnessApproval['gate']) => detail.approvals.filter((approval) => approval.gate === gate).at(-1);
	const groups: LedgerGroup[] = [];
	for (const stage of [...detail.stages].sort((a, b) => a.ordinal - b.ordinal)) {
		const agentRuns = detail.agentRuns
			.filter((agentRun) => agentRun.stageId === stage.id)
			.sort((a, b) => a.round - b.round || Date.parse(a.startedAt ?? '') - Date.parse(b.startedAt ?? ''));
		const rows: LedgerRow[] = agentRuns.map((agentRun) => {
			const again = agentRun.round > 1 && (agentRun.role === 'planner' || agentRun.role === 'coder') ? ' again' : '';
			const retry = (agentRun.attempt ?? 1) > 1 ? ` · try ${agentRun.attempt}` : '';
			return { id: agentRun.id, step: `Round ${agentRun.round} · ${stepWords[agentRun.role]}${again}${retry}`, agentRun };
		});
		if (stage.kind === 'qa' && stage.state !== 'pending') rows.unshift({ id: stage.id, step: `Run ${stage.round} of ${stage.maxRounds} · tests`, stage });
		const open = false;
		if (stage.kind === 'plan') groups.push({ id: stage.id, title: 'Plan', note: decidedNote(approvalOf('plan')), rows, open });
		if (stage.kind === 'implement') groups.push({ id: stage.id, title: 'Build', rows, open });
		if (stage.kind === 'qa') groups.push({ id: stage.id, title: 'Test', note: decidedNote(approvalOf('draft_pr')), rows, open });
		if (stage.kind === 'draft_pr' && stage.state !== 'pending') groups.push({ id: stage.id, title: 'Draft pull request', rows: [{ id: stage.id, step: 'Push and open', stage }], open });
	}
	const kept = groups.filter((group) => group.rows.length > 0);
	// A stage kind that ran more than once (a retry after a stop) is numbered: "Plan 2" is the second Plan stage.
	const counts = new Map<string, number>();
	for (const group of kept) counts.set(group.title, (counts.get(group.title) ?? 0) + 1);
	const seen = new Map<string, number>();
	for (const group of kept) {
		if ((counts.get(group.title) ?? 0) < 2) continue;
		const nth = (seen.get(group.title) ?? 0) + 1;
		seen.set(group.title, nth);
		group.title = `${group.title} ${nth}`;
	}
	// Only the latest stage starts open; everything before it is history and folds.
	const last = kept.at(-1);
	if (last) last.open = true;
	return kept;
}

function sumRows(rows: LedgerRow[]) {
	return rows.reduce((sum, row) => {
		const source = row.agentRun ?? row.stage;
		return { turns: sum.turns + (source?.turns ?? 0), durationMs: sum.durationMs + (source?.durationMs ?? 0), costUsd: sum.costUsd + (source?.costUsd ?? 0) };
	}, { turns: 0, durationMs: 0, costUsd: 0 });
}

export function RunLedger({ detail, onOpenAgentRun }: { detail: HarnessRunDetail; onOpenAgentRun?: (agentRunId: string) => void }) {
	const groups = ledgerGroups(detail);
	const rows = groups.flatMap((group) => group.rows);
	const total = sumRows(rows);
	// Which groups the person folded or unfolded by hand; the rest follow the stage's state.
	const [folded, setFolded] = useState<Record<string, boolean>>({});
	const isOpen = (group: LedgerGroup) => folded[group.id] ?? group.open;
	return (
		<section className="hx-card hx-table-card" aria-label="What each step took">
			<table className="hx-table hx-ledger">
				<thead><tr><th scope="col">Step</th><th scope="col">Result</th><th scope="col" className="num">Turns</th><th scope="col" className="num">Time</th><th scope="col" className="num">Cost</th></tr></thead>
				<tbody>
					{groups.length === 0 && <tr><td colSpan={5} className="hx-muted">Nothing has run yet.</td></tr>}
					{groups.map((group) => {
						const open = isOpen(group);
						const sum = sumRows(group.rows);
						return [
							<tr className="grp" key={group.id}>
								<th scope="rowgroup">
									<span className="hx-ledger-group">
										<button type="button" className="hx-ledger-fold" aria-expanded={open} onClick={() => setFolded((prev) => ({ ...prev, [group.id]: !open }))}>
											<ChevronRight size={14} aria-hidden="true" />{group.title}
										</button>
										<span className="hx-ledger-count">{plural(group.rows.length, 'step')}</span>
									</span>
								</th>
								<td className="note">{group.note}</td>
								<td className="num">{sum.turns}</td>
								<td className="num">{formatMinutes(sum.durationMs)}</td>
								<td className="num">{formatCost(sum.costUsd)}</td>
							</tr>,
							...(open ? group.rows.map((row) => <LedgerRowView key={row.id} row={row} onOpen={onOpenAgentRun} />) : [])
						];
					})}
					{rows.length > 0 && (
						<tr className="total"><th scope="row">Total so far</th><td /><td className="num">{total.turns}</td><td className="num">{formatMinutes(total.durationMs)}</td><td className="num">{formatCost(total.costUsd)}</td></tr>
					)}
				</tbody>
			</table>
		</section>
	);
}

function LedgerRowView({ row, onOpen }: { row: LedgerRow; onOpen?: (agentRunId: string) => void }) {
	const agentRun = row.agentRun;
	const stage = row.stage;
	const status = agentRun ? agentRunStatus(agentRun.state, agentRun.verdict) : stage ? stageStatus(stage.state) : 'not_started';
	const review = agentRun && (agentRun.role === 'plan_reviewer' || agentRun.role === 'code_reviewer');
	const label = review && agentRun.verdict === 'approve' ? 'Approved' : undefined;
	const running = agentRun?.state === 'running' || stage?.state === 'running';
	return (
		<tr>
			<td>{agentRun && onOpen ? <button type="button" className="hx-ledger-step" onClick={() => onOpen(agentRun.id)} title="Open this conversation">{row.step}</button> : row.step}</td>
			<td>
				<HxStatusBadge status={status} label={label} size="sm" />
				{review && agentRun.state === 'succeeded' && <span className="hx-muted"> {agentRun.blocking} must fix · {plural(agentRun.advisory, 'suggestion')}</span>}
			</td>
			<td className="num">{agentRun ? agentRun.turns : '—'}</td>
			<td className="num">{running ? <LiveTimer since={agentRun?.startedAt ?? stage?.startedAt} /> : formatMinutes(agentRun?.durationMs ?? stage?.durationMs)}</td>
			<td className="num">{formatCost(agentRun?.costUsd ?? stage?.costUsd)}</td>
		</tr>
	);
}

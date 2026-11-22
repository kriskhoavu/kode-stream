import type { HarnessAgentRun, HarnessRunDetail, SessionFile } from '../../../lib/types';
import { prLabel } from '../pipeline';
import { agentRoleLabels, formatCost, formatMinutes, hxStatuses, plural, stageKindLabels, stageStatus } from '../vocabulary';
import { type WorkbenchKey } from './workbenchTypes';

export function TaskInfo({ detail, planCount, codeFiles, from }: { detail: HarnessRunDetail; planCount: number; codeFiles: SessionFile[]; from?: string }) {
	const { run, task } = detail;
	const tests = codeFiles.filter((file) => /(^|\/)(test|tests|__tests__)\/|[._-]test\.|Test\.java$|_test\.go$/.test(file.path)).length;
	return (
		<dl>
			<dt>Item</dt><dd>{task.source.key}</dd>
			<dt>Status</dt><dd><span className={`hx-badge hx-badge-sm hx-tone-${hxStatuses[runTone(run.state)].tone}`}>{hxStatuses[runTone(run.state)].label}</span></dd>
			<dt>Branch</dt><dd title={run.branch}>{run.remoteBranch && run.prUrl ? run.remoteBranch : run.branch}</dd>
			{run.prUrl && <><dt>Pull request</dt><dd><a className="hx-link" href={run.prUrl} target="_blank" rel="noreferrer">{prLabel(run.prUrl)} ↗</a></dd></>}
			<dt>Plan</dt><dd>{plural(planCount, 'file')}</dd>
			<dt>Code</dt><dd>{plural(codeFiles.length, 'file')}{tests ? ` · ${plural(tests, 'test')}` : ''}</dd>
			{from && <><dt>Read from</dt><dd>{from === 'branch' ? 'its branch' : 'its own copy'}</dd></>}
			<dt>Agent runs</dt><dd>{detail.agentRuns.length} · {formatMinutes(run.totals.durationMs)}</dd>
			<dt>Cost</dt><dd>{formatCost(run.totals.costUsd)}</dd>
			<dt>Started</dt><dd>{new Date(run.startedAt).toLocaleString()}</dd>
		</dl>
	);
}

export function runTone(state: HarnessRunDetail['run']['state']) {
	switch (state) {
		case 'passed': return 'done' as const;
		case 'running': return 'working' as const;
		case 'blocked': return 'waiting' as const;
		case 'cancelled': return 'stopped' as const;
		case 'failed': return 'failed' as const;
		default: return 'not_started' as const;
	}
}

/** The questions an agent stopped to ask, each with the answer a person gave. */
export function Answers({ detail }: { detail: HarnessRunDetail }) {
	const answered = detail.approvals.filter((approval) => approval.gate === 'question' && approval.context?.questions?.length);
	if (answered.length === 0) return null;
	return (
		<div className="hx-workbench-block">
			<h3 className="hx-workbench-subhead">Questions you answered</h3>
			{answered.map((approval) => (
				<div key={approval.id} className="hx-workbench-qa">
					<div className="q"><b>{approval.context?.role ? agentRoleLabels[approval.context.role] : 'Agent'}</b>{approval.context?.questions?.map((question) => <p key={question}>{question}</p>)}</div>
					{approval.reason ? <div className="a"><b>{approval.decidedBy?.displayName || 'You'}</b><p>{approval.reason}</p></div> : <p className="hx-muted">Not answered yet.</p>}
				</div>
			))}
		</div>
	);
}

export function CheckerList({ agentRuns, onOpen, onOpenRun }: { agentRuns: HarnessAgentRun[]; onOpen: (path: string) => void; onOpenRun: (agentRunId: string) => void }) {
	const reviews = agentRuns
		.filter((agentRun) => (agentRun.role === 'plan_reviewer' || agentRun.role === 'code_reviewer') && agentRun.state === 'succeeded')
		.sort((a, b) => Date.parse(b.startedAt ?? '') - Date.parse(a.startedAt ?? ''));
	if (reviews.length === 0) return <p className="hx-workbench-empty">The Checker has not reviewed anything in this run yet.</p>;
	return (
		<div>
			{reviews.map((review) => (
				<section key={review.id} className="hx-workbench-block">
					<button type="button" className="hx-workbench-subhead" onClick={() => onOpenRun(review.id)} title="Open the Checker's conversation">
						{review.role === 'plan_reviewer' ? 'Plan check' : 'Code check'} · round {review.round}
						<span className={`hx-badge hx-badge-sm ${review.verdict === 'approve' ? 'hx-tone-done' : 'hx-tone-waiting'}`}>{review.verdict === 'approve' ? 'Approved' : 'Changes asked'}</span>
					</button>
					{(review.findings ?? []).length === 0 && <p className="hx-workbench-empty">Nothing to fix and no suggestions.</p>}
					{(review.findings ?? []).map((finding, index) => (
						<button key={index} type="button" className="hx-workbench-finding" disabled={!finding.path} onClick={() => finding.path && onOpen(finding.path)}>
							<span className={`hx-badge hx-badge-sm ${finding.resolved ? 'hx-tone-done' : finding.severity === 'blocking' ? 'hx-tone-stopped' : 'hx-tone-idle'}`}>{finding.resolved ? 'Fixed' : finding.severity === 'blocking' ? 'Must fix' : 'Suggestion'}</span>
							<span className="hx-workbench-finding-title">{finding.title}</span>
							{finding.path && <code>{finding.path.split('/').pop()}{finding.line ? `:${finding.line}` : ''}</code>}
						</button>
					))}
				</section>
			))}
		</div>
	);
}

/** Each stage of the run in order, with its status, rounds, time and cost; a stage with conversations opens them. */
export function StepList({ detail, onOpen }: { detail: HarnessRunDetail; onOpen: (key: WorkbenchKey) => void }) {
	const stages = [...detail.stages].sort((a, b) => a.ordinal - b.ordinal || Date.parse(a.startedAt ?? '') - Date.parse(b.startedAt ?? ''));
	if (stages.length === 0) return <p className="hx-workbench-empty">No step has started yet.</p>;
	const counts: Partial<Record<string, number>> = {};
	const repeated = (kind: string) => stages.filter((stage) => stage.kind === kind).length > 1;
	return (
		<>
			<ul className="hx-workbench-steps">
				{stages.map((stage) => {
					counts[stage.kind] = (counts[stage.kind] ?? 0) + 1;
					const title = repeated(stage.kind) ? `${stageKindLabels[stage.kind]} ${counts[stage.kind]}` : stageKindLabels[stage.kind];
					const status = hxStatuses[stageStatus(stage.state)];
					const talks = detail.agentRuns.some((agentRun) => agentRun.stageId === stage.id);
					const meta = [stage.maxRounds > 1 && stage.round > 0 ? `round ${stage.round} of ${stage.maxRounds}` : '', stage.durationMs ? formatMinutes(stage.durationMs) : '', stage.costUsd ? formatCost(stage.costUsd) : ''].filter(Boolean).join(' · ');
					const body = <><b>{title}</b><span className={`hx-badge hx-badge-sm hx-tone-${status.tone}`}>{status.label}</span>{meta && <small>{meta}</small>}</>;
					return <li key={stage.id}>{talks ? <button type="button" className="hx-workbench-step" onClick={() => onOpen(`stage:${stage.id}`)} title={`Read ${title}'s conversations`}>{body}</button> : <div className="hx-workbench-step">{body}</div>}</li>;
				})}
			</ul>
			<p className="hx-workbench-step-total">Total<span>{plural(detail.agentRuns.length, 'agent run')} · {formatMinutes(detail.run.totals.durationMs)} · {formatCost(detail.run.totals.costUsd)}</span></p>
		</>
	);
}

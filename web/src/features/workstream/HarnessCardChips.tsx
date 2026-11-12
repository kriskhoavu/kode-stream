import { Bot, Hand } from 'lucide-react';
import { HxStatusBadge } from '../harness/HxStatusBadge';
import { WorkerChip } from '../harness/WorkerChip';
import { agentRoleLabels, hxStatuses, stageKindWhere } from '../harness/vocabulary';
import type { CardWork } from './harnessJoin';

/** The harness's line on a card: who works on it, which stage it is in, and how far the run is. */
export function HarnessCardChips({ work }: { work: CardWork }) {
	const { run, status } = work;
	const agent = run?.activeAgentRun;
	const working = status === 'working';
	const note = work.synthetic ? planNote(work) : '';
	const said = working ? [`Harness: ${hxStatuses[status].label}`, work.stageLabel] : [`Harness: ${badgeLabel(work) ?? hxStatuses[status].label}`];
	const label = [...said, note].filter(Boolean).join(', ');
	return (
		<div className="plan-card-harness" role="group" aria-label={label}>
			<span className="plan-card-harness-chips">
				{agent
					? <WorkerChip engine={agent.engine} model={agent.model} role={agentRoleLabels[agent.role]} />
					: working
						? <WorkerChip role={run?.currentStage?.kind === 'qa' ? 'Tester' : 'Harness'} />
						: <span className="hx-chip" title="The harness works on this card"><Bot aria-hidden="true" size={12} />Harness</span>}
				<HxStatusBadge status={status} size="sm" label={badgeLabel(work)} />
				{note && <span className="plan-card-no-plan">{note}</span>}
			</span>
			{work.progress !== undefined && (
				<span className="plan-card-progress" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(work.progress * 100)} aria-label="Run progress">
					<span style={{ width: `${Math.round(work.progress * 100)}%` }} />
				</span>
			)}
		</div>
	);
}

/** The chip on a card no harness task holds. Shown only when the board knows the harness is on, so a quiet board stays quiet. */
export function ByHandChip() {
	return <span className="hx-chip plan-card-by-hand" title="A person works on this card"><Hand aria-hidden="true" size={12} />By hand</span>;
}

/**
 * The status badge's words: the stage while it works; where it ended once a run
 * stops or fails ("Stopped while planning"), so a stop never reads as a stage
 * name, which sounds like the plan itself stopped; the status and the stage otherwise.
 */
export function badgeLabel(work: CardWork): string | undefined {
	if (!work.stageLabel) return undefined;
	if (work.status === 'working') return work.stageLabel;
	const kind = work.run?.currentStage?.kind;
	if (kind && (work.status === 'stopped' || work.status === 'failed')) return `${hxStatuses[work.status].label} ${stageKindWhere[kind]}`;
	return `${hxStatuses[work.status].label} · ${work.stageLabel}`;
}

/** What a ticket card says about its plan: none until a run has written one, then that it waits for the merge. */
export function planNote(work: CardWork): string {
	const kind = work.run?.currentStage?.kind;
	return kind && kind !== 'plan' ? 'plan not merged yet' : 'no plan yet';
}

import { Bot, Hand } from 'lucide-react';
import { HxStatusBadge } from '../harness/HxStatusBadge';
import { WorkerChip } from '../harness/WorkerChip';
import { agentRoleLabels, hxStatuses, stageKindWhere } from '../harness/vocabulary';
import type { CardWork } from './harnessJoin';

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

export function ByHandChip() {
	return <span className="hx-chip plan-card-by-hand" title="A person works on this card"><Hand aria-hidden="true" size={12} />By hand</span>;
}

// A stop reads as where it ended, so it never sounds like the plan itself stopped.
export function badgeLabel(work: CardWork): string | undefined {
	if (work.status === 'reporter' && work.task.waitingFor) return `Waiting for ${work.task.waitingFor}`;
	if (!work.stageLabel) return undefined;
	if (work.status === 'working') return work.stageLabel;
	const kind = work.run?.currentStage?.kind;
	if (kind && (work.status === 'stopped' || work.status === 'failed')) return `${hxStatuses[work.status].label} ${stageKindWhere[kind]}`;
	return `${hxStatuses[work.status].label} · ${work.stageLabel}`;
}

export function planNote(work: CardWork): string {
	const kind = work.run?.currentStage?.kind;
	return kind && kind !== 'plan' ? 'plan not merged yet' : 'no plan yet';
}

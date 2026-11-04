import { CircleCheck, LoaderCircle } from 'lucide-react';
import type { HarnessApproval, HarnessCostSummary, HarnessRoleHealth, HarnessRun, HarnessTask } from '../../lib/types';
import type { HarnessState } from './harnessState';
import { decidedToday, decisionLine, doneToday, waitingApprovals, workingRuns } from './harnessState';
import { GateCard } from './GateCard';
import { HxStatusBadge } from './HxStatusBadge';
import { LiveTimer } from './LiveTimer';
import { SourceBadge } from './SourceBadge';
import { WorkerChip } from './WorkerChip';
import { useNow } from './useNow';
import { agentRoleLabels, formatAgo, formatCost, formatMinutes, formatTime, gateTitles, modelLabel, plural, roleLabels, roleOrder, stageKindDoing } from './vocabulary';

export function InboxView({ state, newestApprovalId, onApprove, onReject, onOpenRun }: {
	state: HarnessState;
	/** The approval that arrived while the page was open, which takes focus and the attention ring. */
	newestApprovalId?: string;
	onApprove: (approval: HarnessApproval, reason?: string) => void;
	onReject: (approval: HarnessApproval, reason: string) => void;
	onOpenRun: (runId: string) => void;
}) {
	const now = useNow(60_000);
	const waiting = waitingApprovals(state);
	// Cards this browser decided stay in place, now saying what happened.
	const justDecided = Object.keys(state.decisions)
		.map((id) => state.approvals[id])
		.filter((approval): approval is HarnessApproval => Boolean(approval) && approval.state !== 'requested');
	const working = workingRuns(state);
	const done = doneToday(state, now);
	const decisions = decidedToday(state, now);

	return (
		<div className="hx-inbox">
			<section className="hx-section" aria-labelledby="hx-needs-you">
				<h2 className="hx-section-title" id="hx-needs-you">Needs you <small>· {waiting.length}</small></h2>
				{waiting.map((approval) => (
					<GateCard
						key={approval.id}
						approval={approval}
						task={state.tasks[approval.taskId]}
						sent={state.decisions[approval.id]}
						attention={approval.id === newestApprovalId}
						autoFocus={approval.id === newestApprovalId}
						onApprove={(reason) => onApprove(approval, reason)}
						onReject={(reason) => onReject(approval, reason)}
						onOpen={approval.runId ? () => onOpenRun(approval.runId as string) : undefined}
					/>
				))}
				{justDecided.map((approval) => (
					<GateCard
						key={approval.id}
						approval={approval}
						task={state.tasks[approval.taskId]}
						sent={state.decisions[approval.id]}
						onApprove={() => undefined}
						onReject={() => undefined}
						onOpen={approval.runId ? () => onOpenRun(approval.runId as string) : undefined}
					/>
				))}
				{waiting.length === 0 && (
					<div className="hx-card hx-empty">
						<CircleCheck aria-hidden="true" size={22} />
						<span>Nothing needs you right now. We'll let you know here and in Webex.</span>
					</div>
				)}
			</section>

			<section className="hx-section" aria-labelledby="hx-working">
				<h2 className="hx-section-title" id="hx-working">Working now <small>· {working.length}</small></h2>
				{working.map((run) => <WorkingRow key={run.id} run={run} task={state.tasks[run.taskId]} onOpen={() => onOpenRun(run.id)} />)}
				{working.length === 0 && <p className="hx-muted hx-empty-line">Nothing is running. The next task in line starts when a Builder is free.</p>}
			</section>

			<section className="hx-section" aria-labelledby="hx-done">
				<h2 className="hx-section-title" id="hx-done">Done today <small>· {done.length}</small></h2>
				{done.map((entry) => entry.kind === 'run'
					? <DoneRow key={entry.run.id} task={entry.task} run={entry.run} onOpen={() => onOpenRun(entry.run.id)} />
					: <DoneRow key={entry.task.id} task={entry.task} />)}
				{done.length === 0 && <p className="hx-muted hx-empty-line">Nothing finished yet today.</p>}
			</section>

			<div className="hx-inbox-pair">
				<SpendToday cost={state.cost} />
				<section className="hx-card hx-panel" aria-labelledby="hx-decisions">
					<h2 className="hx-section-title" id="hx-decisions">Decisions today <small>· {decisions.length}</small></h2>
					{decisions.length === 0
						? <p className="hx-muted">No one has decided anything yet today.</p>
						: <ul className="hx-decisions">{decisions.slice(0, 6).map((approval) => <DecisionLine key={approval.id} approval={approval} task={state.tasks[approval.taskId]} />)}</ul>}
				</section>
			</div>

			<section className="hx-section" aria-labelledby="hx-team">
				<h2 className="hx-section-title" id="hx-team">Team health</h2>
				<TeamHealth roles={state.status?.roles ?? []} now={now} />
			</section>
		</div>
	);
}

function WorkingRow({ run, task, onOpen }: { run: HarnessRun; task?: HarnessTask; onOpen: () => void }) {
	const stage = run.currentStage;
	const agent = run.activeAgentRun;
	const roundWord = stage?.kind === 'qa' ? 'run' : 'round';
	const doing = stage ? `${stageKindDoing[stage.kind]}${stage.maxRounds > 1 ? ` · ${roundWord} ${stage.round} of ${stage.maxRounds}` : ''}` : 'Starting';
	return (
		<div className="hx-row">
			<TaskKey task={task} />
			<span className="hx-row-title"><span>{task?.title ?? run.branch}</span><small className="hx-muted">{doing}</small></span>
			<span className="hx-badge hx-tone-running hx-status-working"><LoaderCircle aria-hidden="true" strokeWidth={2.5} />Working <LiveTimer since={stage?.startedAt ?? run.startedAt} label="for" /></span>
			{agent ? <WorkerChip engine={agent.engine} model={agent.model} role={agentRoleLabels[agent.role]} /> : <WorkerChip role={stage?.kind === 'qa' ? 'Tester' : 'Harness'} />}
			<button type="button" className="hx-link" onClick={onOpen}>See progress</button>
		</div>
	);
}

function DoneRow({ task, run, onOpen }: { task?: HarnessTask; run?: HarnessRun; onOpen?: () => void }) {
	const manual = !run;
	const subline = manual
		? `Finished by ${task?.finishedOutside?.by ?? 'someone'}, outside the harness`
		: run?.prUrl ? `Draft pull request ${prLabel(run.prUrl)}` : 'Finished';
	return (
		<div className="hx-row">
			<TaskKey task={task} />
			<span className="hx-row-title">
				<span>{task?.title ?? run?.branch}</span>
				{run?.prUrl ? <a className="hx-muted" href={run.prUrl} target="_blank" rel="noreferrer">{subline}</a> : <small className="hx-muted">{subline}</small>}
			</span>
			<HxStatusBadge status={manual ? 'manual' : 'done'} />
			<span className="hx-muted hx-num">{run ? `${formatMinutes(run.totals.durationMs)} · ${formatCost(run.totals.costUsd)}` : '—'}</span>
			{onOpen ? <button type="button" className="hx-link" onClick={onOpen}>Open</button> : <span />}
		</div>
	);
}

function TaskKey({ task }: { task?: HarnessTask }) {
	return (
		<span className="hx-row-key">
			<b>{task?.source.key ?? '—'}</b>
			{task && <SourceBadge kind={task.source.kind} />}
		</span>
	);
}

function SpendToday({ cost }: { cost?: HarnessCostSummary }) {
	const byRole = new Map<string, number>();
	for (const group of cost?.groups ?? []) {
		const label = group.key in agentRoleLabels ? agentRoleLabels[group.key as keyof typeof agentRoleLabels] : group.key in roleLabels ? roleLabels[group.key as keyof typeof roleLabels] : group.key;
		byRole.set(label, (byRole.get(label) ?? 0) + group.costUsd);
	}
	return (
		<section className="hx-card hx-panel" aria-labelledby="hx-spend">
			<h2 className="hx-section-title" id="hx-spend">Spend today</h2>
			{cost ? (
				<>
					<p className="hx-spend-total"><b className="hx-num">{formatCost(cost.totalUsd)}</b><span className="hx-muted">across {plural(cost.agentRuns, 'agent run')}</span></p>
					<p className="hx-spend-roles hx-muted hx-num">{[...byRole].map(([label, usd]) => <span key={label}>{label} {formatCost(usd)}</span>)}</p>
				</>
			) : <p className="hx-muted">No spend recorded yet today.</p>}
		</section>
	);
}

function DecisionLine({ approval, task }: { approval: HarnessApproval; task?: HarnessTask }) {
	const who = approval.decidedBy?.displayName ?? 'Someone';
	const what = approval.state === 'approved' ? `approved: ${gateTitles[approval.gate]}` : approval.state === 'rejected' ? `asked for changes on` : decisionLine(approval).toLowerCase();
	return (
		<li>
			<b>{who}</b> {what} <b>{task?.source.key ?? 'a task'}</b>
			<span className="hx-muted"> · {formatTime(approval.decidedAt)} · {approval.channel === 'webex' ? 'via Webex' : approval.channel === 'api' ? 'via the API' : 'here'}</span>
		</li>
	);
}

function TeamHealth({ roles, now }: { roles: HarnessRoleHealth[]; now: number }) {
	const byRole = new Map(roles.map((role) => [role.role, role]));
	return (
		<div className="hx-team">
			{roleOrder.map((id) => {
				const role = byRole.get(id);
				return (
					<div className={role?.paused ? 'hx-tile paused' : 'hx-tile'} key={id}>
						<b>{roleLabels[id]}</b>
						{role?.paused
							? <HxStatusBadge status="stopped" size="sm" label={role.pausedBy ? `Paused by ${role.pausedBy}` : 'Paused'} />
							: <span>{roleSummary(role, now)}</span>}
					</div>
				);
			})}
		</div>
	);
}

function roleSummary(role: HarnessRoleHealth | undefined, now: number): string {
	if (!role) return 'not reported';
	if (role.role === 'scout') return `looked ${formatAgo(role.lastRunAt, now)}${role.found ? ` · ${role.found} new` : ''}`;
	if (role.role === 'sorter') return `${role.queued} in line`;
	if (role.working === 0) return role.queued ? `${role.queued} waiting` : 'idle';
	const free = Math.max(0, role.slots - role.working);
	const model = role.model ? ` · ${modelLabel(role.model)}` : '';
	return free ? `${role.working} working · ${free} free` : `${role.working} working${model}`;
}

function prLabel(url: string): string {
	const match = /\/([^/]+)\/pull(?:s|-requests)?\/(\d+)/.exec(url);
	return match ? `${match[1]}#${match[2]}` : url;
}

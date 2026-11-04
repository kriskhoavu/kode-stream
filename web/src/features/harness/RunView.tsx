import { useEffect, useMemo } from 'react';
import { RefreshCw } from 'lucide-react';
import type { HarnessApproval } from '../../lib/types';
import type { HarnessState } from './harnessState';
import { GateCard } from './GateCard';
import { HxStatusBadge } from './HxStatusBadge';
import { PipelineStepper } from './PipelineStepper';
import { RunLedger } from './RunLedger';
import { RunPicker } from './RunPicker';
import { FindingsPanel } from './FindingsPanel';
import { WorkerChip } from './WorkerChip';
import { pipelineModel, runStatus } from './pipeline';
import { useNow } from './useNow';
import { agentRoleLabels, formatCost, formatMinutes, hxStatuses, plural, sourceLabels, stageKindLabels } from './vocabulary';

/** The pipeline for one run: where it stands, what waits on a person, and what each step took. */
export function RunView({ state, runId, onSelectRun, onRefresh, loadRun, onApprove, onReject, links }: {
	state: HarnessState;
	runId?: string;
	onSelectRun: (runId: string) => void;
	onRefresh: () => void;
	loadRun: (runId: string) => Promise<void>;
	onApprove: (approval: HarnessApproval, reason?: string) => void;
	onReject: (approval: HarnessApproval, reason: string) => void;
	/** Extra links under the findings, such as the drawer's tabs. */
	links?: React.ReactNode;
}) {
	const now = useNow(30_000);
	const summary = runId ? state.runs[runId] : undefined;
	const detail = runId ? state.details[runId] : undefined;
	// Reload the detail whenever the run's summary moves, so polling and events both keep it current.
	const version = summary ? `${summary.revision}:${summary.state}:${summary.currentStageId}:${summary.totals.costUsd}` : '';

	useEffect(() => {
		if (runId) void loadRun(runId);
	}, [loadRun, runId, version]);

	const model = useMemo(() => (detail ? pipelineModel(detail, now) : undefined), [detail, now]);
	const runs = Object.values(state.runs);

	if (!runId) {
		return (
			<section className="hx-card hx-state">
				<p>Choose a task to see its progress.</p>
				<RunPicker runs={runs} tasks={state.tasks} onChange={onSelectRun} />
			</section>
		);
	}
	if (!detail || !model) {
		return <section className="hx-card hx-state" aria-busy="true"><p>Loading progress for this run…</p></section>;
	}

	const { run, task } = detail;
	const status = runStatus(run);
	const stage = run.currentStageId ? detail.stages.find((candidate) => candidate.id === run.currentStageId) : undefined;
	const where = stage ? `${stageKindLabels[stage.kind]}${stage.maxRounds > 1 ? `, round ${stage.round} of ${stage.maxRounds}` : ''}` : '';
	const working = detail.agentRuns.find((agentRun) => agentRun.state === 'running');
	const waiting = detail.approvals.filter((approval) => approval.state === 'requested' || state.decisions[approval.id]);
	const agentRunCount = detail.agentRuns.length;

	return (
		<div className="hx-run">
			<div className="hx-run-head">
				<span className="hx-eyebrow">Pipeline</span>
				<RunPicker runs={runs.length ? runs : [run]} tasks={state.tasks} value={run.id} onChange={onSelectRun} />
				<HxStatusBadge status={status} label={where && (status === 'working' || status === 'waiting') ? `${hxStatuses[status].label} · ${where}` : undefined} />
				{working && <WorkerChip engine={working.engine} model={working.model} role={agentRoleLabels[working.role]} />}
				<span className="hx-run-head-spacer" />
				<span className="hx-muted hx-num">So far: {plural(agentRunCount, 'agent run')} · {formatMinutes(run.totals.durationMs)} · {formatCost(run.totals.costUsd)}</span>
				<button type="button" className="secondary hx-small-button" onClick={() => { onRefresh(); void loadRun(run.id); }}><RefreshCw size={14} aria-hidden="true" />Refresh</button>
			</div>

			{waiting.map((approval) => (
				<GateCard
					key={approval.id}
					approval={state.approvals[approval.id] ?? approval}
					task={task}
					sent={state.decisions[approval.id]}
					onApprove={(reason) => onApprove(state.approvals[approval.id] ?? approval, reason)}
					onReject={(reason) => onReject(state.approvals[approval.id] ?? approval, reason)}
				/>
			))}

			<section className="hx-card hx-stepper-card">
				<PipelineStepper model={model} label={`Progress for ${task.source.key}`} />
			</section>

			<div className="hx-run-split">
				<RunLedger detail={detail} />
				<FindingsPanel agentRuns={detail.agentRuns}>
					{links}
					{task.source.url && <a className="hx-link" href={task.source.url} target="_blank" rel="noreferrer">{sourceLabels[task.source.kind]} {task.source.key}</a>}
					{run.prUrl && <a className="hx-link" href={run.prUrl} target="_blank" rel="noreferrer">Draft pull request</a>}
				</FindingsPanel>
			</div>
		</div>
	);
}

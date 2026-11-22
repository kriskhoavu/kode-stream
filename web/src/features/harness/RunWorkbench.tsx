import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import { ClipboardList } from 'lucide-react';
import type { HarnessRunDetail, HarnessRunPlan, SessionChanges, SessionFile } from '../../lib/types';
import type { HarnessClient } from './harnessClient';
import { prLabel } from './pipeline';
import { TicketTab, liveSteps } from './RunDrawer';
import type { LiveStep } from './RunDrawer';
import { commonFolder, orderPlanFiles } from './RunPlanPanel';
import { plural } from './vocabulary';
import '../content-viewer/content-viewer.css';
import '../workstream-explorer/workstream-explorer.css';
import { type Mode, type Side, type View, type WorkbenchKey, type WorkbenchRequest, planPrefix, modeLabels, sideLabels, sideIcons } from './workbench/workbenchTypes';
import { useRunPlan, useRunChanges } from './workbench/useRunWorkbenchData';
import { PlanTree, ChangesTree, AgentsList } from './workbench/WorkbenchTrees';
import { TaskInfo, Answers, CheckerList, StepList } from './workbench/WorkbenchSide';
import { Reader } from './workbench/WorkbenchReader';

/**
 * A harness run read like a plan: what the run wrote (Plan), what it changed
 * in the code (Changes) and what each agent said (Agents) on the left, the
 * chosen file or conversation in the middle, and the task on the right. It
 * reads the run's own copy while it has one, else its branch, so a finished
 * run still shows its work.
 */
export function RunWorkbench({ client, detail, request }: { client: HarnessClient; detail: HarnessRunDetail; request?: WorkbenchRequest }) {
	const { run, task } = detail;
	const version = `${run.id}:${run.revision}:${run.state}:${run.currentStageId ?? ''}`;
	const plan = useRunPlan(client, run.id, version);
	const changes = useRunChanges(client, run.id, version);
	const planFiles = useMemo(() => orderPlanFiles(plan.value?.files ?? []), [plan.value]);
	const codeFiles = useMemo(() => (changes.value?.files ?? []).filter((file) => !file.path.startsWith(planPrefix)), [changes.value]);
	const steps = useMemo(() => liveSteps(detail.stages, detail.agentRuns), [detail.stages, detail.agentRuns]);

	const [mode, setMode] = useState<Mode>('plan');
	const [side, setSide] = useState<Side>('info');
	const [open, setOpen] = useState<WorkbenchKey[]>([]);
	const [active, setActive] = useState<WorkbenchKey | undefined>();
	const [views, setViews] = useState<Record<string, View>>({});

	const show = (key: WorkbenchKey) => {
		setOpen((current) => current.includes(key) ? current : [...current, key]);
		setActive(key);
		setMode(key.startsWith('plan:') ? 'plan' : key.startsWith('file:') ? 'changes' : 'agents');
	};
	const close = (key: WorkbenchKey) => {
		const rest = open.filter((candidate) => candidate !== key);
		setOpen(rest);
		if (active === key) setActive(rest.at(-1));
	};

	// Start on the plan's first file, once the plan is read.
	useEffect(() => {
		if (!active && planFiles[0]) show(`plan:${planFiles[0].path}`);
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [planFiles]);

	// A step clicked in the pipeline opens what that step made.
	useEffect(() => {
		if (!request) return;
		const { card } = request;
		if (card.step === 'plan_check' || card.step === 'code_check') { setSide('checker'); if (card.agentRun) show(`agent:${card.agentRun.id}`); return; }
		if (card.step === 'plan' || card.step === 'approval') { if (planFiles[0]) show(`plan:${planFiles[0].path}`); else setMode('plan'); return; }
		if ((card.step === 'build' || card.step === 'fix') && codeFiles[0] && !card.agentRun) { show(`file:${codeFiles[0].path}`); return; }
		if (card.agentRun) { show(`agent:${card.agentRun.id}`); return; }
		if (card.step === 'build' || card.step === 'review') { setMode('changes'); if (codeFiles[0]) show(`file:${codeFiles[0].path}`); return; }
		setSide('steps');
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [request]);

	const openFinding = (path: string) => {
		const plain = path.replace(/^\.?\//, '');
		const planFile = planFiles.find((file) => file.path === plain || file.path.endsWith(`/${plain}`));
		if (planFile) return show(`plan:${planFile.path}`);
		const codeFile = codeFiles.find((file) => file.path === plain || file.path.endsWith(`/${plain}`) || plain.endsWith(`/${file.path}`));
		if (codeFile) show(`file:${codeFile.path}`);
	};

	const gridStyle = { '--explorer-left-width': 'clamp(240px, 21vw, 300px)', '--explorer-right-width': 'clamp(260px, 21vw, 300px)' } as CSSProperties;
	return (
		<div className="explorer-grid hx-workbench" style={gridStyle}>
			<aside className="explorer-tree-panel" aria-label="What the run made">
				<div className="explorer-panel-header embedded">
					<div className="explorer-panel-title">
						<div className="segmented-control segmented-control-compact" role="tablist" aria-label="Run view">
							{(['plan', 'changes', 'agents'] as Mode[]).map((candidate) => (
								<button key={candidate} type="button" role="tab" className={mode === candidate ? 'active' : ''} aria-selected={mode === candidate} onClick={() => setMode(candidate)}>{modeLabels[candidate]}</button>
							))}
						</div>
					</div>
				</div>
				<div className="hx-workbench-meta">
					<span className="hx-workbench-meta-label" title={modeLabel(mode, plan.value, changes.value, run)}>{modeLabel(mode, plan.value, changes.value, run)}</span>
					<span className="hx-workbench-meta-count">{modeCount(mode, planFiles.length, codeFiles, steps)}</span>
				</div>
				<div className="explorer-tree" role="tree" aria-label={modeLabels[mode]}>
					{mode === 'plan' && <PlanTree plan={plan} files={planFiles} active={active} onOpen={show} />}
					{mode === 'changes' &&<ChangesTree changes={changes} files={codeFiles} active={active} onOpen={show} />}
					{mode === 'agents' && <AgentsList steps={steps} active={active} onOpen={show} />}
				</div>
			</aside>
			<main className="workspace-file-editor">
				<Reader client={client} detail={detail} keyed={active} open={open} steps={steps} planFiles={planFiles} codeFiles={codeFiles} plan={plan.value} changes={changes.value} view={active ? views[active] : undefined} onView={(view) => active && setViews((current) => ({ ...current, [active]: view }))} onSelect={show} onClose={close} />
			</main>
			<aside className="explorer-inspector metadata-panel side-panel" aria-label="Task">
				<div className="panel-header"><h2><ClipboardList size={16} aria-hidden="true" /> Task</h2></div>
				<div className="side-panel-tabs" role="tablist" aria-label="Task panel">
					{(['info', 'jira', 'checker', 'steps'] as Side[]).map((candidate) => {
						const Icon = sideIcons[candidate];
						return <button key={candidate} type="button" role="tab" className={side === candidate ? 'active' : ''} aria-selected={side === candidate} onClick={() => setSide(candidate)}><Icon size={14} aria-hidden="true" /> {sideLabels[candidate]}</button>;
					})}
				</div>
				{side === 'info' && <TaskInfo detail={detail} planCount={planFiles.length} codeFiles={codeFiles} from={changes.value?.from ?? plan.value?.from} />}
				{side === 'jira' && <><div className="hx-workbench-block"><TicketTab task={task} fixture={false} /></div><Answers detail={detail} /></>}
				{side === 'checker' && <CheckerList agentRuns={detail.agentRuns} onOpen={openFinding} onOpenRun={(id) => show(`agent:${id}`)} />}
				{side === 'steps' && <StepList detail={detail} onOpen={show} />}
			</aside>
		</div>
	);
}

function modeCount(mode: Mode, planCount: number, codeFiles: SessionFile[], steps: LiveStep[]): ReactNode {
	if (mode === 'plan') return plural(planCount, 'file');
	if (mode === 'agents') return plural(steps.reduce((sum, step) => sum + step.agentRuns.length, 0), 'conversation');
	const counted = codeFiles as (SessionFile & { additions?: number; deletions?: number })[];
	const additions = counted.reduce((sum, file) => sum + (file.additions ?? 0), 0);
	const deletions = counted.reduce((sum, file) => sum + (file.deletions ?? 0), 0);
	return <><span className="hx-add">+{additions}</span> <span className="hx-del">−{deletions}</span> · {plural(codeFiles.length, 'file')}</>;
}

/** What the left panel's list holds: the plan's folder, the branch the changes were read from, or the run's conversations. */
function modeLabel(mode: Mode, plan: HarnessRunPlan | undefined, changes: SessionChanges | undefined, run: HarnessRunDetail['run']): string {
	if (mode === 'plan') return plan?.files.length ? commonFolder(plan.files.map((file) => file.path)).replace(/^plans\//, '').replace(/\/$/, '') : 'Plan';
	if (mode === 'agents') return 'By step';
	if (changes?.from === 'branch') return run.prUrl && run.remoteBranch ? run.remoteBranch : run.branch;
	return changes?.from === 'worktree' ? 'Live, not committed' : 'Code';
}

/**
 * The line under a run page's title: where the task came from, as a link,
 * then the run's branch and its pull request once there is one.
 */
export function RunPagePath({ lead, run }: { lead: ReactNode; run?: HarnessRunDetail['run'] }) {
	return (
		<div className="workspace-item-path" aria-label="Item location">
			{lead}
			{run && <span className="hx-path-part"><span className="workspace-item-path-separator">/</span><span className="workspace-item-path-segment" title={run.branch}>{run.prUrl && run.remoteBranch ? run.remoteBranch : run.branch}</span></span>}
{run?.prUrl && <span className="hx-path-part"><span className="workspace-item-path-separator">/</span><a className="workspace-item-path-segment hx-link" href={run.prUrl} target="_blank" rel="noreferrer">Pull request {prLabel(run.prUrl)} ↗</a></span>}
		</div>
	);
}

/* ---------- loading ---------- */

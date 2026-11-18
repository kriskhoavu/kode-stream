import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import { ChevronDown, ChevronRight, ClipboardList, Code2, Eye, FileText, Folder, GitCompare, Info, ListChecks, MessagesSquare, Ticket, X } from 'lucide-react';
import type { HarnessAgentRun, HarnessRunDetail, HarnessRunFile, HarnessRunFileDiff, HarnessRunPlan, SessionChanges, SessionFile } from '../../lib/types';
import { parseGitDiff } from '../../shared/domain/diff';
import { MarkdownPreview } from '../content-viewer/renderers/MarkdownPreview';
import { TranscriptTurns } from '../ai-session/TranscriptTurns';
import type { HarnessClient } from './harnessClient';
import type { StepCard } from './pipeline';
import { prLabel } from './pipeline';
import { RunLedger } from './RunLedger';
import { TicketTab, agentStatus, liveSteps, useAgentTranscript } from './RunDrawer';
import type { LiveStep } from './RunDrawer';
import { commonFolder, orderPlanFiles, planSourceLine } from './RunPlanPanel';
import { plainError } from './useHarness';
import { agentRoleLabels, formatCost, formatMinutes, hxStatuses, plural } from './vocabulary';
import '../workstream-explorer/workstream-explorer.css';

type Mode = 'plan' | 'changes' | 'agents';
type Side = 'info' | 'jira' | 'checker' | 'steps';
type View = 'preview' | 'source' | 'diff';
/** What the reader shows: a plan file, a changed file, one conversation, or a whole section of them. */
export type WorkbenchKey = `plan:${string}` | `file:${string}` | `agent:${string}` | `stage:${string}`;
/** A request from outside the workbench, such as a step clicked in the pipeline. `n` makes a repeat count. */
export interface WorkbenchRequest { card: StepCard; n: number }

const planPrefix = 'plans/';
const statusLetter: Record<string, string> = { added: 'A', modified: 'M', deleted: 'D', A: 'A', M: 'M', D: 'D' };

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
		if (card.step === 'build' || card.step === 'pr_approval') { setMode('changes'); if (codeFiles[0]) show(`file:${codeFiles[0].path}`); return; }
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

	const gridStyle = { '--explorer-left-width': '320px', '--explorer-right-width': '320px' } as CSSProperties;
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
				<p className="hx-workbench-count">{modeCount(mode, planFiles.length, codeFiles, steps)}</p>
				<div className="explorer-tree" role="tree" aria-label={modeLabels[mode]}>
					{mode === 'plan' && <PlanTree plan={plan} files={planFiles} active={active} onOpen={show} />}
					{mode === 'changes' && <ChangesTree changes={changes} files={codeFiles} active={active} onOpen={show} />}
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
				{side === 'jira' && <><TicketTab task={task} fixture={false} /><Answers detail={detail} /></>}
				{side === 'checker' && <CheckerList agentRuns={detail.agentRuns} onOpen={openFinding} onOpenRun={(id) => show(`agent:${id}`)} />}
				{side === 'steps' && <div className="hx-workbench-steps"><RunLedger detail={detail} onOpenAgentRun={(id) => show(`agent:${id}`)} /></div>}
			</aside>
		</div>
	);
}

const modeLabels: Record<Mode, string> = { plan: 'Plan', changes: 'Changes', agents: 'Agents' };
const sideLabels: Record<Side, string> = { info: 'Info', jira: 'Jira', checker: 'Checker', steps: 'Steps' };
const sideIcons = { info: Info, jira: Ticket, checker: ListChecks, steps: MessagesSquare };

function modeCount(mode: Mode, planCount: number, codeFiles: SessionFile[], steps: LiveStep[]): ReactNode {
	if (mode === 'plan') return plural(planCount, 'file');
	if (mode === 'agents') return plural(steps.reduce((sum, step) => sum + step.agentRuns.length, 0), 'conversation');
	const counted = codeFiles as (SessionFile & { additions?: number; deletions?: number })[];
	const additions = counted.reduce((sum, file) => sum + (file.additions ?? 0), 0);
	const deletions = counted.reduce((sum, file) => sum + (file.deletions ?? 0), 0);
	return <><span className="hx-add">+{additions}</span> <span className="hx-del">−{deletions}</span> · {plural(codeFiles.length, 'file')}</>;
}

/**
 * The line under a run page's title: where the task came from, as a link,
 * then the run's branch and its pull request once there is one.
 */
export function RunPagePath({ lead, run }: { lead: ReactNode; run?: HarnessRunDetail['run'] }) {
	return (
		<div className="workspace-item-path" aria-label="Item location">
			{lead}
			{run && <><span className="workspace-item-path-separator">/</span><span className="workspace-item-path-segment" title={run.branch}>{run.prUrl && run.remoteBranch ? run.remoteBranch : run.branch}</span></>}
			{run?.prUrl && <><span className="workspace-item-path-separator">/</span><a className="workspace-item-path-segment hx-link" href={run.prUrl} target="_blank" rel="noreferrer">Pull request {prLabel(run.prUrl)} ↗</a></>}
		</div>
	);
}

/* ---------- loading ---------- */

interface Loaded<T> { value?: T; error: string }

function useRunPlan(client: HarnessClient, runId: string, version: string): Loaded<HarnessRunPlan> {
	const [state, setState] = useState<Loaded<HarnessRunPlan>>({ error: '' });
	useEffect(() => {
		if (!client.runPlan) return;
		const controller = new AbortController();
		client.runPlan(runId, controller.signal)
			.then((value) => { if (!controller.signal.aborted) setState({ value, error: '' }); })
			.catch((caught) => { if (!controller.signal.aborted) setState({ error: plainError(caught, 'The plan could not be read.') }); });
		return () => controller.abort();
	}, [client, runId, version]);
	return state;
}

function useRunChanges(client: HarnessClient, runId: string, version: string): Loaded<SessionChanges> {
	const [state, setState] = useState<Loaded<SessionChanges>>({ error: '' });
	useEffect(() => {
		const controller = new AbortController();
		client.runChanges(runId, controller.signal)
			.then((value) => { if (!controller.signal.aborted) setState({ value, error: '' }); })
			.catch((caught) => { if (!controller.signal.aborted) setState({ error: plainError(caught, 'The changes could not be read.') }); });
		return () => controller.abort();
	}, [client, runId, version]);
	return state;
}

function useFileDiff(client: HarnessClient, runId: string, path: string | undefined, version: string): Loaded<HarnessRunFileDiff> {
	const [state, setState] = useState<Loaded<HarnessRunFileDiff>>({ error: '' });
	useEffect(() => {
		setState({ error: '' });
		if (!path || !client.runFileDiff) return;
		const controller = new AbortController();
		client.runFileDiff(runId, path, controller.signal)
			.then((value) => { if (!controller.signal.aborted) setState({ value, error: '' }); })
			.catch((caught) => { if (!controller.signal.aborted) setState({ error: plainError(caught, 'The diff could not be read.') }); });
		return () => controller.abort();
	}, [client, runId, path, version]);
	return state;
}

/* ---------- left panel ---------- */

interface TreeDir { name: string; dirs: TreeDir[]; files: string[] }

/** Folders and files from paths, a folder with one folder and no file merged into one row, as the item page's tree does. */
export function buildTree(paths: string[], root = ''): TreeDir {
	const top: TreeDir = { name: '', dirs: [], files: [] };
	for (const path of paths) {
		const parts = path.slice(root.length).split('/');
		let node = top;
		parts.slice(0, -1).forEach((part) => {
			let next = node.dirs.find((dir) => dir.name === part);
			if (!next) { next = { name: part, dirs: [], files: [] }; node.dirs.push(next); }
			node = next;
		});
		node.files.push(path);
	}
	const squash = (dir: TreeDir): TreeDir => {
		let node = { ...dir, dirs: dir.dirs.map(squash) };
		while (node.files.length === 0 && node.dirs.length === 1 && node.name) {
			const only = node.dirs[0];
			node = { name: `${node.name}/${only.name}`, dirs: only.dirs, files: only.files };
		}
		return node;
	};
	return { ...top, dirs: top.dirs.map(squash) };
}

function Tree({ dir, depth, prefix, active, row }: { dir: TreeDir; depth: number; prefix: string; active?: WorkbenchKey; row: (path: string, depth: number) => ReactNode }) {
	const [closed, setClosed] = useState<Record<string, boolean>>({});
	return (
		<>
			{dir.dirs.map((child) => {
				const id = `${prefix}${child.name}`;
				const isOpen = !closed[id];
				return (
					<div key={id} role="none">
						<div className="explorer-tree-row" role="treeitem" aria-expanded={isOpen} aria-level={depth + 1} aria-selected={false} style={{ '--explorer-depth': depth } as CSSProperties}>
							<button className="explorer-row-toggle" type="button" tabIndex={-1} aria-label={`${isOpen ? 'Collapse' : 'Expand'} ${child.name}`} onClick={() => setClosed((current) => ({ ...current, [id]: isOpen }))}>{isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}</button>
							<button className="explorer-row-main" type="button" onClick={() => setClosed((current) => ({ ...current, [id]: isOpen }))}><Folder size={15} aria-hidden="true" /><span className="explorer-row-label directory" title={child.name}>{child.name}</span></button>
						</div>
						{isOpen && <Tree dir={child} depth={depth + 1} prefix={`${id}/`} active={active} row={row} />}
					</div>
				);
			})}
			{dir.files.map((path) => <div key={path} role="none">{row(path, depth)}</div>)}
		</>
	);
}

function FileRow({ path, depth, selected, additions, deletions, status, onOpen }: { path: string; depth: number; selected: boolean; additions?: number; deletions?: number; status?: string; onOpen: () => void }) {
	return (
		<div className={selected ? 'explorer-tree-row selected' : 'explorer-tree-row'} role="treeitem" aria-level={depth + 1} aria-selected={selected} style={{ '--explorer-depth': depth } as CSSProperties}>
			<button className="explorer-row-toggle" type="button" tabIndex={-1} disabled />
			<button className="explorer-row-main" type="button" title={path} onClick={onOpen}>
				<FileText size={15} aria-hidden="true" />
				<span className="explorer-row-label file">{path.split('/').pop()}</span>
				<span className="hx-workbench-delta">
					{additions !== undefined ? <><span className="hx-add">+{additions}</span>{deletions ? <> <span className="hx-del">−{deletions}</span></> : null}</> : status && statusLetter[status] ? <span className={`hx-run-plan-status ${status}`}>{statusLetter[status]}</span> : null}
				</span>
			</button>
		</div>
	);
}

function PlanTree({ plan, files, active, onOpen }: { plan: Loaded<HarnessRunPlan>; files: HarnessRunFile[]; active?: WorkbenchKey; onOpen: (key: WorkbenchKey) => void }) {
	if (plan.error) return <p className="hx-banner" role="alert">{plan.error}</p>;
	if (!plan.value) return <p className="hx-workbench-empty" aria-busy="true">Reading the plan…</p>;
	if (files.length === 0) return <p className="hx-workbench-empty">{plan.value.from === '' ? 'This run\'s copy and branch are gone.' : 'The Planner has not written a plan yet.'}</p>;
	const folder = commonFolder(files.map((file) => file.path));
	const ordered = new Map(files.map((file, index) => [file.path, index]));
	const tree = buildTree(files.map((file) => file.path), folder);
	const sortTree = (dir: TreeDir): TreeDir => ({ ...dir, dirs: dir.dirs.map(sortTree), files: [...dir.files].sort((a, b) => (ordered.get(a) ?? 0) - (ordered.get(b) ?? 0)) });
	return (
		<>
			<p className="hx-workbench-folder" title={folder}>{folder.replace(/^plans\//, '').replace(/\/$/, '')}</p>
			<Tree dir={sortTree(tree)} depth={0} prefix="plan:" active={active} row={(path, depth) => {
				const file = files.find((candidate) => candidate.path === path);
				return <FileRow path={path} depth={depth} selected={active === `plan:${path}`} status={file?.status} onOpen={() => onOpen(`plan:${path}`)} />;
			}} />
		</>
	);
}

function ChangesTree({ changes, files, active, onOpen }: { changes: Loaded<SessionChanges>; files: SessionFile[]; active?: WorkbenchKey; onOpen: (key: WorkbenchKey) => void }) {
	if (changes.error) return <p className="hx-banner" role="alert">{changes.error}</p>;
	if (!changes.value) return <p className="hx-workbench-empty" aria-busy="true">Reading the changes…</p>;
	if (changes.value.availability !== 'available') return <p className="hx-workbench-empty">This run's copy and branch are gone, so its changes cannot be read here.</p>;
	if (files.length === 0) return <p className="hx-workbench-empty">No code has changed yet.</p>;
	const counted = files as (SessionFile & { additions?: number; deletions?: number })[];
	return (
		<>
			<p className="hx-workbench-folder">{changes.value.from === 'branch' ? `From ${changes.value.branch ?? 'the branch'}` : 'Live from the harness\'s own copy'}</p>
			<Tree dir={buildTree(files.map((file) => file.path))} depth={0} prefix="file:" active={active} row={(path, depth) => {
				const file = counted.find((candidate) => candidate.path === path);
				return <FileRow path={path} depth={depth} selected={active === `file:${path}`} additions={file?.additions ?? 0} deletions={file?.deletions ?? 0} onOpen={() => onOpen(`file:${path}`)} />;
			}} />
		</>
	);
}

function AgentsList({ steps, active, onOpen }: { steps: LiveStep[]; active?: WorkbenchKey; onOpen: (key: WorkbenchKey) => void }) {
	if (steps.length === 0) return <p className="hx-workbench-empty">No agent has worked on this task yet.</p>;
	return (
		<>
			<button type="button" className={active === 'stage:*' ? 'hx-workbench-all on' : 'hx-workbench-all'} onClick={() => onOpen('stage:*')}>
				<span>All steps</span><small>Read every conversation in order</small>
			</button>
			{steps.map((step) => (
				<div key={step.id} role="group" aria-label={step.title}>
					<button type="button" className={active === `stage:${step.id}` ? 'hx-workbench-group on' : 'hx-workbench-group'} onClick={() => onOpen(`stage:${step.id}`)} title={`Read all of ${step.title}`}>
						<span>{step.title}</span><span className="hx-workbench-group-all">{step.agentRuns.length} · read all</span>
					</button>
					{step.agentRuns.map((agentRun) => {
						const key: WorkbenchKey = `agent:${agentRun.id}`;
						return (
							<div key={agentRun.id} className={active === key ? 'explorer-tree-row selected' : 'explorer-tree-row'} role="treeitem" aria-selected={active === key} style={{ '--explorer-depth': 0 } as CSSProperties}>
								<button className="explorer-row-toggle" type="button" tabIndex={-1} disabled />
								<button className="explorer-row-main" type="button" onClick={() => onOpen(key)}>
									<span className={`hx-workbench-dot hx-tone-${hxStatuses[agentDot(agentRun)].tone}`} aria-hidden="true" />
									<span className="explorer-row-label file"><b>{agentRoleLabels[agentRun.role]}</b> · {agentTitle(agentRun)}</span>
								</button>
							</div>
						);
					})}
				</div>
			))}
		</>
	);
}

function agentDot(agentRun: HarnessAgentRun) {
	if (agentRun.state === 'succeeded' && agentRun.blocking > 0) return 'failed' as const;
	return agentStatus(agentRun);
}

function agentTitle(agentRun: HarnessAgentRun): string {
	const review = agentRun.role === 'plan_reviewer' || agentRun.role === 'code_reviewer';
	const verdict = review && agentRun.verdict ? (agentRun.verdict === 'approve' ? ' · approved' : ` · ${plural(agentRun.blocking, 'must fix', 'must fix')}`) : '';
	return `Round ${agentRun.round}${verdict}`;
}

/* ---------- reader ---------- */

function keyTitle(key: WorkbenchKey, detail: HarnessRunDetail, steps: LiveStep[]): string {
	if (key.startsWith('plan:') || key.startsWith('file:')) return key.slice(5).split('/').pop() ?? key;
	if (key === 'stage:*') return 'All steps';
	if (key.startsWith('stage:')) return `${steps.find((step) => step.id === key.slice(6))?.title ?? 'Step'} · all`;
	const agentRun = detail.agentRuns.find((candidate) => candidate.id === key.slice(6));
	return agentRun ? `${agentRoleLabels[agentRun.role]} · round ${agentRun.round}` : 'Conversation';
}

function Reader({ client, detail, keyed, open, steps, planFiles, codeFiles, plan, changes, view, onView, onSelect, onClose }: {
	client: HarnessClient; detail: HarnessRunDetail; keyed?: WorkbenchKey; open: WorkbenchKey[]; steps: LiveStep[];
	planFiles: HarnessRunFile[]; codeFiles: SessionFile[]; plan?: HarnessRunPlan; changes?: SessionChanges;
	view?: View; onView: (view: View) => void; onSelect: (key: WorkbenchKey) => void; onClose: (key: WorkbenchKey) => void;
}) {
	const { run } = detail;
	const path = keyed && (keyed.startsWith('plan:') || keyed.startsWith('file:')) ? keyed.slice(5) : undefined;
	const isPlan = keyed?.startsWith('plan:');
	const modes: View[] = isPlan ? ['preview', 'source', 'diff'] : ['diff'];
	const current = path ? (view && modes.includes(view) ? view : modes[0]) : undefined;
	const diff = useFileDiff(client, run.id, current === 'diff' ? path : undefined, `${run.revision}:${run.state}`);
	const planFile = isPlan ? planFiles.find((file) => file.path === path) : undefined;
	const codeFile = keyed?.startsWith('file:') ? (codeFiles as (SessionFile & { additions?: number; deletions?: number })[]).find((file) => file.path === path) : undefined;
	const from = isPlan ? plan?.from : changes?.from;
	return (
		<>
			<div className="explorer-breadcrumbs">
				{path ? path.split('/').map((part, index) => <span key={index}>{index ? ' / ' : ''}{part}</span>) : <span>{keyed ? keyTitle(keyed, detail, steps) : task(detail)}</span>}
			</div>
			<div className="editor-file-strip">
				<div className="tab-list file-tab-list" aria-label="Open items">
					{open.map((key) => (
						<div key={key} className={key === keyed ? 'file-tab active' : 'file-tab'}>
							<button type="button" className="file-tab-button" title={key.slice(key.indexOf(':') + 1)} onClick={() => onSelect(key)}>{keyTitle(key, detail, steps)}</button>
							<button type="button" className="file-tab-close" aria-label={`Close ${keyTitle(key, detail, steps)}`} onClick={() => onClose(key)}><X size={13} /></button>
						</div>
					))}
				</div>
			</div>
			{path && (
				<div className="tabs explorer-editor-tabs">
					<div className="editor-toolbar-actions">
						<div className="editor-view-switch" role="tablist" aria-label="File view mode">
							{modes.map((mode) => {
								const Icon = mode === 'preview' ? Eye : mode === 'source' ? Code2 : GitCompare;
								return <button key={mode} type="button" role="tab" aria-selected={current === mode} className={current === mode ? 'active' : ''} onClick={() => onView(mode)}><Icon size={14} aria-hidden="true" /> {mode[0].toUpperCase() + mode.slice(1)}</button>;
							})}
						</div>
						<span className="autosave-state idle">{codeFile ? <><span className="hx-add">+{codeFile.additions ?? 0}</span> <span className="hx-del">−{codeFile.deletions ?? 0}</span> · </> : null}{from === 'branch' ? `from ${run.remoteBranch || run.branch}` : from === 'worktree' ? 'live, not committed yet' : ''}</span>
					</div>
				</div>
			)}
			<section className="content-viewer hx-workbench-reader" data-file-kind={current === 'preview' ? 'markdown' : 'text'}>
				{!keyed && <p className="hx-workbench-empty">Pick a plan file, a changed file or a conversation on the left.</p>}
				{planFile && current === 'preview' && (planFile.truncated ? <p className="hx-workbench-empty">This file is too large to show here.</p> : /\.md$/i.test(planFile.path) ? <article className="content-viewer-markdown"><MarkdownPreview content={planFile.content} /></article> : <pre className="hx-workbench-source">{planFile.content}</pre>)}
				{planFile && current === 'source' && <pre className="hx-workbench-source">{planFile.content}</pre>}
				{path && current === 'diff' && <DiffView diff={diff} />}
				{keyed?.startsWith('agent:') && <Conversation client={client} agentRun={detail.agentRuns.find((agentRun) => agentRun.id === keyed.slice(6))} />}
				{keyed?.startsWith('stage:') && <Section client={client} steps={keyed === 'stage:*' ? steps : steps.filter((step) => step.id === keyed.slice(6))} titled={keyed === 'stage:*'} onOpen={onSelect} />}
				{isPlan && !planFile && plan && <p className="hx-workbench-empty">{planSourceLine(plan) || 'This plan file is no longer there.'}</p>}
			</section>
		</>
	);
}

function task(detail: HarnessRunDetail): string {
	return `${detail.task.source.key} · ${detail.task.title}`;
}

function DiffView({ diff }: { diff: Loaded<HarnessRunFileDiff> }) {
	if (diff.error) return <p className="hx-banner" role="alert">{diff.error}</p>;
	if (!diff.value) return <p className="hx-workbench-empty" aria-busy="true">Reading the diff…</p>;
	const files = parseGitDiff(diff.value.diff);
	if (files.length === 0) return <p className="hx-workbench-empty">{diff.value.from === '' ? 'This run\'s copy and branch are gone.' : 'No difference to show.'}</p>;
	return (
		<div className="diff-review hx-workbench-diff">
			{files.map((file) => (
				<article className="diff-file" key={file.path}>
					<div className="diff-lines">
						{file.lines.map((line, index) => (
							<div className={`diff-line ${line.type}`} key={index}>
								<span className="line-number">{line.oldLine ?? ''}</span>
								<span className="line-number">{line.newLine ?? ''}</span>
								<code>{line.text || ' '}</code>
							</div>
						))}
					</div>
				</article>
			))}
			{diff.value.truncated && <p className="hx-workbench-empty">The diff is longer than this view shows.</p>}
		</div>
	);
}

function Conversation({ client, agentRun }: { client: HarnessClient; agentRun?: HarnessAgentRun }) {
	const transcript = useAgentTranscript(client, agentRun);
	if (!agentRun) return <p className="hx-workbench-empty">This conversation is no longer in the run.</p>;
	return (
		<div className="hx-workbench-thread">
			<p className="hx-workbench-thread-meta">{threadMeta(agentRun)}</p>
			{transcript.error ? <p className="hx-banner" role="alert">{transcript.error}</p> : <TranscriptTurns className="hx-transcript" turns={transcript.turns} label={`${agentRoleLabels[agentRun.role]} conversation`} placeholder="Reading the conversation…" names={{ user: 'Harness', assistant: agentRoleLabels[agentRun.role] }} />}
		</div>
	);
}

function threadMeta(agentRun: HarnessAgentRun): string {
	return [plural(agentRun.turns, 'turn'), formatMinutes(agentRun.durationMs), formatCost(agentRun.costUsd)].join(' · ');
}

/** Every conversation of one section, or of the whole run, one after another. */
function Section({ client, steps, titled, onOpen }: { client: HarnessClient; steps: LiveStep[]; titled: boolean; onOpen: (key: WorkbenchKey) => void }) {
	return (
		<div className="hx-workbench-section">
			{steps.map((step) => (
				<div key={step.id}>
					{titled && <h2 className="hx-workbench-section-title">{step.title}</h2>}
					{step.agentRuns.map((agentRun) => (
						<section key={agentRun.id} className="hx-workbench-section-thread" aria-label={`${agentRoleLabels[agentRun.role]}, round ${agentRun.round}`}>
							<button type="button" className="hx-workbench-section-head" onClick={() => onOpen(`agent:${agentRun.id}`)}>
								<span className={`hx-workbench-dot hx-tone-${hxStatuses[agentDot(agentRun)].tone}`} aria-hidden="true" />
								<b>{agentRoleLabels[agentRun.role]}</b> · {agentTitle(agentRun)}
								<span className="hx-workbench-section-meta">{threadMeta(agentRun)}</span>
								<span className="hx-workbench-section-open">Open</span>
							</button>
							<Conversation client={client} agentRun={agentRun} />
						</section>
					))}
				</div>
			))}
		</div>
	);
}

/* ---------- right panel ---------- */

function TaskInfo({ detail, planCount, codeFiles, from }: { detail: HarnessRunDetail; planCount: number; codeFiles: SessionFile[]; from?: string }) {
	const { run, task } = detail;
	const tests = codeFiles.filter((file) => /(^|\/)(test|tests|__tests__)\/|[._-]test\.|Test\.java$|_test\.go$/.test(file.path)).length;
	return (
		<dl>
			<dt>Item</dt><dd>{task.source.key}</dd>
			<dt>Status</dt><dd><span className={`hx-badge hx-tone-${hxStatuses[runTone(run.state)].tone}`}>{hxStatuses[runTone(run.state)].label}</span></dd>
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

function runTone(state: HarnessRunDetail['run']['state']) {
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
function Answers({ detail }: { detail: HarnessRunDetail }) {
	const answered = detail.approvals.filter((approval) => approval.gate === 'question' && approval.context?.questions?.length);
	if (answered.length === 0) return null;
	return (
		<div className="hx-workbench-answers">
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

function CheckerList({ agentRuns, onOpen, onOpenRun }: { agentRuns: HarnessAgentRun[]; onOpen: (path: string) => void; onOpenRun: (agentRunId: string) => void }) {
	const reviews = agentRuns
		.filter((agentRun) => (agentRun.role === 'plan_reviewer' || agentRun.role === 'code_reviewer') && agentRun.state === 'succeeded')
		.sort((a, b) => Date.parse(b.startedAt ?? '') - Date.parse(a.startedAt ?? ''));
	if (reviews.length === 0) return <p className="hx-workbench-empty">The Checker has not reviewed anything in this run yet.</p>;
	return (
		<div className="hx-workbench-checker">
			{reviews.map((review) => (
				<section key={review.id}>
					<button type="button" className="hx-workbench-subhead" onClick={() => onOpenRun(review.id)}>
						{review.role === 'plan_reviewer' ? 'Plan check' : 'Code check'} · round {review.round}
						<span className={review.verdict === 'approve' ? 'hx-workbench-verdict ok' : 'hx-workbench-verdict'}>{review.verdict === 'approve' ? 'approved' : 'changes asked'}</span>
					</button>
					{(review.findings ?? []).length === 0 && <p className="hx-workbench-empty">Nothing to fix and no suggestions.</p>}
					{(review.findings ?? []).map((finding, index) => (
						<button key={index} type="button" className="hx-workbench-finding" disabled={!finding.path} onClick={() => finding.path && onOpen(finding.path)}>
							<span className={finding.resolved ? 'hx-workbench-verdict ok' : finding.severity === 'blocking' ? 'hx-workbench-verdict bad' : 'hx-workbench-verdict'}>{finding.resolved ? 'Fixed' : finding.severity === 'blocking' ? 'Must fix' : 'Suggestion'}</span>
							<span className="hx-workbench-finding-title">{finding.title}</span>
							{finding.path && <code>{finding.path.split('/').pop()}{finding.line ? `:${finding.line}` : ''}</code>}
						</button>
					))}
				</section>
			))}
		</div>
	);
}

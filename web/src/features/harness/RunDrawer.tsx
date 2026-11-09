import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react';
import type { KeyboardEvent } from 'react';
import { ArrowDown, ArrowUp, ChevronRight, ExternalLink, GripVertical, Pin, PinOff, Play, X } from 'lucide-react';
import { useResizableDrawer } from '../../components/useResizableDrawer';
import type { ClaudeTurn, HarnessAgentRun, HarnessApproval, HarnessStage, HarnessTask, SafeSessionRecord, SessionChanges } from '../../lib/types';
import type { HarnessRunTab } from '../../app/router';
import { SessionChangesPanel } from '../sessions/SessionChangesPanel';
import { TranscriptTurns } from '../ai-session/TranscriptTurns';
import { JiraItemPanel } from '../jira/JiraItemPanel';
import type { HarnessClient } from './harnessClient';
import type { HarnessState } from './harnessState';
import { waitsFor } from './harnessState';
import { FindingsList } from './FindingsPanel';
import { GateCard } from './GateCard';
import { HxStatusBadge } from './HxStatusBadge';
import { PipelineStepper } from './PipelineStepper';
import { SourceBadge } from './SourceBadge';
import { RunPlanPanel } from './RunPlanPanel';
import { pipelineModel, prLabel, runStatus } from './pipeline';
import { useNow } from './useNow';
import { agentRoleLabels, formatCost, hxSteps, hxStatuses, modelLabel, plural, sourceLabels, stageKindLabels, taskStatus } from './vocabulary';
import type { HxStatus } from './vocabulary';

// The live terminal pulls in xterm; load it only when someone opens the Live tab.
const SessionTerminal = lazy(() => import('../ai-session/SessionTerminal').then((module) => ({ default: module.SessionTerminal })));

const tabs: { id: HarnessRunTab; label: string }[] = [
	{ id: 'ticket', label: 'Ticket' },
	{ id: 'plan', label: 'Plan' },
	{ id: 'stages', label: 'Stages' },
	{ id: 'findings', label: 'Findings' },
	{ id: 'changes', label: 'Changes' },
	{ id: 'live', label: 'Live' },
	{ id: 'links', label: 'Links' }
];

/**
 * Everything about one task beside the board, so approving or re-ordering
 * never means leaving it. The waiting gate sits at the top; the tabs hold
 * the stages, the findings, the changed files, the live agent and the links.
 */
export function RunDrawer({ state, client, taskId, tab, agentRunId, queue, onTabChange, onClose, onOpenPipeline, onOpenAgentSpace, loadRun, onApprove, onReject, onPin, onMove, onStart, onSetQuestions, askDefault = true }: {
	state: HarnessState;
	client: HarnessClient;
	taskId: string;
	tab: HarnessRunTab;
	/** The conversation the Live tab opens on; otherwise the working or latest agent. */
	agentRunId?: string;
	/** The workspace's Task Queue in order, for Move up and Move down. */
	queue: HarnessTask[];
	onTabChange: (tab: HarnessRunTab) => void;
	onClose: () => void;
	onOpenPipeline?: (runId: string) => void;
	onOpenAgentSpace?: (workspaceId: string, view?: 'sessions') => void;
	loadRun: (runId: string) => Promise<void>;
	onApprove: (approval: HarnessApproval, reason?: string) => void;
	onReject: (approval: HarnessApproval, reason: string) => void;
	onPin: (task: HarnessTask, pinned: boolean) => void;
	onMove: (task: HarnessTask, before?: HarnessTask) => void;
	/** Starts a task that is up next; absent when nothing can be started here. */
	onStart?: (task: HarnessTask) => void;
	/** Sets this task's own answer to "may an agent stop and ask"; null follows the workspace and the settings. */
	onSetQuestions?: (task: HarnessTask, askQuestions: boolean | null) => void;
	/** What the settings say when the task does not. */
	askDefault?: boolean;
}) {
	const task = state.tasks[taskId];
	const runId = task?.runId;
	const run = runId ? state.runs[runId] : undefined;
	const detail = runId ? state.details[runId] : undefined;
	const headingRef = useRef<HTMLHeadingElement | null>(null);
	const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);
	const now = useNow(30_000);
	const { style: drawerStyle, startResize } = useResizableDrawer({ initial: 560 });
	const version = run ? `${run.revision}:${run.state}:${run.currentStageId}:${run.totals.costUsd}` : '';

	useEffect(() => {
		if (runId) void loadRun(runId);
	}, [loadRun, runId, version]);

	useEffect(() => {
		const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
		headingRef.current?.focus();
		const onKey = (event: globalThis.KeyboardEvent) => {
			if (event.key === 'Escape' && !document.querySelector('[role="dialog"][aria-modal="true"]')) onClose();
		};
		window.addEventListener('keydown', onKey);
		return () => {
			window.removeEventListener('keydown', onKey);
			previous?.focus();
		};
		// The drawer opens once per task, so focus moves only then; `onClose` is read when Escape is pressed.
	}, [taskId]);

	if (!task) return null;
	const waits = waitsFor(task, state.tasks);
	const status = run ? runStatus(run) : taskStatus(task, waits);
	const waiting = Object.values(state.approvals).filter((approval) => approval.taskId === task.id && (approval.state === 'requested' || state.decisions[approval.id]));
	const position = queue.findIndex((item) => item.id === task.id);

	const onTabKey = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
		const delta = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0;
		if (!delta) return;
		event.preventDefault();
		const next = (index + delta + tabs.length) % tabs.length;
		onTabChange(tabs[next].id);
		tabRefs.current[next]?.focus();
	};

	return (
		<>
			<button className="drawer-scrim" style={drawerStyle} type="button" aria-label="Close details" onClick={onClose} />
			<aside className="plan-drawer compact hx hx-drawer" style={drawerStyle} aria-labelledby="hx-drawer-title">
				<button className="drawer-resize-handle" type="button" aria-label="Resize details panel" onPointerDown={startResize}>
					<GripVertical size={16} />
				</button>
				<header className="plan-drawer-header">
					<div>
						<span className="drawer-kicker">{task.source.key} · <SourceBadge kind={task.source.kind} url={task.source.url} /></span>
						<h2 id="hx-drawer-title" ref={headingRef} tabIndex={-1}>{task.title}</h2>
						<p className="hx-drawer-status"><HxStatusBadge status={status} />{task.epic && <span className="hx-muted">{task.epic}</span>}{task.milestone && <span className="hx-muted">{task.milestone}</span>}</p>
					</div>
					<div className="drawer-actions">
						{run && onOpenPipeline && <button type="button" className="secondary drawer-open-details" onClick={() => onOpenPipeline(run.id)}>Open pipeline</button>}
						<button type="button" className="icon-button" aria-label="Close details" onClick={onClose}><X size={16} /></button>
					</div>
				</header>
				<div className="plan-drawer-body hx-drawer-body">
					{waiting.map((approval) => (
						<GateCard key={approval.id} approval={state.approvals[approval.id] ?? approval} task={task} sent={state.decisions[approval.id]}
							onApprove={(reason) => onApprove(state.approvals[approval.id] ?? approval, reason)}
							onReject={(reason) => onReject(state.approvals[approval.id] ?? approval, reason)} />
					))}

					{!run && onStart && (task.state === 'ranked' || task.state === 'discovered') && (
						<section className="hx-drawer-start" aria-label="Start this task">
							{task.startRequested
								? <p><b>Starting soon.</b> The next free Planner takes it, usually within a minute.</p>
								: <p>{waits.length ? <>Starting now skips the wait for <b>{waits.join(', ')}</b>. </> : ''}The Planner writes a plan first, and you approve it before any code is written.</p>}
							<button type="button" className="primary hx-small-button" disabled={Boolean(task.startRequested)} onClick={() => onStart(task)}><Play size={14} aria-hidden="true" />{task.startRequested ? 'Starting…' : 'Start'}</button>
						</section>
					)}

					{onSetQuestions && !['done', 'dismissed', 'stale', 'cancelled'].includes(task.state) && (
						<section className="hx-drawer-questions" aria-label="Questions">
							<label className="hx-field hx-field-inline">
								<span>May an agent stop and ask you?</span>
								<select aria-label="May an agent stop and ask on this task" value={task.askQuestions === undefined || task.askQuestions === null ? 'default' : task.askQuestions ? 'yes' : 'no'} onChange={(event) => onSetQuestions(task, event.target.value === 'default' ? null : event.target.value === 'yes')}>
									<option value="default">Same as the workspace ({askDefault ? 'yes' : 'no'})</option>
									<option value="yes">Yes, stop and ask</option>
									<option value="no">No, go on with a best guess</option>
								</select>
							</label>
							<p className="hx-muted">{(task.askQuestions ?? askDefault) ? 'An agent that cannot decide alone stops and waits for your answer.' : 'An agent decides alone and its open questions show on the plan approval.'}</p>
						</section>
					)}

					<section className="hx-drawer-queue" aria-label="Place in the Task Queue">
						<p>{waits.length ? <>Waits for: <b>{waits.join(', ')}</b></> : 'Waits for nothing.'}{position >= 0 && <span className="hx-muted"> · {position + 1} of {queue.length} in the Task Queue</span>}</p>
						<div className="hx-drawer-queue-actions">
							{position >= 0 && <>
								<button type="button" className="secondary hx-small-button" disabled={position === 0} onClick={() => onMove(task, queue[position - 1])}><ArrowUp size={14} aria-hidden="true" />Move up</button>
								<button type="button" className="secondary hx-small-button" disabled={position >= queue.length - 1} onClick={() => onMove(task, queue[position + 2])}><ArrowDown size={14} aria-hidden="true" />Move down</button>
							</>}
							<button type="button" className="secondary hx-small-button" aria-pressed={task.pinned} onClick={() => onPin(task, !task.pinned)}>{task.pinned ? <PinOff size={14} aria-hidden="true" /> : <Pin size={14} aria-hidden="true" />}{task.pinned ? 'Unpin' : 'Pin first'}</button>
						</div>
					</section>

					<div className="hx-tabs" role="tablist" aria-label="Run details">
						{tabs.map((item, index) => (
							<button key={item.id} ref={(element) => { tabRefs.current[index] = element; }} type="button" role="tab" id={`hx-tab-${item.id}`} aria-controls={`hx-panel-${item.id}`} aria-selected={tab === item.id} tabIndex={tab === item.id ? 0 : -1} className={tab === item.id ? 'on' : undefined} onClick={() => onTabChange(item.id)} onKeyDown={(event) => onTabKey(event, index)}>{item.label}</button>
						))}
					</div>
					<section className="hx-tab-panel" role="tabpanel" id={`hx-panel-${tab}`} aria-labelledby={`hx-tab-${tab}`}>
						{tab === 'ticket' && <TicketTab task={task} fixture={Boolean(client.fixture)} />}
						{tab === 'plan' && (run ? <RunPlanPanel client={client} run={run} /> : <NoRun run={false} />)}
						{tab === 'stages' && (detail ? <PipelineStepper model={pipelineModel(detail, now)} label={`Progress for ${task.source.key}`} orientation="vertical" /> : <NoRun run={Boolean(runId)} />)}
						{tab === 'findings' && (detail ? <FindingsByRound agentRuns={detail.agentRuns} /> : <NoRun run={Boolean(runId)} />)}
						{tab === 'changes' && (runId ? <ChangesTab client={client} runId={runId} agentRun={latestAgentRun(detail?.agentRuns)} /> : <NoRun run={false} />)}
						{tab === 'live' && (runId ? <LiveTab client={client} focus={agentRunId} stages={detail?.stages ?? []} agentRuns={detail?.agentRuns ?? (run?.activeAgentRun ? [run.activeAgentRun] : [])} onReload={() => loadRun(runId)} /> : <NoRun run={false} />)}
						{tab === 'links' && (
							<ul className="hx-links">
								{run?.prUrl && <li><a href={run.prUrl} target="_blank" rel="noreferrer"><ExternalLink size={14} aria-hidden="true" />Draft pull request {prLabel(run.prUrl)}</a></li>}
								{task.source.url && <li><a href={task.source.url} target="_blank" rel="noreferrer"><ExternalLink size={14} aria-hidden="true" />{sourceLabels[task.source.kind]} {task.source.key}</a></li>}
								{onOpenAgentSpace && <li><button type="button" className="hx-link" onClick={() => onOpenAgentSpace(task.workspaceId, 'sessions')}>Open conversation</button></li>}
								{onOpenAgentSpace && <li><button type="button" className="hx-link" onClick={() => onOpenAgentSpace(task.workspaceId)}>Open in Agent Space</button></li>}
								{run && <li className="hx-muted">Branch <code>{run.branch}</code></li>}
							</ul>
						)}
					</section>
				</div>
			</aside>
		</>
	);
}

/** The ticket as its tracker shows it: the Jira panel the Workstream page uses, read by key in the task's workspace. */
function TicketTab({ task, fixture }: { task: HarnessTask; fixture: boolean }) {
	if (task.source.kind === 'jira' && !fixture) return <div className="hx-ticket"><JiraItemPanel workspaceId={task.workspaceId} issueKey={task.source.key} /></div>;
	return (
		<div className="hx-ticket hx-ticket-summary">
			<dl>
				{task.source.status && <><dt>Status</dt><dd>{task.source.status}</dd></>}
				{task.source.priority && <><dt>Priority</dt><dd>{task.source.priority}</dd></>}
				{task.source.assignee && <><dt>Assignee</dt><dd>{task.source.assignee}</dd></>}
				{task.source.labels.length > 0 && <><dt>Labels</dt><dd>{task.source.labels.join(', ')}</dd></>}
			</dl>
			{task.source.url && <a href={task.source.url} target="_blank" rel="noreferrer"><ExternalLink size={14} aria-hidden="true" />Open {task.source.key} in {sourceLabels[task.source.kind]}</a>}
			{fixture && <p className="hx-muted">The sample data has no tracker behind it; a real task shows the whole ticket here.</p>}
		</div>
	);
}

function NoRun({ run }: { run: boolean }) {
	return <p className="hx-muted">{run ? 'Loading…' : 'This task has not started, so there is nothing here yet.'}</p>;
}

function latestAgentRun(agentRuns: HarnessAgentRun[] = []): HarnessAgentRun | undefined {
	return [...agentRuns].sort((a, b) => Date.parse(a.startedAt ?? '') - Date.parse(b.startedAt ?? '')).at(-1);
}

/** Every review in the run, newest first, each with its must-fix items and suggestions. */
function FindingsByRound({ agentRuns }: { agentRuns: HarnessAgentRun[] }) {
	const reviews = agentRuns
		.filter((agentRun) => (agentRun.role === 'plan_reviewer' || agentRun.role === 'code_reviewer') && agentRun.state === 'succeeded')
		.sort((a, b) => Date.parse(b.startedAt ?? '') - Date.parse(a.startedAt ?? ''));
	if (reviews.length === 0) return <p className="hx-muted">The Checker has not reviewed anything in this run yet.</p>;
	return (
		<div className="hx-findings-rounds">
			{reviews.map((review) => (
				<section key={review.id} aria-label={`${hxSteps[review.role === 'plan_reviewer' ? 'plan_check' : 'code_check'].label}, round ${review.round}`}>
					<h3 className="hx-section-title">{hxSteps[review.role === 'plan_reviewer' ? 'plan_check' : 'code_check'].label} · round {review.round} <small>· {review.verdict === 'approve' ? 'approved' : 'changes asked'}</small></h3>
					<FindingsList findings={review.findings ?? []} />
				</section>
			))}
		</div>
	);
}

/**
 * The run's changed files. When the backend exposes the agent's work as a
 * Kode Stream session, the session's own Changes panel (with diffs) is used;
 * otherwise the run's file list.
 */
function ChangesTab({ client, runId, agentRun }: { client: HarnessClient; runId: string; agentRun?: HarnessAgentRun }) {
	const session = useAgentSession(client, agentRun?.id);
	const [changes, setChanges] = useState<SessionChanges | undefined>();
	const [error, setError] = useState('');
	useEffect(() => {
		if (session.record) return;
		const controller = new AbortController();
		client.runChanges(runId, controller.signal).then(setChanges).catch(() => { if (!controller.signal.aborted) setError('The changed files could not be loaded.'); });
		return () => controller.abort();
	}, [client, runId, session.record]);
	if (session.record) return <SessionChangesPanel session={session.record} />;
	if (error) return <p className="hx-banner" role="alert">{error}</p>;
	if (!changes) return <p className="hx-muted">Loading the changed files…</p>;
	if (changes.files.length === 0) return <p className="hx-muted">No files have changed yet.</p>;
	const words: Record<string, string> = { A: 'Added', M: 'Changed', D: 'Deleted', R: 'Renamed', C: 'Copied', '?': 'New' };
	return (
		<div className="hx-changes">
			<p className="hx-muted hx-num">{plural(changes.files.length, 'file')} · +{changes.additions} −{changes.deletions}{changes.truncated ? ' · list cut short' : ''}</p>
			<ul>{changes.files.map((file) => <li key={file.path}><span className="hx-chip">{words[file.status] ?? file.status}</span><code>{file.path}</code></li>)}</ul>
		</div>
	);
}

const transcriptEvery = 5_000;

/**
 * What each agent said, in the order it happened: every conversation of the
 * run as one numbered list, grouped by the stage that ran it, oldest first,
 * with the working agent chosen by default. A running agent's transcript is
 * read again every few seconds; its live terminal is shown instead when the
 * backend exposes the run as a session.
 */
export function LiveTab({ client, focus, stages, agentRuns, onReload }: { client: HarnessClient; focus?: string; stages: HarnessStage[]; agentRuns: HarnessAgentRun[]; onReload: () => Promise<unknown> }) {
	const steps = useMemo(() => liveSteps(stages, agentRuns), [stages, agentRuns]);
	const rounds = useMemo(() => steps.flatMap((step) => step.agentRuns), [steps]);
	const working = rounds.find((agentRun) => agentRun.state === 'running');
	const [chosen, setChosen] = useState<string | undefined>(focus);
	useEffect(() => { if (focus) setChosen(focus); }, [focus]);
	const selected = rounds.find((agentRun) => agentRun.id === chosen) ?? working ?? rounds.at(-1);
	// Which stages the person folded or unfolded by hand; the rest fold unless they hold the chosen or the working agent.
	const [folded, setFolded] = useState<Record<string, boolean>>({});
	const stageOpen = (step: LiveStep) => folded[step.id] ?? step.agentRuns.some((agentRun) => agentRun.id === selected?.id || agentRun.state === 'running');
	const session = useAgentSession(client, selected?.id);
	const transcript = useAgentTranscript(client, selected);
	if (!selected) return <p className="hx-muted">No agent has worked on this task yet.</p>;
	let number = 0;
	return (
		<div className="hx-live">
			{rounds.length > 1 && (
				<ol className="hx-live-steps" role="tablist" aria-orientation="vertical" aria-label="Agent rounds">
					{steps.map((step) => {
						const open = stageOpen(step);
						const first = number + 1;
						if (!open) number += step.agentRuns.length;
						return (
						<li key={step.id} className="hx-live-stage">
							<button type="button" className="hx-live-stage-title" aria-expanded={open} onClick={() => setFolded((prev) => ({ ...prev, [step.id]: !open }))}>
								<ChevronRight size={14} aria-hidden="true" />{step.title}
								<span className="hx-live-stage-count hx-num">{open ? plural(step.agentRuns.length, 'step') : first === number ? `step ${first}` : `steps ${first}–${number}`}</span>
							</button>
							{open && <ol>
								{step.agentRuns.map((agentRun) => {
									number += 1;
									const on = agentRun.id === selected.id;
									return (
										<li key={agentRun.id}>
											<button type="button" role="tab" aria-selected={on} className={on ? 'on' : undefined} onClick={() => setChosen(agentRun.id)}>
												<span className="hx-live-step-no hx-num">{number}</span>
												<HxStatusBadge status={agentStatus(agentRun)} size="sm" label={`${agentRoleLabels[agentRun.role]} · round ${agentRun.round}${(agentRun.attempt ?? 1) > 1 ? ` · try ${agentRun.attempt}` : ''}`} />
												<span className="hx-live-step-meta hx-num">{agentRun.state === 'running' ? 'working' : `${plural(agentRun.turns, 'turn')} · ${formatCost(agentRun.costUsd)}`}</span>
											</button>
										</li>
									);
								})}
							</ol>}
						</li>
						);
					})}
				</ol>
			)}
			<p><b>{agentRoleLabels[selected.role]}</b> · {modelLabel(selected.model)} · <span className="hx-num">{plural(selected.turns, 'turn')} · {formatCost(selected.costUsd)}</span>{selected.state !== 'running' && <span className="hx-muted"> · {hxStatuses[agentStatus(selected)].label.toLowerCase()}</span>}</p>
			{selected.state === 'running' && <p className="hx-live-line">{selected.progress ?? 'Starting…'}</p>}
			{session.record?.live
				? (
					<Suspense fallback={<p className="hx-muted">Opening the live view…</p>}>
						<SessionTerminal record={session.record} visible onReload={onReload} classPrefix="hx-terminal" ariaLabel={`${agentRoleLabels[selected.role]} live terminal`} />
					</Suspense>
				)
				: transcript.error
					? <p className="hx-banner" role="alert">{transcript.error}</p>
					: transcript.turns && transcript.turns.length === 0
						? <p className="hx-muted">{selected.state === 'running' ? 'The conversation has not started yet.' : 'This agent left no conversation the harness can read.'}</p>
						: <TranscriptTurns className="hx-transcript" turns={transcript.turns} label={`${agentRoleLabels[selected.role]} conversation`} placeholder="Reading the conversation…" names={{ user: 'Harness', assistant: agentRoleLabels[selected.role] }} />}
			<p className="hx-muted">What the harness asked, what the {agentRoleLabels[selected.role]} thought, each tool it called, and what it answered. Tool output stays out; the coding tool keeps the full conversation.</p>
		</div>
	);
}

type LiveStep = { id: string; title: string; agentRuns: HarnessAgentRun[] };

/**
 * The run's conversations grouped by the stage that ran them, in the order the
 * stages ran and, within a stage, the order the agents started. A stage of a
 * kind that ran more than once (a retry after a stop) is numbered, so "Plan 2"
 * is the second Plan stage. An agent whose stage is unknown goes last.
 */
export function liveSteps(stages: HarnessStage[], agentRuns: HarnessAgentRun[]): LiveStep[] {
	const byStart = (a: HarnessAgentRun, b: HarnessAgentRun) => Date.parse(a.startedAt ?? '') - Date.parse(b.startedAt ?? '');
	const live = agentRuns.filter((agentRun) => agentRun.state !== 'queued').sort(byStart);
	const ordered = [...stages].sort((a, b) => a.ordinal - b.ordinal || Date.parse(a.startedAt ?? '') - Date.parse(b.startedAt ?? ''));
	const seen: Partial<Record<HarnessStage['kind'], number>> = {};
	const steps: LiveStep[] = [];
	for (const stage of ordered) {
		const own = live.filter((agentRun) => agentRun.stageId === stage.id);
		if (own.length === 0) continue;
		const nth = (seen[stage.kind] ?? 0) + 1;
		seen[stage.kind] = nth;
		const repeated = ordered.filter((other) => other.kind === stage.kind).length > 1;
		steps.push({ id: stage.id, title: repeated ? `${stageKindLabels[stage.kind]} ${nth}` : stageKindLabels[stage.kind], agentRuns: own });
	}
	const placed = new Set(steps.flatMap((step) => step.agentRuns.map((agentRun) => agentRun.id)));
	const rest = live.filter((agentRun) => !placed.has(agentRun.id));
	if (rest.length > 0) steps.push({ id: 'other', title: 'Other', agentRuns: rest });
	return steps;
}

function agentStatus(agentRun: HarnessAgentRun): HxStatus {
	switch (agentRun.state) {
		case 'running': return 'working';
		case 'succeeded': return 'done';
		case 'failed': return 'failed';
		case 'cancelled': return 'stopped';
		default: return 'not_started';
	}
}

/** The transcript of one agent run, read once when it is finished and every few seconds while it runs. */
function useAgentTranscript(client: HarnessClient, agentRun?: HarnessAgentRun): { turns?: ClaudeTurn[]; error: string } {
	const [turns, setTurns] = useState<ClaudeTurn[] | undefined>();
	const [error, setError] = useState('');
	const id = agentRun?.id;
	const running = agentRun?.state === 'running';
	useEffect(() => {
		setTurns(undefined);
		setError('');
		if (!id) return;
		const controller = new AbortController();
		let timer: number | undefined;
		const read = () => {
			client.agentRunTranscript(id, 50, controller.signal)
				.then((result) => {
					if (controller.signal.aborted) return;
					setTurns(result.turns);
					if (running && document.visibilityState !== 'hidden') timer = window.setTimeout(read, transcriptEvery);
					else if (running) timer = window.setTimeout(read, transcriptEvery * 4);
				})
				.catch(() => { if (!controller.signal.aborted) setError('The conversation could not be read.'); });
		};
		read();
		return () => { controller.abort(); window.clearTimeout(timer); };
	}, [client, id, running]);
	return { turns, error };
}

function useAgentSession(client: HarnessClient, agentRunId?: string): { record?: SafeSessionRecord } {
	const [record, setRecord] = useState<SafeSessionRecord | undefined>();
	useEffect(() => {
		setRecord(undefined);
		if (!agentRunId) return;
		const controller = new AbortController();
		client.agentRun(agentRunId, controller.signal).then((result) => { if (!controller.signal.aborted) setRecord(result.session); }).catch(() => undefined);
		return () => controller.abort();
	}, [agentRunId, client]);
	return useMemo(() => ({ record }), [record]);
}

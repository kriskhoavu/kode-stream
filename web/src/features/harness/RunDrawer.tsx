import { useEffect, useRef } from 'react';
import type { KeyboardEvent } from 'react';
import { ArrowDown, ArrowUp, ExternalLink, GripVertical, Pin, PinOff, Play, X } from 'lucide-react';
import { useResizableDrawer } from '../../components/useResizableDrawer';
import { StopRunButton, canStopRun } from './StopRunButton';
import { RemoveTaskButton } from './RemoveTaskButton';
import { ResetTaskButton, canResetTask } from './ResetTaskButton';
import type { HarnessApproval, HarnessRun, HarnessTask } from '../../lib/types';
import type { HarnessRunTab } from '../../app/router';
import type { HarnessClient } from './harnessClient';
import type { HarnessState } from './harnessState';
import { canStartTask, startPending, waitsFor } from './harnessState';
import { GateCard } from './GateCard';
import { HxStatusBadge } from './HxStatusBadge';
import { PipelineStepper } from './PipelineStepper';
import { SourceBadge } from './SourceBadge';
import { RunPlanPanel } from './RunPlanPanel';
import { pipelineModel, prLabel, runStatus } from './pipeline';
import { useNow } from './useNow';
import { sourceLabels, taskStatus } from './vocabulary';
import { LiveTab } from './LiveTab';
import { TicketTab, NoRun, FindingsByRound, ChangesTab, latestAgentRun } from './runTabs';

const tabs: { id: HarnessRunTab; label: string }[] = [
	{ id: 'ticket', label: 'Ticket' },
	{ id: 'plan', label: 'Plan' },
	{ id: 'stages', label: 'Stages' },
	{ id: 'findings', label: 'Findings' },
	{ id: 'changes', label: 'Changes' },
	{ id: 'live', label: 'Live' },
	{ id: 'links', label: 'Links' }
];

export function RunDrawer({ state, client, taskId, tab, agentRunId, queue, onTabChange, onClose, onOpenDetails, onCancel, onOpenAgentSpace, loadRun, onApprove, onReject, onPin, onMove, onStart, onReset, onRemove, onSetQuestions, askDefault = true }: {
	state: HarnessState;
	client: HarnessClient;
	taskId: string;
	tab: HarnessRunTab;
	agentRunId?: string;
	queue: HarnessTask[];
	onTabChange: (tab: HarnessRunTab) => void;
	onClose: () => void;
	onOpenDetails?: (runId: string) => void;
	onCancel?: (run: HarnessRun) => Promise<void>;
	onOpenAgentSpace?: (workspaceId: string, view?: 'sessions') => void;
	loadRun: (runId: string) => Promise<void>;
	onApprove: (approval: HarnessApproval, reason?: string) => void;
	onReject: (approval: HarnessApproval, reason: string) => void;
	onPin: (task: HarnessTask, pinned: boolean) => void;
	onMove: (task: HarnessTask, before?: HarnessTask) => void;
	onStart?: (task: HarnessTask) => void;
	onReset?: (task: HarnessTask) => Promise<void>;
	onRemove?: (task: HarnessTask) => Promise<void>;
	onSetQuestions?: (task: HarnessTask, askQuestions: boolean | null) => void;
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
		// Only a new task moves focus. `onClose` is read when Escape is pressed.
	}, [taskId]);

	if (!task) return null;
	const waits = waitsFor(task, state.tasks);
	const status = task.waitingFor ? 'reporter' : run ? runStatus(run) : taskStatus(task, waits);
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
						{onCancel && canStopRun(run) && <StopRunButton task={task} run={run} onCancel={onCancel} />}
						{onReset && canResetTask(task) && <ResetTaskButton task={task} onReset={onReset} />}
						{onRemove && <RemoveTaskButton task={task} onRemove={onRemove} />}
						{run && onOpenDetails && <button type="button" className="secondary drawer-open-details" onClick={() => onOpenDetails(run.id)}>Open details</button>}
						<button type="button" className="icon-button" aria-label="Close details" onClick={onClose}><X size={16} /></button>
					</div>
				</header>
				<div className="plan-drawer-body hx-drawer-body">
					{waiting.map((approval) => (
						<GateCard key={approval.id} approval={state.approvals[approval.id] ?? approval} task={task} sent={state.decisions[approval.id]}
							onApprove={(reason) => onApprove(state.approvals[approval.id] ?? approval, reason)}
							onReject={(reason) => onReject(state.approvals[approval.id] ?? approval, reason)} />
					))}

					{onStart && canStartTask(task) && (!run || task.state === 'cancelled') && (
						<section className="hx-drawer-start" aria-label="Start this task">
							{startPending(task)
								? <p><b>Starting soon.</b> The next free Planner takes it, usually within a minute.</p>
								: task.state === 'cancelled'
									? <p><b>Stopped.</b> {task.note ? <>{task.note} </> : ''}Start again begins a new run from a fresh plan. The stopped run stays in its history.</p>
									: <p>{waits.length ? <>Starting now skips the wait for <b>{waits.join(', ')}</b>. </> : ''}The Planner writes a plan first, and you approve it before any code is written.</p>}
							<button type="button" className="primary hx-small-button" disabled={startPending(task)} onClick={() => onStart(task)}><Play size={14} aria-hidden="true" />{startPending(task) ? 'Starting…' : task.state === 'cancelled' ? 'Start again' : 'Start'}</button>
						</section>
					)}

					{onSetQuestions && !['done', 'dismissed', 'stale'].includes(task.state) && (
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

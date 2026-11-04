import { memo } from 'react';
import type { DragEvent } from 'react';
import { Pin, PinOff } from 'lucide-react';
import type { HarnessRun, HarnessTask } from '../../lib/types';
import { HxStatusBadge } from './HxStatusBadge';
import { SourceBadge } from './SourceBadge';
import { prLabel } from './pipeline';
import type { HxStatus } from './vocabulary';
import { agentRoleLabels, hxStatuses, modelLabel, roleLabels, stageKindLabels } from './vocabulary';

/** The short line under a card's title: who is on it, or what it waits for. */
export function workerLine(task: HarnessTask, status: HxStatus, waitsFor: string[], run?: HarnessRun): string {
	if (status === 'manual') return `done by hand${task.finishedOutside?.by ? ` · ${task.finishedOutside.by}` : ''}`;
	if (status === 'done') return run?.prUrl ? `Draft PR ${prLabel(run.prUrl)}` : 'done';
	if (status === 'waiting') return run?.currentStage?.kind === 'pr_approval' ? 'approve the draft PR' : run?.state === 'blocked' ? 'stuck · needs you' : 'approve the plan';
	if (status === 'stopped') return run?.state === 'blocked' ? 'stuck · needs you' : 'stopped';
	if (status === 'working') {
		const agent = run?.activeAgentRun;
		if (agent) return `${agentRoleLabels[agent.role]} · ${modelLabel(agent.model)}`;
		return run?.currentStage?.kind === 'qa' ? `${roleLabels.qa} · running tests` : run?.currentStage ? stageKindLabels[run.currentStage.kind] : 'starting';
	}
	if (waitsFor.length) return `waits for ${waitsFor.join(', ')}`;
	if (status === 'up_next') return `next for the ${roleLabels.planner}`;
	return '';
}

export const TaskCard = memo(function TaskCard({ task, status, waitsFor, run, critical, pending, highlighted, registerAnchor, onOpen, onPin, onHover, onDragStart, onDragEnd }: {
	task: HarnessTask;
	status: HxStatus;
	waitsFor: string[];
	run?: HarnessRun;
	critical: boolean;
	pending: boolean;
	highlighted: boolean;
	registerAnchor: (id: string, element: HTMLElement | null) => void;
	onOpen: (task: HarnessTask) => void;
	onPin: (task: HarnessTask, pinned: boolean) => void;
	onHover: (id: string | undefined) => void;
	onDragStart: (event: DragEvent<HTMLElement>, task: HarnessTask) => void;
	onDragEnd: () => void;
}) {
	const worker = workerLine(task, status, waitsFor, run);
	const label = [task.source.key, task.title, hxStatuses[status].label, critical ? 'on the critical path' : '', waitsFor.length ? `waits for ${waitsFor.join(', ')}` : '', task.pinned ? 'pinned' : ''].filter(Boolean).join(', ');
	const classes = ['hx-task', `hx-task-${status}`, critical ? 'critical' : '', pending ? 'move-pending' : '', highlighted ? 'related' : ''].filter(Boolean).join(' ');
	return (
		<article
			ref={(element) => registerAnchor(task.id, element)}
			className={classes}
			data-task-id={task.id}
			draggable={!pending}
			aria-busy={pending || undefined}
			onDragStart={(event) => onDragStart(event, task)}
			onDragEnd={onDragEnd}
			onMouseEnter={() => onHover(task.id)}
			onMouseLeave={() => onHover(undefined)}
			onFocus={() => onHover(task.id)}
			onBlur={() => onHover(undefined)}
		>
			<button type="button" className="hx-task-open" aria-label={label} onClick={() => onOpen(task)}>
				<span className="hx-task-r1">
					<b>{task.source.key}</b>
					<SourceBadge kind={task.source.kind} />
					{task.milestone && <span>{task.milestone}</span>}
					<HxStatusBadge status={status} size="sm" />
				</span>
				<span className="hx-task-title">{task.title}</span>
				<span className="hx-task-r3">
					{critical && <strong className="hx-critical-word">Critical</strong>}
					{worker && <span className="hx-task-worker">{worker}</span>}
					{task.epic && <span>{task.epic}</span>}
					{task.risk === 'high' && <span className="hx-risk">High risk</span>}
				</span>
			</button>
			<button type="button" className={task.pinned ? 'hx-task-pin on' : 'hx-task-pin'} aria-pressed={task.pinned} aria-label={task.pinned ? `Unpin ${task.source.key}` : `Pin ${task.source.key} first in its wave`} title={task.pinned ? 'Pinned: keeps its place' : 'Pin first in its wave'} onClick={() => onPin(task, !task.pinned)}>
				{task.pinned ? <Pin size={12} aria-hidden="true" /> : <PinOff size={12} aria-hidden="true" />}
			</button>
		</article>
	);
});

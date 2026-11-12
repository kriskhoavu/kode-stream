import { GripVertical, Pin, PinOff, Play, RotateCcw } from 'lucide-react';
import type { DragEvent } from 'react';
import { useState } from 'react';
import type { HarnessTask } from '../../lib/types';
import { HxStatusBadge } from './HxStatusBadge';
import { SourceBadge } from './SourceBadge';
import { taskStatus } from './vocabulary';
import { canStartTask, startPending, waitsFor } from './harnessState';

/**
 * The queue in rank order: what the Sorter will hand out next, first. With
 * the optional handlers it also shows each row's score, lets a person start
 * or retry a task, select a row for the side panel, and drag a row above
 * another to move it in the Task Queue.
 */
export function QueueList({ tasks, allTasks, onPin, onOpen, onSelect, selectedId, showScore = false, onStart, onRetry, canRetry, onMove, pendingIds }: {
	tasks: HarnessTask[];
	allTasks: Record<string, HarnessTask>;
	onPin: (task: HarnessTask, pinned: boolean) => void;
	onOpen: (task: HarnessTask) => void;
	/** Chooses a row for the side panel. */
	onSelect?: (task: HarnessTask) => void;
	selectedId?: string;
	showScore?: boolean;
	/** Starts a task now; shown on a task not yet started. */
	onStart?: (task: HarnessTask) => void;
	/** Retries a task that is stuck; shown where `canRetry` says so. */
	onRetry?: (task: HarnessTask) => void;
	canRetry?: (task: HarnessTask) => boolean;
	/** Move `task` above `before` in the Task Queue; no `before` means last. */
	onMove?: (task: HarnessTask, before?: HarnessTask) => void;
	pendingIds?: Set<string>;
}) {
	const [dragId, setDragId] = useState<string | undefined>();
	if (tasks.length === 0) return <div className="hx-card hx-empty"><span>The Task Queue is empty. The Finder adds tasks from Jira, GitHub and Bitbucket when it next looks.</span></div>;
	const dragStart = (event: DragEvent<HTMLElement>, task: HarnessTask) => {
		event.dataTransfer.effectAllowed = 'move';
		event.dataTransfer.setData('text/plain', task.id);
		setDragId(task.id);
	};
	const drop = (event: DragEvent<HTMLElement>, before?: HarnessTask) => {
		if (!dragId || !onMove) return;
		event.preventDefault();
		const task = allTasks[dragId] ?? tasks.find((candidate) => candidate.id === dragId);
		setDragId(undefined);
		if (!task || before?.id === task.id) return;
		onMove(task, before);
	};
	return (
		<ol className="hx-queue-list" aria-label="Task Queue in order" onDragOver={(event) => { if (dragId && onMove) event.preventDefault(); }} onDrop={(event) => { if ((event.target as HTMLElement).closest('[data-task-id]')) return; drop(event); }}>
			{tasks.map((task, index) => {
				const waiting = waitsFor(task, allTasks);
				const status = taskStatus(task, waiting);
				const pending = pendingIds?.has(task.id) ?? false;
				const selected = selectedId === task.id;
				const startable = onStart && canStartTask(task);
				const starting = startPending(task);
				const retryable = onRetry && canRetry?.(task);
				const classes = ['hx-row', 'hx-queue-row', selected ? 'selected' : '', onMove ? 'draggable' : '', dragId === task.id ? 'dragging' : '', pending ? 'move-pending' : ''].filter(Boolean).join(' ');
				return (
					<li
						key={task.id}
						className={classes}
						data-task-id={task.id}
						aria-current={selected ? 'true' : undefined}
						aria-busy={pending || undefined}
						draggable={Boolean(onMove) && !pending}
						onDragStart={(event) => dragStart(event, task)}
						onDragEnd={() => setDragId(undefined)}
						onDragOver={(event) => { if (dragId && onMove) { event.preventDefault(); event.dataTransfer.dropEffect = 'move'; } }}
						onDrop={(event) => { event.stopPropagation(); drop(event, task); }}
						onClick={onSelect ? () => onSelect(task) : undefined}
					>
						<span className="hx-queue-rank hx-num" aria-label={`Position ${index + 1}`}>{onMove && <GripVertical size={13} aria-hidden="true" className="hx-queue-grip" />}{index + 1}</span>
						<span className="hx-row-key"><b>{task.source.key}</b><SourceBadge kind={task.source.kind} url={task.source.url} /></span>
						<span className="hx-row-title">
							<button type="button" className="hx-title-button" onClick={(event) => { event.stopPropagation(); onOpen(task); }}>{task.title}</button>
							<small className="hx-muted" title={task.scoreReasons.join(' · ') || undefined}>
								{waiting.length ? `Waits for ${waiting.join(', ')}` : task.scoreReasons[0] ?? ''}
							</small>
						</span>
						{showScore && <span className="hx-queue-score hx-num" title="The Sorter's score: higher goes first">{Math.round(task.score)}</span>}
						<HxStatusBadge status={status} />
						<span className="hx-queue-actions">
							{startable && <button type="button" className="primary hx-small-button" disabled={pending || starting} onClick={(event) => { event.stopPropagation(); onStart(task); }}><Play size={13} aria-hidden="true" />{starting ? 'Starting…' : task.state === 'cancelled' ? 'Start again' : 'Start'}</button>}
							{retryable && <button type="button" className="secondary hx-small-button" disabled={pending} onClick={(event) => { event.stopPropagation(); onRetry(task); }}><RotateCcw size={13} aria-hidden="true" />Retry</button>}
							<button type="button" className={task.pinned ? 'hx-icon-toggle on' : 'hx-icon-toggle'} aria-pressed={task.pinned} aria-label={task.pinned ? `Unpin ${task.source.key}` : `Pin ${task.source.key} to the top`} disabled={pending} onClick={(event) => { event.stopPropagation(); onPin(task, !task.pinned); }}>
								{task.pinned ? <PinOff size={15} aria-hidden="true" /> : <Pin size={15} aria-hidden="true" />}
								<span>{task.pinned ? 'Pinned' : 'Pin'}</span>
							</button>
						</span>
					</li>
				);
			})}
		</ol>
	);
}

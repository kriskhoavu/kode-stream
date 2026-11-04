import { Pin, PinOff } from 'lucide-react';
import type { HarnessTask } from '../../lib/types';
import { HxStatusBadge } from './HxStatusBadge';
import { SourceBadge } from './SourceBadge';
import { taskStatus } from './vocabulary';
import { waitsFor } from './harnessState';

/** The queue in rank order: what the Sorter will hand out next, first. */
export function QueueList({ tasks, allTasks, onPin, onOpen }: {
	tasks: HarnessTask[];
	allTasks: Record<string, HarnessTask>;
	onPin: (task: HarnessTask, pinned: boolean) => void;
	onOpen: (task: HarnessTask) => void;
}) {
	if (tasks.length === 0) return <div className="hx-card hx-empty"><span>The queue is empty. The Finder adds tasks from Jira, GitHub and Bitbucket when it next looks.</span></div>;
	return (
		<ol className="hx-queue-list" aria-label="Queue in order">
			{tasks.map((task, index) => {
				const waiting = waitsFor(task, allTasks);
				return (
					<li key={task.id} className="hx-row hx-queue-row">
						<span className="hx-queue-rank hx-num" aria-label={`Position ${index + 1}`}>{index + 1}</span>
						<span className="hx-row-key"><b>{task.source.key}</b><SourceBadge kind={task.source.kind} url={task.source.url} /></span>
						<span className="hx-row-title">
							<button type="button" className="hx-title-button" onClick={() => onOpen(task)}>{task.title}</button>
							<small className="hx-muted" title={task.scoreReasons.join(' · ') || undefined}>
								{waiting.length ? `Waits for ${waiting.join(', ')}` : task.scoreReasons[0] ?? ''}
							</small>
						</span>
						<HxStatusBadge status={taskStatus(task, waiting)} />
						<button type="button" className={task.pinned ? 'hx-icon-toggle on' : 'hx-icon-toggle'} aria-pressed={task.pinned} aria-label={task.pinned ? `Unpin ${task.source.key}` : `Pin ${task.source.key} to the top`} onClick={() => onPin(task, !task.pinned)}>
							{task.pinned ? <PinOff size={15} aria-hidden="true" /> : <Pin size={15} aria-hidden="true" />}
							<span>{task.pinned ? 'Pinned' : 'Pin'}</span>
						</button>
					</li>
				);
			})}
		</ol>
	);
}

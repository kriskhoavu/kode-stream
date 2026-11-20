import { useState } from 'react';
import { ExternalLink, Radar } from 'lucide-react';
import type { HarnessTask } from '../../lib/types';
import { HxStatusBadge } from '../harness/HxStatusBadge';
import { SourceBadge } from '../harness/SourceBadge';
import { prLabel } from '../harness/pipeline';
import { RemoveTaskButton } from '../harness/RemoveTaskButton';
import { isFinishedTask } from './harnessJoin';
import { QueueList } from '../harness/QueueList';
import { compareTasks, waitsFor } from '../harness/harnessState';
import type { HarnessController } from '../harness/useHarness';
import { useNow } from '../harness/useNow';
import { formatAgo, hxStatuses, plural, taskStatus } from '../harness/vocabulary';
import { useTaskActions } from './useTaskActions';

/**
 * The queue as a view of the board: the ranked rows, and beside them why the
 * chosen row sits where it does, what starts next, and when the Finder last
 * looked.
 */
export function QueueView({ tasks, finished = [], harness, onOpenTask }: {
	tasks: HarnessTask[];
	/** Finished tasks, newest first, listed apart under Show finished. */
	finished?: HarnessTask[];
	harness: HarnessController;
	onOpenTask: (task: HarnessTask) => void;
}) {
	const { state } = harness;
	const actions = useTaskActions(harness);
	const [selectedId, setSelectedId] = useState<string | undefined>();
	const [shown, setShown] = useState<QueueShown>('active');
	const ordered = tasks.filter((task) => !isFinishedTask(task)).sort(compareTasks);
	const selected = ordered.find((task) => task.id === selectedId) ?? ordered[0];
	return (
		<div className="queue-view hx">
			<div className="queue-main">
			<div className="queue-toolbar hx-seg hx-seg-small" role="group" aria-label="Show">
				{queueShownOptions.map((option) => (
					<button key={option.id} type="button" className={shown === option.id ? 'on' : undefined} aria-pressed={shown === option.id} onClick={() => setShown(option.id)}>
						{option.label} <span className="hx-num">{option.id === 'active' ? ordered.length : option.id === 'finished' ? finished.length : ordered.length + finished.length}</span>
					</button>
				))}
			</div>
			{shown === 'all' && <h3 className="queue-section-title">Active</h3>}
			{shown !== 'finished' && <QueueList
				tasks={ordered}
				allTasks={state.tasks}
				runs={state.runs}
				onStop={harness.cancelRun}
				onRemove={harness.removeTask}
				onPin={actions.pin}
				onOpen={onOpenTask}
				onSelect={(task) => setSelectedId(task.id)}
				selectedId={selected?.id}
				showScore
				onStart={actions.start}
				onRetry={actions.retry}
				canRetry={actions.canRetry}
				onMove={actions.move}
				pendingIds={actions.pendingIds}
			/>}
			{shown === 'all' && <h3 className="queue-section-title">Finished</h3>}
			{shown !== 'active' && <FinishedList tasks={finished} harness={harness} onOpenTask={onOpenTask} />}
			</div>
			<QueueSidePanel selected={selected} tasks={ordered} harness={harness} />
		</div>
	);
}

function QueueSidePanel({ selected, tasks, harness }: { selected?: HarnessTask; tasks: HarnessTask[]; harness: HarnessController }) {
	const { state } = harness;
	const now = useNow(60_000);
	const [looking, setLooking] = useState(false);
	const [lookNote, setLookNote] = useState('');
	const next = tasks.find((task) => taskStatus(task, waitsFor(task, state.tasks)) === 'up_next');
	const finder = state.status?.roles.find((role) => role.role === 'scout');
	const lookNow = async () => {
		if (!harness.client.pollScouts) return;
		setLooking(true);
		setLookNote('');
		try {
			const result = await harness.client.pollScouts();
			setLookNote(result.started ? 'The Finder is looking now. New tasks appear here as it finds them.' : 'The Finder is already looking.');
			await harness.refresh();
		} catch {
			setLookNote('The Finder could not be asked. Try again.');
		} finally {
			setLooking(false);
		}
	};
	return (
		<aside className="queue-side hx-card" aria-label="About the Task Queue">
			{selected ? (
				<section className="queue-side-section" aria-labelledby="queue-why">
					<h3 id="queue-why">Why here</h3>
					<p className="queue-side-task"><b>{selected.source.key}</b> · {selected.title}</p>
					<p className="queue-side-score"><span>Score</span><b className="hx-num">{Math.round(selected.score)}</b>{selected.pinned && <span className="queue-side-flag">Pinned first</span>}{selected.priorityOverride ? <span className="queue-side-flag">{`Moved by hand ${selected.priorityOverride > 0 ? '+' : ''}${selected.priorityOverride}`}</span> : null}</p>
					{selected.scoreReasons.length > 0
						? <ul className="queue-side-reasons" aria-label="What makes the score">{selected.scoreReasons.map((reason) => {
							const [, label, points] = /^(.*?)\s*([+-]\d+)?$/.exec(reason) ?? [reason, reason, undefined];
							return <li key={reason}><span>{label}</span>{points && <span className="hx-num">{points}</span>}</li>;
						})}</ul>
						: <p className="hx-muted">The Sorter gave no reasons for this score.</p>}
					{waitsFor(selected, state.tasks).length > 0 && <p className="hx-muted">Waits for {waitsFor(selected, state.tasks).join(', ')}.</p>}
				</section>
			) : <section className="queue-side-section"><p className="hx-muted">Choose a row to see why it sits where it does.</p></section>}

			<section className="queue-side-section" aria-labelledby="queue-next">
				<h3 id="queue-next">What starts next</h3>
				{next
					? <><p className="queue-side-task"><b>{next.source.key}</b> · {next.title}</p><p className="hx-muted">{state.status?.autoStart ? 'Starts when a Planner is free.' : 'Starts when you press Start.'}</p></>
					: <p className="hx-muted">Nothing is ready to start. {tasks.some((task) => hxStatuses[taskStatus(task, waitsFor(task, state.tasks))].tone === 'idle') ? 'The next tasks wait for others to finish.' : ''}</p>}
			</section>

			<section className="queue-side-section" aria-labelledby="queue-finder">
				<h3 id="queue-finder"><Radar size={14} aria-hidden="true" /> The Finder</h3>
				<p className="hx-muted">{finder?.paused ? 'Paused: no new tasks are found.' : `Last looked ${formatAgo(finder?.lastRunAt, now)}${finder?.found ? ` · ${plural(finder.found, 'new task')}` : ''}.`}</p>
				<button type="button" className="secondary hx-small-button" disabled={looking || Boolean(finder?.paused) || !harness.client.pollScouts} onClick={() => void lookNow()}>{looking ? 'Looking…' : 'Look now'}</button>
				{lookNote && <p className="hx-muted" role="status">{lookNote}</p>}
			</section>
		</aside>
	);
}

/** What the Task Queue lists: the tasks still in play (the default), the finished ones, or both. */
type QueueShown = 'active' | 'finished' | 'all';
const queueShownOptions: { id: QueueShown; label: string }[] = [{ id: 'active', label: 'Active' }, { id: 'finished', label: 'Finished' }, { id: 'all', label: 'All' }];

/** Every task that is done, stopped, dismissed or closed at its source, with Remove. */
function FinishedList({ tasks, harness, onOpenTask }: { tasks: HarnessTask[]; harness: HarnessController; onOpenTask: (task: HarnessTask) => void }) {
	const { state } = harness;
	const now = useNow(60_000);
	if (tasks.length === 0) return <div className="hx-card hx-empty"><span>Nothing has finished yet.</span></div>;
	return (
		<>
			<ol className="hx-queue-list queue-finished" aria-label="Finished tasks">
				{tasks.map((task) => {
					const run = task.runId ? state.runs[task.runId] : undefined;
					const at = run?.finishedAt ?? task.finishedOutside?.at ?? task.updatedAt;
					return (
						<li key={task.id} className="hx-row hx-queue-row" data-task-id={task.id}>
							<span className="hx-queue-rank" />
							<span className="hx-row-key"><b>{task.source.key}</b><SourceBadge kind={task.source.kind} url={task.source.url} /></span>
							<span className="hx-row-title">
								<button type="button" className="hx-title-button" onClick={() => onOpenTask(task)}>{task.title}</button>
								<small className="hx-muted">{at ? `Finished ${formatAgo(at, now)}` : 'Finished'}{task.note ? ` · ${task.note}` : ''}</small>
							</span>
							<span />
							<HxStatusBadge status={taskStatus(task)} />
							<span className="hx-queue-actions">
								{run?.prUrl && <a className="hx-link" href={run.prUrl} target="_blank" rel="noreferrer"><ExternalLink size={13} aria-hidden="true" /> Draft PR {prLabel(run.prUrl)}</a>}
								<RemoveTaskButton task={task} onRemove={harness.removeTask} />
							</span>
						</li>
					);
				})}
			</ol>
		</>
	);
}

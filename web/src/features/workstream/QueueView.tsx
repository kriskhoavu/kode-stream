import { useState } from 'react';
import { Radar } from 'lucide-react';
import type { HarnessTask } from '../../lib/types';
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
export function QueueView({ tasks, harness, onOpenTask }: {
	tasks: HarnessTask[];
	harness: HarnessController;
	onOpenTask: (task: HarnessTask) => void;
}) {
	const { state } = harness;
	const actions = useTaskActions(harness);
	const [selectedId, setSelectedId] = useState<string | undefined>();
	const ordered = [...tasks].sort(compareTasks);
	const selected = ordered.find((task) => task.id === selectedId) ?? ordered[0];
	return (
		<div className="queue-view hx">
			<QueueList
				tasks={ordered}
				allTasks={state.tasks}
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
			/>
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
		<aside className="queue-side hx-card" aria-label="About the queue">
			{selected ? (
				<section className="queue-side-section" aria-labelledby="queue-why">
					<h3 id="queue-why">Why here</h3>
					<p className="queue-side-task"><b>{selected.source.key}</b> · {selected.title}</p>
					<p className="hx-muted">Score <span className="hx-num">{Math.round(selected.score)}</span>{selected.pinned ? ' · pinned first' : ''}{selected.priorityOverride ? ` · moved by hand (${selected.priorityOverride > 0 ? '+' : ''}${selected.priorityOverride})` : ''}</p>
					{selected.scoreReasons.length > 0
						? <ul className="queue-side-reasons">{selected.scoreReasons.map((reason) => <li key={reason}>{reason}</li>)}</ul>
						: <p className="hx-muted">The Sorter gave no reasons for this score.</p>}
					{waitsFor(selected, state.tasks).length > 0 && <p className="hx-muted">Waits for {waitsFor(selected, state.tasks).join(', ')}.</p>}
				</section>
			) : <p className="hx-muted">Choose a row to see why it sits where it does.</p>}

			<section className="queue-side-section" aria-labelledby="queue-next">
				<h3 id="queue-next">What starts next</h3>
				{next
					? <p><b>{next.source.key}</b> · {next.title}<br /><span className="hx-muted">{state.status?.autoStart ? 'Starts when a Planner is free.' : 'Starts when you press Start.'}</span></p>
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

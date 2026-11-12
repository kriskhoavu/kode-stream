import { useState } from 'react';
import { Square } from 'lucide-react';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import type { HarnessAgentRun, HarnessRun, HarnessTask } from '../../lib/types';
import { agentRoleLabels } from './vocabulary';

/** Whether a run can still be stopped: it is working, or waiting on a person. */
export function canStopRun(run: HarnessRun | undefined): run is HarnessRun {
	return run?.state === 'running' || run?.state === 'blocked';
}

/** What stopping does, said before a person confirms it. */
export function stopMessage(task: Pick<HarnessTask, 'source'>, working?: Pick<HarnessAgentRun, 'role'>): string {
	const key = task.source.key;
	const what = working ? `The ${agentRoleLabels[working.role]} is stopped mid-step and the run is cancelled.` : 'The run is cancelled.';
	return `${what} ${key} stays in the Task Queue as Stopped, and Start again begins a new run. Nothing is committed to the baseline.`;
}

/**
 * Stop, behind a confirmation: the same control on the run page, the task
 * panel and a Task Queue row, so a run can be stopped wherever it shows.
 */
export function StopRunButton({ task, run, working, onCancel, compact = false }: { task: Pick<HarnessTask, 'source'>; run: HarnessRun; working?: Pick<HarnessAgentRun, 'role'>; onCancel: (run: HarnessRun) => Promise<void>; compact?: boolean }) {
	const [confirm, setConfirm] = useState(false);
	const [stopping, setStopping] = useState(false);
	return (
		// A Task Queue row selects on click; the button and its dialog keep their clicks.
		<span className="hx-stop" onClick={(event) => event.stopPropagation()}>
			<button type="button" className="secondary hx-small-button hx-stop-button" disabled={stopping} aria-label={compact ? `Stop ${task.source.key}` : undefined} onClick={() => setConfirm(true)}><Square size={compact ? 13 : 14} aria-hidden="true" />{stopping ? 'Stopping…' : 'Stop'}</button>
			{confirm && (
				<ConfirmDialog
					title={`Stop ${task.source.key}?`}
					message={stopMessage(task, working ?? run.activeAgentRun)}
					confirmLabel="Stop this task"
					danger
					onCancel={() => setConfirm(false)}
					onConfirm={() => { setConfirm(false); setStopping(true); void onCancel(run).finally(() => setStopping(false)); }}
				/>
			)}
		</span>
	);
}

import { useState } from 'react';
import { RotateCcw } from 'lucide-react';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import type { HarnessTask } from '../../lib/types';
import { plainError } from './useHarness';

/** Whether a task has work to throw away: a run, now or before, and it is still in the queue. */
export function canResetTask(task: Pick<HarnessTask, 'state' | 'runId' | 'attempts'>): boolean {
	if (task.state === 'dismissed' || task.state === 'stale') return false;
	return Boolean(task.runId) || task.attempts > 0;
}

/** What resetting does, said before a person confirms it. */
export function resetMessage(task: Pick<HarnessTask, 'source'>): string {
	const key = task.source.key;
	return `A run in progress stops. Every run of ${key} is deleted with its plan, steps, conversations, working copy and local branch, so no code change is left on this machine. ${key} goes back to the Task Queue and waits for Start. A branch already pushed and a draft pull request stay at the remote.`;
}

/**
 * Reset, behind a confirmation: throws away everything the harness did for a
 * task so that Start begins again from nothing.
 */
export function ResetTaskButton({ task, onReset }: { task: HarnessTask; onReset: (task: HarnessTask) => Promise<void> }) {
	const [confirm, setConfirm] = useState(false);
	const [resetting, setResetting] = useState(false);
	const [error, setError] = useState('');
	return (
		<span className="hx-reset" onClick={(event) => event.stopPropagation()}>
			<button type="button" className="secondary hx-small-button" disabled={resetting} onClick={() => { setError(''); setConfirm(true); }}><RotateCcw size={14} aria-hidden="true" />{resetting ? 'Resetting…' : 'Reset'}</button>
			{error && <span className="hx-banner" role="alert">{error}</span>}
			{confirm && (
				<ConfirmDialog
					title={`Reset ${task.source.key}?`}
					message={resetMessage(task)}
					confirmLabel="Reset this task"
					danger
					onCancel={() => setConfirm(false)}
					onConfirm={() => {
						setConfirm(false);
						setResetting(true);
						void onReset(task)
							.catch((caught: unknown) => setError(plainError(caught, `${task.source.key} could not be reset.`)))
							.finally(() => setResetting(false));
					}}
				/>
			)}
		</span>
	);
}

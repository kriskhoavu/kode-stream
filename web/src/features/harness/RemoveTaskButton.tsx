import { useState } from 'react';
import { Trash2 } from 'lucide-react';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import type { HarnessTask } from '../../lib/types';
import { plainError } from './useHarness';

/** What removing does, said before a person confirms it. */
export function removeWarning(task: Pick<HarnessTask, 'source'>): string {
	const key = task.source.key;
	return `A run in progress stops. ${key} is deleted from Kode Stream with its runs, plan, steps, conversations, sessions, working copy and local branch. A branch already pushed and a draft pull request stay at the remote. If ${key} is still open at its source and the Finder's search still matches it, the Finder adds it back as Up next on its next look.`;
}

/**
 * Remove, behind a confirmation: deletes a task in any state with
 * everything the harness did for it.
 */
export function RemoveTaskButton({ task, onRemove, compact = false }: {
	task: HarnessTask;
	onRemove: (task: HarnessTask) => Promise<void>;
	/** A queue row shows the icon alone. */
	compact?: boolean;
}) {
	const [confirm, setConfirm] = useState(false);
	const [removing, setRemoving] = useState(false);
	const [error, setError] = useState('');
	const label = removing ? 'Removing…' : 'Remove';
	return (
		<span className="hx-reset" onClick={(event) => event.stopPropagation()}>
			<button type="button" className="secondary hx-small-button" disabled={removing} aria-label={compact ? `Remove ${task.source.key}` : undefined} title={compact ? `Remove ${task.source.key}` : undefined} onClick={() => { setError(''); setConfirm(true); }}>
				<Trash2 size={13} aria-hidden="true" />{compact ? null : label}
			</button>
			{error && <span className="hx-banner" role="alert">{error}</span>}
			{confirm && (
				<ConfirmDialog
					title={`Remove ${task.source.key}?`}
					message={removeWarning(task)}
					confirmLabel="Remove"
					danger
					onCancel={() => setConfirm(false)}
					onConfirm={() => {
						setConfirm(false);
						setRemoving(true);
						void onRemove(task)
							.catch((caught: unknown) => setError(plainError(caught, `${task.source.key} could not be removed.`)))
							.finally(() => setRemoving(false));
					}}
				/>
			)}
		</span>
	);
}

import { useEffect, useState } from 'react';
import type { HarnessAgentRole, HarnessSettings, HarnessTask, ItemSummary, WorkspaceConfig } from '../../lib/types';
import { useModalDialog } from '../../components/overlay';
import type { HarnessController } from '../harness/useHarness';
import { plainError } from '../harness/useHarness';
import { agentRoleLabels, workerLabel } from '../harness/vocabulary';

const roleRows: { role: HarnessAgentRole; does: string }[] = [
	{ role: 'planner', does: 'writes the plan' },
	{ role: 'plan_reviewer', does: 'checks the plan' },
	{ role: 'coder', does: 'writes the code' },
	{ role: 'code_reviewer', does: 'checks the code' }
];

/** The branch the harness works on, as the backend names it: `harness/<key>-<six of the run id>`. */
export function harnessBranchFor(key: string): string {
	let slug = key.replace(/[^A-Za-z0-9._-]+/g, '-').replace(/^[-.]+|[-.]+$/g, '');
	if (slug.length > 40) slug = slug.slice(0, 40);
	return `harness/${slug || 'task'}-…`;
}

/**
 * Hands a card to the harness. The branch and the models are what the harness
 * uses for every task, shown so nobody is surprised; the approvals are the
 * gates it always asks for, plus whether its agents may stop and ask; and the
 * card starts now or waits its turn in the Task Queue.
 */
export function RunWithHarnessDialog({ item, workspace, harness, onClose, onStarted }: {
	item: ItemSummary;
	workspace?: WorkspaceConfig;
	harness: HarnessController;
	onClose: () => void;
	onStarted: (task: HarnessTask) => void;
}) {
	const dialog = useModalDialog(onClose);
	const [settings, setSettings] = useState<HarnessSettings | undefined>();
	const [askQuestions, setAskQuestions] = useState<boolean | undefined>();
	const [startNow, setStartNow] = useState(true);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState('');
	useEffect(() => {
		const controller = new AbortController();
		harness.client.settings(controller.signal)
			.then((loaded) => { if (!controller.signal.aborted) setSettings(loaded); })
			.catch(() => undefined);
		return () => controller.abort();
	}, [harness.client]);
	const key = item.identifier.trim();
	const asks = askQuestions ?? settings?.askQuestions ?? true;
	const submit = async () => {
		setBusy(true);
		setError('');
		try {
			const task = await harness.createTask({ workspaceId: item.workspaceId, key, title: item.title, body: item.description ?? '', startNow });
			if (settings && asks !== settings.askQuestions) await harness.setQuestions(task, asks);
			onStarted(task);
		} catch (caught) {
			setError(plainError(caught, 'The harness could not take this card.'));
			setBusy(false);
		}
	};
	return (
		<div className="confirm-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
			<section ref={dialog.ref} className="confirm-dialog hx-dialog run-with-harness" role="dialog" aria-modal="true" aria-labelledby={dialog.titleId}>
				<header>
					<h2 id={dialog.titleId}>Run {key} with the harness</h2>
					<button className="icon-button" type="button" aria-label="Close dialog" onClick={onClose}>×</button>
				</header>
				<form onSubmit={(event) => { event.preventDefault(); void submit(); }}>
					<p className="run-with-harness-title">{item.title}</p>
					<dl className="run-with-harness-facts">
						<dt>Branch</dt>
						<dd><code>{harnessBranchFor(key)}</code> from <code>{workspace?.baselineBranch || 'the baseline'}</code>. The harness works on a branch of its own and never pushes to the baseline.</dd>
						<dt>Model per role</dt>
						<dd>
							{settings ? (
								<ul className="run-with-harness-roles">
									{roleRows.map((row) => {
										const policy = settings.modelPolicy[row.role];
										return <li key={row.role}><b>{agentRoleLabels[row.role]}</b> {row.does}: {workerLabel(policy?.engine, policy?.model)}{row.role === 'coder' && policy?.hardModel ? ` (${workerLabel(policy.engine, policy.hardModel)} when the plan is hard)` : ''}</li>;
									})}
								</ul>
							) : <span className="hx-muted">Loading…</span>}
							<span className="hx-muted">Set for every task under Settings.</span>
						</dd>
						<dt>Approvals wanted</dt>
						<dd>
							<ul className="run-with-harness-roles">
								<li>The plan: always yours to approve before any code is written.</li>
								<li>Before hand-off: {settings?.requireReview === false ? 'the draft pull request opens without asking.' : 'you try the change in a session of your own, then hand it off or send it back.'}</li>
							</ul>
							<label className="run-with-harness-check"><input type="checkbox" checked={asks} onChange={(event) => setAskQuestions(event.target.checked)} /> The agents may stop and ask me when they cannot decide alone</label>
						</dd>
						<dt>When</dt>
						<dd>
							<label className="run-with-harness-check"><input type="radio" name="run-when" checked={startNow} onChange={() => setStartNow(true)} /> Start now: the next free Planner takes it</label>
							<label className="run-with-harness-check"><input type="radio" name="run-when" checked={!startNow} onChange={() => setStartNow(false)} /> Queue it: the Sorter gives it a place and it waits its turn</label>
						</dd>
					</dl>
					{error && <p className="hx-field-error" role="alert">{error}</p>}
					<footer>
						<button className="ghost" type="button" onClick={onClose}>Back</button>
						<button className="primary" type="submit" disabled={busy || !key}>{busy ? 'Handing over…' : startNow ? 'Run with the harness' : 'Add to the Task Queue'}</button>
					</footer>
				</form>
			</section>
		</div>
	);
}

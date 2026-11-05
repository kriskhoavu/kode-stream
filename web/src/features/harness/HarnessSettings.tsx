import { useEffect, useId, useState } from 'react';
import { Workflow } from 'lucide-react';
import type { HarnessAgentRole, HarnessEngineId, HarnessRoleHealth, HarnessRoleId, HarnessRolePolicy, HarnessSettings as Settings, HarnessSourceKind, HarnessSourceSettings, HarnessStatus } from '../../lib/types';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import type { HarnessClient, HarnessWorkspaceOption } from './harnessClient';
import { plainError } from './useHarness';
import { useNow } from './useNow';
import { engineLabel, formatAgo, modelLabel, plural, roleLabels, roleOrder, sourceLabels } from './vocabulary';

const modelSuggestions = ['opus', 'sonnet', 'haiku'];

/** The kinds a person can add. Bitbucket Server has no issue tracker, so its work comes in through Jira. */
const addableKinds: HarnessSourceKind[] = ['jira', 'github'];

/** A sensible first query: the workspace's own Jira project, open issues, newest change first. */
export function defaultSourceQuery(kind: HarnessSourceKind, workspace?: HarnessWorkspaceOption): string {
	if (kind === 'jira' && workspace?.jiraProjectKey) {
		return `project = ${workspace.jiraProjectKey} AND sprint in openSprints() AND statusCategory != Done ORDER BY updated DESC`;
	}
	return '';
}

const policyRows: { role: HarnessAgentRole; name: string; does: string }[] = [
	{ role: 'planner', name: 'Planner', does: 'Writes the plan' },
	{ role: 'plan_reviewer', name: 'Checker', does: 'Checks the plan' },
	{ role: 'coder', name: 'Builder', does: 'Writes the code' },
	{ role: 'code_reviewer', name: 'Checker', does: 'Checks the code' },
	{ role: 'qa_fixer', name: 'Builder', does: 'Fixes a failed test' }
];

const pauseConsequence: Record<HarnessRoleId, string> = {
	scout: 'Paused: no new tasks are found. The queue keeps what it has.',
	sorter: 'Paused: the queue keeps its current order.',
	planner: 'Paused: no new plans start. A plan already being written finishes.',
	coder: 'Paused: no new code is written. Work already running finishes.',
	reviewer: 'Paused: plans and code wait for their check.',
	qa: 'Paused: finished work waits before its tests run.'
};

/** Who does what, where work comes from, and which roles are paused. Plain words; the advanced search query stays folded away. */
export function HarnessSettings({ client, status, onPause, onResume, onSaved }: {
	client: HarnessClient;
	status?: HarnessStatus;
	onPause: (role: HarnessRoleId) => Promise<void>;
	onResume: (role: HarnessRoleId) => Promise<void>;
	onSaved: () => void;
}) {
	const [saved, setSaved] = useState<Settings | undefined>();
	const [draft, setDraft] = useState<Settings | undefined>();
	const [error, setError] = useState('');
	const [notice, setNotice] = useState('');
	const [saving, setSaving] = useState(false);
	const [confirmPause, setConfirmPause] = useState<HarnessRoleHealth | undefined>();
	const [confirmClear, setConfirmClear] = useState<string | undefined>();
	const [clearing, setClearing] = useState(false);
	const [workspaces, setWorkspaces] = useState<HarnessWorkspaceOption[]>([]);
	const [addKind, setAddKind] = useState<HarnessSourceKind>('jira');
	const [addWorkspace, setAddWorkspace] = useState('');
	const now = useNow(60_000);

	const load = () => {
		setError('');
		client.settings().then((settings) => { setSaved(settings); setDraft(settings); }).catch((caught) => setError(plainError(caught, 'The harness settings could not be loaded.')));
	};
	useEffect(load, [client]);
	useEffect(() => {
		if (!client.workspaces) return;
		const controller = new AbortController();
		client.workspaces(controller.signal).then((list) => { setWorkspaces(list); setAddWorkspace((current) => current || list[0]?.id || ''); }).catch(() => setWorkspaces([]));
		return () => controller.abort();
	}, [client]);

	if (!draft || !saved) {
		return error
			? <section className="hx-card hx-state" role="alert"><p>{error}</p><button type="button" className="primary" onClick={load}>Try again</button></section>
			: <section className="hx-card hx-state" aria-busy="true">Loading the harness settings…</section>;
	}

	const dirty = JSON.stringify(draft) !== JSON.stringify(saved);
	const update = (patch: Partial<Settings>) => { setDraft({ ...draft, ...patch }); setNotice(''); };
	const updatePolicy = (role: HarnessAgentRole, patch: Partial<HarnessRolePolicy>) => update({ modelPolicy: { ...draft.modelPolicy, [role]: { ...draft.modelPolicy[role], ...patch } } });
	const updateSource = (index: number, patch: Partial<HarnessSourceSettings>) => update({ sources: draft.sources.map((source, position) => position === index ? { ...source, ...patch } : source) });
	const removeSource = (index: number) => update({ sources: draft.sources.filter((_, position) => position !== index) });
	const clearQueue = async (workspaceId: string) => {
		setConfirmClear(undefined);
		setClearing(true);
		setError('');
		try {
			const { removed } = await client.clearQueue(workspaceId);
			setNotice(removed === 0 ? 'The queue had nothing to clear.' : `Cleared ${plural(removed, 'task')} from the queue. The Finder adds back what it still finds.`);
			onSaved();
		} catch (caught) {
			setError(plainError(caught, 'The queue could not be cleared.'));
		} finally {
			setClearing(false);
		}
	};
	const engines = status?.engines ?? [];
	const roles = new Map((status?.roles ?? []).map((role) => [role.role, role]));
	const workspaceNames = new Map(workspaces.map((workspace) => [workspace.id, workspace.name]));
	const addTarget = workspaces.find((workspace) => workspace.id === addWorkspace);
	const addWorkspaceId = addTarget?.id ?? addWorkspace.trim();
	const duplicateSource = draft.sources.some((source) => source.workspaceId === addWorkspaceId && source.kind === addKind);
	const addSource = () => {
		if (!addWorkspaceId || duplicateSource) return;
		const source: HarnessSourceSettings = { workspaceId: addWorkspaceId, kind: addKind, enabled: true, query: defaultSourceQuery(addKind, addTarget) };
		if (addKind === 'github') { source.labels = []; source.tokenEnvVar = 'GITHUB_TOKEN'; }
		update({ sources: [...draft.sources, source] });
		setNotice(source.query ? 'Added. Save to let the Finder look there.' : 'Added. Fill in the repository under Advanced, then save.');
	};

	const save = async () => {
		setSaving(true);
		setError('');
		try {
			const next = await client.saveSettings(draft);
			setSaved(next);
			setDraft(next);
			setNotice('Saved.');
			onSaved();
		} catch (caught) {
			setError(plainError(caught, 'The settings were not saved.'));
		} finally {
			setSaving(false);
		}
	};

	const togglePause = (role: HarnessRoleId, paused: boolean) => {
		const health = roles.get(role);
		if (!paused && health && health.working > 0) {
			setConfirmPause(health);
			return;
		}
		void (paused ? onResume(role) : onPause(role));
	};

	return (
		<div className="settings-page hx-settings">
			<section className="settings-section">
				<header><div><span className="settings-group-label">Harness</span><h2>The harness</h2><p>What runs on its own, and what waits for a person.</p></div></header>
				<div className="settings-toggle-list">
					<Toggle label="Turn the harness on" hint="When off, nothing is found, planned or built." checked={draft.enabled} onChange={(enabled) => update({ enabled })} />
					<Toggle label="Start tasks on their own" hint="When off, a person approves each task before the Planner starts." checked={draft.autoStart} onChange={(autoStart) => update({ autoStart })} />
					<Toggle label="Ask before opening a draft pull request" hint="When on, a person approves before the branch leaves this machine." checked={draft.requirePrApproval} onChange={(requirePrApproval) => update({ requirePrApproval })} />
					<Toggle label="Let an agent stop and ask you" hint="When on, an agent that cannot decide alone stops its round and waits for your answer. When off, it goes on with its best guess and its questions show on the plan approval. A workspace or a task can say otherwise." checked={draft.askQuestions} onChange={(askQuestions) => update({ askQuestions })} />
				</div>
			</section>

			<section className="settings-section hx-settings-policy">
				<header><div><span className="settings-group-label">Models</span><h2>Who does what</h2><p>Each role runs a coding tool already on this machine. The Tester runs the project's own tests and uses no model.</p></div></header>
				<div className="hx-table-card">
					<table className="hx-table">
						<thead><tr><th scope="col">Role</th><th scope="col">Tool</th><th scope="col">Model</th><th scope="col">Most turns</th></tr></thead>
						<tbody>
							{policyRows.map((row) => {
								const policy = draft.modelPolicy[row.role];
								return (
									<tr key={row.role}>
										<th scope="row"><b>{row.name}</b><br /><small className="hx-muted">{row.does}</small></th>
										<td><EngineSelect value={policy.engine} engines={engines} label={`${row.name} tool for “${row.does}”`} onChange={(engine) => updatePolicy(row.role, { engine })} /></td>
										<td>
											{row.role === 'coder' ? (
												<div className="hx-model-pair">
													<ModelInput label="Easy" value={policy.model} onChange={(model) => updatePolicy(row.role, { model })} />
													<ModelInput label="Hard" value={policy.hardModel ?? policy.model} onChange={(hardModel) => updatePolicy(row.role, { hardModel })} />
													<details className="hx-explainer">
														<summary>What counts as hard?</summary>
														<p>The Planner rates each plan easy or hard. Hard means the plan touches many files, changes how two parts talk to each other, or its check found must-fix items twice. Hard work goes to the stronger model.</p>
													</details>
												</div>
											) : <ModelInput label={`${row.name} model for “${row.does}”`} hideLabel value={policy.model} onChange={(model) => updatePolicy(row.role, { model })} />}
										</td>
										<td><input className="hx-number" type="number" min={1} max={500} aria-label={`Most turns for “${row.does}”`} value={policy.maxTurns} onChange={(event) => updatePolicy(row.role, { maxTurns: Math.max(1, Number(event.target.value) || 1) })} /></td>
									</tr>
								);
							})}
							<tr><th scope="row"><b>Tester</b><br /><small className="hx-muted">Runs the tests twice</small></th><td colSpan={3} className="hx-muted">No model.</td></tr>
						</tbody>
					</table>
				</div>
				{engines.some((engine) => !engine.available || engine.limits?.length) && (
					<ul className="hx-engine-notes">
						{engines.filter((engine) => !engine.available || engine.limits?.length).map((engine) => (
							<li key={engine.id}><b>{engineLabel(engine.id)}</b>{engine.available ? '' : ' is not installed on this machine.'} {engine.limits?.join(' ')}</li>
						))}
					</ul>
				)}
			</section>

			<section className="settings-section">
				<header><div><span className="settings-group-label">Sources</span><h2>Where work comes from</h2><p>The Finder looks at these and adds new tasks to the queue.</p></div></header>
				<div className="hx-sources">
					{draft.sources.length === 0 && <p className="hx-muted">No sources yet. Add one below.</p>}
					{draft.sources.map((source, index) => (
						<SourceRow key={`${source.workspaceId}-${source.kind}-${index}`} source={source} workspaceName={workspaceNames.get(source.workspaceId)} now={now} askDefault={draft.askQuestions} firstOfWorkspace={draft.sources.findIndex((other) => other.workspaceId === source.workspaceId) === index} onChange={(patch) => updateSource(index, patch)} onWorkspaceQuestions={(askQuestions) => update({ sources: draft.sources.map((other) => other.workspaceId === source.workspaceId ? { ...other, askQuestions } : other) })} onRemove={() => removeSource(index)} onClear={() => setConfirmClear(source.workspaceId)} clearing={clearing} />
					))}
				</div>
				<form className="hx-add-source" aria-label="Add a source" onSubmit={(event) => { event.preventDefault(); addSource(); }}>
					<label className="hx-field"><span>Workspace</span>
						{workspaces.length > 0
							? <select value={addWorkspace} onChange={(event) => setAddWorkspace(event.target.value)}>{workspaces.map((workspace) => <option key={workspace.id} value={workspace.id}>{workspace.name}</option>)}</select>
							: <input value={addWorkspace} placeholder="Workspace id" onChange={(event) => setAddWorkspace(event.target.value)} />}
					</label>
					<label className="hx-field"><span>Look in</span>
						<select value={addKind} onChange={(event) => setAddKind(event.target.value as HarnessSourceKind)}>{addableKinds.map((kind) => <option key={kind} value={kind}>{sourceLabels[kind]}</option>)}</select>
					</label>
					<button type="submit" className="secondary" disabled={!addWorkspaceId || duplicateSource}>Add source</button>
					{duplicateSource && <span className="hx-muted">{sourceLabels[addKind]} is already set for this workspace.</span>}
					{!duplicateSource && addKind === 'jira' && addTarget && !addTarget.jiraProjectKey && <span className="hx-muted">This workspace has no Jira connection yet. Set one in the workspace settings, or type a search under Advanced after adding.</span>}
				</form>
			</section>

			<section className="settings-section">
				<header><div><span className="settings-group-label">Roles</span><h2>Pause a role</h2><p>A paused role starts nothing new. Pausing takes effect at once.</p></div></header>
				<div className="settings-toggle-list">
					{roleOrder.map((role) => {
						const health = roles.get(role);
						const paused = Boolean(health?.paused ?? draft.paused[role]);
						return (
							<div className="settings-toggle-row hx-role-row" key={role}>
								<label className="hx-role-switch">
									<input type="checkbox" role="switch" checked={!paused} onChange={() => togglePause(role, paused)} />
									<span><strong>{roleLabels[role]}</strong><small>{paused ? `${pauseConsequence[role]}${health?.pausedBy ? ` Paused by ${health.pausedBy}.` : ''}` : health && health.working > 0 ? `${health.working} working now.` : 'Running.'}</small></span>
								</label>
								<label className="hx-slots">
									<span>At once</span>
									<input className="hx-number" type="number" min={0} max={8} value={draft.slots[role]} onChange={(event) => update({ slots: { ...draft.slots, [role]: Math.max(0, Number(event.target.value) || 0) } })} />
								</label>
							</div>
						);
					})}
				</div>
			</section>

			<div className="settings-actions hx-settings-actions">
				<button type="button" className="primary" disabled={!dirty || saving} onClick={() => void save()}>{saving ? 'Saving…' : 'Save'}</button>
				<button type="button" className="secondary" disabled={!dirty || saving} onClick={() => { setDraft(saved); setError(''); }}>Undo changes</button>
				<span role="status" className="hx-muted">{notice}</span>
				{error && <p className="hx-banner" role="alert">{error} <button type="button" className="hx-link" onClick={load}>Reload</button></p>}
			</div>

			{confirmClear !== undefined && (
				<ConfirmDialog
					title={`Clear the queue for ${workspaceNames.get(confirmClear) ?? confirmClear}?`}
					message="Every task that has not started yet is deleted from the queue, including stale and dismissed ones. Work that is running, waiting on you or done stays. The Finder adds back whatever its search still returns on its next look."
					confirmLabel="Clear the queue"
					onCancel={() => setConfirmClear(undefined)}
					onConfirm={() => void clearQueue(confirmClear)}
				/>
			)}

			{confirmPause && (
				<ConfirmDialog
					title={`Pause the ${roleLabels[confirmPause.role]}?`}
					message={`${plural(confirmPause.working, 'task')} ${confirmPause.working === 1 ? 'is' : 'are'} with the ${roleLabels[confirmPause.role]} now. ${confirmPause.working === 1 ? 'It finishes' : 'They finish'}; nothing new starts until you turn it back on.`}
					confirmLabel="Pause"
					onCancel={() => setConfirmPause(undefined)}
					onConfirm={() => { const role = confirmPause.role; setConfirmPause(undefined); void onPause(role); }}
				/>
			)}
		</div>
	);
}

function Toggle({ label, hint, checked, onChange }: { label: string; hint: string; checked: boolean; onChange: (checked: boolean) => void }) {
	return (
		<label className="settings-toggle-row">
			<span><strong>{label}</strong><small>{hint}</small></span>
			<input type="checkbox" role="switch" checked={checked} onChange={(event) => onChange(event.target.checked)} />
		</label>
	);
}

function EngineSelect({ value, engines, label, onChange }: { value: HarnessEngineId; engines: HarnessStatus['engines']; label: string; onChange: (engine: HarnessEngineId) => void }) {
	const ids: HarnessEngineId[] = engines.length ? engines.map((engine) => engine.id) : [value];
	return (
		<select className="hx-select" aria-label={label} value={value} onChange={(event) => onChange(event.target.value as HarnessEngineId)}>
			{ids.map((id) => {
				const engine = engines.find((candidate) => candidate.id === id);
				const unavailable = engine && !engine.available;
				return <option key={id} value={id} disabled={unavailable && id !== value}>{engineLabel(id)}{unavailable ? ' (not installed)' : ''}</option>;
			})}
		</select>
	);
}

function ModelInput({ label, value, hideLabel = false, onChange }: { label: string; value: string; hideLabel?: boolean; onChange: (model: string) => void }) {
	const listId = useId();
	return (
		<label className="hx-model-input">
			<span className={hideLabel ? 'sr-only' : undefined}>{label}</span>
			<input value={value} list={listId} title={value} onChange={(event) => onChange(event.target.value.trim())} />
			<datalist id={listId}>{modelSuggestions.map((model) => <option key={model} value={model}>{modelLabel(model)}</option>)}</datalist>
			{modelLabel(value) !== value && <small className="hx-muted">{modelLabel(value)}</small>}
		</label>
	);
}

function SourceRow({ source, workspaceName, now, askDefault, firstOfWorkspace, onChange, onWorkspaceQuestions, onRemove, onClear, clearing }: { source: HarnessSourceSettings; workspaceName?: string; now: number; askDefault: boolean; firstOfWorkspace: boolean; onChange: (patch: Partial<HarnessSourceSettings>) => void; onWorkspaceQuestions: (askQuestions: boolean | null) => void; onRemove: () => void; onClear: () => void; clearing: boolean }) {
	const checked = source.lastCheckedAt ? `Looked ${formatAgo(source.lastCheckedAt, now)}${source.lastFound !== undefined ? ` · ${source.lastFound} new` : ''}` : 'Not looked yet';
	const queryLabel = source.kind === 'jira' ? 'Jira search (JQL)' : source.kind === 'github' ? 'Repository (owner/name)' : 'Repository (workspace/name)';
	return (
		<div className="hx-source-row">
			<div className="hx-source-head">
				<span><Workflow size={14} aria-hidden="true" /> <b>{sourceLabels[source.kind]}</b> <span className="hx-muted">· {workspaceName ?? source.workspaceId}</span></span>
				<span className="hx-muted">{checked}</span>
				<label className="hx-inline-switch"><input type="checkbox" role="switch" checked={source.enabled} onChange={(event) => onChange({ enabled: event.target.checked })} /> Look here</label>
				<button type="button" className="ghost" disabled={clearing} onClick={onClear} aria-label={`Clear the queue for ${workspaceName ?? source.workspaceId}`}>{clearing ? 'Clearing…' : 'Clear the queue'}</button>
				<button type="button" className="ghost" onClick={onRemove} aria-label={`Remove ${sourceLabels[source.kind]} for ${workspaceName ?? source.workspaceId}`}>Remove</button>
			</div>
			{source.lastError && <p className="hx-banner" role="alert">{source.lastError}</p>}
			{firstOfWorkspace && (
				<label className="hx-field hx-field-inline">
					<span>May an agent stop and ask you here?</span>
					<select aria-label={`May an agent stop and ask in ${workspaceName ?? source.workspaceId}`} value={source.askQuestions === undefined || source.askQuestions === null ? 'default' : source.askQuestions ? 'yes' : 'no'} onChange={(event) => onWorkspaceQuestions(event.target.value === 'default' ? null : event.target.value === 'yes')}>
						<option value="default">Same as everywhere ({askDefault ? 'yes' : 'no'})</option>
						<option value="yes">Yes, stop and ask</option>
						<option value="no">No, go on with a best guess</option>
					</select>
				</label>
			)}
			<details className="hx-advanced" open={source.query === '' ? true : undefined}>
				<summary>Advanced</summary>
				<div className="hx-advanced-fields">
					<label className="hx-field"><span>{queryLabel}</span><input value={source.query} onChange={(event) => onChange({ query: event.target.value })} /></label>
					{source.kind !== 'jira' && <label className="hx-field"><span>Labels to pick up (comma separated)</span><input value={(source.labels ?? []).join(', ')} onChange={(event) => onChange({ labels: event.target.value.split(',').map((label) => label.trim()).filter(Boolean) })} /></label>}
					{source.kind !== 'jira' && <label className="hx-field"><span>Name of the environment setting that holds the token</span><input value={source.tokenEnvVar ?? ''} onChange={(event) => onChange({ tokenEnvVar: event.target.value.trim() || undefined })} /></label>}
				</div>
			</details>
		</div>
	);
}

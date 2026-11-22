import { useEffect, useId, useState, type ReactNode } from 'react';
import { Workflow } from 'lucide-react';
import type { HarnessAgentRole, HarnessEngineId, HarnessRoleHealth, HarnessRoleId, HarnessRoleModel, HarnessSettings as Settings, HarnessSourceKind, HarnessSourceSettings, HarnessStatus, HarnessWorkspaceSettings } from '../../lib/types';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import type { HarnessClient, HarnessWorkspaceOption } from './harnessClient';
import { plainError } from './useHarness';
import { useNow } from './useNow';
import { engineLabel, formatAgo, modelLabel, plural, roleLabels, roleOrder, sourceLabels } from './vocabulary';

const modelSuggestions = ['opus', 'sonnet', 'haiku'];

/** The kinds a person can add. Bitbucket Server has no issue tracker, so its work comes in through Jira. A local folder is for trying the pipeline. */
const addableKinds: HarnessSourceKind[] = ['jira', 'github', 'local'];

/** A sensible first query: the workspace's own Jira project, open issues, newest change first. */
export function defaultSourceQuery(kind: HarnessSourceKind, workspace?: HarnessWorkspaceOption): string {
	if (kind === 'jira' && workspace?.jiraProjectKey) {
		return `project = ${workspace.jiraProjectKey} AND sprint in openSprints() AND statusCategory != Done ORDER BY updated DESC`;
	}
	if (kind === 'local') return '.harness/tasks';
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
	scout: 'Paused: no new tasks are found. The Task Queue keeps what it has.',
	sorter: 'Paused: the Task Queue keeps its current order.',
	planner: 'Paused: no new plans start. A plan already being written finishes.',
	coder: 'Paused: no new code is written. Work already running finishes.',
	reviewer: 'Paused: plans and code wait for their check.',
	qa: 'Paused: finished work waits before its tests run.'
};

/** A workspace with no choices of its own yet: it follows the defaults everywhere. */
export function emptyWorkspaceSettings(workspaceId: string): HarnessWorkspaceSettings {
	return { workspaceId, verificationCommands: [], scope: [], models: {}, procedure: {} };
}

/**
 * The harness's settings. Without a workspace they are the defaults every
 * workspace starts from: what runs on its own and which roles are paused.
 * With one, they are that workspace's own: its choices over those defaults,
 * its models and role skills, or, as part "sources", where its Finder looks,
 * which sits beside the workspace's Jira connection.
 */
/** One half of a workspace's own harness settings. */
export type HarnessSettingsView = 'choices' | 'models';

export function HarnessSettings({ client, status, workspaceId, part = 'workspace', view, onPause, onResume, onSaved }: {
	client: HarnessClient;
	status?: HarnessStatus;
	/** Shows this workspace's own choices; absent shows the defaults. */
	workspaceId?: string;
	/** With a workspace: its own choices, or only its sources. */
	part?: 'workspace' | 'sources';
	/** Of part "workspace": its choices alone, or its models and role skills alone; absent shows both. The draft and Save cover all of it. */
	view?: HarnessSettingsView;
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
		const updateSource = (index: number, patch: Partial<HarnessSourceSettings>) => update({ sources: draft.sources.map((source, position) => position === index ? { ...source, ...patch } : source) });
	const removeSource = (index: number) => update({ sources: draft.sources.filter((_, position) => position !== index) });
	const clearQueue = async (workspaceId: string) => {
		setConfirmClear(undefined);
		setClearing(true);
		setError('');
		try {
			const { removed } = await client.clearQueue(workspaceId);
			setNotice(removed === 0 ? 'The Task Queue had nothing to clear.' : `Cleared ${plural(removed, 'task')} from the Task Queue. The Finder adds back what it still finds.`);
			onSaved();
		} catch (caught) {
			setError(plainError(caught, 'The Task Queue could not be cleared.'));
		} finally {
			setClearing(false);
		}
	};
	const engines = status?.engines ?? [];
	const roles = new Map((status?.roles ?? []).map((role) => [role.role, role]));
	const workspaceNames = new Map(workspaces.map((workspace) => [workspace.id, workspace.name]));
	const addTarget = workspaces.find((workspace) => workspace.id === (workspaceId ?? addWorkspace));
	const addWorkspaceId = workspaceId ?? addTarget?.id ?? addWorkspace.trim();
	const own = draft.workspaces.find((workspace) => workspace.workspaceId === workspaceId) ?? emptyWorkspaceSettings(workspaceId ?? '');
	const updateOwn = (patch: Partial<HarnessWorkspaceSettings>) => {
		const next = { ...own, ...patch };
		update({ workspaces: draft.workspaces.some((workspace) => workspace.workspaceId === own.workspaceId) ? draft.workspaces.map((workspace) => workspace.workspaceId === own.workspaceId ? next : workspace) : [...draft.workspaces, next] });
	};
	/** A role's model in this workspace: its own, else the defaults it starts from. */
	const roleModel = (role: HarnessAgentRole): Required<HarnessRoleModel> => {
		const fallback = draft.modelPolicy[role];
		const mine = own.models[role];
		return {
			engine: mine?.engine || fallback.engine,
			model: mine?.model || fallback.model,
			highDifficultyModel: mine ? mine.highDifficultyModel ?? '' : fallback.hardModel ?? '',
			maxTurns: mine?.maxTurns || fallback.maxTurns
		};
	};
	const updateModel = (role: HarnessAgentRole, patch: Partial<HarnessRoleModel>) => {
		updateOwn({ models: { ...own.models, [role]: { ...roleModel(role), ...patch } } });
	};
	const updateSkill = (role: HarnessAgentRole, patch: Partial<{ skill: string; track: string }>) => {
		const current = own.procedure[role] ?? { skill: '' };
		const next = { ...current, ...patch };
		const procedure = { ...own.procedure };
		if (next.skill || next.track) procedure[role] = { skill: next.skill, ...(next.track ? { track: next.track } : {}) };
		else delete procedure[role];
		updateOwn({ procedure });
	};
	const shownSources = draft.sources.map((source, index) => ({ source, index })).filter(({ source }) => !workspaceId || source.workspaceId === workspaceId);
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
			{!workspaceId && <>
			<section className="settings-section">
				<header><div><span className="settings-group-label">Harness</span><h2>The harness</h2><p>What runs on its own, and what waits for a person.</p></div></header>
				<div className="settings-toggle-list">
					<Toggle label="Turn the harness on" hint="When off, nothing is found, planned or built." checked={draft.enabled} onChange={(enabled) => update({ enabled })} />
					<Toggle label="Start tasks on their own" hint="When off, a person approves each task before the Planner starts." checked={draft.autoStart} onChange={(autoStart) => update({ autoStart })} />
					<Toggle label="Review before hand-off" hint="When on, a person tries each change in a session of their own once its tests pass, then hands it off or sends it back to Build or Plan. When off, the draft pull request opens at once." checked={draft.requireReview} onChange={(requireReview) => update({ requireReview })} />
					<Toggle label="Let an agent stop and ask you" hint="When on, an agent that cannot decide alone stops its round and waits for your answer. When off, it goes on with its best guess and its questions show on the plan approval. A workspace or a task can say otherwise." checked={draft.askQuestions} onChange={(askQuestions) => update({ askQuestions })} />
				</div>
				<p className="hx-muted">Every workspace starts from these. A workspace's own choices, models, tests and role skills are on its page under Workspaces, in AI and harness. Its sources are in Integrations.</p>
			</section>

			</>}

			{workspaceId && part === 'workspace' && view !== 'models' && (
			<section className="settings-section">
				<header><div><span className="settings-group-label">Harness</span><h2>The harness in this workspace</h2><p>Each choice follows the defaults until you set it here.</p></div></header>
				<div className="settings-toggle-list">
					<Override label="Run the harness here" hint="Off: the Finder skips this workspace and none of its tasks start." value={own.enabled} fallback={draft.enabled} onChange={(enabled) => updateOwn({ enabled })} />
					<Override label="Start tasks on their own" hint="Off: a person approves each task before the Planner starts." value={own.autoStart} fallback={draft.autoStart} onChange={(autoStart) => updateOwn({ autoStart })} />
					<Override label="Review before hand-off" hint="On: a person tries each change once its tests pass, then hands it off or sends it back." value={own.requireReview} fallback={draft.requireReview} onChange={(requireReview) => updateOwn({ requireReview })} />
					<Override label="Let an agent stop and ask you" hint="Off: an agent goes on with its best guess and its questions show on the plan approval." value={own.askQuestions} fallback={draft.askQuestions} onChange={(askQuestions) => updateOwn({ askQuestions })} />
					<label className="settings-toggle-row">
						<span><strong>Test command</strong><small>Runs in the task's working copy after each build. Empty: the Test step runs nothing and succeeds.</small></span>
						<input className="hx-select" aria-label="Test command" placeholder="None" value={own.verificationCommands.join(' && ')} onChange={(event) => updateOwn({ verificationCommands: event.target.value.trim() ? [event.target.value] : [] })} />
					</label>
					<label className="settings-toggle-row">
						<span><strong>Paths the Builder may change</strong><small>Comma separated. Empty: any path that is not protected.</small></span>
						<input className="hx-select" aria-label="Scope" placeholder="Any path" value={own.scope.join(', ')} onChange={(event) => updateOwn({ scope: event.target.value.split(',').map((path) => path.trim()).filter(Boolean) })} />
					</label>
				</div>
			</section>
			)}

			{workspaceId && part === 'workspace' && view !== 'choices' && <>
			<section className="settings-section hx-settings-policy">
				<header><div><span className="settings-group-label">Models</span><h2>Who does what here</h2><p>Each role runs a coding tool already on this machine. These are this workspace's own: a new workspace starts from Kode Stream's defaults, and changing them here changes no other workspace. The Tester runs the project's own tests and uses no model.</p></div></header>
				<div className="hx-table-card">
					<table className="hx-table">
						<thead><tr><th scope="col">Role</th><th scope="col">Tool</th><th scope="col">Model</th><th scope="col">Most turns</th></tr></thead>
						<tbody>
							{policyRows.map((row) => {
								const model = roleModel(row.role);
								return (
									<tr key={row.role}>
										<th scope="row"><b>{row.name}</b><br /><small className="hx-muted">{row.does}</small></th>
										<td><EngineSelect value={model.engine} engines={engines} label={`${row.name} tool for “${row.does}”`} onChange={(engine) => updateModel(row.role, { engine })} /></td>
										<td>
											{row.role === 'coder' ? (
												<div className="hx-model-pair">
													<ModelInput label="Easy" value={model.model} onChange={(value) => updateModel(row.role, { model: value })} />
													<ModelInput label="Hard" value={model.highDifficultyModel} placeholder="Same as Easy" onChange={(value) => updateModel(row.role, { highDifficultyModel: value })} aside={(
														<details className="hx-explainer">
															<summary>What counts as hard?</summary>
															<p>The Planner rates each plan easy or hard. Hard means the plan touches many files, changes how two parts talk to each other, or its check found must-fix items twice. Hard work goes to the stronger model; blank keeps it on the easy one.</p>
														</details>
													)} />
												</div>
											) : <ModelInput label={`${row.name} model for “${row.does}”`} hideLabel value={model.model} onChange={(value) => updateModel(row.role, { model: value })} />}
										</td>
										<td><input className="hx-number" type="number" min={1} max={200} aria-label={`Most turns for “${row.does}”`} value={model.maxTurns} onChange={(event) => updateModel(row.role, { maxTurns: Math.max(1, Number(event.target.value) || 1) })} /></td>
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
				<header><div><span className="settings-group-label">Role skills</span><h2>The repository's own procedure</h2><p>A role given a skill follows that skill of the repository, and the track of it when one is named. A role without one keeps the harness's own instructions.</p></div></header>
				<div className="hx-table-card">
					<table className="hx-table">
						<thead><tr><th scope="col">Role</th><th scope="col">Skill</th><th scope="col">Track</th></tr></thead>
						<tbody>
							{policyRows.map((row) => (
								<tr key={row.role}>
									<th scope="row"><b>{row.name}</b><br /><small className="hx-muted">{row.does}</small></th>
									<td><input className="hx-select hx-skill-input" aria-label={`Skill for “${row.does}”`} placeholder="Built-in instructions" value={own.procedure[row.role]?.skill ?? ''} onChange={(event) => updateSkill(row.role, { skill: event.target.value.trim() })} /></td>
									<td><input className="hx-select hx-skill-input" aria-label={`Track for “${row.does}”`} placeholder="Whole skill" value={own.procedure[row.role]?.track ?? ''} onChange={(event) => updateSkill(row.role, { track: event.target.value })} /></td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			</section>
			</>}

			{workspaceId && part === 'sources' && <>
			<section className="settings-section">
				<header><div><span className="settings-group-label">Sources</span><h2>Where work comes from</h2><p>The Finder looks at these and adds new tasks to this workspace's Task Queue.</p></div></header>
				<div className="hx-sources">
					{shownSources.length === 0 && <p className="hx-muted">No sources yet. Add one below.</p>}
					{shownSources.map(({ source, index }) => (
						<SourceRow key={`${source.workspaceId}-${source.kind}-${index}`} source={source} workspaceName={workspaceNames.get(source.workspaceId)} now={now} onChange={(patch) => updateSource(index, patch)} onRemove={() => removeSource(index)} onClear={() => setConfirmClear(source.workspaceId)} clearing={clearing} />
					))}
				</div>
				<form className="hx-add-source" aria-label="Add a source" onSubmit={(event) => { event.preventDefault(); addSource(); }}>
					<label className="hx-field"><span>Look in</span>
						<select value={addKind} onChange={(event) => setAddKind(event.target.value as HarnessSourceKind)}>{addableKinds.map((kind) => <option key={kind} value={kind}>{sourceLabels[kind]}</option>)}</select>
					</label>
					<button type="submit" className="secondary" disabled={!addWorkspaceId || duplicateSource}>Add source</button>
					{duplicateSource && <span className="hx-muted">{sourceLabels[addKind]} is already set for this workspace.</span>}
					{!duplicateSource && addKind === 'jira' && addTarget && !addTarget.jiraProjectKey && <span className="hx-muted">This workspace has no Jira connection yet. Connect Jira above, or type a search under Advanced after adding.</span>}
				</form>
			</section>
			</>}

			{!workspaceId && <section className="settings-section">
				<header><div><span className="settings-group-label">Roles</span><h2>Pause a role</h2><p>A paused role starts nothing new. Pausing takes effect at once.</p></div></header>
				<div className="settings-toggle-list hx-role-list">
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
			</section>}

			<div className="settings-actions hx-settings-actions">
				<button type="button" className="primary" disabled={!dirty || saving} onClick={() => void save()}>{saving ? 'Saving…' : 'Save'}</button>
				<button type="button" className="secondary" disabled={!dirty || saving} onClick={() => { setDraft(saved); setError(''); }}>Undo changes</button>
				<span role="status" className="hx-muted">{notice}</span>
				{error && <p className="hx-banner" role="alert">{error} <button type="button" className="hx-link" onClick={load}>Reload</button></p>}
			</div>

			{confirmClear !== undefined && (
				<ConfirmDialog
					title={`Clear the Task Queue for ${workspaceNames.get(confirmClear) ?? confirmClear}?`}
					message="Every task that has not started yet is deleted from the Task Queue, including stale and dismissed ones. Work that is running, waiting on you or done stays. The Finder adds back whatever its search still returns on its next look."
					confirmLabel="Clear the Task Queue"
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

/** A workspace's on or off as two small cards. Until a person picks one, the default's value is chosen. */
function Override({ label, hint, value, fallback, onChange }: { label: string; hint: string; value?: boolean | null; fallback: boolean; onChange: (value: boolean) => void }) {
	const name = useId();
	const on = value === undefined || value === null ? fallback : value;
	return (
		<div className="settings-toggle-row">
			<span><strong>{label}</strong><small>{hint}</small></span>
			<div className="hx-choice-cards" role="radiogroup" aria-label={label}>
				{[true, false].map((choice) => (
					<label key={String(choice)} className={`hx-choice-card${on === choice ? ' on' : ''}`}>
						<input type="radio" name={name} value={choice ? 'on' : 'off'} checked={on === choice} onChange={() => onChange(choice)} />
						<strong>{choice ? 'On' : 'Off'}</strong>
					</label>
				))}
			</div>
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

function ModelInput({ label, value, placeholder, hideLabel = false, aside, onChange }: { label: string; value: string; placeholder?: string; hideLabel?: boolean; aside?: ReactNode; onChange: (model: string) => void }) {
	const listId = useId();
	const inputId = useId();
	return (
		<div className="hx-model-input">
			<span className={hideLabel ? 'sr-only' : 'hx-model-label'}><label htmlFor={inputId}>{label}</label>{aside}</span>
			<input id={inputId} value={value} list={listId} title={value} placeholder={placeholder} onChange={(event) => onChange(event.target.value.trim())} />
			<datalist id={listId}>{modelSuggestions.map((model) => <option key={model} value={model}>{modelLabel(model)}</option>)}</datalist>
		</div>
	);
}

function SourceRow({ source, workspaceName, now, onChange, onRemove, onClear, clearing }: { source: HarnessSourceSettings; workspaceName?: string; now: number; onChange: (patch: Partial<HarnessSourceSettings>) => void; onRemove: () => void; onClear: () => void; clearing: boolean }) {
	const checked = source.lastCheckedAt ? `Looked ${formatAgo(source.lastCheckedAt, now)}${source.lastFound !== undefined ? ` · ${source.lastFound} new` : ''}` : 'Not looked yet';
	const queryLabel = source.kind === 'jira' ? 'Jira search (JQL)' : source.kind === 'github' ? 'Repository (owner/name)' : source.kind === 'local' ? 'Task folder (inside the workspace, or a full path)' : 'Repository (workspace/name)';
	const workspaceLabel = workspaceName ?? source.workspaceId;
	return (
		<div className="hx-source-row">
			<div className="hx-source-head">
				<span><Workflow size={14} aria-hidden="true" /> <b>{sourceLabels[source.kind]}</b> <span className="hx-muted">· {workspaceName ?? source.workspaceId}</span></span>
				<span className="hx-muted">{checked}</span>
				<label className="hx-inline-switch"><input type="checkbox" role="switch" checked={source.enabled} onChange={(event) => onChange({ enabled: event.target.checked })} /> Look here</label>
				<button type="button" className="ghost" disabled={clearing} onClick={onClear} aria-label={`Clear the Task Queue for ${workspaceName ?? source.workspaceId}`}>{clearing ? 'Clearing…' : 'Clear the Task Queue'}</button>
				<button type="button" className="ghost" onClick={onRemove} aria-label={`Remove ${sourceLabels[source.kind]} for ${workspaceName ?? source.workspaceId}`}>Remove</button>
			</div>
			{source.lastError && <p className="hx-banner" role="alert">{source.lastError}</p>}
			{source.kind === 'jira' && <ReadyCheckFields source={source} onChange={onChange} />}
			{source.kind === 'local' && <p className="hx-muted">One Markdown file per task: its name is the key, its first heading the title. Hand-off pushes the branch and opens no pull request.</p>}
			<details className="hx-advanced" open={source.query === '' ? true : undefined}>
				<summary>Advanced</summary>
				<div className="hx-advanced-fields">
					<label className="hx-field"><span>{queryLabel}</span><input value={source.query} onChange={(event) => onChange({ query: event.target.value })} /></label>
					{source.kind !== 'jira' && source.kind !== 'local' && <label className="hx-field"><span>Labels to pick up (comma separated)</span><input value={(source.labels ?? []).join(', ')} onChange={(event) => onChange({ labels: event.target.value.split(',').map((label) => label.trim()).filter(Boolean) })} /></label>}
					{source.kind !== 'jira' && source.kind !== 'local' && <label className="hx-field"><span>Name of the environment setting that holds the token</span><input value={source.tokenEnvVar ?? ''} onChange={(event) => onChange({ tokenEnvVar: event.target.value.trim() || undefined })} /></label>}
				</div>
			</details>
		</div>
	);
}

/**
 * A Jira source's readiness check: a ticket in a ready status starts on its
 * own, and its Planner first judges whether the ticket says enough.
 */
function ReadyCheckFields({ source, onChange }: { source: HarnessSourceSettings; onChange: (patch: Partial<HarnessSourceSettings>) => void }) {
	const on = source.checkReady === true;
	const [text, setText] = useState((source.readyStatuses ?? []).join(', '));
	return (
		<div className="hx-ready-check">
			<label className="hx-inline-switch"><input type="checkbox" role="switch" checked={on} onChange={(event) => onChange({ checkReady: event.target.checked ? true : undefined })} /> Start ready tickets on their own</label>
			<label className="hx-field">
				<span>Ready statuses (comma separated)</span>
				<input value={text} placeholder="READY" disabled={!on} onChange={(event) => {
					setText(event.target.value);
					const statuses = event.target.value.split(',').map((status) => status.trim()).filter(Boolean);
					onChange({ readyStatuses: statuses.length ? statuses : undefined });
				}} />
			</label>
			<p className="hx-muted">A ticket in one of these statuses starts without Start. The Planner first checks that the ticket says enough to plan from. If it does not, the task waits for the ticket's reporter, with a comment you can post to ask them, and a reply on the ticket sends the Planner on. Nothing is built before you approve the plan.</p>
		</div>
	);
}

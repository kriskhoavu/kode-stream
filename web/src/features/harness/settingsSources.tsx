import { useState } from 'react';
import { Workflow } from 'lucide-react';
import type { HarnessSourceKind, HarnessSourceSettings } from '../../lib/types';
import type { HarnessWorkspaceOption } from './harnessClient';
import { formatAgo, sourceLabels } from './vocabulary';

/** A sensible first query: the workspace's own Jira project, open issues, newest change first. */
export function defaultSourceQuery(kind: HarnessSourceKind, workspace?: HarnessWorkspaceOption): string {
	if (kind === 'jira' && workspace?.jiraProjectKey) {
		return `project = ${workspace.jiraProjectKey} AND sprint in openSprints() AND statusCategory != Done ORDER BY updated DESC`;
	}
	if (kind === 'local') return '.harness/tasks';
	return '';
}

export function SourceRow({ source, workspaceName, now, onChange, onRemove, onClear, clearing }: { source: HarnessSourceSettings; workspaceName?: string; now: number; onChange: (patch: Partial<HarnessSourceSettings>) => void; onRemove: () => void; onClear: () => void; clearing: boolean }) {
	const checked = source.lastCheckedAt ? `Looked ${formatAgo(source.lastCheckedAt, now)}${source.lastFound !== undefined ? ` · ${source.lastFound} new` : ''}` : 'Not looked yet';
	const queryLabel = source.kind === 'jira' ? 'Jira search (JQL)' : source.kind === 'github' ? 'Repository (owner/name)' : source.kind === 'local' ? 'Task folder (inside the workspace, or a full path)' : 'Repository (workspace/name)';
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
export function ReadyCheckFields({ source, onChange }: { source: HarnessSourceSettings; onChange: (patch: Partial<HarnessSourceSettings>) => void }) {
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

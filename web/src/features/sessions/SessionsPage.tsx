import { useState } from 'react';
import { Workflow } from 'lucide-react';
import type { SessionScope } from '../../lib/types';
import { readStringPreference, writePreference } from '../../shared/preferences/store';
import { useGlobalSessions } from './useGlobalSessions';
import { SessionBrowser } from './SessionBrowser';

const scopePreferenceKey = 'sessions.scope';

export function SessionsPage({ onOpenSession, onOpenWorkbench }: {
	onOpenSession: (workspaceId: string, sessionNodeId: string) => void;
	onOpenWorkbench: () => void;
}) {
	const [scope, setScope] = useState<SessionScope>(() => readScopePreference());
	const sessions = useGlobalSessions(scope);

	const changeScope = (next: SessionScope) => {
		setScope(next);
		writePreference(scopePreferenceKey, next);
	};

	return (
		<section className="sessions-page" aria-label="Sessions">
			<header className="page-title sessions-page-header">
				<div className="workstream-heading"><div><h1>Sessions</h1><p>Every AI session across your workspaces.</p></div></div>
			</header>
			<div className="sessions-toolbar">
				<div className="segmented-control segmented-control-compact" role="group" aria-label="Session scope">
					<button type="button" className={scope === 'active' ? 'active' : ''} aria-pressed={scope === 'active'} onClick={() => changeScope('active')}>Active</button>
					<button type="button" className={scope === 'all' ? 'active' : ''} aria-pressed={scope === 'all'} onClick={() => changeScope('all')}>All</button>
				</div>
			</div>
			{sessions.error && <div className="sessions-state error" role="alert">{sessions.error}<button type="button" onClick={() => sessions.refresh()}>Retry</button></div>}
			{sessions.truncated && <p className="sessions-note">Showing the most recent sessions only; older ones are not listed.</p>}
			{!sessions.error && sessions.rows.length === 0 && (
				<div className="empty-state">
					<p>No sessions running. Start one from a plan on the Workbench.</p>
					<button type="button" className="primary" onClick={onOpenWorkbench}><Workflow size={15} /> Go to Workbench</button>
				</div>
			)}
			{sessions.rows.length > 0 && (
				<SessionBrowser
					rows={sessions.rows}
					grouping="workspace-plan"
					persistKey="sessions-page"
					onStop={(row) => sessions.stop(row.record.id)}
					onReload={() => sessions.refresh()}
					onOpenOnBoard={(row) => onOpenSession(row.record.workspaceId, `session:${row.record.id}`)}
				/>
			)}
		</section>
	);
}

function readScopePreference(): SessionScope {
	const stored = readStringPreference(scopePreferenceKey, 'active');
	return stored === 'all' ? 'all' : 'active';
}

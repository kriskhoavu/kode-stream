import { useEffect, useMemo, useState } from 'react';
import { Workflow } from 'lucide-react';
import type { GlobalSessionRow, SessionChanges, SessionScope } from '../../lib/types';
import { api } from '../../shared/api';
import { readStringPreference, writePreference } from '../../shared/preferences/store';
import { useModalDialog } from '../../components/overlay';
import { useGlobalSessions } from './useGlobalSessions';
import { SessionsTable } from './SessionsTable';
import { SessionChangesPanel } from './SessionChangesPanel';

const scopePreferenceKey = 'sessions.scope';

export function SessionsPage({ onOpenSession, onOpenWorkbench }: {
	onOpenSession: (workspaceId: string, sessionNodeId: string) => void;
	onOpenWorkbench: () => void;
}) {
	const [scope, setScope] = useState<SessionScope>(() => readScopePreference());
	const sessions = useGlobalSessions(scope);
	const [workspaceFilter, setWorkspaceFilter] = useState<string[]>([]);
	const [providerFilter, setProviderFilter] = useState<string[]>([]);
	const [query, setQuery] = useState('');
	const [confirming, setConfirming] = useState<GlobalSessionRow>();
	const [stopping, setStopping] = useState(false);
	const [stopError, setStopError] = useState('');
	const [changesFor, setChangesFor] = useState<GlobalSessionRow>();

	const changeScope = (next: SessionScope) => {
		setScope(next);
		writePreference(scopePreferenceKey, next);
	};

	const workspaceOptions = useMemo(() => Array.from(new Set(sessions.rows.map((row) => row.workspaceName))).sort(), [sessions.rows]);
	const providerOptions = useMemo(() => Array.from(new Set(sessions.rows.map((row) => row.record.provider))).sort(), [sessions.rows]);

	// A workspace or provider filter that no longer has a matching row would
	// otherwise stay selected invisibly, leaving the table empty with no chip
	// left to un-toggle.
	useEffect(() => {
		setWorkspaceFilter((current) => {
			const next = current.filter((name) => workspaceOptions.includes(name));
			return next.length === current.length ? current : next;
		});
	}, [workspaceOptions]);

	useEffect(() => {
		setProviderFilter((current) => {
			const next = current.filter((value) => providerOptions.includes(value));
			return next.length === current.length ? current : next;
		});
	}, [providerOptions]);

	const clearFilters = () => {
		setWorkspaceFilter([]);
		setProviderFilter([]);
		setQuery('');
	};

	const filteredRows = useMemo(() => {
		const normalizedQuery = query.trim().toLowerCase();
		return sessions.rows.filter((row) => {
			if (workspaceFilter.length > 0 && !workspaceFilter.includes(row.workspaceName)) return false;
			if (providerFilter.length > 0 && !providerFilter.includes(row.record.provider)) return false;
			if (!normalizedQuery) return true;
			const haystack = `${row.planTitle ?? ''} ${row.record.intent}`.toLowerCase();
			return haystack.includes(normalizedQuery);
		});
	}, [sessions.rows, workspaceFilter, providerFilter, query]);

	const toggle = (list: string[], value: string, set: (next: string[]) => void) => {
		set(list.includes(value) ? list.filter((entry) => entry !== value) : [...list, value]);
	};

	const requestStop = (row: GlobalSessionRow) => {
		setStopError('');
		setConfirming(row);
	};
	const confirmStop = async () => {
		if (!confirming) return;
		setStopping(true);
		setStopError('');
		try {
			await sessions.stop(confirming.record.id);
			setConfirming(undefined);
		} catch (caught) {
			setStopError(caught instanceof Error ? caught.message : 'Could not stop the session.');
		} finally {
			setStopping(false);
		}
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
				<input type="search" className="sessions-search" placeholder="Search plan title or intent" aria-label="Search sessions" value={query} onChange={(event) => setQuery(event.target.value)} />
			</div>
			{workspaceOptions.length > 0 && (
				<div className="sessions-filter-row" role="group" aria-label="Filter by workspace">
					{workspaceOptions.map((workspaceName) => (
						<button key={workspaceName} type="button" className={workspaceFilter.includes(workspaceName) ? 'sessions-filter-chip active' : 'sessions-filter-chip'} aria-pressed={workspaceFilter.includes(workspaceName)} onClick={() => toggle(workspaceFilter, workspaceName, setWorkspaceFilter)}>{workspaceName}</button>
					))}
				</div>
			)}
			{providerOptions.length > 0 && (
				<div className="sessions-filter-row" role="group" aria-label="Filter by provider">
					{providerOptions.map((provider) => (
						<button key={provider} type="button" className={providerFilter.includes(provider) ? 'sessions-filter-chip active' : 'sessions-filter-chip'} aria-pressed={providerFilter.includes(provider)} onClick={() => toggle(providerFilter, provider, setProviderFilter)}>{provider}</button>
					))}
				</div>
			)}
			{sessions.error && <div className="sessions-state error" role="alert">{sessions.error}<button type="button" onClick={() => sessions.refresh()}>Retry</button></div>}
			{sessions.truncated && <p className="sessions-note">Showing the most recent sessions only; older ones are not listed.</p>}
			{!sessions.error && filteredRows.length === 0 && (
				<div className="empty-state">
					<p>{sessions.rows.length === 0 ? 'No sessions running. Start one from a plan on the Workbench.' : 'No sessions match the current filters.'}</p>
					{sessions.rows.length === 0 && <button type="button" className="primary" onClick={onOpenWorkbench}><Workflow size={15} /> Go to Workbench</button>}
					{sessions.rows.length > 0 && <button type="button" className="ghost" onClick={clearFilters}>Clear filters</button>}
				</div>
			)}
			{filteredRows.length > 0 && (
				<SessionsTable
					rows={filteredRows}
					onOpen={(row) => onOpenSession(row.record.workspaceId, `session:${row.record.id}`)}
					onStop={requestStop}
					onChanges={(row) => setChangesFor(row)}
				/>
			)}
			{confirming && (
				<StopSessionDialog
					label={confirming.planTitle || confirming.record.intent}
					busy={stopping}
					error={stopError}
					onCancel={() => setConfirming(undefined)}
					onConfirm={() => void confirmStop()}
				/>
			)}
			{changesFor && (
				<SessionChangesDrawer row={changesFor} onClose={() => setChangesFor(undefined)} />
			)}
		</section>
	);
}

function readScopePreference(): SessionScope {
	const stored = readStringPreference(scopePreferenceKey, 'active');
	return stored === 'all' ? 'all' : 'active';
}

function StopSessionDialog({ label, busy, error, onCancel, onConfirm }: { label: string; busy: boolean; error: string; onCancel: () => void; onConfirm: () => void }) {
	const dialog = useModalDialog(onCancel, !busy);
	return (
		<div className="confirm-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onCancel(); }}>
			<section ref={dialog.ref} className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby={dialog.titleId}>
				<header>
					<h2 id={dialog.titleId}>Stop this session?</h2>
					<button className="icon-button" type="button" aria-label="Close dialog" disabled={busy} onClick={onCancel}>×</button>
				</header>
				<p>Stop the session for &ldquo;{label}&rdquo;? Any unsaved work in its terminal is left as-is; the process itself ends.</p>
				{error && <p className="confirm-dialog-error" role="alert">{error}</p>}
				<footer>
					<button data-autofocus className="ghost" type="button" disabled={busy} onClick={onCancel}>Cancel</button>
					<button className="danger-confirm" type="button" disabled={busy} onClick={onConfirm}>Stop session</button>
				</footer>
			</section>
		</div>
	);
}

function SessionChangesDrawer({ row, onClose }: { row: GlobalSessionRow; onClose: () => void }) {
	const dialog = useModalDialog(onClose);
	const [changes, setChanges] = useState<SessionChanges>();
	const [error, setError] = useState('');
	const label = row.planTitle || row.record.intent;

	useEffect(() => {
		let cancelled = false;
		setChanges(undefined);
		setError('');
		api.sessionChanges(row.record.id).then((result) => {
			if (!cancelled) setChanges(result);
		}).catch((caught: unknown) => {
			if (!cancelled) setError(caught instanceof Error ? caught.message : 'Could not load changes.');
		});
		return () => { cancelled = true; };
	}, [row.record.id]);

	return (
		<div className="session-changes-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
			<aside ref={dialog.ref} className="session-changes-drawer" role="dialog" aria-modal="true" aria-labelledby={dialog.titleId}>
				<header>
					<h2 id={dialog.titleId}>Changes · {label}</h2>
					<button className="icon-button" type="button" aria-label="Close changes" onClick={onClose}>×</button>
				</header>
				{error && <p className="session-changes-note" role="alert">{error}</p>}
				{!error && !changes && <p className="session-changes-note">Loading changes…</p>}
				{changes && <SessionChangesPanel sessionId={row.record.id} changes={changes} />}
			</aside>
		</div>
	);
}

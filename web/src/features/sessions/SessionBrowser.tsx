import { useEffect, useMemo, useRef, useState } from 'react';
import type { GlobalSessionRow } from '../../lib/types';
import { SessionBrowserTerminal } from './SessionBrowserTerminal';
import { SessionChangesPanel } from './SessionChangesPanel';
import { SessionCard, sessionTitle } from './SessionCard';
import { formatDuration, formatSessionDate, isSessionLive, SessionConfirmDialog } from './sessionDisplay';
import { useSessionBrowserSelection } from './useSessionBrowserSelection';

export type SessionBrowserGrouping = 'plan' | 'workspace-plan';

type Group = { key: string; title: string; rows: GlobalSessionRow[] };

function groupLabel(row: GlobalSessionRow): string {
	return row.planTitle || row.record.intent || row.record.provider;
}

function byLiveThenNewest(a: GlobalSessionRow, b: GlobalSessionRow): number {
	const liveDiff = Number(isSessionLive(b.record)) - Number(isSessionLive(a.record));
	if (liveDiff !== 0) return liveDiff;
	return Date.parse(b.record.startedAt) - Date.parse(a.record.startedAt);
}

function buildGroups(rows: GlobalSessionRow[], grouping: SessionBrowserGrouping): Group[] {
	const buckets = new Map<string, Group>();
	for (const row of rows) {
		const key = grouping === 'workspace-plan' ? `${row.workspaceName} ${groupLabel(row)}` : groupLabel(row);
		const title = grouping === 'workspace-plan' ? `${row.workspaceName} · ${groupLabel(row)}` : groupLabel(row);
		const existing = buckets.get(key);
		if (existing) existing.rows.push(row);
		else buckets.set(key, { key, title, rows: [row] });
	}
	const groups = [...buckets.values()];
	for (const group of groups) group.rows.sort(byLiveThenNewest);
	groups.sort((a, b) => {
		const liveDiff = Number(b.rows.some((row) => isSessionLive(row.record))) - Number(a.rows.some((row) => isSessionLive(row.record)));
		if (liveDiff !== 0) return liveDiff;
		return a.title.localeCompare(b.title);
	});
	return groups;
}

/**
 * The session browser: a filterable, grouped card list on the left and the
 * selected session's live terminal (or its Changes) on the right. Shared by
 * the Workbench Sessions view (grouped by plan). The grouping option remains
 * reusable for callers that aggregate more than one workspace.
 */
export function SessionBrowser({ rows, grouping, persistKey, onStop, onRemove, onReload, onOpenOnBoard, onOpenOnCard, emptyMessage = 'No sessions yet. Start one from a plan on the Workbench.' }: {
	rows: GlobalSessionRow[];
	grouping: SessionBrowserGrouping;
	persistKey: string;
	onStop: (row: GlobalSessionRow) => Promise<unknown>;
	onRemove?: (row: GlobalSessionRow) => Promise<unknown>;
	onReload: () => Promise<unknown> | void;
	onOpenOnBoard?: (row: GlobalSessionRow) => void;
	/** Opens the session in its plan's Workstream card drawer; offered only for a session launched from a plan. */
	onOpenOnCard?: (row: GlobalSessionRow) => void;
	emptyMessage?: string;
}) {
	const [query, setQuery] = useState('');
	const [liveOnly, setLiveOnly] = useState(() => rows.some((row) => isSessionLive(row.record)));
	const [removeTarget, setRemoveTarget] = useState<GlobalSessionRow>();
	const [removing, setRemoving] = useState(false);
	const [removeError, setRemoveError] = useState('');
	const cardRefs = useRef(new Map<string, HTMLButtonElement>());

	const filteredRows = useMemo(() => {
		const normalizedQuery = query.trim().toLowerCase();
		return rows.filter((row) => {
			if (liveOnly && !isSessionLive(row.record)) return false;
			if (!normalizedQuery) return true;
			const haystack = `${sessionTitle(row)} ${row.record.intent ?? ''} ${row.record.provider}`.toLowerCase();
			return haystack.includes(normalizedQuery);
		});
	}, [rows, liveOnly, query]);

	const groups = useMemo(() => buildGroups(filteredRows, grouping), [filteredRows, grouping]);
	const visibleRows = useMemo(() => groups.flatMap((group) => group.rows), [groups]);
	const { selectedId, select } = useSessionBrowserSelection(persistKey, rows, visibleRows, liveOnly, setLiveOnly);
	const selectedRow = rows.find((row) => row.record.id === selectedId);
	const rovingId = selectedId && visibleRows.some((row) => row.record.id === selectedId) ? selectedId : visibleRows[0]?.record.id;

	const moveSelection = (currentId: string, delta: 1 | -1) => {
		if (visibleRows.length === 0) return;
		const index = visibleRows.findIndex((row) => row.record.id === currentId);
		const nextIndex = index === -1 ? 0 : (index + delta + visibleRows.length) % visibleRows.length;
		const next = visibleRows[nextIndex];
		select(next.record.id);
		cardRefs.current.get(next.record.id)?.focus();
	};

	const onCardKeyDown = (row: GlobalSessionRow) => (event: React.KeyboardEvent<HTMLButtonElement>) => {
		if (event.key === 'ArrowDown' || event.key === 'ArrowRight') { event.preventDefault(); moveSelection(row.record.id, 1); }
		else if (event.key === 'ArrowUp' || event.key === 'ArrowLeft') { event.preventDefault(); moveSelection(row.record.id, -1); }
		else if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); select(row.record.id); }
	};
	const remove = async () => {
		if (!removeTarget || !onRemove) return;
		setRemoving(true);
		setRemoveError('');
		try {
			await onRemove(removeTarget);
			setRemoveTarget(undefined);
		} catch (caught) {
			setRemoveError(caught instanceof Error ? caught.message : 'The session could not be removed.');
		} finally {
			setRemoving(false);
		}
	};

	return <div className="session-browser">
		<div className="session-browser-list">
			<div className="session-browser-toolbar">
				<input type="search" className="session-browser-filter" aria-label="Filter sessions" placeholder="Filter by title or provider" value={query} onChange={(event) => setQuery(event.target.value)} />
				<button type="button" className={liveOnly ? 'session-browser-live-toggle active' : 'session-browser-live-toggle'} aria-pressed={liveOnly} onClick={() => setLiveOnly((value) => !value)}>Live only</button>
			</div>
			{rows.length === 0 && <p className="session-browser-empty">{emptyMessage}</p>}
			{rows.length > 0 && visibleRows.length === 0 && <p className="session-browser-empty">No sessions match the current filter.</p>}
			{groups.map((group) => <section key={group.key} className="session-browser-group">
				<h3 className="session-browser-group-title">{group.title}</h3>
				<ul className="session-browser-cards">
					{group.rows.map((row) => <SessionCard key={row.record.id} row={row} selected={row.record.id === selectedId} tabIndex={row.record.id === rovingId ? 0 : -1} cardRef={(element) => { if (element) cardRefs.current.set(row.record.id, element); else cardRefs.current.delete(row.record.id); }} onSelect={() => select(row.record.id)} onRemove={onRemove ? () => { setRemoveError(''); setRemoveTarget(row); } : undefined} onKeyDown={onCardKeyDown(row)} />)}
				</ul>
			</section>)}
		</div>
		<div className="session-browser-detail">
			{selectedRow ? <SessionDetail row={selectedRow} onStop={onStop} onReload={onReload} onOpenOnBoard={onOpenOnBoard} onOpenOnCard={onOpenOnCard} /> : <p className="session-browser-detail-empty">Select a session to see its terminal.</p>}
		</div>
		{removeTarget && <SessionConfirmDialog title="Remove session?" message={`Remove ${sessionTitle(removeTarget)} from session history? This removes its saved terminal metadata; workspace files and Git changes are untouched.`} confirmLabel="Remove session" busyLabel="Removing…" busy={removing} error={removeError} onCancel={() => { setRemoveTarget(undefined); setRemoveError(''); }} onConfirm={() => void remove()} />}
	</div>;
}

function SessionDetail({ row, onStop, onReload, onOpenOnBoard, onOpenOnCard }: { row: GlobalSessionRow; onStop: (row: GlobalSessionRow) => Promise<unknown>; onReload: () => Promise<unknown> | void; onOpenOnBoard?: (row: GlobalSessionRow) => void; onOpenOnCard?: (row: GlobalSessionRow) => void }) {
	const [tab, setTab] = useState<'terminal' | 'changes'>('terminal');
	const [confirming, setConfirming] = useState(false);
	const [stopping, setStopping] = useState(false);
	const [stopError, setStopError] = useState('');
	const [, setTick] = useState(0);
	const live = isSessionLive(row.record);
	const title = sessionTitle(row);

	useEffect(() => { setTab('terminal'); }, [row.record.id]);

	// Re-renders the header every second while the session is live, so the
	// running duration keeps advancing instead of freezing at first render.
	useEffect(() => {
		if (!live) return;
		const timer = window.setInterval(() => setTick((tick) => tick + 1), 1000);
		return () => window.clearInterval(timer);
	}, [live]);

	const stop = async () => {
		setStopping(true);
		setStopError('');
		try {
			await onStop(row);
			setConfirming(false);
		} catch (caught) {
			setStopError(caught instanceof Error ? caught.message : 'The session could not be stopped.');
		} finally {
			setStopping(false);
		}
	};

	return <div className="session-browser-detail-content">
		<header className="session-browser-detail-header">
			<div>
				<h3>{title}</h3>
				<dl>
					<dt>Plan</dt><dd>{row.planTitle || '—'}</dd>
					<dt>Branch</dt><dd>{row.record.requestedBranch}</dd>
					<dt>Launch commit</dt><dd><code>{shortSHA(row.record.observedCommit)}</code></dd>
					<dt>Started</dt><dd>{formatSessionDate(row.record.startedAt)}</dd>
					<dt>Duration</dt><dd>{formatDuration(row.record)}</dd>
				</dl>
			</div>
			<div className="session-browser-detail-actions">
				{onOpenOnBoard && <button type="button" className="secondary" onClick={() => onOpenOnBoard(row)}>Open on board</button>}
				{onOpenOnCard && row.record.planRef && <button type="button" className="secondary" onClick={() => onOpenOnCard(row)}>Open on card</button>}
				{live && <button type="button" className="danger-confirm" onClick={() => setConfirming(true)}>{`Stop ${title} session`}</button>}
			</div>
		</header>
		<div className="side-panel-tabs session-browser-detail-tabs" role="tablist" aria-label="Session detail tabs">
			<button type="button" className={tab === 'terminal' ? 'active' : ''} aria-selected={tab === 'terminal'} onClick={() => setTab('terminal')}>Terminal</button>
			<button type="button" className={tab === 'changes' ? 'active' : ''} aria-selected={tab === 'changes'} onClick={() => setTab('changes')}>Changes</button>
		</div>
		{tab === 'terminal' && <SessionBrowserTerminal record={row.record} visible onReload={onReload} />}
		{tab === 'changes' && <SessionChangesPanel session={row.record} />}
		{confirming && <SessionConfirmDialog title="Stop session?" message={`Stop the ${title} session on ${row.record.requestedBranch}? Its durable session metadata will remain available.`} confirmLabel="Stop session" busyLabel="Stopping…" busy={stopping} error={stopError} onCancel={() => { setConfirming(false); setStopError(''); }} onConfirm={() => void stop()} />}
	</div>;
}

function shortSHA(value?: string): string { return value ? value.slice(0, 8) : 'Unavailable'; }

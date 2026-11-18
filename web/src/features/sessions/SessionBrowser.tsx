import { useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import type { GlobalSessionRow } from '../../lib/types';
import { byLiveThenNewest, harnessSession, interactiveSession, sessionGroup, sessionIsLive } from './agentSessions';
import type { AgentSession, HarnessSessionRow, SessionKind } from './agentSessions';
import { SessionBrowserTerminal } from './SessionBrowserTerminal';
import { SessionChangesPanel } from './SessionChangesPanel';
import { HarnessSessionCard, SessionCard, SessionKindTag, sessionTitle } from './SessionCard';
import { formatDuration, formatSessionDate, isSessionLive, SessionConfirmDialog } from './sessionDisplay';
import { useSessionBrowserSelection } from './useSessionBrowserSelection';
import { agentRoleLabels } from '../harness/vocabulary';
import '../harness/harness.css';
import './sessions.css';

export type SessionBrowserGrouping = 'plan' | 'workspace-plan';

type Group = { key: string; title: string; rows: AgentSession[] };
type KindFilter = 'all' | SessionKind;

function buildGroups(rows: AgentSession[], grouping: SessionBrowserGrouping): Group[] {
	const buckets = new Map<string, Group>();
	for (const row of rows) {
		const own = sessionGroup(row);
		const prefix = grouping === 'workspace-plan' && row.kind === 'interactive' ? row.row.workspaceName : '';
		const key = prefix ? `${prefix} ${own.key}` : own.key;
		const title = prefix ? `${prefix} · ${own.title}` : own.title;
		const existing = buckets.get(key);
		if (existing) {
			existing.rows.push(row);
			// A harness task names the group by its key and title, which reads better than a plan title alone.
			if (row.kind === 'harness') existing.title = title;
		} else buckets.set(key, { key, title, rows: [row] });
	}
	const groups = [...buckets.values()];
	for (const group of groups) group.rows.sort(byLiveThenNewest);
	groups.sort((a, b) => {
		const liveDiff = Number(b.rows.some(sessionIsLive)) - Number(a.rows.some(sessionIsLive));
		if (liveDiff !== 0) return liveDiff;
		return a.title.localeCompare(b.title);
	});
	return groups;
}

function searchText(session: AgentSession): string {
	if (session.kind === 'interactive') return `${sessionTitle(session.row)} ${session.row.record.intent ?? ''} ${session.row.record.provider} ${session.row.planKey ?? ''}`;
	const { task, key, runs } = session.harness;
	const agents = runs.flatMap((detail) => detail.agentRuns).map((agentRun) => `${agentRoleLabels[agentRun.role]} ${agentRun.engine} ${agentRun.model}`).join(' ');
	return `${key} ${task?.title ?? ''} ${runs.map((detail) => detail.run.branch).join(' ')} ${agents} harness run`;
}

/**
 * The session browser: a filterable, grouped card list on the left and the
 * selected session on the right. It lists two kinds of session side by side
 * (see `agentSessions.ts`): an interactive session shows its live terminal,
 * a harness session its conversation, and both their Changes. Shared by the
 * Workbench Sessions view (grouped by plan). The grouping option remains
 * reusable for callers that aggregate more than one workspace.
 */
export function SessionBrowser({ rows, harness, renderHarnessDetail, grouping, persistKey, onStop, onRemove, onReload, onOpenOnBoard, onOpenOnCard, emptyMessage = 'No sessions yet. Start one from a plan on the Workbench.' }: {
	rows: GlobalSessionRow[];
	/** The harness's agents, listed beside the interactive sessions. Without it the list has one kind and no kind filter. */
	harness?: HarnessSessionRow[];
	renderHarnessDetail?: (row: HarnessSessionRow) => ReactNode;
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
	const [kind, setKind] = useState<KindFilter>('all');
	const sessions = useMemo<AgentSession[]>(() => [...rows.map(interactiveSession), ...(harness ?? []).map(harnessSession)], [rows, harness]);
	const [liveOnly, setLiveOnly] = useState(() => sessions.some(sessionIsLive));
	const mixed = harness !== undefined;
	const [removeTarget, setRemoveTarget] = useState<GlobalSessionRow>();
	const [removing, setRemoving] = useState(false);
	const [removeError, setRemoveError] = useState('');
	const cardRefs = useRef(new Map<string, HTMLButtonElement>());

	const filteredRows = useMemo(() => {
		const normalizedQuery = query.trim().toLowerCase();
		return sessions.filter((session) => {
			if (kind !== 'all' && session.kind !== kind) return false;
			if (liveOnly && !sessionIsLive(session)) return false;
			if (!normalizedQuery) return true;
			return searchText(session).toLowerCase().includes(normalizedQuery);
		});
	}, [sessions, kind, liveOnly, query]);

	const groups = useMemo(() => buildGroups(filteredRows, grouping), [filteredRows, grouping]);
	const visibleRows = useMemo(() => groups.flatMap((group) => group.rows), [groups]);
	const { selectedId, select } = useSessionBrowserSelection(persistKey, sessions, visibleRows, liveOnly, setLiveOnly);
	const selected = sessions.find((session) => session.id === selectedId);
	const rovingId = selectedId && visibleRows.some((row) => row.id === selectedId) ? selectedId : visibleRows[0]?.id;

	const moveSelection = (currentId: string, delta: 1 | -1) => {
		if (visibleRows.length === 0) return;
		const index = visibleRows.findIndex((row) => row.id === currentId);
		const nextIndex = index === -1 ? 0 : (index + delta + visibleRows.length) % visibleRows.length;
		const next = visibleRows[nextIndex];
		select(next.id);
		cardRefs.current.get(next.id)?.focus();
	};

	const onCardKeyDown = (id: string) => (event: React.KeyboardEvent<HTMLButtonElement>) => {
		if (event.key === 'ArrowDown' || event.key === 'ArrowRight') { event.preventDefault(); moveSelection(id, 1); }
		else if (event.key === 'ArrowUp' || event.key === 'ArrowLeft') { event.preventDefault(); moveSelection(id, -1); }
		else if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); select(id); }
	};
	const cardRef = (id: string) => (element: HTMLButtonElement | null) => { if (element) cardRefs.current.set(id, element); else cardRefs.current.delete(id); };
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
				<input type="search" className="session-browser-filter" aria-label="Filter sessions" placeholder={mixed ? 'Filter by title, ticket or agent' : 'Filter by title or provider'} value={query} onChange={(event) => setQuery(event.target.value)} />
				<button type="button" className={liveOnly ? 'session-browser-live-toggle active' : 'session-browser-live-toggle'} aria-pressed={liveOnly} onClick={() => setLiveOnly((value) => !value)}>Live only</button>
			</div>
			{mixed && <div className="hx-seg hx-seg-small session-browser-kinds" role="tablist" aria-label="Session kind">
				{(['all', 'interactive', 'harness'] as KindFilter[]).map((candidate) => <button key={candidate} type="button" role="tab" aria-selected={kind === candidate} className={kind === candidate ? 'on' : ''} title={kindHints[candidate]} onClick={() => setKind(candidate)}>{kindLabels[candidate]}</button>)}
			</div>}
			{sessions.length === 0 && <p className="session-browser-empty">{emptyMessage}</p>}
			{sessions.length > 0 && visibleRows.length === 0 && <p className="session-browser-empty">No sessions match the current filter.</p>}
			{groups.map((group) => <section key={group.key} className="session-browser-group">
				<h3 className="session-browser-group-title" title={group.title}><span>{group.title}</span></h3>
				<ul className="session-browser-cards">
					{group.rows.map((session) => session.kind === 'interactive'
						? <SessionCard key={session.id} row={session.row} tagged={mixed} selected={session.id === selectedId} tabIndex={session.id === rovingId ? 0 : -1} cardRef={cardRef(session.id)} onSelect={() => select(session.id)} onRemove={onRemove ? () => { setRemoveError(''); setRemoveTarget(session.row); } : undefined} onKeyDown={onCardKeyDown(session.id)} />
						: <HarnessSessionCard key={session.id} row={session.harness} selected={session.id === selectedId} tabIndex={session.id === rovingId ? 0 : -1} cardRef={cardRef(session.id)} onSelect={() => select(session.id)} onKeyDown={onCardKeyDown(session.id)} />)}
				</ul>
			</section>)}
		</div>
		<div className="session-browser-detail">
			{selected?.kind === 'interactive' && <SessionDetail row={selected.row} tagged={mixed} onStop={onStop} onReload={onReload} onOpenOnBoard={onOpenOnBoard} onOpenOnCard={onOpenOnCard} />}
			{selected?.kind === 'harness' && renderHarnessDetail?.(selected.harness)}
			{!selected && <p className="session-browser-detail-empty">{mixed ? 'Select a session to see its terminal or its conversation.' : 'Select a session to see its terminal.'}</p>}
		</div>
		{removeTarget && <SessionConfirmDialog title="Remove session?" message={`Remove ${sessionTitle(removeTarget)} from session history? This removes its saved terminal metadata; workspace files and Git changes are untouched.`} confirmLabel="Remove session" busyLabel="Removing…" busy={removing} error={removeError} onCancel={() => { setRemoveTarget(undefined); setRemoveError(''); }} onConfirm={() => void remove()} />}
	</div>;
}

const kindLabels: Record<KindFilter, string> = { all: 'All', interactive: 'Interactive', harness: 'Harness' };
const kindHints: Record<KindFilter, string> = {
	all: 'Every agent session of this workspace',
	interactive: 'Agents a person started in a terminal and types into',
	harness: 'Agents the harness ran for a task, headless, in a worktree of their own'
};

function SessionDetail({ row, tagged, onStop, onReload, onOpenOnBoard, onOpenOnCard }: { row: GlobalSessionRow; tagged?: boolean; onStop: (row: GlobalSessionRow) => Promise<unknown>; onReload: () => Promise<unknown> | void; onOpenOnBoard?: (row: GlobalSessionRow) => void; onOpenOnCard?: (row: GlobalSessionRow) => void }) {
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
			<div className="session-browser-detail-heading">
				<h3>{tagged && <SessionKindTag kind="interactive" />}<span>{title}</span></h3>
				<dl>
					<div><dt>Plan</dt><dd>{row.planTitle || '—'}</dd></div>
					<div><dt>Branch</dt><dd><code>{row.record.requestedBranch}</code></dd></div>
					<div><dt>Launch commit</dt><dd><code>{shortSHA(row.record.observedCommit)}</code></dd></div>
					<div><dt>Started</dt><dd>{formatSessionDate(row.record.startedAt)}</dd></div>
					<div><dt>Duration</dt><dd>{formatDuration(row.record)}</dd></div>
				</dl>
			</div>
			<div className="session-browser-detail-actions">
				{onOpenOnBoard && <button type="button" className="secondary" onClick={() => onOpenOnBoard(row)}>Open on board</button>}
				{onOpenOnCard && row.record.planRef && <button type="button" className="secondary" onClick={() => onOpenOnCard(row)}>Open on card</button>}
				{live && <button type="button" className="danger-confirm" onClick={() => setConfirming(true)}>{`Stop ${title} session`}</button>}
			</div>
		</header>
		<div className="hx-tabs session-browser-detail-tabs" role="tablist" aria-label="Session detail tabs">
			<button type="button" role="tab" className={tab === 'terminal' ? 'on' : ''} aria-selected={tab === 'terminal'} onClick={() => setTab('terminal')}>Terminal</button>
			<button type="button" role="tab" className={tab === 'changes' ? 'on' : ''} aria-selected={tab === 'changes'} onClick={() => setTab('changes')}>Changes</button>
		</div>
		{tab === 'terminal' && <SessionBrowserTerminal record={row.record} visible onReload={onReload} />}
		{tab === 'changes' && <SessionChangesPanel session={row.record} />}
		{confirming && <SessionConfirmDialog title="Stop session?" message={`Stop the ${title} session on ${row.record.requestedBranch}? Its durable session metadata will remain available.`} confirmLabel="Stop session" busyLabel="Stopping…" busy={stopping} error={stopError} onCancel={() => { setConfirming(false); setStopError(''); }} onConfirm={() => void stop()} />}
	</div>;
}

function shortSHA(value?: string): string { return value ? value.slice(0, 8) : 'Unavailable'; }

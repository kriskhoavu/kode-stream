import { useEffect, useState } from 'react';
import { useModalDialog } from '../../components/overlay';
import { api } from '../../shared/api';
import type { GlobalSessionRow, SafeSessionRecord } from '../../lib/types';

/** A session record counted as live: launched and still active. */
export function isSessionLive(record: SafeSessionRecord): boolean {
	return Boolean(record.live) && (record.state === 'starting' || record.state === 'running');
}

export function formatSessionDate(value?: string): string {
	if (!value) return '—';
	const date = new Date(value);
	return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function formatDuration(record: SafeSessionRecord): string {
	const started = Date.parse(record.startedAt);
	if (Number.isNaN(started)) return '—';
	const endValue = record.endedAt ?? (isSessionLive(record) ? undefined : record.lastKnownAt);
	const end = endValue ? Date.parse(endValue) : Date.now();
	const totalSeconds = Math.max(0, Math.floor((end - started) / 1000));
	const hours = Math.floor(totalSeconds / 3600);
	const minutes = Math.floor((totalSeconds % 3600) / 60);
	const seconds = totalSeconds % 60;
	const pad = (value: number) => String(value).padStart(2, '0');
	return hours > 0 ? `${hours}:${pad(minutes)}:${pad(seconds)}` : `${minutes}:${pad(seconds)}`;
}

/**
 * Shared table for the Workbench Sessions tab (phase F2) and the Sessions
 * page (phase F3). Data-source agnostic: callers supply rows of
 * `GlobalSessionRow` shape and the Open/Changes callbacks; Stop is
 * self-contained since every caller cancels the same way.
 */
export function SessionsTable({ rows, onOpen, onChanges, onStopped, emptyMessage = 'No sessions.' }: {
	rows: GlobalSessionRow[];
	onOpen: (row: GlobalSessionRow) => void;
	onChanges: (row: GlobalSessionRow) => void;
	onStopped?: (row: GlobalSessionRow) => void;
	emptyMessage?: string;
}) {
	const [stopTarget, setStopTarget] = useState<GlobalSessionRow>();
	const [stopping, setStopping] = useState(false);
	const [stopError, setStopError] = useState('');
	const [, setDurationTick] = useState(0);
	const anyLive = rows.some((row) => isSessionLive(row.record));

	// Re-renders the table every second while any row is live, so a live
	// row's duration keeps advancing instead of freezing at its first render.
	useEffect(() => {
		if (!anyLive) return;
		const timer = window.setInterval(() => setDurationTick((tick) => tick + 1), 1000);
		return () => window.clearInterval(timer);
	}, [anyLive]);

	const confirmStop = async () => {
		if (!stopTarget) return;
		setStopping(true);
		setStopError('');
		try {
			await api.cancelEmbeddedAISession(stopTarget.record.id);
			onStopped?.(stopTarget);
			setStopTarget(undefined);
		} catch (caught) {
			setStopError(caught instanceof Error ? caught.message : 'The session could not be stopped.');
		} finally {
			setStopping(false);
		}
	};

	if (rows.length === 0) return <p className="sessions-table-empty">{emptyMessage}</p>;

	return <>
		<table className="sessions-table">
			<thead><tr><th scope="col">State</th><th scope="col">Provider</th><th scope="col">Plan</th><th scope="col">Started</th><th scope="col">Duration</th><th scope="col">Changes</th><th scope="col">Actions</th></tr></thead>
			<tbody>
				{rows.map((row) => {
					const live = isSessionLive(row.record);
					const label = row.planTitle || row.record.provider;
					return <tr key={row.record.id}>
						<td><span className={`sessions-table-state state-${row.record.state}`}>{row.record.state}</span></td>
						<td>{row.record.provider}</td>
						<td>{row.planTitle || '—'}</td>
						<td>{formatSessionDate(row.record.startedAt)}</td>
						<td>{formatDuration(row.record)}</td>
						<td>{row.changesCount ?? '—'}</td>
						<td className="sessions-table-actions">
							<button type="button" className="secondary" onClick={() => onOpen(row)}>{`Open ${label} session`}</button>
							{live && <button type="button" className="danger-confirm" onClick={() => { setStopError(''); setStopTarget(row); }}>{`Stop ${label} session`}</button>}
							<button type="button" className="secondary" onClick={() => onChanges(row)}>{`View changes for ${label} session`}</button>
						</td>
					</tr>;
				})}
			</tbody>
		</table>
		{stopTarget && <StopConfirmDialog title="Stop session?" message={`Stop the ${stopTarget.planTitle || stopTarget.record.provider} session on ${stopTarget.record.requestedBranch}? Its durable session metadata will remain available.`} busy={stopping} error={stopError} onCancel={() => { setStopTarget(undefined); setStopError(''); }} onConfirm={() => void confirmStop()} />}
	</>;
}

/**
 * A Cancel-default confirmation dialog with a focus trap, shared by every
 * Stop action in the Workbench (the Sessions tab table here, and the
 * session node's Terminal tab summary).
 */
export function StopConfirmDialog({ title, message, busy, error, onCancel, onConfirm }: { title: string; message: string; busy: boolean; error?: string; onCancel: () => void; onConfirm: () => void }) {
	const dialog = useModalDialog(onCancel, !busy);
	return <div className="confirm-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onCancel(); }}>
		<section ref={dialog.ref} className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby={dialog.titleId}>
			<header><h2 id={dialog.titleId}>{title}</h2></header>
			<p>{message}</p>
			{error && <p className="error" role="alert">{error}</p>}
			<footer>
				<button data-autofocus className="ghost" type="button" disabled={busy} onClick={onCancel}>Cancel</button>
				<button className="danger-confirm" type="button" disabled={busy} onClick={onConfirm}>{busy ? 'Stopping…' : 'Stop session'}</button>
			</footer>
		</section>
	</div>;
}

import { useModalDialog } from '../../components/overlay';
import type { SafeSessionRecord } from '../../lib/types';

/** A session record counted as live: launched and still active. */
export function isSessionLive(record: SafeSessionRecord): boolean {
	return Boolean(record.live) && (record.state === 'starting' || record.state === 'running');
}

export function formatSessionDate(value?: string): string {
	if (!value) return '—';
	const date = new Date(value);
	return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

/** hh:mm:ss (or mm:ss) elapsed since start, frozen at the end timestamp once a session has finished. */
export function formatDuration(record: SafeSessionRecord): string {
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

/** Relative "age" since a session started, for the browser's compact card ("5m ago", "2h ago"). */
export function formatAge(startedAt: string): string {
	const started = Date.parse(startedAt);
	if (Number.isNaN(started)) return '—';
	const totalSeconds = Math.max(0, Math.floor((Date.now() - started) / 1000));
	if (totalSeconds < 60) return `${totalSeconds}s ago`;
	const minutes = Math.floor(totalSeconds / 60);
	if (minutes < 60) return `${minutes}m ago`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours}h ago`;
	const days = Math.floor(hours / 24);
	return `${days}d ago`;
}

/**
 * A Cancel-default confirmation dialog with a focus trap, shared by every
 * Stop action across the Canvas Workbench and the Session Browser.
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

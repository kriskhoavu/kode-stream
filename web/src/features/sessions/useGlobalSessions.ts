import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../../shared/api';
import type { GlobalSessionRow, SessionScope } from '../../lib/types';

const pollIntervalMs = 5000;
const failureBackoffMs = [5000, 10000, 20000];

/**
 * Loads the cross-workspace session list for a scope. Polls every 5s only
 * while at least one loaded row is still live -- the same behaviour the
 * Canvas already follows for its own session records, so the Sessions page
 * never polls once nothing is running. Every load is guarded by a request
 * token: a stale response (a scope switch mid-flight, or an unmount) is
 * dropped instead of clobbering newer state or scheduling another poll.
 *
 * A failed poll no longer stops polling for good: while the last known rows
 * still contained a live row, the load is retried with a simple backoff
 * (5s, 10s, then 20s, capped) so a transient failure doesn't strand the page
 * on stale data forever.
 */
export function useGlobalSessions(scope: SessionScope) {
	const [rows, setRows] = useState<GlobalSessionRow[]>([]);
	const [truncated, setTruncated] = useState(false);
	const [error, setError] = useState('');
	const [loading, setLoading] = useState(false);
	const requestRef = useRef(0);
	const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
	const rowsRef = useRef<GlobalSessionRow[]>([]);
	const failureCountRef = useRef(0);

	const load = useCallback(async () => {
		const id = ++requestRef.current;
		if (timerRef.current) clearTimeout(timerRef.current);
		setLoading(true);
		try {
			const response = await api.listSessions(scope);
			if (requestRef.current !== id) return;
			setRows(response.rows);
			rowsRef.current = response.rows;
			setTruncated(response.truncated);
			setError('');
			failureCountRef.current = 0;
			if (response.rows.some(isLive)) {
				timerRef.current = setTimeout(() => { void load(); }, pollIntervalMs);
			}
		} catch (caught) {
			if (requestRef.current !== id) return;
			setError(caught instanceof Error ? caught.message : 'Could not load sessions.');
			if (rowsRef.current.some(isLive)) {
				const delay = failureBackoffMs[Math.min(failureCountRef.current, failureBackoffMs.length - 1)];
				failureCountRef.current += 1;
				timerRef.current = setTimeout(() => { void load(); }, delay);
			}
		} finally {
			if (requestRef.current === id) setLoading(false);
		}
	}, [scope]);

	useEffect(() => {
		void load();
		return () => {
			// Invalidate the in-flight request (if any) and stop any scheduled poll,
			// so nothing from a prior scope or an unmounted page can still land.
			requestRef.current += 1;
			if (timerRef.current) clearTimeout(timerRef.current);
		};
	}, [load]);

	const refresh = useCallback(() => { void load(); }, [load]);

	const stop = useCallback(async (id: string) => {
		await api.cancelEmbeddedAISession(id);
		await load();
	}, [load]);

	return { rows, truncated, error, loading, refresh, stop };
}

function isLive(row: GlobalSessionRow): boolean {
	return Boolean(row.record.live) && (row.record.state === 'starting' || row.record.state === 'running');
}

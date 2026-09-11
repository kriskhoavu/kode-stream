import { useEffect, useRef, useState } from 'react';
import { api } from '../../shared/api';
import type { SessionChanges } from '../../lib/types';

const POLL_INTERVAL_MS = 5000;

/**
 * Loads a session's changes and, while the session is live, refreshes every
 * 5s. A request token guards against a stale response landing after the
 * session or liveness changes, and the poll is cancelled on unmount.
 */
export function useSessionChanges(sessionId: string | undefined, live: boolean) {
	const [changes, setChanges] = useState<SessionChanges>();
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const tokenRef = useRef(0);

	useEffect(() => {
		if (!sessionId) {
			setChanges(undefined);
			setError('');
			setLoading(false);
			return;
		}
		let cancelled = false;
		const token = ++tokenRef.current;
		const load = () => {
			setLoading(true);
			api.sessionChanges(sessionId).then((result) => {
				if (cancelled || tokenRef.current !== token) return;
				setChanges(result);
				setError('');
			}).catch((caught) => {
				if (cancelled || tokenRef.current !== token) return;
				setError(caught instanceof Error ? caught.message : 'Session changes could not be loaded.');
			}).finally(() => {
				if (cancelled || tokenRef.current !== token) return;
				setLoading(false);
			});
		};
		load();
		if (!live) return () => { cancelled = true; };
		const timer = window.setInterval(load, POLL_INTERVAL_MS);
		return () => { cancelled = true; window.clearInterval(timer); };
	}, [sessionId, live]);

	return { changes, loading, error };
}

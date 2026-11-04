import { useCallback, useEffect, useState } from 'react';
import { api } from '../../shared/api';

const refreshMs = 60_000;

/**
 * The number on the Harness nav button. Off the harness page it asks the
 * status route once a minute while the tab is visible; on the page,
 * HarnessPage reports the live count through `report`. Any failure shows no
 * number rather than a wrong one.
 */
export function useHarnessWaitingCount(poll: boolean): [number, (count: number) => void] {
	const [count, setCount] = useState(0);
	const report = useCallback((next: number) => setCount(next), []);
	useEffect(() => {
		if (!poll) return;
		let cancelled = false;
		const load = async () => {
			if (document.visibilityState === 'hidden') return;
			try {
				const status = await api.harnessStatus();
				if (!cancelled) setCount(typeof status?.waitingCount === 'number' ? status.waitingCount : 0);
			} catch {
				if (!cancelled) setCount(0);
			}
		};
		void load();
		const timer = window.setInterval(() => void load(), refreshMs);
		return () => {
			cancelled = true;
			window.clearInterval(timer);
		};
	}, [poll]);
	return [count, report];
}

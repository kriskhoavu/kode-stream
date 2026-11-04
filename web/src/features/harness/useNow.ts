import { useCallback, useSyncExternalStore } from 'react';

/**
 * One shared clock per interval, so twenty live timers on a page cost one
 * timer, and every timer on screen ticks in the same frame.
 */
const clocks = new Map<number, { now: number; listeners: Set<() => void>; timer?: number }>();

function clock(intervalMs: number) {
	let entry = clocks.get(intervalMs);
	if (!entry) {
		entry = { now: Date.now(), listeners: new Set() };
		clocks.set(intervalMs, entry);
	}
	return entry;
}

function subscribe(intervalMs: number, listener: () => void) {
	const entry = clock(intervalMs);
	entry.listeners.add(listener);
	if (entry.timer === undefined) {
		// The clock was idle: catch up once, so a page opened later never shows an old time.
		entry.now = Date.now();
		entry.timer = window.setInterval(() => {
			entry.now = Date.now();
			entry.listeners.forEach((notify) => notify());
		}, intervalMs);
	}
	return () => {
		entry.listeners.delete(listener);
		if (entry.listeners.size === 0 && entry.timer !== undefined) {
			window.clearInterval(entry.timer);
			entry.timer = undefined;
		}
	};
}

export function useNow(intervalMs = 1000): number {
	// A stable subscribe, so React does not resubscribe (and restart the clock) on every render.
	const listen = useCallback((listener: () => void) => subscribe(intervalMs, listener), [intervalMs]);
	return useSyncExternalStore(
		listen,
		() => clock(intervalMs).now,
		() => clock(intervalMs).now
	);
}

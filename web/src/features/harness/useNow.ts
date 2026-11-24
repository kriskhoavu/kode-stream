import { useCallback, useSyncExternalStore } from 'react';

// One shared clock per interval, so many live timers cost one timer and tick together.
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
		// An idle clock catches up once, so a late page never shows an old time.
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
	// A stable subscribe stops React from resubscribing and restarting the clock.
	const listen = useCallback((listener: () => void) => subscribe(intervalMs, listener), [intervalMs]);
	return useSyncExternalStore(
		listen,
		() => clock(intervalMs).now,
		() => clock(intervalMs).now
	);
}

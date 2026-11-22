import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

/** What closing a tab leaves: the tabs still open and the one to show next, if any. */
export interface ClosedTab<T> {
	remaining: T[];
	next: T | null;
}

/**
 * A row of open file tabs and which one is active, each tab named by `keyOf`. `keyOf` must be stable
 * (a module-level function). Remembering a tab already open refreshes it in place. Closing the active
 * tab picks its right-hand neighbour, else its left-hand one, as `next`; the caller loads and
 * activates it. When nothing is left, no tab is active.
 */
export function useFileTabs<T>(keyOf: (tab: T) => string) {
	const [openTabs, setOpenTabs] = useState<T[]>([]);
	const [activeKey, setActiveKey] = useState('');
	const openTabsRef = useRef<T[]>([]);
	useEffect(() => { openTabsRef.current = openTabs; }, [openTabs]);
	const activeTab = useMemo(() => openTabs.find((tab) => keyOf(tab) === activeKey) ?? null, [activeKey, keyOf, openTabs]);

	const remember = useCallback((tab: T) => {
		const key = keyOf(tab);
		setOpenTabs((current) => {
			const existingIndex = current.findIndex((candidate) => keyOf(candidate) === key);
			if (existingIndex < 0) return [...current, tab];
			const next = current.slice();
			next[existingIndex] = tab;
			return next;
		});
		setActiveKey(key);
	}, [keyOf]);

	const close = useCallback((tabToClose: T): ClosedTab<T> => {
		const key = keyOf(tabToClose);
		const currentTabs = openTabsRef.current;
		const index = currentTabs.findIndex((tab) => keyOf(tab) === key);
		const remaining = currentTabs.filter((tab) => keyOf(tab) !== key);
		const next = key === activeKey ? remaining[index] ?? remaining[index - 1] ?? null : currentTabs.find((tab) => keyOf(tab) === activeKey) ?? null;
		setOpenTabs(remaining);
		if (!next) setActiveKey('');
		return { remaining, next };
	}, [activeKey, keyOf]);

	/** Keeps only the tabs `keep` accepts; an active tab it drops gives way to the first one left. */
	const retain = useCallback((keep: (tab: T) => boolean): T[] => {
		const remaining = openTabsRef.current.filter(keep);
		setOpenTabs(remaining);
		setActiveKey((current) => remaining.some((tab) => keyOf(tab) === current) ? current : remaining[0] ? keyOf(remaining[0]) : '');
		return remaining;
	}, [keyOf]);

	const reset = useCallback(() => {
		setOpenTabs([]);
		setActiveKey('');
	}, []);

	return { openTabs, activeKey, activeTab, activate: setActiveKey, remember, close, retain, reset };
}

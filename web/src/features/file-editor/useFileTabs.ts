import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

export interface ClosedTab<T> {
	remaining: T[];
	next: T | null;
}

// `keyOf` must be stable, such as a module-level function.
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

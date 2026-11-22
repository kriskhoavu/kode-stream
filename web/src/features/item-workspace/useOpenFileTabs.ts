import { useCallback, useEffect, useRef, useState } from 'react';
import type { FileContent } from '../../lib/types';
import { lastPathSegment } from '../../shared/domain/paths';
import type { OpenItemFileTab } from './itemWorkspaceTypes';

/** What closing a tab leaves: the tabs still open and the one to show next, if any. */
export interface ClosedTab {
	remaining: OpenItemFileTab[];
	next: OpenItemFileTab | null;
}

/**
 * The item page's open file tabs and which one is active. Opening a file that already has a tab
 * refreshes that tab in place; closing the active tab moves to its right-hand neighbour, else its
 * left-hand one.
 */
export function useOpenFileTabs() {
	const [openTabs, setOpenTabs] = useState<OpenItemFileTab[]>([]);
	const [activeTabId, setActiveTabId] = useState('');
	const openTabsRef = useRef<OpenItemFileTab[]>([]);
	useEffect(() => { openTabsRef.current = openTabs; }, [openTabs]);

	const remember = useCallback((file: FileContent) => {
		const nextTab = { id: file.id, path: file.path, editable: file.editable, name: lastPathSegment(file.path) };
		setOpenTabs((current) => {
			const existingIndex = current.findIndex((tab) => tab.id === nextTab.id);
			if (existingIndex < 0) return [...current, nextTab];
			const next = current.slice();
			next[existingIndex] = nextTab;
			return next;
		});
		setActiveTabId(nextTab.id);
	}, []);

	const close = useCallback((tabToClose: OpenItemFileTab): ClosedTab => {
		const currentTabs = openTabsRef.current;
		const index = currentTabs.findIndex((tab) => tab.id === tabToClose.id);
		const remaining = currentTabs.filter((tab) => tab.id !== tabToClose.id);
		const next = tabToClose.id === activeTabId ? remaining[index] ?? remaining[index - 1] ?? null : currentTabs.find((tab) => tab.id === activeTabId) ?? null;
		setOpenTabs(remaining);
		setActiveTabId(next?.id ?? '');
		return { remaining, next };
	}, [activeTabId]);

	const reset = useCallback(() => {
		setOpenTabs([]);
		setActiveTabId('');
	}, []);

	return { openTabs, activeTabId, activate: setActiveTabId, remember, close, reset };
}

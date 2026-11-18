import { useEffect, useRef, useState } from 'react';
import { readStringPreference, writePreference } from '../../shared/preferences/store';
import { byLiveThenNewest } from './agentSessions';
import type { AgentSession } from './agentSessions';

/**
 * Remembers which session a Session Browser has selected, scoped by
 * `persistKey` (one browser instance per workspace, e.g. a Workbench
 * Sessions view keyed by workspace id).
 * Falls back to the live-first, newest-next visible row whenever nothing is
 * stored or the stored id no longer matches a visible row. When the browser
 * opens on a remembered session that the Live-only filter is the only thing
 * hiding, that filter is turned off instead of silently replacing the
 * remembered selection. Only on opening: a person who turns Live only on
 * later means it, so the selection moves to a live row instead.
 */
export function useSessionBrowserSelection(persistKey: string, rows: AgentSession[], visibleRows: AgentSession[], liveOnly: boolean, setLiveOnly: (value: boolean) => void) {
	const storageKey = `sessionBrowser.selection.${persistKey}`;
	const [selectedId, setSelectedId] = useState<string | undefined>(() => readStringPreference(storageKey, '') || undefined);
	// True until the remembered selection has been restored once for this scope.
	const restoring = useRef(true);

	// A different browser scope (e.g. the Workbench Sessions view pointed at a
	// new workspace) re-reads its own stored selection instead of carrying
	// over the previous scope's id.
	useEffect(() => {
		setSelectedId(readStringPreference(storageKey, '') || undefined);
		restoring.current = true;
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [storageKey]);

	useEffect(() => {
		if (rows.length === 0) return;
		const opening = restoring.current;
		restoring.current = false;
		const stillExists = Boolean(selectedId && rows.some((row) => row.id === selectedId));
		if (stillExists && visibleRows.some((row) => row.id === selectedId)) return;
		if (stillExists && liveOnly && opening) { setLiveOnly(false); return; }
		const next = visibleRows.slice().sort(byLiveThenNewest)[0];
		setSelectedId(next?.id);
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [rows, visibleRows, selectedId, liveOnly, storageKey]);

	const select = (id: string | undefined) => {
		setSelectedId(id);
		writePreference(storageKey, id ?? '');
	};

	return { selectedId, select };
}

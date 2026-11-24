import { useEffect, useRef, useState } from 'react';
import { readStringPreference, writePreference } from '../../shared/preferences/store';
import { byLiveThenNewest } from './agentSessions';
import type { AgentSession } from './agentSessions';

// On open, a Live-only filter that hides the remembered session is turned off.
export function useSessionBrowserSelection(persistKey: string, rows: AgentSession[], visibleRows: AgentSession[], liveOnly: boolean, setLiveOnly: (value: boolean) => void) {
	const storageKey = `sessionBrowser.selection.${persistKey}`;
	const [selectedId, setSelectedId] = useState<string | undefined>(() => readStringPreference(storageKey, '') || undefined);
	const restoring = useRef(true);

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

import { useEffect, useState } from 'react';
import type { GlobalSessionRow } from '../../lib/types';
import { readStringPreference, writePreference } from '../../shared/preferences/store';
import { isSessionLive } from './sessionDisplay';

function byLiveThenNewest(a: GlobalSessionRow, b: GlobalSessionRow): number {
	const liveDiff = Number(isSessionLive(b.record)) - Number(isSessionLive(a.record));
	if (liveDiff !== 0) return liveDiff;
	return Date.parse(b.record.startedAt) - Date.parse(a.record.startedAt);
}

/**
 * Remembers which session a Session Browser has selected, scoped by
 * `persistKey` (one browser instance per workspace, e.g. a Workbench
 * Sessions view keyed by workspace id).
 * Falls back to the live-first, newest-next visible row whenever nothing is
 * stored or the stored id no longer matches a visible row. When the stored
 * id still names a real row but the Live-only filter is the only thing
 * hiding it, that filter is turned off instead of silently replacing the
 * remembered selection.
 */
export function useSessionBrowserSelection(persistKey: string, rows: GlobalSessionRow[], visibleRows: GlobalSessionRow[], liveOnly: boolean, setLiveOnly: (value: boolean) => void) {
	const storageKey = `sessionBrowser.selection.${persistKey}`;
	const [selectedId, setSelectedId] = useState<string | undefined>(() => readStringPreference(storageKey, '') || undefined);

	// A different browser scope (e.g. the Workbench Sessions view pointed at a
	// new workspace) re-reads its own stored selection instead of carrying
	// over the previous scope's id.
	useEffect(() => {
		setSelectedId(readStringPreference(storageKey, '') || undefined);
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [storageKey]);

	useEffect(() => {
		if (rows.length === 0) return;
		const stillExists = Boolean(selectedId && rows.some((row) => row.record.id === selectedId));
		if (stillExists && visibleRows.some((row) => row.record.id === selectedId)) return;
		if (stillExists && liveOnly) { setLiveOnly(false); return; }
		const next = visibleRows.slice().sort(byLiveThenNewest)[0];
		setSelectedId(next?.record.id);
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [rows, visibleRows, selectedId, liveOnly, storageKey]);

	const select = (id: string | undefined) => {
		setSelectedId(id);
		writePreference(storageKey, id ?? '');
	};

	return { selectedId, select };
}

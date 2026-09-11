import type { SafeSessionRecord } from '../../lib/types';
import { CanvasSessionTerminal } from '../canvas/CanvasBoard';
import { providerLabel } from './SessionCard';

/**
 * Reattaches to a session's live terminal for the Session Browser's detail
 * pane. A thin wrapper around CanvasBoard.tsx's `CanvasSessionTerminal`
 * (the browser is a second, independent place a session's terminal can be
 * viewed from) with this feature's own class names and aria-label.
 */
export function SessionBrowserTerminal({ record, visible, onReload }: { record: SafeSessionRecord; visible: boolean; onReload: () => Promise<unknown> | void }) {
	return <CanvasSessionTerminal record={record} visible={visible} onReload={onReload} classPrefix="session-browser-terminal" ariaLabel={`${providerLabel(record.provider)} terminal`} />;
}

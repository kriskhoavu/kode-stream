import type { SafeSessionRecord } from '../../lib/types';
import { SessionTerminal } from '../ai-session/SessionTerminal';
import { providerLabel } from '../ai-session/providerDisplay';

/**
 * Reattaches to a session's live terminal for the Session Browser's detail
 * pane. A thin wrapper around `SessionTerminal`
 * (the browser is a second, independent place a session's terminal can be
 * viewed from) with this feature's own class names and aria-label.
 */
export function SessionBrowserTerminal({ record, visible, onReload }: { record: SafeSessionRecord; visible: boolean; onReload: () => Promise<unknown> | void }) {
	return <SessionTerminal record={record} visible={visible} onReload={onReload} classPrefix="session-browser-terminal" ariaLabel={`${providerLabel(record.provider)} terminal`} />;
}

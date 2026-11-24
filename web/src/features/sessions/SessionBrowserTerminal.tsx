import type { SafeSessionRecord } from '../../lib/types';
import { SessionTerminal } from '../ai-session/SessionTerminal';
import { providerLabel } from '../ai-session/providerDisplay';

export function SessionBrowserTerminal({ record, visible, onReload }: { record: SafeSessionRecord; visible: boolean; onReload: () => Promise<unknown> | void }) {
	return <SessionTerminal record={record} visible={visible} onReload={onReload} classPrefix="session-browser-terminal" ariaLabel={`${providerLabel(record.provider)} terminal`} />;
}

import type { GlobalSessionRow, SafeSessionRecord } from '../../lib/types';
import { Trash2 } from 'lucide-react';
import { formatAge, isSessionLive } from './sessionDisplay';

export type SessionGlyph = { kind: 'spinner' | 'dot'; tone: 'live' | 'success' | 'danger' | 'neutral'; label: string };

/** State glyph + text label for a session card: spinner while live, a coloured dot once it has settled. */
export function sessionGlyph(record: SafeSessionRecord): SessionGlyph {
	if (isSessionLive(record)) return { kind: 'spinner', tone: 'live', label: record.state === 'starting' ? 'Starting' : 'Running' };
	if (record.state === 'cancelled') return { kind: 'dot', tone: 'neutral', label: 'Cancelled' };
	if (record.state === 'exited' && record.exitCode === 0) return { kind: 'dot', tone: 'success', label: 'Exited' };
	if (record.state === 'interrupted') return { kind: 'dot', tone: 'danger', label: 'Interrupted' };
	if (record.state === 'failed') return { kind: 'dot', tone: 'danger', label: 'Failed' };
	return { kind: 'dot', tone: 'danger', label: 'Exited' };
}

const providerLabels: Record<string, string> = { codex: 'Codex', claude: 'Claude', copilot: 'Copilot', opencode: 'OpenCode' };

export function providerLabel(provider: string): string {
	return providerLabels[provider] ?? provider;
}

/** Title shown on a card and in the detail header: plan title, else intent, else provider. */
export function sessionTitle(row: GlobalSessionRow): string {
	return row.planTitle || row.record.intent || row.record.provider;
}

export function SessionCard({ row, selected, tabIndex, cardRef, onSelect, onRemove, onKeyDown }: {
	row: GlobalSessionRow;
	selected: boolean;
	tabIndex: number;
	cardRef: (element: HTMLButtonElement | null) => void;
	onSelect: () => void;
	onRemove?: () => void;
	onKeyDown: (event: React.KeyboardEvent<HTMLButtonElement>) => void;
}) {
	const glyph = sessionGlyph(row.record);
	const title = sessionTitle(row);
	const finished = !isSessionLive(row.record);
	return <li className="session-card-item">
		<button
			type="button"
			ref={cardRef}
			className={selected ? 'session-card selected' : 'session-card'}
			tabIndex={tabIndex}
			aria-current={selected ? 'true' : undefined}
			aria-label={`${title}, ${glyph.label}`}
			onClick={onSelect}
			onKeyDown={onKeyDown}
		>
			<span className={`session-card-glyph glyph-${glyph.kind} tone-${glyph.tone}`} aria-hidden="true" />
			<span className="session-card-body">
				<span className="session-card-title">{title}</span>
				<span className="session-card-meta">
					<span className="session-card-state">{glyph.label}</span>
					<span>{providerLabel(row.record.provider)}</span>
					<span>{formatAge(row.record.startedAt)}</span>
					{finished && typeof row.record.exitCode === 'number' && <span>{`Exit ${row.record.exitCode}`}</span>}
				</span>
			</span>
		</button>
		{finished && onRemove && <button type="button" className="session-card-remove" aria-label={`Remove ${title} session`} title="Remove session" onClick={onRemove}><Trash2 size={14} /></button>}
	</li>;
}

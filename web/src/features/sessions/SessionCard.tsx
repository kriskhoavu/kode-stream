import type { GlobalSessionRow, HarnessAgentRun, HarnessRun, HarnessRunDetail, SafeSessionRecord } from '../../lib/types';
import { agentRoleLabels, engineLabel, modelLabel, plural } from '../harness/vocabulary';
import { harnessRun } from './agentSessions';
import type { HarnessSessionRow, SessionKind } from './agentSessions';
import { Bot, SquareTerminal, Trash2 } from 'lucide-react';
import { HxStatusBadge } from '../harness/HxStatusBadge';
import type { HxStatus } from '../harness/vocabulary';
import { formatAge, isSessionLive } from './sessionDisplay';
import { providerLabel } from '../ai-session/providerDisplay';

export type SessionGlyph = { kind: 'spinner' | 'dot'; tone: 'live' | 'success' | 'danger' | 'neutral'; label: string; status: HxStatus };

export function sessionGlyph(record: SafeSessionRecord): SessionGlyph {
	if (isSessionLive(record)) return { kind: 'spinner', tone: 'live', label: record.state === 'starting' ? 'Starting' : 'Running', status: record.state === 'starting' ? 'starting' : 'working' };
	if (record.state === 'cancelled') return { kind: 'dot', tone: 'neutral', label: 'Cancelled', status: 'stopped' };
	if (record.state === 'exited' && record.exitCode === 0) return { kind: 'dot', tone: 'success', label: 'Exited', status: 'done' };
	if (record.state === 'interrupted') return { kind: 'dot', tone: 'danger', label: 'Interrupted', status: 'needs_fix' };
	if (record.state === 'failed') return { kind: 'dot', tone: 'danger', label: 'Failed', status: 'failed' };
	return { kind: 'dot', tone: 'danger', label: 'Exited', status: 'failed' };
}

export function sessionTitle(row: GlobalSessionRow): string {
	return row.planTitle || row.record.intent || row.record.provider;
}

export function SessionCard({ row, selected, tabIndex, cardRef, onSelect, onRemove, onKeyDown, tagged = false }: {
	row: GlobalSessionRow;
	tagged?: boolean;
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
			<span className="session-card-body">
				<span className="session-card-head">
					<span className="session-card-title">{tagged && <SessionKindTag kind="interactive" />}<span>{title}</span></span>
					<span className="session-card-age">{formatAge(row.record.startedAt)}</span>
				</span>
				<span className="session-card-meta">
					<HxStatusBadge status={glyph.status} label={glyph.label} size="sm" className="session-card-state" />
					<span>{providerLabel(row.record.provider)}</span>
					{finished && typeof row.record.exitCode === 'number' && <span>{`Exit ${row.record.exitCode}`}</span>}
				</span>
			</span>
		</button>
		{finished && onRemove && <button type="button" className="session-card-remove" aria-label={`Remove ${title} session`} title="Remove session" onClick={onRemove}><Trash2 size={14} /></button>}
	</li>;
}

export function SessionKindTag({ kind }: { kind: SessionKind }) {
	const Icon = kind === 'harness' ? Bot : SquareTerminal;
	return <span className={`hx-chip session-kind-tag ${kind}`}><Icon aria-hidden="true" size={12} />{kind === 'harness' ? 'Harness' : 'Interactive'}</span>;
}

export function harnessGlyph(agentRun: HarnessAgentRun): SessionGlyph {
	switch (agentRun.state) {
		case 'running': return { kind: 'spinner', tone: 'live', label: 'Working', status: 'working' };
		case 'queued': return { kind: 'dot', tone: 'neutral', label: 'Queued', status: 'up_next' };
		case 'failed': return { kind: 'dot', tone: 'danger', label: 'Failed', status: 'failed' };
		case 'interrupted': return { kind: 'dot', tone: 'danger', label: 'Interrupted', status: 'needs_fix' };
		case 'cancelled': return { kind: 'dot', tone: 'neutral', label: 'Stopped', status: 'stopped' };
		default:
			if (agentRun.verdict === 'changes_requested') return { kind: 'dot', tone: 'danger', label: 'Changes asked', status: 'needs_fix' };
			if (agentRun.verdict === 'approve') return { kind: 'dot', tone: 'success', label: 'Approved', status: 'done' };
			return { kind: 'dot', tone: 'success', label: 'Done', status: 'done' };
	}
}

export function harnessAgentTitle(agentRun: HarnessAgentRun): string {
	return `${agentRoleLabels[agentRun.role]} · round ${agentRun.round}`;
}

export function harnessAgentLabel(agentRun: HarnessAgentRun): string {
	return [engineLabel(agentRun.engine), modelLabel(agentRun.model)].filter(Boolean).join(' · ');
}

export function startedAgents(detail: HarnessRunDetail): HarnessAgentRun[] {
	return detail.agentRuns
		.filter((agentRun) => agentRun.state !== 'queued')
		.sort((a, b) => Date.parse(a.startedAt ?? '') - Date.parse(b.startedAt ?? ''));
}

export function focusAgent(detail: HarnessRunDetail): HarnessAgentRun | undefined {
	const started = startedAgents(detail);
	return started.find((agentRun) => agentRun.state === 'running') ?? started.at(-1);
}

export function harnessRunGlyph(run: HarnessRun): SessionGlyph {
	switch (run.state) {
		case 'running': return run.currentStage?.state === 'awaiting_approval'
			? { kind: 'dot', tone: 'live', label: 'Waiting on you', status: 'waiting' }
			: { kind: 'spinner', tone: 'live', label: 'Working', status: 'working' };
		case 'blocked': return { kind: 'dot', tone: 'live', label: 'Waiting on you', status: 'waiting' };
		case 'passed': return { kind: 'dot', tone: 'success', label: 'Done', status: 'done' };
		case 'failed': return { kind: 'dot', tone: 'danger', label: 'Failed', status: 'failed' };
		default: return { kind: 'dot', tone: 'neutral', label: 'Stopped', status: 'stopped' };
	}
}

export function HarnessSessionCard({ row, selected, tabIndex, cardRef, onSelect, onKeyDown }: {
	row: HarnessSessionRow;
	selected: boolean;
	tabIndex: number;
	cardRef: (element: HTMLButtonElement | null) => void;
	onSelect: () => void;
	onKeyDown: (event: React.KeyboardEvent<HTMLButtonElement>) => void;
}) {
	const detail = row.runs[0];
	const run = harnessRun(row);
	const glyph = harnessRunGlyph(run);
	const agent = focusAgent(detail);
	const rounds = startedAgents(detail).length;
	const title = row.runs.length > 1 ? `Harness run · ${plural(row.runs.length, 'run')}` : 'Harness run';
	return <li className="session-card-item">
		<button type="button" ref={cardRef} className={selected ? 'session-card selected' : 'session-card'} tabIndex={tabIndex} aria-current={selected ? 'true' : undefined} aria-label={`${row.task ? `${row.key} ` : ''}harness run, ${glyph.label}`} onClick={onSelect} onKeyDown={onKeyDown}>
			<span className="session-card-body">
				<span className="session-card-head">
					<span className="session-card-title"><SessionKindTag kind="harness" /><span>{title}</span></span>
					<span className="session-card-age">{formatAge(run.startedAt)}</span>
				</span>
				<span className="session-card-meta">
					<HxStatusBadge status={glyph.status} label={glyph.label} size="sm" className="session-card-state" />
					{agent && <span>{harnessAgentTitle(agent)}</span>}
					<span>{plural(rounds, 'round')}</span>
				</span>
			</span>
		</button>
	</li>;
}

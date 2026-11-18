import type { GlobalSessionRow, HarnessAgentRun, HarnessRun, HarnessRunDetail, SafeSessionRecord } from '../../lib/types';
import { agentRoleLabels, engineLabel, modelLabel, plural } from '../harness/vocabulary';
import { harnessRun } from './agentSessions';
import type { HarnessSessionRow, SessionKind } from './agentSessions';
import { Trash2 } from 'lucide-react';
import { formatAge, isSessionLive } from './sessionDisplay';
import { providerLabel } from '../ai-session/providerDisplay';

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

/** Title shown on a card and in the detail header: plan title, else intent, else provider. */
export function sessionTitle(row: GlobalSessionRow): string {
	return row.planTitle || row.record.intent || row.record.provider;
}

export function SessionCard({ row, selected, tabIndex, cardRef, onSelect, onRemove, onKeyDown, tagged = false }: {
	row: GlobalSessionRow;
	/** Shows the Interactive tag, where harness sessions share the list. */
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
			<span className={`session-card-glyph glyph-${glyph.kind} tone-${glyph.tone}`} aria-hidden="true" />
			<span className="session-card-body">
				<span className="session-card-head">
					<span className="session-card-title">{tagged && <SessionKindTag kind="interactive" />}<span>{title}</span></span>
					<span className="session-card-age">{formatAge(row.record.startedAt)}</span>
				</span>
				<span className="session-card-meta">
					<span className="session-card-state">{glyph.label}</span>
					<span>{providerLabel(row.record.provider)}</span>
					{finished && typeof row.record.exitCode === 'number' && <span>{`Exit ${row.record.exitCode}`}</span>}
				</span>
			</span>
		</button>
		{finished && onRemove && <button type="button" className="session-card-remove" aria-label={`Remove ${title} session`} title="Remove session" onClick={onRemove}><Trash2 size={14} /></button>}
	</li>;
}

/** The tag that says who drives a session: a person at a terminal, or the harness. */
export function SessionKindTag({ kind }: { kind: SessionKind }) {
	return <span className={`session-kind-tag ${kind}`}>{kind === 'harness' ? 'Harness' : 'Interactive'}</span>;
}

/** Glyph and label for a harness agent: spinner while it works, else how it ended. */
export function harnessGlyph(agentRun: HarnessAgentRun): SessionGlyph {
	switch (agentRun.state) {
		case 'running': return { kind: 'spinner', tone: 'live', label: 'Working' };
		case 'queued': return { kind: 'dot', tone: 'neutral', label: 'Queued' };
		case 'failed': return { kind: 'dot', tone: 'danger', label: 'Failed' };
		case 'interrupted': return { kind: 'dot', tone: 'danger', label: 'Interrupted' };
		case 'cancelled': return { kind: 'dot', tone: 'neutral', label: 'Stopped' };
		default:
			if (agentRun.verdict === 'changes_requested') return { kind: 'dot', tone: 'danger', label: 'Changes asked' };
			if (agentRun.verdict === 'approve') return { kind: 'dot', tone: 'success', label: 'Approved' };
			return { kind: 'dot', tone: 'success', label: 'Done' };
	}
}

/** An agent's name in a run: its role and round, "Builder · round 2". */
export function harnessAgentTitle(agentRun: HarnessAgentRun): string {
	return `${agentRoleLabels[agentRun.role]} · round ${agentRun.round}`;
}

export function harnessAgentLabel(agentRun: HarnessAgentRun): string {
	return [engineLabel(agentRun.engine), modelLabel(agentRun.model)].filter(Boolean).join(' · ');
}

/** A run's agents that have started, in the order they started. */
export function startedAgents(detail: HarnessRunDetail): HarnessAgentRun[] {
	return detail.agentRuns
		.filter((agentRun) => agentRun.state !== 'queued')
		.sort((a, b) => Date.parse(a.startedAt ?? '') - Date.parse(b.startedAt ?? ''));
}

/** The agent a run's card and detail open on: the one working, else the last to start. */
export function focusAgent(detail: HarnessRunDetail): HarnessAgentRun | undefined {
	const started = startedAgents(detail);
	return started.find((agentRun) => agentRun.state === 'running') ?? started.at(-1);
}

/** Glyph and label for a whole run: spinner while an agent works, else where it stands. */
export function harnessRunGlyph(run: HarnessRun): SessionGlyph {
	switch (run.state) {
		case 'running': return run.currentStage?.state === 'awaiting_approval'
			? { kind: 'dot', tone: 'live', label: 'Waiting on you' }
			: { kind: 'spinner', tone: 'live', label: 'Working' };
		case 'blocked': return { kind: 'dot', tone: 'live', label: 'Waiting on you' };
		case 'passed': return { kind: 'dot', tone: 'success', label: 'Done' };
		case 'failed': return { kind: 'dot', tone: 'danger', label: 'Failed' };
		default: return { kind: 'dot', tone: 'neutral', label: 'Stopped' };
	}
}

/**
 * One card per ticket: the harness's agents for it, however many rounds they
 * took. It names the agent in play and the run's totals; the detail walks the
 * rounds.
 */
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
			<span className={`session-card-glyph glyph-${glyph.kind} tone-${glyph.tone}`} aria-hidden="true" />
			<span className="session-card-body">
				<span className="session-card-head">
					<span className="session-card-title"><SessionKindTag kind="harness" /><span>{title}</span></span>
					<span className="session-card-age">{formatAge(run.startedAt)}</span>
				</span>
				<span className="session-card-meta">
					<span className="session-card-state">{glyph.label}</span>
					{agent && <span>{harnessAgentTitle(agent)}</span>}
					<span>{plural(rounds, 'round')}</span>
				</span>
			</span>
		</button>
	</li>;
}

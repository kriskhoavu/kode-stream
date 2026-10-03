import { useEffect, useState } from 'react';
import type { MouseEvent, PointerEvent } from 'react';
import { ArrowLeft, Bot, ChevronRight, GitCompare, PanelBottomOpen, Plus, TerminalSquare } from 'lucide-react';
import { ApiError, api } from '../../shared/api';
import type { ClaudeTranscript, ClaudeTurn, EmbeddedAISessionResult, SafeSessionRecord } from '../../lib/types';
import { AISessionLaunchControl } from '../ai-session/AISessionLaunchControl';
import { AgentChip, providerLabel } from '../ai-session/providerDisplay';
import { SessionTerminal } from '../ai-session/SessionTerminal';
import { attachEmbeddedSession } from '../ai-session/terminalSessions';
import { SessionChangesPanel } from '../sessions/SessionChangesPanel';
import { sessionGlyph } from '../sessions/SessionCard';
import { formatAge } from '../sessions/sessionDisplay';
import { transcriptActive, type ItemAgents } from './useItemSessions';
import './agents.css';

/** What the card drawer's main pane shows: the document, a session, a transcript, or the launcher. */
export type AgentFocus = { kind: 'session'; id: string } | { kind: 'transcript'; id: string } | { kind: 'launch' };

const CLUSTER_LIMIT = 3;
const FORK_WARNING_MS = 2 * 60_000;

export function agentFocusKey(focus?: AgentFocus): string {
	return !focus ? '' : focus.kind === 'launch' ? 'launch' : `${focus.kind}:${focus.id}`;
}

/** The Claude conversation a session record is named after, if it is a Claude session. */
export function claudeConversationOf(record: SafeSessionRecord): string | undefined {
	return /^embedded-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?:-|$)/.exec(record.id)?.[1];
}

const stop = (event: MouseEvent | PointerEvent) => event.stopPropagation();

function newIdempotencyKey() { return globalThis.crypto?.randomUUID?.() ?? `card-${Date.now()}-${Math.random().toString(16).slice(2)}`; }

/**
 * The card's agent chips: one per live session, then `+N`; a hollow chip for
 * a Claude conversation still active in the user's terminal; and, when none
 * of those exists, a launcher shown on hover or focus. Every chip is its own
 * button, so clicking it opens that agent rather than previewing the card.
 */
export function AgentCluster({ agents, title, onOpen }: { agents?: ItemAgents; title: string; onOpen: (focus: AgentFocus) => void }) {
	const live = agents?.live ?? [];
	const active = (agents?.transcripts ?? []).filter((transcript) => transcriptActive(transcript));
	const shown = live.slice(0, CLUSTER_LIMIT);
	const external = active.slice(0, Math.max(0, CLUSTER_LIMIT - shown.length));
	const hidden = live.length + active.length - shown.length - external.length;
	if (shown.length === 0 && external.length === 0) {
		return <button type="button" className="agent-cluster-launcher" aria-label={`Start an agent session for ${title}`} title="Start an agent session" onPointerDown={stop} onClick={(event) => { stop(event); onOpen({ kind: 'launch' }); }}>
			<Bot size={14} /><Plus size={10} className="agent-cluster-launcher-plus" />
		</button>;
	}
	return <span className="agent-cluster" role="group" aria-label={`Agents on ${title}`}>
		{shown.map((record) => <button key={record.id} type="button" className="agent-cluster-chip" aria-label={`Open the running ${providerLabel(record.provider)} session`} title={`${providerLabel(record.provider)} · ${sessionGlyph(record).label} · started ${formatAge(record.startedAt)}`} onPointerDown={stop} onClick={(event) => { stop(event); onOpen({ kind: 'session', id: record.id }); }}>
			<AgentChip provider={record.provider} live />
		</button>)}
		{external.map((transcript) => <button key={transcript.sessionId} type="button" className="agent-cluster-chip" aria-label="Open the Claude Code conversation active in your terminal" title={`Claude Code · active in your terminal · ${formatAge(transcript.lastActiveAt)}`} onPointerDown={stop} onClick={(event) => { stop(event); onOpen({ kind: 'transcript', id: transcript.sessionId }); }}>
			<AgentChip provider="claude" hollow />
		</button>)}
		{hidden > 0 && <button type="button" className="agent-cluster-more" aria-label={`Show all ${live.length + active.length} agents`} onPointerDown={stop} onClick={(event) => { stop(event); onOpen(shown[0] ? { kind: 'session', id: shown[0].id } : { kind: 'launch' }); }}>+{hidden}</button>}
	</span>;
}

/** The drawer's Agents side tab: a launcher, then the card's live sessions, its Claude conversations, and its ended sessions. */
export function AgentsTab({ itemId, agents, focus, onFocus, onLaunched, onError }: { itemId: string; agents?: ItemAgents; focus?: AgentFocus; onFocus: (focus: AgentFocus) => void; onLaunched: (result: EmbeddedAISessionResult) => void; onError: (message: string) => void }) {
	const [earlierOpen, setEarlierOpen] = useState(false);
	const [message, setMessage] = useState('');
	const selected = agentFocusKey(focus);
	const live = agents?.live ?? [];
	const transcripts = agents?.transcripts ?? [];
	const ended = agents?.ended ?? [];
	return <section className="agents-tab" aria-label="Agents">
		<div className="agents-tab-launch">
			<AISessionLaunchControl itemId={itemId} buttonLabel="New session" onLaunched={setMessage} onError={(caught) => onError(caught instanceof Error ? caught.message : 'The session could not be started.')} onEmbeddedLaunched={onLaunched} />
			{message && <p className="agents-tab-message" role="status">{message}</p>}
		</div>
		<AgentGroup title="Live" empty="No session is running for this card.">
			{live.map((record) => <AgentRow key={record.id} selected={selected === `session:${record.id}`} chip={<AgentChip provider={record.provider} live />} title={providerLabel(record.provider)} meta={[sessionGlyph(record).label, formatAge(record.startedAt), record.requestedBranch]} onClick={() => onFocus({ kind: 'session', id: record.id })} />)}
		</AgentGroup>
		{transcripts.length > 0 && <AgentGroup title="In your terminal">
			{transcripts.map((transcript) => <AgentRow key={transcript.sessionId} selected={selected === `transcript:${transcript.sessionId}`} chip={<AgentChip provider="claude" hollow />} title="Claude Code" meta={[transcriptActive(transcript) ? 'Active' : 'Idle', `last ${formatAge(transcript.lastActiveAt)}`, `${transcript.messages} messages`, transcript.branch ?? '']} onClick={() => onFocus({ kind: 'transcript', id: transcript.sessionId })} />)}
		</AgentGroup>}
		{ended.length > 0 && <section className="agents-group">
			<button type="button" className="agents-group-toggle" aria-expanded={earlierOpen} onClick={() => setEarlierOpen((open) => !open)}><ChevronRight size={13} /> Earlier <span>{ended.length}</span></button>
			{earlierOpen && <ul>{ended.map((record) => <AgentRow key={record.id} selected={selected === `session:${record.id}`} chip={<AgentChip provider={record.provider} />} title={providerLabel(record.provider)} meta={[sessionGlyph(record).label, formatAge(record.startedAt), record.requestedBranch]} onClick={() => onFocus({ kind: 'session', id: record.id })} />)}</ul>}
		</section>}
	</section>;
}

function AgentGroup({ title, empty, children }: { title: string; empty?: string; children: React.ReactNode[] }) {
	return <section className="agents-group">
		<h4>{title}</h4>
		{children.length > 0 ? <ul>{children}</ul> : empty && <p className="agents-empty">{empty}</p>}
	</section>;
}

function AgentRow({ selected, chip, title, meta, onClick }: { selected: boolean; chip: React.ReactNode; title: string; meta: string[]; onClick: () => void }) {
	return <li><button type="button" className={selected ? 'agent-row selected' : 'agent-row'} aria-current={selected ? 'true' : undefined} onClick={onClick}>
		{chip}
		<span className="agent-row-body"><strong>{title}</strong><span>{meta.filter(Boolean).join(' · ')}</span></span>
	</button></li>;
}

/**
 * Fetches a fresh grant and gives a running session to the terminal dock, so
 * the process keeps a viewer once the drawer's terminal unmounts.
 */
export async function handOffToDock(record: SafeSessionRecord): Promise<void> {
	const [session, grant] = await Promise.all([api.embeddedAISession(record.id), api.embeddedAISessionGrant(record.id)]);
	attachEmbeddedSession({ session, grant, record });
}

/** The drawer's main pane while an agent is in focus. */
export function AgentPane({ focus, itemId, workspaceId, agents, onBack, onFocus, onReload }: { focus: AgentFocus; itemId: string; workspaceId?: string; agents?: ItemAgents; onBack: () => void; onFocus: (focus: AgentFocus) => void; onReload: () => Promise<unknown> | void }) {
	const back = <button type="button" className="secondary agent-pane-back" onClick={onBack}><ArrowLeft size={14} /> Back to document</button>;
	if (focus.kind === 'launch') {
		return <section className="agent-pane agent-pane-empty" aria-label="Agent">
			<header className="agent-pane-header">{back}</header>
			<div className="agent-pane-placeholder"><Bot size={28} /><p>Start a session from the Agents tab. Its terminal opens here, and you can keep prompting it.</p></div>
		</section>;
	}
	if (focus.kind === 'transcript') {
		const transcript = agents?.transcripts.find((candidate) => candidate.sessionId === focus.id);
		return <TranscriptPane conversation={focus.id} transcript={transcript} itemId={itemId} workspaceId={workspaceId} back={back} onResumed={(result) => { void onReload(); onFocus({ kind: 'session', id: result.session.id }); }} />;
	}
	const record = agents?.live.find((candidate) => candidate.id === focus.id) ?? agents?.ended.find((candidate) => candidate.id === focus.id);
	return <SessionPane key={focus.id} record={record} itemId={itemId} back={back} onBack={onBack} onReload={onReload} onFocus={onFocus} />;
}

function SessionPane({ record, itemId, back, onBack, onReload, onFocus }: { record?: SafeSessionRecord; itemId: string; back: React.ReactNode; onBack: () => void; onReload: () => Promise<unknown> | void; onFocus: (focus: AgentFocus) => void }) {
	const [view, setView] = useState<'terminal' | 'changes'>('terminal');
	const [error, setError] = useState('');
	const [resuming, setResuming] = useState(false);
	if (!record) {
		return <section className="agent-pane" aria-label="Agent">
			<header className="agent-pane-header">{back}</header>
			<p className="agent-pane-placeholder" role="status">Loading session…</p>
		</section>;
	}
	const live = record.live && (record.state === 'starting' || record.state === 'running');
	const conversation = claudeConversationOf(record);
	const popOut = async () => {
		setError('');
		try {
			await handOffToDock(record);
			onBack();
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'The session could not be moved to the dock.');
		}
	};
	const resume = async () => {
		if (!conversation) return;
		setResuming(true);
		setError('');
		try {
			const result = await api.startEmbeddedAISession(itemId, { provider: record.provider, contextMode: 'card_context', resumeClaudeSessionId: conversation, idempotencyKey: newIdempotencyKey(), columns: 80, rows: 24 });
			await Promise.resolve(onReload());
			onFocus({ kind: 'session', id: result.session.id });
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'The conversation could not be resumed.');
		} finally {
			setResuming(false);
		}
	};
	return <section className="agent-pane" aria-label={`${providerLabel(record.provider)} session`}>
		<header className="agent-pane-header">
			{back}
			<span className="agent-pane-title"><AgentChip provider={record.provider} live={live} /><strong>{providerLabel(record.provider)}</strong><span>{sessionGlyph(record).label} · {record.requestedBranch}</span></span>
			<div className="agent-pane-views" role="tablist" aria-label="Session view">
				<button type="button" role="tab" aria-selected={view === 'terminal'} className={view === 'terminal' ? 'active' : ''} onClick={() => setView('terminal')}><TerminalSquare size={14} /> Terminal</button>
				<button type="button" role="tab" aria-selected={view === 'changes'} className={view === 'changes' ? 'active' : ''} onClick={() => setView('changes')}><GitCompare size={14} /> Changes</button>
			</div>
			{live && <button type="button" className="secondary" onClick={() => void popOut()} title="Keep this session in the terminal dock"><PanelBottomOpen size={14} /> Pop out to dock</button>}
			{!live && conversation && <button type="button" className="primary" disabled={resuming} onClick={() => void resume()}>{resuming ? 'Resuming…' : 'Continue here'}</button>}
		</header>
		{error && <p className="error" role="alert">{error}</p>}
		<div className="agent-pane-body">
			<SessionTerminal record={record} visible={view === 'terminal'} onReload={onReload} classPrefix="agent-pane-terminal" ariaLabel={`${providerLabel(record.provider)} terminal`} />
			{view === 'changes' && <SessionChangesPanel session={record} />}
		</div>
	</section>;
}

function TranscriptPane({ conversation, transcript, itemId, workspaceId, back, onResumed }: { conversation: string; transcript?: ClaudeTranscript; itemId: string; workspaceId?: string; back: React.ReactNode; onResumed: (result: EmbeddedAISessionResult) => void }) {
	const [turns, setTurns] = useState<ClaudeTurn[]>();
	const [error, setError] = useState('');
	const [resuming, setResuming] = useState(false);
	useEffect(() => {
		if (!workspaceId) return;
		const controller = new AbortController();
		setTurns(undefined);
		setError('');
		api.claudeTranscriptTail(workspaceId, conversation, 20, controller.signal)
			.then((result) => { if (!controller.signal.aborted) setTurns(result); })
			.catch((caught) => { if (!controller.signal.aborted) setError(caught instanceof ApiError && caught.status === 404 ? 'This conversation is no longer in the workspace.' : 'The conversation could not be read.'); });
		return () => controller.abort();
	}, [conversation, workspaceId, transcript?.lastActiveAt]);
	const mayBeOpen = transcript ? Date.now() - Date.parse(transcript.lastActiveAt) <= FORK_WARNING_MS : false;
	const resume = async () => {
		setResuming(true);
		setError('');
		try {
			onResumed(await api.startEmbeddedAISession(itemId, { provider: 'claude', contextMode: 'card_context', resumeClaudeSessionId: conversation, idempotencyKey: newIdempotencyKey(), columns: 80, rows: 24 }));
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'The conversation could not be resumed.');
		} finally {
			setResuming(false);
		}
	};
	return <section className="agent-pane" aria-label="Claude Code conversation">
		<header className="agent-pane-header">
			{back}
			<span className="agent-pane-title"><AgentChip provider="claude" hollow /><strong>Claude Code</strong><span>{transcript ? `In your terminal · last ${formatAge(transcript.lastActiveAt)}` : 'In your terminal'}</span></span>
			<button type="button" className="primary" disabled={resuming} onClick={() => void resume()}>{resuming ? 'Resuming…' : 'Continue here'}</button>
		</header>
		{mayBeOpen && <p className="agent-pane-warning" role="note">This conversation may still be open in your terminal; continuing here forks it.</p>}
		{error && <p className="error" role="alert">{error}</p>}
		<ol className="agent-transcript" aria-label="Latest turns">
			{turns === undefined && !error && <li className="agent-pane-placeholder" role="status">Reading the conversation…</li>}
			{turns?.map((turn, index) => <li key={index} className={`agent-turn role-${turn.role}`}><span className="agent-turn-role">{turn.role === 'user' ? 'You' : 'Claude'}</span><p>{turn.text}</p></li>)}
		</ol>
	</section>;
}

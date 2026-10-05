import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../../shared/api';
import type { ClaudeTranscript, SafeSessionRecord } from '../../lib/types';
import { isSessionLive } from '../sessions/sessionDisplay';
import { embeddedSessionEndedEvent, embeddedSessionStartedEvent } from '../ai-session/terminalSessions';

const POLL_MS = 10_000;
/** A transcript written to this recently is taken to be a conversation still open in the user's terminal. */
export const TRANSCRIPT_ACTIVE_MS = 10 * 60_000;

/** Everything the board knows about the agents working on one card. */
export interface ItemAgents {
	/** Stable across polls while nothing changed, so memoised cards do not re-render. */
	key: string;
	live: SafeSessionRecord[];
	ended: SafeSessionRecord[];
	transcripts: ClaudeTranscript[];
}

export function transcriptActive(transcript: ClaudeTranscript, now = Date.now()): boolean {
	return now - Date.parse(transcript.lastActiveAt) <= TRANSCRIPT_ACTIVE_MS;
}

/**
 * Loads the branch's session records and the workspace's Claude transcripts,
 * grouped by item, and keeps them current: it polls while the page is visible
 * and reloads when a session starts anywhere in the app. A session reported as
 * ended leaves the live list at once, before the reload confirms it. Transcripts exist only
 * in Local mode; a refusal is remembered and transcripts are no longer asked for.
 */
export function useItemSessions(workspaceId: string | undefined, branch: string) {
	const [byItem, setByItem] = useState<Map<string, ItemAgents>>(() => new Map());
	const previousRef = useRef<Map<string, ItemAgents>>(new Map());
	const transcriptsAvailableRef = useRef(true);
	const generationRef = useRef(0);
	// Sessions reported ended, kept not-live until a reload agrees, so a poll that
	// races the cancellation cannot bring the chip back.
	const endedRef = useRef<Map<string, number>>(new Map());

	const load = useCallback(async () => {
		if (!workspaceId) {
			previousRef.current = new Map();
			setByItem(previousRef.current);
			return;
		}
		const generation = ++generationRef.current;
		const [records, transcripts] = await Promise.all([
			Promise.resolve().then(() => api.aiSessionRecords(workspaceId, branch || undefined)).catch(() => undefined),
			transcriptsAvailableRef.current ? Promise.resolve().then(() => api.claudeTranscripts(workspaceId, branch || undefined)).catch(() => { transcriptsAvailableRef.current = false; return [] as ClaudeTranscript[]; }) : Promise.resolve([] as ClaudeTranscript[])
		]);
		if (generation !== generationRef.current || !Array.isArray(records)) return;
		const now = Date.now();
		for (const [id, until] of endedRef.current) if (until < now) endedRef.current.delete(id);
		const settled = records.map((record) => endedRef.current.has(record.id) && record.live ? { ...record, live: false } : record);
		const next = groupByItem(settled, Array.isArray(transcripts) ? transcripts : [], previousRef.current);
		previousRef.current = next;
		setByItem(next);
	}, [branch, workspaceId]);

	useEffect(() => {
		transcriptsAvailableRef.current = true;
		void load();
		const poll = window.setInterval(() => { if (document.visibilityState === 'visible') void load(); }, POLL_MS);
		const reload = () => { void load(); };
		let settle: number | undefined;
		const ended = (event: Event) => {
			const { sessionId } = (event as CustomEvent<{ sessionId: string }>).detail;
			endedRef.current.set(sessionId, Date.now() + 15_000);
			const next = withoutLive(previousRef.current, sessionId);
			if (next !== previousRef.current) {
				previousRef.current = next;
				setByItem(next);
			}
			window.clearTimeout(settle);
			// The record turns final once the process has exited; read it again then.
			settle = window.setTimeout(() => { void load(); }, 1_000);
		};
		window.addEventListener(embeddedSessionStartedEvent, reload);
		window.addEventListener(embeddedSessionEndedEvent, ended);
		document.addEventListener('visibilitychange', reload);
		return () => {
			generationRef.current++;
			window.clearInterval(poll);
			window.clearTimeout(settle);
			window.removeEventListener(embeddedSessionStartedEvent, reload);
			window.removeEventListener(embeddedSessionEndedEvent, ended);
			document.removeEventListener('visibilitychange', reload);
		};
	}, [load]);

	return { byItem, reload: load };
}

/** The map with sessionId moved from live to ended; the same map when no item holds it live. */
function withoutLive(byItem: Map<string, ItemAgents>, sessionId: string): Map<string, ItemAgents> {
	for (const [itemId, agents] of byItem) {
		const record = agents.live.find((candidate) => candidate.id === sessionId);
		if (!record) continue;
		const next = new Map(byItem);
		next.set(itemId, { ...agents, key: `${agents.key}~${sessionId}`, live: agents.live.filter((candidate) => candidate !== record), ended: [{ ...record, live: false }, ...agents.ended] });
		return next;
	}
	return byItem;
}

function groupByItem(records: SafeSessionRecord[], transcripts: ClaudeTranscript[], previous: Map<string, ItemAgents>): Map<string, ItemAgents> {
	const grouped = new Map<string, Omit<ItemAgents, 'key'>>();
	const entry = (itemId: string) => {
		let value = grouped.get(itemId);
		if (!value) grouped.set(itemId, value = { live: [], ended: [], transcripts: [] });
		return value;
	};
	for (const record of records) {
		const itemId = record.planRef?.itemId;
		if (!itemId) continue;
		(isSessionLive(record) ? entry(itemId).live : entry(itemId).ended).push(record);
	}
	for (const transcript of transcripts) {
		for (const itemId of transcript.itemIds ?? []) entry(itemId).transcripts.push(transcript);
	}
	const next = new Map<string, ItemAgents>();
	for (const [itemId, value] of grouped) {
		const byNewest = (a: SafeSessionRecord, b: SafeSessionRecord) => b.startedAt.localeCompare(a.startedAt) || a.id.localeCompare(b.id);
		value.live.sort(byNewest);
		value.ended.sort(byNewest);
		value.transcripts.sort((a, b) => b.lastActiveAt.localeCompare(a.lastActiveAt));
		const key = [
			...value.live.map((record) => `${record.id}:${record.state}`),
			'|', ...value.ended.map((record) => `${record.id}:${record.state}`),
			'|', ...value.transcripts.map((transcript) => `${transcript.sessionId}:${transcript.lastActiveAt}:${transcript.messages}`)
		].join(',');
		const prior = previous.get(itemId);
		next.set(itemId, prior?.key === key ? prior : { key, ...value });
	}
	return next;
}

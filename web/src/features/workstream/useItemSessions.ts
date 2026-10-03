import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../../shared/api';
import type { ClaudeTranscript, SafeSessionRecord } from '../../lib/types';
import { isSessionLive } from '../sessions/sessionDisplay';
import { embeddedSessionStartedEvent } from '../ai-session/terminalSessions';

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
 * and reloads when a session starts anywhere in the app. Transcripts exist only
 * in Local mode; a refusal is remembered and transcripts are no longer asked for.
 */
export function useItemSessions(workspaceId: string | undefined, branch: string) {
	const [byItem, setByItem] = useState<Map<string, ItemAgents>>(() => new Map());
	const previousRef = useRef<Map<string, ItemAgents>>(new Map());
	const transcriptsAvailableRef = useRef(true);
	const generationRef = useRef(0);

	const load = useCallback(async () => {
		if (!workspaceId) {
			previousRef.current = new Map();
			setByItem(previousRef.current);
			return;
		}
		const generation = ++generationRef.current;
		const [records, transcripts] = await Promise.all([
			api.aiSessionRecords(workspaceId, branch || undefined).catch(() => undefined),
			transcriptsAvailableRef.current ? api.claudeTranscripts(workspaceId, branch || undefined).catch(() => { transcriptsAvailableRef.current = false; return [] as ClaudeTranscript[]; }) : Promise.resolve([] as ClaudeTranscript[])
		]);
		if (generation !== generationRef.current || !Array.isArray(records)) return;
		const next = groupByItem(records, Array.isArray(transcripts) ? transcripts : [], previousRef.current);
		previousRef.current = next;
		setByItem(next);
	}, [branch, workspaceId]);

	useEffect(() => {
		transcriptsAvailableRef.current = true;
		void load();
		const poll = window.setInterval(() => { if (document.visibilityState === 'visible') void load(); }, POLL_MS);
		const reload = () => { void load(); };
		window.addEventListener(embeddedSessionStartedEvent, reload);
		document.addEventListener('visibilitychange', reload);
		return () => {
			generationRef.current++;
			window.clearInterval(poll);
			window.removeEventListener(embeddedSessionStartedEvent, reload);
			document.removeEventListener('visibilitychange', reload);
		};
	}, [load]);

	return { byItem, reload: load };
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

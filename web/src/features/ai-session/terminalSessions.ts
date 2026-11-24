import type { EmbeddedAISessionResult } from '../../lib/types';

export const embeddedSessionStartedEvent = 'kode-stream:embedded-session-started';
export const embeddedSessionAttachEvent = 'kode-stream:embedded-session-attach';
export const embeddedSessionEndedEvent = 'kode-stream:embedded-session-ended';

export function openEmbeddedSession(result: EmbeddedAISessionResult) {
	window.dispatchEvent(new CustomEvent<EmbeddedAISessionResult>(embeddedSessionStartedEvent, { detail: result }));
}

export function notifyEmbeddedSessionEnded(sessionId: string) {
	window.dispatchEvent(new CustomEvent<{ sessionId: string }>(embeddedSessionEndedEvent, { detail: { sessionId } }));
}

// The result carries a fresh grant, so the dock connects on its own after the caller's terminal unmounts.
export function attachEmbeddedSession(result: EmbeddedAISessionResult) {
	window.dispatchEvent(new CustomEvent<EmbeddedAISessionResult>(embeddedSessionAttachEvent, { detail: result }));
}

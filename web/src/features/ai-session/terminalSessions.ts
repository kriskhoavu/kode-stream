import type { EmbeddedAISessionResult } from '../../lib/types';

export const embeddedSessionStartedEvent = 'kode-stream:embedded-session-started';
export const embeddedSessionAttachEvent = 'kode-stream:embedded-session-attach';
export const embeddedSessionEndedEvent = 'kode-stream:embedded-session-ended';

export function openEmbeddedSession(result: EmbeddedAISessionResult) {
	window.dispatchEvent(new CustomEvent<EmbeddedAISessionResult>(embeddedSessionStartedEvent, { detail: result }));
}

/**
 * Tells listeners, at once, that a session stopped or is being cancelled, so a
 * view showing it as live need not wait for its next poll to drop it.
 */
export function notifyEmbeddedSessionEnded(sessionId: string) {
	window.dispatchEvent(new CustomEvent<{ sessionId: string }>(embeddedSessionEndedEvent, { detail: { sessionId } }));
}

/**
 * Hands an already running session to the terminal dock, which then holds it
 * as one of its tabs. The result carries a fresh grant: the dock's terminal
 * connects on its own, so the process keeps a viewer after the caller's
 * terminal unmounts.
 */
export function attachEmbeddedSession(result: EmbeddedAISessionResult) {
	window.dispatchEvent(new CustomEvent<EmbeddedAISessionResult>(embeddedSessionAttachEvent, { detail: result }));
}

import type { EmbeddedAISessionResult } from '../../lib/types';

export const embeddedSessionStartedEvent = 'kode-stream:embedded-session-started';
export const embeddedSessionAttachEvent = 'kode-stream:embedded-session-attach';

export function openEmbeddedSession(result: EmbeddedAISessionResult) {
	window.dispatchEvent(new CustomEvent<EmbeddedAISessionResult>(embeddedSessionStartedEvent, { detail: result }));
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

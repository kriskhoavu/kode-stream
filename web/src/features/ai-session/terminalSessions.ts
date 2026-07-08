import type { EmbeddedAISessionResult } from '../../lib/types';

export const embeddedSessionStartedEvent = 'kode-stream:embedded-session-started';

export function openEmbeddedSession(result: EmbeddedAISessionResult) {
	window.dispatchEvent(new CustomEvent<EmbeddedAISessionResult>(embeddedSessionStartedEvent, { detail: result }));
}

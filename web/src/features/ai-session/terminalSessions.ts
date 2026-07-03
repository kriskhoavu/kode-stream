import type { EmbeddedAISessionResult } from '../../lib/types';

export const embeddedSessionStartedEvent = 'plan-manager:embedded-session-started';

export function openEmbeddedSession(result: EmbeddedAISessionResult) {
	window.dispatchEvent(new CustomEvent<EmbeddedAISessionResult>(embeddedSessionStartedEvent, { detail: result }));
}

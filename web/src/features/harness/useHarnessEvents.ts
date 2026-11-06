import { useEffect, useRef } from 'react';
import type { HarnessEvent } from '../../lib/types';
import type { HarnessFeedHandlers } from './harnessClient';
import type { HarnessController } from './useHarness';

/** How long a broken feed may try to reconnect before the page falls back to polling. */
export const reconnectGraceMs = 10_000;
/** How often the page polls a snapshot while the feed is down (only while the tab is visible). */
export const pollMs = 10_000;
/** How often the page tries the feed again while it polls. */
export const retryFeedMs = 30_000;

/**
 * Opens the server's event feed in the browser (`EventSource`, cookie auth),
 * resuming after the last event the page holds. Each event is folded into the
 * reducer. When the feed opens, a fresh snapshot covers anything missed; when
 * it breaks, the page shows "Reconnecting…", then polls every 10 s while
 * visible and tries the feed again every 30 s.
 */
export function openEventSourceFeed(url: string, handlers: HarnessFeedHandlers): () => void {
	const source = new EventSource(url, { withCredentials: true });
	source.onopen = () => handlers.onOpen();
	source.onmessage = (message) => {
		try {
			const event = JSON.parse(message.data) as HarnessEvent;
			if (event && typeof event.seq === 'number' && typeof event.type === 'string') handlers.onEvent(event);
		} catch {
			// A malformed frame is skipped; the next snapshot corrects anything it carried.
		}
	};
	source.addEventListener('reset', () => handlers.onReset());
	source.onerror = () => handlers.onError();
	return () => source.close();
}

export function useHarnessEvents(harness: HarnessController, active = true): void {
	const { client, dispatch, refresh } = harness;
	const lastSeq = useRef(harness.state.lastSeq);
	lastSeq.current = harness.state.lastSeq;

	useEffect(() => {
		// The board reads the harness beside its items; while the harness is off there is nothing to follow.
		if (!active) return;
		if (!client.subscribe) {
			// No feed at all: poll for good.
			dispatch({ type: 'connection', connection: 'polling' });
			const timer = window.setInterval(() => { if (document.visibilityState !== 'hidden') void refresh(); }, pollMs);
			return () => window.clearInterval(timer);
		}
		const subscribe = client.subscribe.bind(client);
		let close: (() => void) | undefined;
		let grace: number | undefined;
		let poll: number | undefined;
		let retry: number | undefined;
		let stopped = false;

		const clearTimers = () => {
			window.clearTimeout(grace);
			window.clearInterval(poll);
			window.clearTimeout(retry);
			grace = poll = retry = undefined;
		};

		const open = () => {
			close?.();
			close = subscribe(lastSeq.current, {
				onOpen: () => {
					if (stopped) return;
					clearTimers();
					dispatch({ type: 'connection', connection: 'live' });
					void refresh();
				},
				onEvent: (event) => { if (!stopped) dispatch({ type: 'event', event }); },
				onReset: () => { if (!stopped) void refresh(); },
				onError: () => {
					if (stopped) return;
					close?.();
					close = undefined;
					broken();
				}
			});
		};

		const broken = () => {
			if (grace !== undefined || poll !== undefined) return;
			dispatch({ type: 'connection', connection: 'reconnecting' });
			grace = window.setTimeout(() => {
				grace = undefined;
				dispatch({ type: 'connection', connection: 'polling' });
				poll = window.setInterval(() => { if (document.visibilityState !== 'hidden') void refresh(); }, pollMs);
			}, reconnectGraceMs);
			const again = () => {
				retry = window.setTimeout(() => {
					if (stopped) return;
					open();
					again();
				}, retryFeedMs);
			};
			again();
			// One immediate attempt, as a dropped connection often comes straight back.
			window.setTimeout(() => { if (!stopped && !close) open(); }, 1000);
		};

		dispatch({ type: 'connection', connection: 'connecting' });
		open();
		return () => {
			stopped = true;
			clearTimers();
			close?.();
		};
	}, [active, client, dispatch, refresh]);
}

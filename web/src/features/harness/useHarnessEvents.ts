import { useEffect, useRef } from 'react';
import type { HarnessEvent } from '../../lib/types';
import type { HarnessFeedHandlers } from './harnessClient';
import type { HarnessController } from './useHarness';

export const reconnectGraceMs = 10_000;
export const pollMs = 10_000;
export const retryFeedMs = 30_000;

export function openEventSourceFeed(url: string, handlers: HarnessFeedHandlers): () => void {
	const source = new EventSource(url, { withCredentials: true });
	source.onopen = () => handlers.onOpen();
	source.onmessage = (message) => {
		try {
			const event = JSON.parse(message.data) as HarnessEvent;
			if (event && typeof event.seq === 'number' && typeof event.type === 'string') handlers.onEvent(event);
		} catch {
			// Skip a malformed frame. The next snapshot corrects it.
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
		if (!active) return;
		if (!client.subscribe) {
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
			// Retry once at once, because a dropped connection often comes straight back.
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

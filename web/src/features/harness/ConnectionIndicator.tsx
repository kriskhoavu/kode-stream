import type { HarnessConnection } from './harnessState';

const words: Record<HarnessConnection, string> = {
	connecting: 'Connecting…',
	live: 'Live',
	reconnecting: 'Reconnecting…',
	polling: 'Checking every 10 s',
	offline: 'Offline'
};

/** Whether the page is current: a dot and always a word, so it never relies on colour or motion alone. */
export function ConnectionIndicator({ connection }: { connection: HarnessConnection }) {
	return (
		<span className={`hx-connection ${connection}`} title={connection === 'polling' ? 'The live feed is down, so the page asks for news every 10 seconds.' : undefined}>
			<span className="hx-connection-dot" aria-hidden="true" />
			{words[connection]}
		</span>
	);
}

import { useNow } from './useNow';
import { formatClock } from './vocabulary';

/** Time since `since`, ticking once a second on the shared clock. */
export function LiveTimer({ since, label }: { since?: string; label?: string }) {
	const now = useNow(1000);
	const start = since ? Date.parse(since) : Number.NaN;
	if (!Number.isFinite(start)) return null;
	const text = formatClock(now - start);
	return <time className="hx-num" dateTime={since} aria-label={label ? `${label} ${text}` : undefined}>{text}</time>;
}

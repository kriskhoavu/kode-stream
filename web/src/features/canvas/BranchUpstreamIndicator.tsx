import { ArrowDown, ArrowUp, Check, CircleDashed } from 'lucide-react';
import type { GitStatus } from '../../lib/types';

// How far the checkout has drifted from its upstream, in the shape an IDE shows
// it. Git measures ahead/behind against the last-fetched remote-tracking ref, so
// the counts are only as current as the last fetch. Nothing here claims more than
// that: with no fetch on record there is no comparison to report, and saying
// "in sync" would assert exactly the kind of unverified freshness this replaced.
export function BranchUpstreamIndicator({ git }: { git?: GitStatus }) {
	if (!git?.upstream) return null;
	const age = fetchAge(git.fetchedAt);
	const drifted = git.ahead > 0 || git.behind > 0;
	const reading = drifted ? driftLabel(git.ahead, git.behind) : age.known ? 'in sync' : 'not fetched';
	return <span className="branch-upstream-indicator" title={`${git.branch || 'Branch'} vs ${git.upstream} · ${age.label}`}>
		<span className="branch-upstream-ref">{git.upstream}</span>
		{drifted
			? <span className="branch-upstream-drift">
				{git.ahead > 0 && <span className="branch-upstream-ahead" aria-hidden="true"><ArrowUp size={12} />{git.ahead}</span>}
				{git.behind > 0 && <span className="branch-upstream-behind" aria-hidden="true"><ArrowDown size={12} />{git.behind}</span>}
				<span className="sr-only">{reading}</span>
			</span>
			: <span className={age.known ? 'branch-upstream-synced' : 'branch-upstream-unknown'}>
				{age.known ? <Check size={12} /> : <CircleDashed size={12} />}
				<span className="branch-upstream-reading">{reading}</span>
			</span>}
		<span className="sr-only">{age.label}</span>
	</span>;
}

function driftLabel(ahead: number, behind: number) {
	return [ahead > 0 ? `${ahead} ahead` : '', behind > 0 ? `${behind} behind` : ''].filter(Boolean).join(', ');
}

// `known` is false when nothing has fetched since the clone, which makes the
// ahead/behind counts describe a remote state of unknown age rather than the
// current one.
function fetchAge(fetchedAt?: string): { known: boolean; label: string } {
	const unknown = { known: false, label: 'Never fetched' };
	if (!fetchedAt) return unknown;
	const stamp = new Date(fetchedAt).getTime();
	// A Go zero time serializes as year 1 and parses as a real date, so treat
	// anything implausibly old as the absence it actually represents.
	if (!Number.isFinite(stamp) || stamp < Date.UTC(2000, 0, 1)) return unknown;
	// The stamp comes from the server's filesystem and the comparison from the
	// viewer's clock, so a fetch can appear slightly in the future. That is skew,
	// not an absent fetch: clamp it rather than calling the freshest data unknown.
	const minutes = Math.floor(Math.max(0, Date.now() - stamp) / 60000);
	if (minutes < 1) return { known: true, label: 'Fetched just now' };
	if (minutes < 60) return { known: true, label: `Fetched ${minutes}m ago` };
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return { known: true, label: `Fetched ${hours}h ago` };
	return { known: true, label: `Fetched ${Math.floor(hours / 24)}d ago` };
}

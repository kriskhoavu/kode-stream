import { Check } from 'lucide-react';
import type { HarnessAgentRun, HarnessFinding } from '../../lib/types';
import { agentRoleLabels } from './vocabulary';

function finishedReviews(agentRuns: HarnessAgentRun[]): HarnessAgentRun[] {
	return agentRuns
		.filter((agentRun) => (agentRun.role === 'plan_reviewer' || agentRun.role === 'code_reviewer') && (agentRun.verdict || agentRun.findings?.length))
		.sort((a, b) => Date.parse(a.startedAt ?? '') - Date.parse(b.startedAt ?? ''));
}

/** The review that stands now: the latest one the Checker finished. An earlier round it replaced no longer applies. */
export function featuredReview(agentRuns: HarnessAgentRun[]): HarnessAgentRun | undefined {
	return finishedReviews(agentRuns).at(-1);
}

/** For a review that approved, what the earlier rounds of the same stage asked to fix, said in one line. */
export function earlierFixes(agentRuns: HarnessAgentRun[], review: HarnessAgentRun): string {
	if (review.blocking > 0) return '';
	const earlier = finishedReviews(agentRuns).filter((other) => other.role === review.role && other.stageId === review.stageId && other.round < review.round && other.blocking > 0);
	if (!earlier.length) return '';
	const count = earlier.reduce((sum, other) => sum + other.blocking, 0);
	const rounds = earlier.map((other) => other.round).join(', ');
	return `Round${earlier.length > 1 ? 's' : ''} ${rounds} asked for ${count} must-fix item${count === 1 ? '' : 's'}. Round ${review.round} found them fixed.`;
}

/** Must fix, then suggestions, each with its place in the code. Never animated. */
export function FindingsList({ findings }: { findings: HarnessFinding[] }) {
	const blocking = findings.filter((finding) => finding.severity === 'blocking');
	const advisory = findings.filter((finding) => finding.severity === 'advisory');
	return (
		<div className="hx-findings">
			{blocking.length > 0 && (
				<details className="hx-findings-section" open>
					<summary><h3 className="hx-findings-head must">Must fix · {blocking.length}</h3></summary>
					<ul>{blocking.map((finding, index) => <FindingItem key={index} finding={finding} open />)}</ul>
				</details>
			)}
			{advisory.length > 0 && (
				<details className="hx-findings-section" open={blocking.length === 0}>
					<summary><h3 className="hx-findings-head">Suggestions · {advisory.length}</h3></summary>
					<ul>{advisory.map((finding, index) => <FindingItem key={index} finding={finding} />)}</ul>
				</details>
			)}
			{findings.length === 0 && <p className="hx-muted">Nothing to fix and no suggestions.</p>}
		</div>
	);
}

/** The title and place always show; the Checker's reasoning folds under them. Must-fix items start open. */
function FindingItem({ finding, open = false }: { finding: HarnessFinding; open?: boolean }) {
	const head = (
		<>
			<span className="hx-finding-title">{finding.resolved && <Check aria-label="Fixed" size={14} className="hx-finding-fixed" />}{finding.title}</span>
			{finding.path && <span className="hx-finding-where"><code>{finding.path}{finding.line ? `:${finding.line}` : ''}</code></span>}
		</>
	);
	if (!finding.detail) return <li className="hx-finding">{head}</li>;
	return (
		<li className="hx-finding">
			<details open={open}>
				<summary>{head}</summary>
				<p className="hx-finding-detail">{finding.detail}</p>
			</details>
		</li>
	);
}

export function FindingsPanel({ agentRuns, children }: { agentRuns: HarnessAgentRun[]; children?: React.ReactNode }) {
	const review = featuredReview(agentRuns);
	const where = review ? `${review.role === 'plan_reviewer' ? 'plan' : 'code'}, round ${review.round}` : '';
	const fixes = review ? earlierFixes(agentRuns, review) : '';
	return (
		<section className="hx-card hx-panel" aria-labelledby="hx-findings-title">
			<h2 className="hx-section-title" id="hx-findings-title">What the {review ? agentRoleLabels[review.role] : 'Checker'} said {where && <small>· {where}</small>}</h2>
			{fixes && <p className="hx-muted">{fixes}</p>}
			{review ? <FindingsList findings={review.findings ?? []} /> : <p className="hx-muted">The Checker has not reviewed anything in this run yet.</p>}
			{children && <div className="hx-panel-links">{children}</div>}
		</section>
	);
}

import { Check } from 'lucide-react';
import type { HarnessAgentRun, HarnessFinding } from '../../lib/types';
import { agentRoleLabels } from './vocabulary';

/** The review that matters most: the latest one with a must-fix item, else the latest one. */
export function featuredReview(agentRuns: HarnessAgentRun[]): HarnessAgentRun | undefined {
	const reviews = agentRuns
		.filter((agentRun) => (agentRun.role === 'plan_reviewer' || agentRun.role === 'code_reviewer') && agentRun.findings?.length)
		.sort((a, b) => Date.parse(a.startedAt ?? '') - Date.parse(b.startedAt ?? ''));
	return reviews.filter((review) => review.blocking > 0).at(-1) ?? reviews.at(-1);
}

/** Must fix, then suggestions, each with its place in the code and whether a later round fixed it. Never animated. */
export function FindingsList({ findings, fixedInRound }: { findings: HarnessFinding[]; fixedInRound?: number }) {
	const blocking = findings.filter((finding) => finding.severity === 'blocking');
	const advisory = findings.filter((finding) => finding.severity === 'advisory');
	const resolved = blocking.filter((finding) => finding.resolved).length;
	return (
		<div className="hx-findings">
			{blocking.length > 0 && (
				<details className="hx-findings-section" open>
					<summary><h3 className="hx-findings-head must">Must fix · {blocking.length}{resolved && fixedInRound ? ` · fixed in round ${fixedInRound}` : ''}</h3></summary>
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
	const later = review ? agentRuns.find((agentRun) => agentRun.role === review.role && agentRun.stageId === review.stageId && agentRun.round > review.round) : undefined;
	return (
		<section className="hx-card hx-panel" aria-labelledby="hx-findings-title">
			<h2 className="hx-section-title" id="hx-findings-title">What the {review ? agentRoleLabels[review.role] : 'Checker'} said {where && <small>· {where}</small>}</h2>
			{review?.findings ? <FindingsList findings={review.findings} fixedInRound={later?.round} /> : <p className="hx-muted">The Checker has not reviewed anything in this run yet.</p>}
			{children && <div className="hx-panel-links">{children}</div>}
		</section>
	);
}

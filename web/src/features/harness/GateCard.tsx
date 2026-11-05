import { useEffect, useRef, useState } from 'react';
import { Check, Hand, MessageCircleQuestion, TriangleAlert } from 'lucide-react';
import type { HarnessApproval, HarnessApprovalGate, HarnessTask } from '../../lib/types';
import { useModalDialog } from '../../components/overlay';
import type { SentDecision } from './harnessState';
import { decisionLine } from './harnessState';
import { HxStatusBadge } from './HxStatusBadge';
import { agentRoleLabels, formatAgo, formatCost, formatMinutes, gateTitles, plural } from './vocabulary';
import { useNow } from './useNow';

type GateWords = { approve: string; reject: string; askTitle: string; askLabel: string; askPlaceholder: string; answerOnApprove?: boolean; answerLabel?: string; answerPlaceholder?: string; answerSubmit?: string };

const gateWords: Record<HarnessApprovalGate, GateWords> = {
	start: { approve: 'Start', reject: 'Not now', askTitle: 'Why not now?', askLabel: 'Reason', askPlaceholder: 'Say why this task should wait. The Sorter keeps it in line.' },
	plan: { approve: 'Approve', reject: 'Ask for changes', askTitle: 'What should change?', askLabel: 'What should change?', askPlaceholder: 'Tell the Planner what to change. This is sent back with the plan.' },
	resume: { approve: 'Answer and retry', reject: 'Stop this task', askTitle: 'Why stop it?', askLabel: 'Reason', askPlaceholder: 'Say why. The task leaves the queue until its ticket changes.', answerOnApprove: true },
	draft_pr: { approve: 'Open draft PR', reject: 'Not yet', askTitle: 'Why not yet?', askLabel: 'Reason', askPlaceholder: 'Say what should happen first. The Builder gets this note.' },
	question: { approve: 'Answer and continue', reject: 'Stop this task', askTitle: 'Why stop it?', askLabel: 'Reason', askPlaceholder: 'Say why. The task leaves the queue until its ticket changes.', answerOnApprove: true, answerLabel: 'Your answers', answerPlaceholder: 'Answer each question in a line or two. The agent continues from your answers and does not ask again.', answerSubmit: 'Send and continue' }
};

function askerLabel(approval: HarnessApproval): string {
	return approval.context?.role ? agentRoleLabels[approval.context.role] : 'Planner';
}

/** The plain sentence that says what approving does, with the usual time and cost when the harness knows them. */
export function consequence(approval: HarnessApproval): string {
	const estimate = approval.estimate;
	const minutes = estimate?.minMinutes && estimate.maxMinutes ? `about ${estimate.minMinutes}–${estimate.maxMinutes} minutes` : estimate?.maxMinutes ? `about ${estimate.maxMinutes} minutes` : '';
	const cost = estimate?.costUsd !== undefined ? `around ${formatCost(estimate.costUsd)}` : '';
	const usually = minutes || cost ? ` That usually takes ${[minutes, cost].filter(Boolean).join(' and ')}.` : '';
	switch (approval.gate) {
		case 'start': return `Approving lets the Planner start on this task. Nothing runs before that.${usually}`;
		case 'plan': return `Approving lets the Builder start writing code.${usually}`;
		case 'draft_pr': return 'The tests passed twice. Approving pushes the branch and opens a draft pull request for a person to review. Nothing merges on its own.';
		case 'resume': return approval.summary ?? 'The harness stopped and cannot decide alone how to continue.';
		case 'question': return `${approval.summary ?? `The ${askerLabel(approval)} stopped to ask you before going on.`} Nothing runs on this task until you answer.`;
	}
}

export function gateHeading(approval: HarnessApproval, task?: HarnessTask): string {
	const name = task ? `${task.source.key} · ${task.title}` : 'this task';
	if (approval.gate === 'resume') return `Waiting on you: ${name} is stuck`;
	if (approval.gate === 'question') return `Waiting on you: the ${askerLabel(approval)} asks about ${name}`;
	return `Waiting on you: ${gateTitles[approval.gate]} ${name}`;
}

export function GateCard({ approval, task, sent, attention = false, autoFocus = false, onApprove, onReject, onOpen, openLabel = 'See the plan' }: {
	approval: HarnessApproval;
	task?: HarnessTask;
	sent?: SentDecision;
	attention?: boolean;
	autoFocus?: boolean;
	onApprove: (reason?: string) => void;
	onReject: (reason: string) => void;
	onOpen?: () => void;
	openLabel?: string;
}) {
	const now = useNow(30_000);
	const [dialog, setDialog] = useState<'reject' | 'answer' | undefined>();
	const approveRef = useRef<HTMLButtonElement | null>(null);
	const words = gateWords[approval.gate];
	const decided = approval.state !== 'requested';
	const sending = sent?.state === 'sending';
	const line = decisionLine(approval, sent);

	useEffect(() => {
		if (autoFocus && !decided) approveRef.current?.focus();
	}, [autoFocus, decided]);

	if (decided) {
		return (
			<article className="hx-card hx-gate hx-gate-decided" aria-label={`${task?.source.key ?? 'Task'} decided`}>
				<div className="hx-gate-icon ok" aria-hidden="true"><Check strokeWidth={2.5} className={sent ? 'hx-check-draw' : undefined} /></div>
				<div className="hx-gate-body">
					<h3>{task ? `${task.source.key} · ${task.title}` : 'Decision recorded'}</h3>
					<p>{line}{approval.decidedAt ? ` · ${formatAgo(approval.decidedAt, now)}` : ''}{approval.reason ? `. “${approval.reason}”` : ''}</p>
				</div>
				{onOpen && <button type="button" className="hx-link" onClick={onOpen}>See progress</button>}
			</article>
		);
	}

	const context = approval.context;
	const stuck = approval.gate === 'resume';
	const asking = approval.gate === 'question';
	const questions = context?.questions ?? [];
	const facts = [
		context?.blocking !== undefined ? `${context.blocking} must fix` : '',
		context?.advisory !== undefined ? plural(context.advisory, 'suggestion') : '',
		context?.rounds ? plural(context.rounds, 'round') : '',
		context?.durationMs ? formatMinutes(context.durationMs) : '',
		context?.costUsd !== undefined ? formatCost(context.costUsd) : ''
	].filter(Boolean).join(' · ');

	return (
		<article className={['hx-card', 'hx-gate', attention ? 'hx-gate-attention' : ''].filter(Boolean).join(' ')} aria-label={gateHeading(approval, task)}>
			<div className="hx-gate-icon" aria-hidden="true">{stuck ? <TriangleAlert /> : asking ? <MessageCircleQuestion /> : <Hand />}</div>
			<div className="hx-gate-body">
				<h3>{gateHeading(approval, task)}</h3>
				<p>{consequence(approval)}</p>
				{questions.length > 0 && (
					<div className="hx-gate-questions">
						{!asking && <p className="hx-muted">The {askerLabel(approval)} left these open and went on with its own best guess:</p>}
						<ol aria-label={asking ? `What the ${askerLabel(approval)} asks` : `Open questions from the ${askerLabel(approval)}`}>
							{questions.map((question, index) => <li key={index}>{question}</li>)}
						</ol>
					</div>
				)}
				<div className="hx-gate-facts">
					{context?.verdict === 'approve' && <HxStatusBadge status="done" label={approval.gate === 'plan' ? 'Plan check passed' : 'Checks passed'} />}
					{stuck && <HxStatusBadge status="stopped" />}
					{facts && <span className="hx-muted hx-num">{facts}</span>}
					{onOpen && <button type="button" className="hx-link" onClick={onOpen}>{stuck ? 'See what the Checker said' : openLabel}</button>}
				</div>
				{line && <p className={sent?.state === 'failed' ? 'hx-gate-status error' : 'hx-gate-status'} role={sent?.state === 'failed' ? 'alert' : undefined}>{line}</p>}
			</div>
			<div className="hx-gate-actions">
				<button ref={approveRef} type="button" className="primary" disabled={sending} data-autofocus={autoFocus || undefined} onClick={() => words.answerOnApprove ? setDialog('answer') : onApprove()}>{sending && sent?.decision === 'approve' ? 'Sending…' : words.approve}</button>
				<button type="button" className="secondary" disabled={sending} onClick={() => setDialog('reject')}>{words.reject}</button>
			</div>
			{dialog && (
				<GateDecisionDialog
					title={dialog === 'answer' ? (words.answerLabel ?? 'Your answer') : words.askTitle}
					label={dialog === 'answer' ? (words.answerLabel ?? 'Answer the question the Planner could not decide') : words.askLabel}
					placeholder={dialog === 'answer' ? (words.answerPlaceholder ?? 'Your answer is added to the task and the Planner starts a fresh round.') : words.askPlaceholder}
					submitLabel={dialog === 'answer' ? (words.answerSubmit ?? 'Send and retry') : approval.gate === 'plan' ? 'Send to Planner' : 'Send'}
					questions={dialog === 'answer' && asking ? questions : undefined}
					onCancel={() => setDialog(undefined)}
					onSubmit={(reason) => {
						setDialog(undefined);
						if (dialog === 'answer') onApprove(reason);
						else onReject(reason);
					}}
				/>
			)}
		</article>
	);
}

/** Asks for the reason a decision needs. The reason box comes first; Cmd/Ctrl+Enter sends. */
export function GateDecisionDialog({ title, label, placeholder, submitLabel, questions, onCancel, onSubmit }: {
	title: string;
	label: string;
	placeholder: string;
	submitLabel: string;
	/** The questions being answered, repeated above the box so nobody has to scroll back. */
	questions?: string[];
	onCancel: () => void;
	onSubmit: (reason: string) => void;
}) {
	const dialog = useModalDialog(onCancel);
	const [reason, setReason] = useState('');
	const [error, setError] = useState('');
	const submit = () => {
		if (!reason.trim()) {
			setError('Write a few words first. The harness sends them with your decision.');
			return;
		}
		onSubmit(reason.trim());
	};
	return (
		<div className="confirm-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onCancel(); }}>
			<section ref={dialog.ref} className="confirm-dialog hx-dialog" role="dialog" aria-modal="true" aria-labelledby={dialog.titleId}>
				<header>
					<h2 id={dialog.titleId}>{title}</h2>
					<button className="icon-button" type="button" aria-label="Close dialog" onClick={onCancel}>×</button>
				</header>
				<form onSubmit={(event) => { event.preventDefault(); submit(); }}>
					{questions && questions.length > 0 && <ol className="hx-dialog-questions" aria-label="The questions">{questions.map((question, index) => <li key={index}>{question}</li>)}</ol>}
					<label className="hx-field">
						<span>{label}</span>
						<textarea
							data-autofocus
							value={reason}
							placeholder={placeholder}
							aria-invalid={Boolean(error) || undefined}
							aria-describedby={error ? `${dialog.titleId}-error` : undefined}
							onChange={(event) => { setReason(event.target.value); setError(''); }}
							onKeyDown={(event) => {
								if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
									event.preventDefault();
									submit();
								}
							}}
						/>
					</label>
					{error && <p className="hx-field-error" id={`${dialog.titleId}-error`} role="alert">{error}</p>}
					<footer>
						<button className="ghost" type="button" onClick={onCancel}>Back</button>
						<button className="primary" type="submit">{submitLabel}</button>
					</footer>
				</form>
			</section>
		</div>
	);
}

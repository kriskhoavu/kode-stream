import { useState } from 'react';
import { Hand } from 'lucide-react';
import type { HarnessApproval, HarnessTask } from '../../lib/types';
import { gateWords } from '../harness/GateCard';
import type { HarnessState } from '../harness/harnessState';
import { decisionLine, waitingApprovals } from '../harness/harnessState';
import { gateTitles } from '../harness/vocabulary';

/**
 * What waits on you, above the board: one row per approval, with the gate's
 * own words. A plan asks for Approve or Ask for changes; a stuck task asks
 * for Retry; a question wants an answer before the task goes on.
 */
export function NeedsYouStrip({ state, onApprove, onReject, onOpenTask }: {
	state: HarnessState;
	onApprove: (approval: HarnessApproval, reason?: string) => void;
	onReject: (approval: HarnessApproval, reason: string) => void;
	/** Opens the card that holds the task, when the board has one. */
	onOpenTask?: (task: HarnessTask) => void;
}) {
	const waiting = waitingApprovals(state);
	if (waiting.length === 0) return null;
	return (
		<section className="needs-you" aria-labelledby="needs-you-title">
			<h2 id="needs-you-title" className="needs-you-title"><Hand aria-hidden="true" size={15} /> Needs you <small>· {waiting.length}</small></h2>
			<ul className="needs-you-list">
				{waiting.map((approval) => <NeedsYouRow key={approval.id} approval={approval} task={state.tasks[approval.taskId]} sent={state.decisions[approval.id]} onApprove={onApprove} onReject={onReject} onOpenTask={onOpenTask} />)}
			</ul>
		</section>
	);
}

function NeedsYouRow({ approval, task, sent, onApprove, onReject, onOpenTask }: {
	approval: HarnessApproval;
	task?: HarnessTask;
	sent?: HarnessState['decisions'][string];
	onApprove: (approval: HarnessApproval, reason?: string) => void;
	onReject: (approval: HarnessApproval, reason: string) => void;
	onOpenTask?: (task: HarnessTask) => void;
}) {
	const words = gateWords[approval.gate];
	const [asking, setAsking] = useState<'reject' | 'answer' | undefined>();
	const [text, setText] = useState('');
	const sending = sent?.state === 'sending';
	const name = task ? `${task.source.key} · ${task.title}` : 'A task';
	const asks = approval.gate === 'resume' ? 'is stuck' : approval.gate === 'question' ? 'has a question' : `waits for you to ${gateTitles[approval.gate]} it`;
	const send = () => {
		const reason = text.trim();
		if (asking === 'reject') {
			if (!reason) return;
			onReject(approval, reason);
		} else {
			onApprove(approval, reason || undefined);
		}
		setAsking(undefined);
		setText('');
	};
	return (
		<li className="needs-you-row" data-approval-id={approval.id}>
			<span className="needs-you-text">
				{task && onOpenTask ? <button type="button" className="hx-link needs-you-name" onClick={() => onOpenTask(task)}>{name}</button> : <strong className="needs-you-name">{name}</strong>}
				<span className="needs-you-asks">{asks}{approval.summary ? ` · ${approval.summary}` : ''}</span>
				{sent && <span className="needs-you-sent">{decisionLine(approval, sent)}</span>}
			</span>
			{asking ? (
				<form className="needs-you-ask" onSubmit={(event) => { event.preventDefault(); send(); }}>
					<label>
						<span className="sr-only">{asking === 'answer' ? (words.answerLabel ?? 'Your answer') : words.askLabel}</span>
						<textarea rows={2} value={text} autoFocus onChange={(event) => setText(event.target.value)} placeholder={asking === 'answer' ? words.answerPlaceholder : words.askPlaceholder} />
					</label>
					<button type="submit" className="primary" disabled={asking === 'reject' && !text.trim()}>{asking === 'answer' ? (words.answerSubmit ?? 'Send') : 'Send'}</button>
					<button type="button" className="ghost" onClick={() => { setAsking(undefined); setText(''); }}>Back</button>
				</form>
			) : (
				<span className="needs-you-actions">
					<button type="button" className="primary" disabled={sending} onClick={() => words.answerOnApprove ? setAsking('answer') : onApprove(approval)}>{sending && sent?.decision === 'approve' ? 'Sending…' : words.approve}</button>
					<button type="button" className="secondary" disabled={sending} onClick={() => setAsking('reject')}>{words.reject}</button>
				</span>
			)}
		</li>
	);
}

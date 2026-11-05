import type { ReactNode } from 'react';
import type { ClaudeTurn } from '../../lib/types';
import './transcript.css';

/**
 * The text turns of a Claude Code conversation, as the Workstream page and the
 * harness drawer both show them: the person's turns on the right, the model's
 * on the left, tool calls left out.
 */
export function TranscriptTurns({ turns, label, placeholder, names = { user: 'You', assistant: 'Claude' }, className }: {
	turns?: ClaudeTurn[];
	label: string;
	/** Shown while `turns` is undefined. */
	placeholder?: ReactNode;
	names?: { user: string; assistant: string };
	className?: string;
}) {
	return (
		<ol className={className ? `agent-transcript ${className}` : 'agent-transcript'} aria-label={label}>
			{turns === undefined && placeholder !== undefined && <li className="agent-pane-placeholder" role="status">{placeholder}</li>}
			{turns?.map((turn, index) => <li key={index} className={`agent-turn role-${turn.role}`}><span className="agent-turn-role">{turn.role === 'user' ? names.user : names.assistant}</span><p>{turn.text}</p></li>)}
		</ol>
	);
}

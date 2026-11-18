import type { ReactNode } from 'react';
import type { ClaudeTurn } from '../../lib/types';
import './transcript.css';

/**
 * The text turns of a Claude Code conversation, as the Workstream page and the
 * harness drawer both show them: the person's turns on the right, the model's
 * on the left. Where the reader included them, the model's tool calls and
 * thinking are folded into the answer they led to, as one line that opens;
 * work the model has not answered yet stays open, so a running agent can be
 * followed.
 */
function plural(count: number, one: string): string { return `${count} ${count === 1 ? one : `${one}s`}`; }

export function TranscriptTurns({ turns, label, placeholder, names = { user: 'You', assistant: 'Claude' }, className, foldUser }: {
	turns?: ClaudeTurn[];
	label: string;
	/** Shown while `turns` is undefined. */
	placeholder?: ReactNode;
	names?: { user: string; assistant: string };
	className?: string;
	/** Folds the user's turns to one line that opens them, for a long brief the reader rarely needs. */
	foldUser?: string;
}) {
	const blocks = turns ? transcriptBlocks(turns) : undefined;
	return (
		<ol className={className ? `agent-transcript ${className}` : 'agent-transcript'} aria-label={label}>
			{turns === undefined && placeholder !== undefined && <li className="agent-pane-placeholder" role="status">{placeholder}</li>}
			{blocks?.map((block, index) => {
				const role = block.turn?.role ?? 'assistant';
				return (
					<li key={index} className={`agent-turn role-${role}`}>
						<span className="agent-turn-role">{role === 'user' ? names.user : names.assistant}</span>
						{block.work.length > 0 && (
							<details className="agent-work" open={!block.turn}>
								<summary>{workSummary(block.work)}</summary>
								<ol>
									{block.work.map((turn, at) => (
										<li key={at} className={`agent-work-item role-${turn.role}`}>
											<span className="agent-turn-role">{turn.role === 'tool' ? 'Tool' : `${names.assistant} thinks`}</span>
											<p>{turn.text}</p>
										</li>
									))}
								</ol>
							</details>
						)}
						{block.turn && (role === 'user' && foldUser
							? <details className="agent-brief"><summary>{`${foldUser} · ${plural(block.turn.text.split('\n').length, 'line')}`}</summary><p>{block.turn.text}</p></details>
							: <p>{block.turn.text}</p>)}
					</li>
				);
			})}
		</ol>
	);
}

type TranscriptBlock = { work: ClaudeTurn[]; turn?: ClaudeTurn };

/**
 * Groups a conversation into blocks: each text turn, with the tool calls and
 * thinking that came just before an assistant turn folded into it. Work that
 * no answer follows yet is a block of its own.
 */
export function transcriptBlocks(turns: ClaudeTurn[]): TranscriptBlock[] {
	const blocks: TranscriptBlock[] = [];
	let work: ClaudeTurn[] = [];
	for (const turn of turns) {
		if (turn.role === 'tool' || turn.role === 'thinking') { work.push(turn); continue; }
		if (turn.role === 'assistant') { blocks.push({ work, turn }); work = []; continue; }
		if (work.length > 0) { blocks.push({ work }); work = []; }
		blocks.push({ work: [], turn });
	}
	if (work.length > 0) blocks.push({ work });
	return blocks;
}

function workSummary(work: ClaudeTurn[]): string {
	const tools = work.filter((turn) => turn.role === 'tool').length;
	const thoughts = work.length - tools;
	const parts = [];
	if (thoughts > 0) parts.push(thoughts === 1 ? '1 thought' : `${thoughts} thoughts`);
	if (tools > 0) parts.push(tools === 1 ? '1 tool call' : `${tools} tool calls`);
	return parts.join(' · ');
}

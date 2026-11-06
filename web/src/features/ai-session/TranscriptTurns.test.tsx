import { cleanup, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import type { ClaudeTurn } from '../../lib/types';
import { TranscriptTurns, transcriptBlocks } from './TranscriptTurns';

afterEach(cleanup);

const turns: ClaudeTurn[] = [
	{ role: 'user', text: 'Brief for the planner.' },
	{ role: 'thinking', text: 'Where does the list come from?' },
	{ role: 'tool', text: 'Grep · api/lineitem' },
	{ role: 'tool', text: 'Read · LineItemSearchController.java' },
	{ role: 'assistant', text: 'I am reading the search service.' },
	{ role: 'tool', text: 'Grep · webapp/src' }
];

describe('TranscriptTurns', () => {
	it('folds the work before an answer into it, and leaves unanswered work open', () => {
		expect(transcriptBlocks(turns).map((block) => [block.work.length, block.turn?.role])).toEqual([[0, 'user'], [3, 'assistant'], [1, undefined]]);
		render(<TranscriptTurns turns={turns} label="Planner conversation" names={{ user: 'Harness', assistant: 'Planner' }} />);
		const items = within(screen.getByRole('list', { name: 'Planner conversation' })).getAllByRole('listitem').filter((item) => item.classList.contains('agent-turn'));
		expect(items).toHaveLength(3);
		const answer = items[1];
		const work = answer.querySelector('details');
		expect(work).not.toHaveAttribute('open');
		expect(work?.querySelector('summary')).toHaveTextContent('1 thought · 2 tool calls');
		expect(answer).toHaveTextContent('I am reading the search service.');
		expect(items[2].querySelector('details')).toHaveAttribute('open');
		expect(items[2].querySelector('summary')).toHaveTextContent('1 tool call');
	});
});

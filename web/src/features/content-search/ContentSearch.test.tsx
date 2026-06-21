import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ContentSearchResults } from './ContentSearch';

const result = { id: 'one', workspaceId: 'ws', workspaceName: 'Workspace', path: 'docs/a.md', name: 'a.md', kind: 'markdown' as const, language: 'markdown', lineNumber: 2, columnStart: 3, columnEnd: 9, snippet: 'A needle here', ignored: false };

describe('ContentSearchResults', () => {
	it('highlights matches and supports keyboard opening and clearing', () => {
		const onOpen = vi.fn();
		const onEscape = vi.fn();
		render(<ContentSearchResults query="needle" results={[result]} truncated={false} loading={false} error="" activeIndex={0} onActiveIndex={vi.fn()} onOpen={onOpen} onEscape={onEscape} />);
		expect(screen.getByText('needle', { selector: 'mark' })).toBeInTheDocument();
		const listbox = screen.getByRole('listbox');
		fireEvent.keyDown(listbox, { key: 'Enter' });
		expect(onOpen).toHaveBeenCalledWith(result);
		fireEvent.keyDown(listbox, { key: 'Escape' });
		expect(onEscape).toHaveBeenCalled();
	});

	it('keeps narrow-panel rows compact and limits the visible result count', () => {
		const longSnippet = `${'prefix '.repeat(20)}needle${' suffix'.repeat(20)}`;
		const results = Array.from({ length: 24 }, (_, index) => ({ ...result, id: `result-${index}`, path: `plans/api/TICKET-${index}/implementation-plan.md`, snippet: longSnippet, lineNumber: index + 1 }));
		render(<ContentSearchResults query="needle" results={results} truncated={false} loading={false} error="" activeIndex={0} onActiveIndex={vi.fn()} onOpen={vi.fn()} onEscape={vi.fn()} showWorkspaceContext={false} />);

		expect(screen.getAllByRole('option')).toHaveLength(20);
		expect(screen.getByText('Showing the first 20 of 24 matches. Refine the query to narrow the list.')).toBeInTheDocument();
		expect(screen.queryByText(/columns 3–9/i)).not.toBeInTheDocument();
		expect(screen.getAllByText('api / TICKET-0')[0]).toBeInTheDocument();
		expect(screen.getAllByText('needle', { selector: 'mark' })[0].parentElement?.textContent?.length).toBeLessThanOrEqual(122);
	});
});

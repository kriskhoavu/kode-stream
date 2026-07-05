import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { KnowledgePage } from '../../lib/types';
import { KnowledgeBrowser } from './KnowledgeBrowser';

const pages: KnowledgePage[] = [
	{ slug: 'offer-overview', title: 'Offer Overview', path: 'offer/overview.md', domain: 'offer', pageType: 'CONCEPT', roles: ['BA'], topics: ['workflow'], summary: 'Offer lifecycle', sourceRefs: [], links: [], backlinks: [] },
	{ slug: 'article-import', title: 'Article Import', path: 'master-data/article/import.md', domain: 'master-data/article', pageType: 'HOW_TO', roles: ['DEVELOPER'], topics: ['import'], summary: 'Import products', sourceRefs: [], links: [], backlinks: [] }
];

describe('KnowledgeBrowser', () => {
	it('groups pages, filters metadata, and renders warnings', () => {
		render(<KnowledgeBrowser pages={pages} selectedSlug="offer-overview" warnings={[{ slug: 'offer-overview', code: 'unresolved_link', message: 'Missing target' }]} onSelect={vi.fn()} />);
		expect(screen.getByRole('heading', { name: 'offer' })).toBeInTheDocument();
		expect(screen.getByText('Index diagnostics (1)')).toBeInTheDocument();
		fireEvent.change(screen.getByRole('textbox', { name: 'Filter Knowledge pages' }), { target: { value: 'developer' } });
		expect(screen.queryByRole('button', { name: /Offer Overview/ })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: /Article Import/ })).toBeInTheDocument();
	});

	it('supports keyboard movement and activation', () => {
		const onSelect = vi.fn();
		render(<KnowledgeBrowser pages={pages} warnings={[]} onSelect={onSelect} />);
		const first = screen.getByRole('button', { name: /Offer Overview/ });
		const second = screen.getByRole('button', { name: /Article Import/ });
		first.focus(); fireEvent.keyDown(first, { key: 'ArrowDown' }); expect(second).toHaveFocus();
		fireEvent.keyDown(second, { key: 'Enter' }); expect(onSelect).toHaveBeenCalledWith('article-import');
	});

	it('explains an empty valid Wiki', () => {
		render(<KnowledgeBrowser pages={[]} warnings={[]} onSelect={vi.fn()} />);
		expect(screen.getByRole('heading', { name: 'No valid pages indexed' })).toBeInTheDocument();
	});
});

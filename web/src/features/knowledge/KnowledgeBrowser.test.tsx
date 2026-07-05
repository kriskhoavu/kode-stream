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

	it('promotes a domain index page into an interactive parent', () => {
		const onSelect = vi.fn();
		const domainPages: KnowledgePage[] = [
			{ ...pages[0], slug: 'a12-index', title: 'A12 Documentation', path: 'a12/README.md', domain: 'A12' },
			{ ...pages[1], slug: 'a12-analysis', title: 'A12 Architecture Analysis', path: 'a12/architecture.md', domain: 'A12' }
		];
		render(<KnowledgeBrowser pages={domainPages} warnings={[]} onSelect={onSelect} />);

		fireEvent.click(screen.getByRole('button', { name: 'Open A12 index' }));
		expect(onSelect).toHaveBeenCalledWith('a12-index');
		expect(screen.queryByRole('button', { name: /A12 Documentation/ })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: /A12 Architecture Analysis/ })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Open A12 index' }).querySelector('.lucide-book-marked')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Open A12 index' }).querySelector('.lucide-chevron-right')).not.toBeInTheDocument();
	});

	it('renders slash-separated domains as a nested hierarchy with simple page type text', () => {
		const nestedPages: KnowledgePage[] = [
			{ ...pages[0], slug: 'master-data-index', path: 'master-data/index.md', domain: 'master-data' },
			{ ...pages[1], slug: 'article-reference', path: 'master-data/article/reference.md', domain: 'master-data/article', pageType: 'REFERENCE' },
			{ ...pages[1], slug: 'customer-how-to', path: 'master-data/customer/import.md', domain: 'master-data/customer', pageType: 'HOW_TO' }
		];
		render(<KnowledgeBrowser pages={nestedPages} warnings={[]} onSelect={vi.fn()} />);

		const parent = screen.getByRole('heading', { name: 'master-data' }).closest('.knowledge-domain');
		expect(parent).toContainElement(screen.getByRole('heading', { name: 'article' }));
		expect(parent).toContainElement(screen.getByRole('heading', { name: 'customer' }));
		expect(screen.getByText('REFERENCE')).toHaveClass('knowledge-page-type');
		expect(screen.getByText('REFERENCE')).not.toHaveClass('knowledge-type-badge');
		expect(screen.getByText('HOW-TO')).toHaveClass('knowledge-page-type');
	});

	it('keeps the selected page title readable without inheriting the accent color', () => {
		render(<KnowledgeBrowser pages={pages} selectedSlug="offer-overview" warnings={[]} onSelect={vi.fn()} />);
		const selected = screen.getByRole('button', { name: /Offer Overview/ });
		expect(selected).toHaveClass('active');
		expect(screen.getByText('Offer Overview')).toHaveClass('knowledge-page-title');
	});

	it('collapses and expands domains independently from opening their landing page', () => {
		const onSelect = vi.fn();
		const domainPages: KnowledgePage[] = [
			{ ...pages[0], slug: 'a12-index', path: 'a12/README.md', domain: 'A12' },
			{ ...pages[1], slug: 'a12-analysis', title: 'A12 Architecture Analysis', path: 'a12/architecture.md', domain: 'A12' }
		];
		render(<KnowledgeBrowser pages={domainPages} warnings={[]} onSelect={onSelect} />);

		const collapse = screen.getByRole('button', { name: 'Collapse A12' });
		expect(collapse).toHaveAttribute('aria-expanded', 'true');
		fireEvent.click(collapse);
		expect(screen.queryByRole('button', { name: /A12 Architecture Analysis/ })).not.toBeInTheDocument();
		expect(onSelect).not.toHaveBeenCalled();
		fireEvent.change(screen.getByRole('textbox', { name: 'Filter Knowledge pages' }), { target: { value: 'architecture' } });
		expect(screen.getByRole('button', { name: /A12 Architecture Analysis/ })).toBeInTheDocument();
		fireEvent.change(screen.getByRole('textbox', { name: 'Filter Knowledge pages' }), { target: { value: '' } });

		fireEvent.click(screen.getByRole('button', { name: 'Expand A12' }));
		expect(screen.getByRole('button', { name: /A12 Architecture Analysis/ })).toBeInTheDocument();
	});

	it('explains an empty valid Wiki', () => {
		render(<KnowledgeBrowser pages={[]} warnings={[]} onSelect={vi.fn()} />);
		expect(screen.getByRole('heading', { name: 'No valid pages indexed' })).toBeInTheDocument();
	});
});

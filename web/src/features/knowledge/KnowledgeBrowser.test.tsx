import { fireEvent, render, screen, waitFor } from '@testing-library/react';
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
		const rootPages = pages.map((page) => ({ ...page, domain: '' }));
		render(<KnowledgeBrowser pages={rootPages} warnings={[]} onSelect={onSelect} />);
		const first = screen.getByRole('button', { name: /Offer Overview/ });
		const second = screen.getByRole('button', { name: /Article Import/ });
		first.focus(); fireEvent.keyDown(first, { key: 'ArrowDown' }); expect(second).toHaveFocus();
		fireEvent.keyDown(second, { key: 'Enter' }); expect(onSelect).toHaveBeenCalledWith('article-import');
	});

	it('always places the root domain before other top-level domains', () => {
		const unorderedPages: KnowledgePage[] = [
			...pages,
			{ ...pages[0], slug: 'root-page', title: 'Root Page', path: 'root-page.md', domain: '' }
		];
		render(<KnowledgeBrowser pages={unorderedPages} warnings={[]} onSelect={vi.fn()} />);

		expect(screen.getAllByRole('heading', { level: 3 })[0]).toHaveTextContent('root');
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
		const childPage = screen.getByRole('button', { name: /A12 Architecture Analysis/ });
		expect(childPage).toBeInTheDocument();
		const landingControl = screen.getByRole('button', { name: 'Open A12 index' });
		expect(landingControl.firstElementChild).toHaveClass('lucide-library');
		expect(landingControl.lastElementChild).toHaveTextContent('A12');
		expect(screen.getByRole('button', { name: 'Open A12 index' }).querySelector('.lucide-library')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Open A12 index' }).querySelector('.lucide-chevron-right')).not.toBeInTheDocument();
	});

	it('renders slash-separated domains as a nested hierarchy with simple page type text', () => {
		const nestedPages: KnowledgePage[] = [
			{ ...pages[0], slug: 'master-data-index', path: 'master-data/index.md', domain: 'master-data' },
			{ ...pages[1], slug: 'article-reference', path: 'master-data/article/reference.md', domain: 'master-data/article', pageType: 'REFERENCE' },
			{ ...pages[1], slug: 'customer-how-to', path: 'master-data/customer/import.md', domain: 'master-data/customer', pageType: 'HOW_TO' }
		];
		render(<KnowledgeBrowser pages={nestedPages} warnings={[]} onSelect={vi.fn()} />);

		fireEvent.change(screen.getByRole('textbox', { name: 'Filter Knowledge pages' }), { target: { value: 'developer' } });
		const parent = screen.getByRole('heading', { name: 'master-data' }).closest('.knowledge-domain');
		expect(parent).toContainElement(screen.getByRole('heading', { name: 'article' }));
		expect(parent).toContainElement(screen.getByRole('heading', { name: 'customer' }));
		expect(screen.getByText('REFERENCE')).toHaveClass('knowledge-page-type');
		expect(screen.getByText('REFERENCE')).not.toHaveClass('knowledge-type-badge');
		expect(screen.getByText('HOW-TO')).toHaveClass('knowledge-page-type');
	});

	it('keeps the selected page title readable without inheriting the accent color', () => {
		const rootPages = pages.map((page) => ({ ...page, domain: '' }));
		render(<KnowledgeBrowser pages={rootPages} selectedSlug="offer-overview" warnings={[]} onSelect={vi.fn()} />);
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

		const expand = screen.getByRole('button', { name: 'Expand A12' });
		expect(expand).toHaveAttribute('aria-expanded', 'false');
		expect(screen.queryByRole('button', { name: /A12 Architecture Analysis/ })).not.toBeInTheDocument();
		fireEvent.click(expand);
		expect(screen.getByRole('button', { name: /A12 Architecture Analysis/ })).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Collapse A12' }));
		expect(onSelect).not.toHaveBeenCalled();
		fireEvent.change(screen.getByRole('textbox', { name: 'Filter Knowledge pages' }), { target: { value: 'architecture' } });
		expect(screen.getByRole('button', { name: /A12 Architecture Analysis/ })).toBeInTheDocument();
		fireEvent.change(screen.getByRole('textbox', { name: 'Filter Knowledge pages' }), { target: { value: '' } });

		fireEvent.click(screen.getByRole('button', { name: 'Expand A12' }));
		expect(screen.getByRole('button', { name: /A12 Architecture Analysis/ })).toBeInTheDocument();
	});

	it('toggles an already selected landing page folder from its title', () => {
		const domainPages: KnowledgePage[] = [
			{ ...pages[0], slug: 'a12-index', path: 'a12/README.md', domain: 'A12' },
			{ ...pages[1], slug: 'a12-analysis', title: 'A12 Architecture Analysis', path: 'a12/architecture.md', domain: 'A12' }
		];
		render(<KnowledgeBrowser pages={domainPages} selectedSlug="a12-index" warnings={[]} onSelect={vi.fn()} />);

		expect(screen.getByRole('button', { name: /A12 Architecture Analysis/ })).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Open A12 index' }));
		expect(screen.queryByRole('button', { name: /A12 Architecture Analysis/ })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Expand A12' })).toHaveAttribute('aria-expanded', 'false');
	});

	it('provides a left-border resize handle for the Knowledge Quality side panel', () => {
		render(<KnowledgeBrowser pages={pages} warnings={[]} onSelect={vi.fn()} sidePanel={<div>Quality content</div>} />);
		expect(screen.getByRole('button', { name: 'Resize Knowledge Quality panel' })).toHaveClass('panel-resize-handle-right');
	});

	it('opens only the root domain by default', () => {
		const domainPages: KnowledgePage[] = [
			{ ...pages[0], slug: 'root-page', title: 'Root Page', path: 'root.md', domain: '' },
			{ ...pages[1], slug: 'article-page', title: 'Article Page', path: 'article/page.md', domain: 'article' }
		];
		render(<KnowledgeBrowser pages={domainPages} warnings={[]} onSelect={vi.fn()} />);

		expect(screen.getByRole('button', { name: /Root Page/ })).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /Article Page/ })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Collapse root' })).toHaveAttribute('aria-expanded', 'true');
		expect(screen.getByRole('button', { name: 'Expand article' })).toHaveAttribute('aria-expanded', 'false');
		expect(screen.getByRole('heading', { name: 'root' }).querySelector('.lucide-library')).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'article' }).querySelector('.lucide-library')).toBeInTheDocument();
	});

	it('expands and focuses a referenced page selected outside the tree', async () => {
		const nestedPages: KnowledgePage[] = [
			{ ...pages[0], slug: 'root-page', path: 'root.md', domain: '' },
			{ ...pages[1], slug: 'article-reference', title: 'Article Reference', path: 'master-data/article/reference.md', domain: 'master-data/article' }
		];
		const view = render(<KnowledgeBrowser pages={nestedPages} warnings={[]} onSelect={vi.fn()} />);
		fireEvent.change(screen.getByRole('textbox', { name: 'Filter Knowledge pages' }), { target: { value: 'root' } });

		view.rerender(<KnowledgeBrowser pages={nestedPages} selectedSlug="article-reference" warnings={[]} onSelect={vi.fn()} />);

		await waitFor(() => expect(screen.getByRole('button', { name: /Article Reference/ })).toHaveFocus());
		expect(screen.getByRole('button', { name: 'Collapse master-data' })).toHaveAttribute('aria-expanded', 'true');
		expect(screen.getByRole('button', { name: 'Collapse master-data/article' })).toHaveAttribute('aria-expanded', 'true');
		expect(screen.getByRole('textbox', { name: 'Filter Knowledge pages' })).toHaveValue('');
	});

	it('explains an empty valid Wiki', () => {
		render(<KnowledgeBrowser pages={[]} warnings={[]} onSelect={vi.fn()} />);
		expect(screen.getByRole('heading', { name: 'No valid pages indexed' })).toBeInTheDocument();
	});
});

describe('taxonomy grouping', () => {
	const taxonomyPages: KnowledgePage[] = [
		{ slug: 'wiki-index', title: 'Documentation Index', path: 'index.md', domain: 'root', bucket: '', area: '', tier: '', pageType: 'REFERENCE', roles: [], topics: [], sourceRefs: [], links: [], backlinks: [] },
		{ slug: 'offer-index', title: 'Offer Documentation', path: 'domains/offer/README.md', domain: 'domains/offer', bucket: 'domains', area: 'offer', tier: '', pageType: 'CONCEPT', roles: [], topics: [], sourceRefs: [], links: [], backlinks: [] },
		{ slug: 'offer-approval', title: 'Offer Approval', path: 'domains/offer/concepts/approval.md', domain: 'domains/offer/concepts', bucket: 'domains', area: 'offer', tier: 'concepts', pageType: 'HOW_TO', roles: [], topics: [], sourceRefs: [], links: [], backlinks: [] },
		{ slug: 'offer-permissions', title: 'Offer Permissions', path: 'domains/offer/reference/permissions.md', domain: 'domains/offer/reference', bucket: 'domains', area: 'offer', tier: 'reference', pageType: 'REFERENCE', roles: [], topics: [], sourceRefs: [], links: [], backlinks: [] },
		{ slug: 'article-spec', title: 'Article Fields', path: 'domains/master-data/article/reference/spec.md', domain: 'domains/master-data/article/reference', bucket: 'domains', area: 'master-data/article', tier: 'reference', pageType: 'REFERENCE', roles: [], topics: [], sourceRefs: [], links: [], backlinks: [] },
		{ slug: 'rollback', title: 'Deployment Rollback', path: 'platform/concepts/rollback.md', domain: 'platform/concepts', bucket: 'platform', area: '', tier: 'concepts', pageType: 'CONCEPT', roles: [], topics: [], sourceRefs: [], links: [], backlinks: [] }
	];

	// The reported defect: a tier directory has no README anywhere in the corpus,
	// so it rendered as a non-interactive label with only a chevron to open it.
	it('reaches a tier page without the tier being a folder', () => {
		render(<KnowledgeBrowser pages={taxonomyPages} warnings={[]} onSelect={vi.fn()} />);

		fireEvent.click(screen.getByRole('button', { name: /^domains$/i }));
		fireEvent.click(screen.getByRole('button', { name: 'Open domains/offer index' }));

		expect(screen.getByRole('button', { name: /Offer Approval/i })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: /Offer Permissions/i })).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /^concepts$/i })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /^reference$/i })).not.toBeInTheDocument();
	});

	it('labels tier partitions inside their area', () => {
		render(<KnowledgeBrowser pages={taxonomyPages} warnings={[]} onSelect={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: /^domains$/i }));
		fireEvent.click(screen.getByRole('button', { name: 'Open domains/offer index' }));

		const tiers = screen.getAllByTestId('knowledge-tier-label').map((element) => element.textContent);
		expect(tiers).toEqual(expect.arrayContaining(['concepts', 'reference']));
	});

	it('shows a nested area as one row', () => {
		render(<KnowledgeBrowser pages={taxonomyPages} warnings={[]} onSelect={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: /^domains$/i }));

		expect(screen.getByRole('button', { name: /master-data \/ article/i })).toBeInTheDocument();
	});

	it('toggles a section from its header even with no landing page', () => {
		render(<KnowledgeBrowser pages={taxonomyPages} warnings={[]} onSelect={vi.fn()} />);

		const platform = screen.getByRole('button', { name: /^platform$/i });
		expect(platform).toHaveAttribute('aria-expanded', 'false');
		fireEvent.click(platform);
		expect(screen.getByRole('button', { name: /^platform$/i })).toHaveAttribute('aria-expanded', 'true');
		expect(screen.getByRole('button', { name: /Deployment Rollback/i })).toBeInTheDocument();
	});

	it('expands the bucket and area of a selected deep page', () => {
		render(<KnowledgeBrowser pages={taxonomyPages} selectedSlug="article-spec" warnings={[]} onSelect={vi.fn()} />);

		expect(screen.getByRole('button', { name: /Article Fields/i })).toBeInTheDocument();
	});
});

describe('page type badges', () => {
	const typedPages: KnowledgePage[] = [
		{ slug: 'how-to', title: 'Offer Approval', path: 'domains/offer/concepts/approval.md', domain: 'domains/offer/concepts', bucket: 'domains', area: 'offer', tier: 'concepts', pageType: 'HOW_TO', roles: [], topics: [], sourceRefs: [], links: [], backlinks: [] },
		{ slug: 'concept', title: 'Offer Overview', path: 'domains/offer/concepts/overview.md', domain: 'domains/offer/concepts', bucket: 'domains', area: 'offer', tier: 'concepts', pageType: 'CONCEPT', roles: [], topics: [], sourceRefs: [], links: [], backlinks: [] },
		{ slug: 'ref', title: 'Offer Permissions', path: 'domains/offer/reference/perms.md', domain: 'domains/offer/reference', bucket: 'domains', area: 'offer', tier: 'reference', pageType: 'REFERENCE', roles: [], topics: [], sourceRefs: [], links: [], backlinks: [] },
		{ slug: 'decision', title: 'Offer Decision', path: 'domains/offer/concepts/decision.md', domain: 'domains/offer/concepts', bucket: 'domains', area: 'offer', tier: 'concepts', pageType: 'DECISION', roles: [], topics: [], sourceRefs: [], links: [], backlinks: [] }
	];

	const badgeFor = (title: string) =>
		screen.getByRole('button', { name: new RegExp(title, 'i') }).querySelector('.knowledge-page-type');

	// Page type is what varies inside a tier, so it carries the colour. Tier
	// labels stay muted, keeping colour meaning one thing per axis.
	it('marks the page types that vary within a group', () => {
		render(<KnowledgeBrowser pages={typedPages} warnings={[]} onSelect={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: /^domains$/i }));
		fireEvent.click(screen.getByRole('button', { name: /^offer$/i }));

		expect(badgeFor('Offer Approval')).toHaveClass('page-type-how-to');
		expect(badgeFor('Offer Permissions')).toHaveClass('page-type-reference');
	});

	it('leaves the default and rare page types neutral', () => {
		render(<KnowledgeBrowser pages={typedPages} warnings={[]} onSelect={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: /^domains$/i }));
		fireEvent.click(screen.getByRole('button', { name: /^offer$/i }));

		expect(badgeFor('Offer Overview')?.className).toBe('knowledge-page-type');
		expect(badgeFor('Offer Decision')?.className).toBe('knowledge-page-type');
	});
});

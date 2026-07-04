import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../lib/api';
import { KnowledgePage } from './KnowledgePage';

vi.mock('../lib/api', () => ({
	api: {
		knowledgeWikis: vi.fn(),
		knowledgePages: vi.fn(),
		knowledgePage: vi.fn(),
		knowledgeGraph: vi.fn(),
		rescanKnowledge: vi.fn(),
		syncKnowledge: vi.fn(),
		enrichKnowledge: vi.fn(),
		gitStatus: vi.fn()
	}
}));

const workspace = { id: 'discovery', name: 'Discovery', path: '/repo', baselineBranch: 'main', sources: ['docs'], createdAt: '' };
const page = { slug: 'overview', title: 'Offer Overview', path: 'offer/overview.md', domain: 'offer', pageType: 'CONCEPT', roles: ['BA'], topics: ['workflow'], summary: 'Offer lifecycle.', sourceRefs: [], links: [], backlinks: [] };

describe('KnowledgePage layout', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.knowledgeWikis).mockResolvedValue([{ workspaceId: 'discovery', root: 'docs', displayName: 'Docs', pages: [], warnings: [], indexedAt: '' }]);
		vi.mocked(api.knowledgePages).mockResolvedValue({ pages: [page], warnings: Array.from({ length: 26 }, (_, index) => ({ path: `page-${index}.md`, code: 'invalid_front_matter', message: 'Missing YAML front matter' })) });
	});

	it('uses application controls, removes empty status space, and collapses large warning lists', async () => {
		const { container } = render(<KnowledgePage workspaces={[workspace]} location={{ workspaceId: 'discovery', root: 'docs', slug: 'overview', view: 'browse' }} onLocationChange={vi.fn()} onOpenExplorer={vi.fn()} />);

		await waitFor(() => expect(screen.getByRole('heading', { name: 'Offer Overview' })).toBeInTheDocument());
		expect(screen.getByRole('button', { name: 'Rescan' })).toHaveClass('secondary');
		expect(screen.getByRole('button', { name: 'Sync' })).toHaveClass('secondary');
		expect(screen.getByRole('button', { name: 'Browse' })).toHaveClass('active');
		expect(screen.getByRole('button', { name: 'Read page' })).toHaveClass('primary');
		expect(container.querySelector('.knowledge-status')).not.toBeInTheDocument();
		const warningDetails = screen.getByText('26 warnings').closest('details');
		expect(warningDetails).not.toHaveAttribute('open');
		expect(screen.getByText('Filter Knowledge pages')).toHaveClass('knowledge-visually-hidden');
	});
});

import { fireEvent, render, screen, waitFor } from '@testing-library/react';
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
const page = { slug: 'overview', title: 'Offer Overview', path: 'overview.md', domain: 'root', pageType: 'CONCEPT', roles: ['BA'], topics: ['workflow'], summary: 'Offer lifecycle.', sourceRefs: [], links: [], backlinks: [] };

describe('KnowledgePage layout', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.knowledgeWikis).mockResolvedValue([{ workspaceId: 'discovery', root: 'docs', displayName: 'Docs', pages: [], warnings: [], indexedAt: '' }]);
		vi.mocked(api.knowledgePages).mockResolvedValue({ pages: [page], warnings: Array.from({ length: 26 }, (_, index) => ({ path: `page-${index}.md`, code: 'invalid_front_matter', message: 'Missing YAML front matter' })) });
		vi.mocked(api.knowledgePage).mockResolvedValue({ ...page, warnings: [], content: { id: 'overview', path: 'offer/overview.md', content: '# Offer Overview\n\nFull document content.', language: 'markdown', hash: 'hash', kind: 'markdown', sizeBytes: 48, editable: false } });
	});

	it('uses application controls, removes empty status space, and collapses large warning lists', async () => {
		const { container } = render(<KnowledgePage workspaces={[workspace]} location={{ workspaceId: 'discovery', root: 'docs', slug: 'overview', view: 'browse' }} onLocationChange={vi.fn()} />);

		await waitFor(() => expect(screen.getByRole('button', { name: /Offer Overview/ })).toBeInTheDocument());
		expect(screen.getByRole('button', { name: 'Sync' })).toHaveClass('secondary');
		expect(screen.getByRole('button', { name: 'Pull' })).toHaveClass('secondary');
		expect(screen.getByRole('button', { name: 'Pages' })).toHaveClass('active');
		expect(container.querySelector('.knowledge-status')).not.toBeInTheDocument();
		const warningDetails = screen.getByText('Index diagnostics (26)').closest('details');
		expect(warningDetails).not.toHaveAttribute('open');
		expect(screen.getByText(/not errors in the selected page/i)).toBeInTheDocument();
		expect(screen.getByText('Filter Knowledge pages')).toHaveClass('knowledge-visually-hidden');
	});

	it('opens full page content with one click and keeps the index visible', async () => {
		const onLocationChange = vi.fn();
		const first = render(<KnowledgePage workspaces={[workspace]} location={{ workspaceId: 'discovery', root: 'docs', view: 'browse' }} onLocationChange={onLocationChange} />);
		const row = await screen.findByRole('button', { name: /Offer Overview/ });
		fireEvent.click(row);
		expect(onLocationChange).toHaveBeenLastCalledWith({ workspaceId: 'discovery', root: 'docs', slug: 'overview', view: 'read' });

		first.rerender(<KnowledgePage workspaces={[workspace]} location={{ workspaceId: 'discovery', root: 'docs', slug: 'overview', view: 'read' }} onLocationChange={onLocationChange} />);
		await waitFor(() => expect(api.knowledgePage).toHaveBeenCalledWith('discovery', 'docs', 'overview'));
		expect(await screen.findByText('Full document content.', {}, { timeout: 5_000 })).toBeInTheDocument();
		expect(screen.getByRole('navigation', { name: 'Knowledge pages' })).toBeInTheDocument();
		expect(screen.queryByText('Loading page…')).not.toBeInTheDocument();
	});
});

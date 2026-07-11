import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { KnowledgePageDetail } from '../../lib/types';
import { KnowledgeReader, prepareKnowledgeMarkdown } from './KnowledgeReader';

const detail: KnowledgePageDetail = {
	slug: 'overview', title: 'Overview', path: 'overview.md', domain: 'offer', pageType: 'CONCEPT', roles: ['BA'], topics: ['workflow'], summary: 'Summary', sourceRefs: ['plans/api/PM-1/README.md'], sourceCount: 1,
	links: [
		{ sourceSlug: 'overview', rawTarget: 'target', label: 'Target page', targetSlug: 'target', resolution: 'resolved' },
		{ sourceSlug: 'overview', rawTarget: 'missing', resolution: 'unresolved' }
	], backlinks: ['source'], warnings: [{ slug: 'overview', code: 'unresolved_link', message: 'Missing target' }],
	content: { id: 'overview', path: 'overview.md', content: '---\nslug: overview\ntitle: Overview\n---\n# Overview\n[[target|Target page]]\n\n[External](https://example.test)', language: 'markdown', hash: 'hash', kind: 'markdown', sizeBytes: 100, editable: false }
};

describe('KnowledgeReader', () => {
	it('renders shared Markdown, metadata, and relationships', async () => {
		const onNavigate = vi.fn();
		render(<KnowledgeReader detail={detail} onNavigate={onNavigate} />);
		expect(await screen.findByRole('heading', { name: 'Overview', level: 1 }, { timeout: 3000 })).toBeInTheDocument();
		expect(screen.getByText('plans/api/PM-1/README.md')).toBeInTheDocument();
		expect(screen.getByText('Missing target')).toBeInTheDocument();
		fireEvent.click(screen.getAllByRole('button', { name: 'Target page' })[0]);
		expect(onNavigate).toHaveBeenCalledWith('target');
	});

	it('intercepts rendered Wiki links and keeps external links safe', async () => {
		const onNavigate = vi.fn();
		render(<KnowledgeReader detail={detail} onNavigate={onNavigate} />);
		const internal = await screen.findByRole('link', { name: 'Target page' });
		fireEvent.click(internal);
		expect(onNavigate).toHaveBeenCalledWith('target');
		const external = screen.getByRole('link', { name: 'External' });
		expect(external).toHaveAttribute('target', '_blank');
		expect(external).toHaveAttribute('rel', expect.stringContaining('noopener'));
	});

	it('does not convert Wiki syntax inside code', async () => {
		const markdown = prepareKnowledgeMarkdown('---\nslug: x\ntitle: X\n---\n`[[inline]]`\n```\n[[fenced]]\n```\n[[page]]');
		expect(markdown).toContain('`[[inline]]`');
		expect(markdown).toContain('[[fenced]]');
		expect(markdown).toContain('[page](./__knowledge__/page)');
	});
});

import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ItemWorkspacePage } from './ItemWorkspacePage';

const apiMock = vi.hoisted(() => ({
	item: vi.fn(), files: vi.fn(), file: vi.fn(), diff: vi.fn(), gitStatus: vi.fn(), workspaceBranches: vi.fn(), switchBranch: vi.fn()
}));

vi.mock('../lib/api', () => ({
	api: apiMock,
	statusLabels: {},
	ApiError: class ApiError extends Error { recoveryHint?: string }
}));

vi.mock('./WorkstreamExplorer', () => ({
	WorkstreamExplorer: () => <div data-testid="embedded-explorer" />
}));

describe('ItemWorkspacePage embedded explorer', () => {
	beforeEach(() => {
		Object.values(apiMock).forEach((mock) => mock.mockReset());
		apiMock.item.mockResolvedValue({ id: 'item-1', workspaceId: 'ws', workspaceName: 'Workspace', title: 'Item', scope: 'platform', identifier: 'PM-009', branch: 'main', status: 'draft', tags: [], metadataSource: 'plan.yaml', documents: [], metadata: {}, counts: { files: 1 } });
		apiMock.files.mockResolvedValue([]);
		apiMock.diff.mockResolvedValue({ diff: '' });
		apiMock.gitStatus.mockResolvedValue({ workspaceId: 'ws', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] });
		apiMock.workspaceBranches.mockResolvedValue({ workspaceId: 'ws', current: 'main', branches: ['feature/item', 'main'] });
		apiMock.file.mockResolvedValue({ id: 'README_md', path: 'README.md', content: '# Match', language: 'markdown', hash: 'hash', kind: 'markdown', sizeBytes: 7, editable: true });
	});

	it('renders the merged explorer surface for item details', async () => {
		render(<ItemWorkspacePage itemId="item-1" refreshKey={0} workspaces={[{ id: 'ws', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '2026-07-10T00:00:00Z' }]} onBack={vi.fn()} onOpenItem={vi.fn()} />);
		expect(await screen.findByTestId('embedded-explorer')).toBeInTheDocument();
	});
});

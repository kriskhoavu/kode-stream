import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import type { KnowledgeWiki, WorkspaceConfig } from '../../lib/types';
import { useKnowledgeController } from './useKnowledgeController';

vi.mock('../../lib/api', () => ({ api: { knowledgeWikis: vi.fn(), knowledgePages: vi.fn(), rescanKnowledge: vi.fn(), syncKnowledge: vi.fn(), enrichKnowledge: vi.fn() } }));

const workspaces: WorkspaceConfig[] = [
	{ id: 'one', name: 'One', path: '/one', baselineBranch: 'main', sources: ['docs'], createdAt: '' },
	{ id: 'two', name: 'Two', path: '/two', baselineBranch: 'main', sources: ['wiki'], createdAt: '' }
];
const wiki = (workspaceId: string, root: string): KnowledgeWiki => ({ workspaceId, root, displayName: root, pages: [], warnings: [], indexedAt: '' });

describe('useKnowledgeController', () => {
	beforeEach(() => { vi.clearAllMocks(); vi.mocked(api.knowledgePages).mockResolvedValue({ pages: [], warnings: [] }); });

	it('falls back to the first workspace and Wiki and synchronizes the route', async () => {
		vi.mocked(api.knowledgeWikis).mockResolvedValue([wiki('one', 'docs')]);
		const onLocationChange = vi.fn();
		const { result } = renderHook(() => useKnowledgeController(workspaces, undefined, onLocationChange));
		await waitFor(() => expect(result.current.loading).toBe(false));
		expect(api.knowledgePages).toHaveBeenCalledWith('one', 'docs');
		expect(onLocationChange).toHaveBeenCalledWith({ workspaceId: 'one', root: 'docs', view: 'browse' });
	});

	it('ignores a stale Wiki response after the workspace changes', async () => {
		let resolveFirst!: (value: KnowledgeWiki[]) => void;
		vi.mocked(api.knowledgeWikis).mockImplementation((workspaceId) => workspaceId === 'one' ? new Promise((resolve) => { resolveFirst = resolve; }) : Promise.resolve([wiki('two', 'wiki')]));
		const onLocationChange = vi.fn();
		const { result, rerender } = renderHook(({ workspaceId }) => useKnowledgeController(workspaces, { workspaceId }, onLocationChange), { initialProps: { workspaceId: 'one' } });
		rerender({ workspaceId: 'two' });
		await waitFor(() => expect(api.knowledgePages).toHaveBeenCalledWith('two', 'wiki'));
		await act(async () => resolveFirst([wiki('one', 'docs')]));
		expect(result.current.wikis.map((item) => item.root)).toEqual(['wiki']);
	});

	it('invalidates data after a successful action', async () => {
		vi.mocked(api.knowledgeWikis).mockResolvedValue([wiki('one', 'docs')]);
		vi.mocked(api.rescanKnowledge).mockResolvedValue({ ok: true, operation: 'rescan', wikis: [], warnings: [], logTruncated: false, completedAt: '' });
		const { result } = renderHook(() => useKnowledgeController(workspaces, { workspaceId: 'one', root: 'docs', view: 'browse' }, vi.fn()));
		await waitFor(() => expect(result.current.loading).toBe(false));
		await act(async () => { await result.current.runAction('rescan'); });
		expect(api.knowledgeWikis).toHaveBeenCalledTimes(2);
		expect(result.current.actionResult?.operation).toBe('rescan');
	});
});

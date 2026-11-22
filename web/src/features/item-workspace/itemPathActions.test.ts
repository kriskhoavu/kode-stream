import { beforeEach, describe, expect, it, vi } from 'vitest';

const apiMock = vi.hoisted(() => ({ createWorkspaceDirectory: vi.fn(), createWorkspaceFile: vi.fn(), renameWorkspacePath: vi.fn() }));
vi.mock('../../shared/api', () => ({ api: apiMock }));

import { createItemPath, relativePathParts, renamedPath } from './itemPathActions';

describe('item path actions', () => {
	beforeEach(() => Object.values(apiMock).forEach((mock) => mock.mockReset()));

	it('accepts a relative path and refuses empty, "." and ".." segments', () => {
		expect(relativePathParts(' docs/notes.md ')).toEqual(['docs', 'notes.md']);
		expect(relativePathParts('docs//notes.md')).toBeNull();
		expect(relativePathParts('../notes.md')).toBeNull();
		expect(relativePathParts('./notes.md')).toBeNull();
	});

	it('renames within the same folder and refuses a path or a dot name', () => {
		expect(renamedPath('docs/old.md', ' new.md ')).toBe('docs/new.md');
		expect(renamedPath('old.md', 'new.md')).toBe('new.md');
		expect(renamedPath('docs/old.md', 'a/b.md')).toBeNull();
		expect(renamedPath('docs/old.md', '..')).toBeNull();
	});

	it('creates only the missing folders before the file', async () => {
		const existing = new Set(['docs']);
		await createItemPath({ workspaceId: 'ws', itemPath: 'plans/PM-1' }, 'file', ['docs', 'drafts', 'a.md'], '', existing);
		expect(apiMock.createWorkspaceDirectory.mock.calls).toEqual([['ws', { parentPath: 'plans/PM-1/docs', name: 'drafts' }]]);
		expect(apiMock.createWorkspaceFile).toHaveBeenCalledWith('ws', { parentPath: 'plans/PM-1/docs/drafts', name: 'a.md', content: '' });
		expect(existing.has('docs/drafts')).toBe(true);
	});

	it('creates every folder of a directory path below the selected folder', async () => {
		await createItemPath({ workspaceId: 'ws', itemPath: 'plans/PM-1' }, 'directory', ['x', 'y'], 'docs', new Set());
		expect(apiMock.createWorkspaceDirectory.mock.calls).toEqual([
			['ws', { parentPath: 'plans/PM-1/docs', name: 'x' }],
			['ws', { parentPath: 'plans/PM-1/docs/x', name: 'y' }],
		]);
		expect(apiMock.createWorkspaceFile).not.toHaveBeenCalled();
	});
});

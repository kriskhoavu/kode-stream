import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { WorkspaceExplorerPage } from './WorkspaceExplorerPage';

const apiMock = vi.hoisted(() => ({
  items: vi.fn(), workspaceTree: vi.fn(), workspaceFile: vi.fn(), workspaceFileDiff: vi.fn(),
  saveWorkspaceFile: vi.fn(), revertWorkspaceFile: vi.fn(), openPath: vi.fn(), gitStatus: vi.fn(), workspaceHealth: vi.fn(),
  searchWorkspacePaths: vi.fn(), workspacePathGitStates: vi.fn(), createWorkspaceFile: vi.fn(), createWorkspaceDirectory: vi.fn(), renameWorkspacePath: vi.fn()
}));

vi.mock('../lib/api', () => ({
  api: apiMock,
  ApiError: class ApiError extends Error { recoveryHint?: string }
}));

const workspace = { id: 'ws', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: [], createdAt: '' };

describe('WorkspaceExplorerPage', () => {
  beforeEach(() => {
    localStorage.clear();
    Object.values(apiMock).forEach((mock) => mock.mockReset());
    apiMock.items.mockResolvedValue([]);
    apiMock.workspaceTree.mockResolvedValue({ workspaceId: 'ws', path: '', hiddenCount: 0, entries: [
      { id: 'readme', name: 'README.md', path: 'README.md', type: 'file', hasChildren: false, ignored: false, hidden: false, editable: true, kind: 'markdown' }
    ] });
    apiMock.gitStatus.mockResolvedValue({ workspaceId: 'ws', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] });
    apiMock.workspaceHealth.mockResolvedValue({ workspaceId: 'ws', checkedAt: '', summary: 'ok', checks: [] });
    apiMock.workspacePathGitStates.mockResolvedValue([]);
    apiMock.searchWorkspacePaths.mockResolvedValue({ results: [], truncated: false });
  });

  it('loads one directory when a workspace root expands', async () => {
		const { container } = render(<WorkspaceExplorerPage workspaces={[workspace]} location={{ mode: 'all' }} onLocationChange={vi.fn()} onOpenKanban={vi.fn()} />);
    fireEvent.click(container.querySelector('.explorer-row-toggle') as HTMLButtonElement);
    await waitFor(() => expect(apiMock.workspaceTree).toHaveBeenCalledWith('ws', '', false));
    expect(await screen.findByText('README.md')).toBeInTheDocument();
  });

  it('keeps Open Kanban explicit for a selected workspace root', async () => {
    const onOpenKanban = vi.fn();
    render(<WorkspaceExplorerPage workspaces={[workspace]} location={{ workspaceId: 'ws' }} onLocationChange={vi.fn()} onOpenKanban={onOpenKanban} />);
    fireEvent.click(await screen.findByRole('button', { name: /Open Kanban/i }));
    expect(onOpenKanban).toHaveBeenCalledWith(workspace);
  });

  it('searches unloaded paths and opens a result', async () => {
    const onLocationChange = vi.fn();
    apiMock.searchWorkspacePaths.mockResolvedValue({ results: [{ id: 'result', workspaceId: 'ws', workspaceName: 'Workspace', name: 'guide.md', path: 'docs/guide.md', type: 'file', ignored: false, context: 'docs' }], truncated: false });
    render(<WorkspaceExplorerPage workspaces={[workspace]} location={{ workspaceId: 'ws' }} onLocationChange={onLocationChange} onOpenKanban={vi.fn()} />);
    fireEvent.change(screen.getByRole('textbox', { name: 'Search workspace paths' }), { target: { value: 'guide' } });
    expect(await screen.findByRole('option', { name: /guide.md/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('option', { name: /guide.md/i }));
    await waitFor(() => expect(onLocationChange).toHaveBeenCalledWith({ workspaceId: 'ws', path: 'docs/guide.md' }));
  });

  it('creates a Markdown file from the selected workspace root', async () => {
    apiMock.createWorkspaceFile.mockResolvedValue({ workspaceId: 'ws', path: 'notes.md', type: 'file', invalidatedPaths: [''], refreshed: false });
    render(<WorkspaceExplorerPage workspaces={[workspace]} location={{ workspaceId: 'ws' }} onLocationChange={vi.fn()} onOpenKanban={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /New file/i }));
    fireEvent.change(screen.getByRole('textbox', { name: 'Name' }), { target: { value: 'notes.md' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));
    await waitFor(() => expect(apiMock.createWorkspaceFile).toHaveBeenCalledWith('ws', { parentPath: '', name: 'notes.md', content: '' }));
  });
});

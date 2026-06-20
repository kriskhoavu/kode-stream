import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { WorkspaceExplorerPage } from './WorkspaceExplorerPage';

const apiMock = vi.hoisted(() => ({
  items: vi.fn(), workspaceTree: vi.fn(), workspaceFile: vi.fn(), workspaceFileDiff: vi.fn(),
  saveWorkspaceFile: vi.fn(), revertWorkspaceFile: vi.fn(), openPath: vi.fn(), gitStatus: vi.fn(), workspaceHealth: vi.fn()
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
  });

  it('loads one directory when a workspace root expands', async () => {
    const { container } = render(<WorkspaceExplorerPage workspaces={[workspace]} onLocationChange={vi.fn()} onOpenKanban={vi.fn()} />);
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
});

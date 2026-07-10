import { useState } from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { WorkstreamExplorer } from './WorkstreamExplorer';

const apiMock = vi.hoisted(() => ({
  items: vi.fn(), workspaceTree: vi.fn(), workspaceFile: vi.fn(), workspaceFileDiff: vi.fn(),
  saveWorkspaceFile: vi.fn(), revertWorkspaceFile: vi.fn(), openPath: vi.fn(), gitStatus: vi.fn(), gitActivity: vi.fn(), workspaceHealth: vi.fn(),
	searchWorkspacePaths: vi.fn(), searchWorkspaceContent: vi.fn(), searchItemContent: vi.fn(), workspacePathGitStates: vi.fn(), workspaceBranches: vi.fn(), switchBranch: vi.fn(), createWorkspaceFile: vi.fn(), createWorkspaceDirectory: vi.fn(), renameWorkspacePath: vi.fn()
}));

vi.mock('../lib/api', () => ({
  api: apiMock,
  ApiError: class ApiError extends Error { recoveryHint?: string }
}));

const workspace = { id: 'ws', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: [], createdAt: '' };

describe('WorkstreamExplorer', () => {
  beforeEach(() => {
    localStorage.clear();
    Object.values(apiMock).forEach((mock) => mock.mockReset());
    apiMock.items.mockResolvedValue([]);
    apiMock.workspaceTree.mockResolvedValue({ workspaceId: 'ws', path: '', hiddenCount: 0, entries: [
      { id: 'readme', name: 'README.md', path: 'README.md', type: 'file', hasChildren: false, ignored: false, hidden: false, editable: true, kind: 'markdown' }
    ] });
    apiMock.gitStatus.mockResolvedValue({ workspaceId: 'ws', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] });
    apiMock.gitActivity.mockResolvedValue([]);
    apiMock.workspaceHealth.mockResolvedValue({ workspaceId: 'ws', checkedAt: '', summary: 'ok', checks: [] });
    apiMock.workspacePathGitStates.mockResolvedValue([]);
    apiMock.workspaceBranches.mockResolvedValue({ workspaceId: 'ws', current: 'main', branches: ['feature/explorer', 'main'] });
    apiMock.searchWorkspacePaths.mockResolvedValue({ results: [], truncated: false });
		apiMock.searchWorkspaceContent.mockResolvedValue({ results: [], truncated: false, filesVisited: 0, bytesRead: 0, skippedFiles: 0 });
  });

  it('loads one directory when a workspace root expands', async () => {
		const { container } = render(<WorkstreamExplorer workspaces={[workspace]} location={{ mode: 'all' }} onLocationChange={vi.fn()} onOpenWorkstream={vi.fn()} />);
    fireEvent.click(container.querySelector('.explorer-row-toggle') as HTMLButtonElement);
    await waitFor(() => expect(apiMock.workspaceTree).toHaveBeenCalledWith('ws', '', false));
    expect(await screen.findByText('README.md')).toBeInTheDocument();
  });

  it('toggles workspace roots and configured directories from their names in Source folders mode', async () => {
    apiMock.workspaceTree.mockImplementation((_workspaceId: string, path: string) => Promise.resolve({
      workspaceId: 'ws', path, hiddenCount: 0, entries: path === 'docs'
        ? [{ id: 'guide', name: 'guide.md', path: 'docs/guide.md', type: 'file', hasChildren: false, ignored: false, hidden: false, editable: true, kind: 'markdown' }]
        : []
    }));
    const configuredWorkspace = { ...workspace, sources: ['docs'] };
    render(<WorkstreamExplorer workspaces={[configuredWorkspace]} location={{ mode: 'sources' }} onLocationChange={vi.fn()} onOpenWorkstream={vi.fn()} />);

    const workspaceButton = screen.getByRole('button', { name: 'Workspace' });
    fireEvent.doubleClick(workspaceButton);
    const docsButton = await screen.findByRole('button', { name: 'docs' });
    expect(workspaceButton.closest('[role="treeitem"]')).toHaveAttribute('aria-expanded', 'true');

    fireEvent.doubleClick(docsButton);
    await waitFor(() => expect(apiMock.workspaceTree).toHaveBeenCalledWith('ws', 'docs', false));
    expect(await screen.findByText('guide.md')).toBeInTheDocument();

    fireEvent.doubleClick(docsButton);
    expect(screen.queryByText('guide.md')).not.toBeInTheDocument();
    fireEvent.doubleClick(workspaceButton);
    expect(screen.queryByRole('button', { name: 'docs' })).not.toBeInTheDocument();
  });

  it('keeps a selected file parent collapsed after directory cache updates', async () => {
    let resolveDocs: ((value: { workspaceId: string; path: string; hiddenCount: number; entries: Array<Record<string, unknown>> }) => void) | undefined;
    apiMock.workspaceTree.mockImplementation((_workspaceId: string, path: string) => path === 'docs'
      ? new Promise((resolve) => { resolveDocs = resolve; })
      : Promise.resolve({ workspaceId: 'ws', path, hiddenCount: 0, entries: [] }));
    apiMock.workspaceFile.mockResolvedValue({ id: 'guide', path: 'docs/guide.md', content: '# Guide', language: 'markdown', hash: 'hash', kind: 'markdown', sizeBytes: 7, editable: true, truncated: false });
    apiMock.workspaceFileDiff.mockResolvedValue({ diff: '' });
    const configuredWorkspace = { ...workspace, sources: ['docs'] };
    render(<WorkstreamExplorer workspaces={[configuredWorkspace]} location={{ workspaceId: 'ws', path: 'docs/guide.md', mode: 'sources' }} onLocationChange={vi.fn()} onOpenWorkstream={vi.fn()} />);

    const docsButton = await screen.findByRole('button', { name: 'docs' });
    fireEvent.doubleClick(docsButton);
    expect(docsButton.closest('[role="treeitem"]')).toHaveAttribute('aria-expanded', 'false');

    await act(async () => resolveDocs?.({
      workspaceId: 'ws', path: 'docs', hiddenCount: 0,
      entries: [{ id: 'guide', name: 'guide.md', path: 'docs/guide.md', type: 'file', hasChildren: false, ignored: false, hidden: false, editable: true, kind: 'markdown' }]
    }));
    expect(docsButton.closest('[role="treeitem"]')).toHaveAttribute('aria-expanded', 'false');
  });

  it('switches the selected workspace branch and clears its file selection', async () => {
    apiMock.switchBranch.mockResolvedValue({
      ok: true,
      status: { workspaceId: 'ws', branch: 'feature/explorer', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }
    });
    apiMock.workspaceBranches
      .mockResolvedValueOnce({ workspaceId: 'ws', current: 'main', branches: ['feature/explorer', 'main'] })
      .mockResolvedValue({ workspaceId: 'ws', current: 'feature/explorer', branches: ['feature/explorer', 'main'] });
    const onLocationChange = vi.fn();
    render(<WorkstreamExplorer workspaces={[workspace]} location={{ workspaceId: 'ws', path: 'README.md', mode: 'all' }} onLocationChange={onLocationChange} onOpenWorkstream={vi.fn()} />);
    const selector = await screen.findByRole('combobox', { name: 'Branch for Workspace' });
    await waitFor(() => expect(selector).toBeEnabled());

    fireEvent.change(selector, { target: { value: 'feature/explorer' } });

    await waitFor(() => expect(apiMock.switchBranch).toHaveBeenCalledWith('ws', { name: 'feature/explorer', confirm: false }));
    await waitFor(() => expect(onLocationChange).toHaveBeenCalledWith({ workspaceId: 'ws', mode: 'all' }));
  });

  it('toggles folders from their names without folder-level Git badges', async () => {
    apiMock.items.mockResolvedValue([{ id: 'item', workspaceId: 'ws', itemPath: 'docs', identifier: 'DI-170', title: 'Hidden Explorer description', status: 'active', branch: 'main', scope: 'api', tags: [], metadataSource: 'plan.yaml' }]);
    apiMock.workspaceTree.mockImplementation((_workspaceId: string, path: string) => Promise.resolve({
      workspaceId: 'ws', path, hiddenCount: 0, entries: path === ''
        ? [{ id: 'docs', name: 'docs', path: 'docs', type: 'directory', hasChildren: true, ignored: false, hidden: false, editable: false }]
        : [{ id: 'guide', name: 'guide.md', path: 'docs/guide.md', type: 'file', hasChildren: false, ignored: false, hidden: false, editable: true, kind: 'markdown' }]
    }));
    apiMock.workspacePathGitStates.mockResolvedValue([{ path: 'docs/guide.md', status: 'modified', conflict: false }]);
    const { container } = render(<WorkstreamExplorer workspaces={[workspace]} location={{ mode: 'all' }} onLocationChange={vi.fn()} onOpenWorkstream={vi.fn()} />);
    fireEvent.click(container.querySelector('.explorer-row-toggle') as HTMLButtonElement);
    const folderButton = await screen.findByRole('button', { name: 'docs' });
    expect(folderButton.querySelector('.explorer-row-label')).toHaveClass('directory');
    expect(folderButton.closest('.explorer-tree-row')?.querySelector('.tree-state-icon')).toBeNull();
    expect(folderButton).not.toHaveTextContent('DI-170');
    expect(folderButton.closest('.explorer-tree-row')?.querySelector('.item-status-dot')).toBeNull();
    fireEvent.doubleClick(folderButton);
    await waitFor(() => expect(apiMock.workspaceTree).toHaveBeenCalledWith('ws', 'docs', false));
    expect(await screen.findByText('guide.md')).toBeInTheDocument();
    expect(screen.getByText('guide.md')).toHaveClass('explorer-row-label', 'file');
    expect(screen.getByLabelText('Modified file not committed')).toHaveClass('tree-state-icon', 'modified');
    fireEvent.doubleClick(folderButton);
    expect(folderButton.closest('[role="treeitem"]')).toHaveAttribute('aria-expanded', 'false');
  });

  it('enables Raw mode for editable text files', async () => {
    apiMock.workspaceTree.mockResolvedValue({ workspaceId: 'ws', path: '', hiddenCount: 0, entries: [
      { id: 'main_go', name: 'main.go', path: 'main.go', type: 'file', hasChildren: false, ignored: false, hidden: false, editable: true, kind: 'code' }
    ] });
    apiMock.workspaceFile.mockResolvedValue({ id: 'main_go', path: 'main.go', content: 'package main\n', language: 'go', hash: 'hash', kind: 'code', sizeBytes: 13, editable: true, truncated: false });
    apiMock.workspaceFileDiff.mockResolvedValue({ diff: '' });
    const onLocationChange = vi.fn();
    const { container, rerender } = render(<WorkstreamExplorer workspaces={[workspace]} location={{ mode: 'all' }} onLocationChange={onLocationChange} onOpenWorkstream={vi.fn()} />);
    fireEvent.click(container.querySelector('.explorer-row-toggle') as HTMLButtonElement);
    fireEvent.click(await screen.findByRole('button', { name: 'main.go' }));
    await waitFor(() => expect(onLocationChange).toHaveBeenCalledWith({ workspaceId: 'ws', path: 'main.go' }));
    rerender(<WorkstreamExplorer workspaces={[workspace]} location={{ workspaceId: 'ws', path: 'main.go', mode: 'all' }} onLocationChange={onLocationChange} onOpenWorkstream={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /Source/i }));
    const editor = container.querySelector('.raw-editor') as HTMLTextAreaElement;
    await waitFor(() => expect(editor).toBeEnabled());
    fireEvent.change(editor, { target: { value: 'package planmanager\n' } });
    expect(editor).toHaveValue('package planmanager\n');
  });

  it('keeps the current file open when selecting a directory', async () => {
    apiMock.workspaceTree.mockImplementation((_workspaceId: string, path: string) => Promise.resolve({
      workspaceId: 'ws', path, hiddenCount: 0, entries: path === ''
        ? [{ id: 'docs', name: 'docs', path: 'docs', type: 'directory', hasChildren: true, ignored: false, hidden: false, editable: false }]
        : [{ id: 'guide', name: 'guide.md', path: 'docs/guide.md', type: 'file', hasChildren: false, ignored: false, hidden: false, editable: true, kind: 'markdown' }]
    }));
    apiMock.workspaceFile.mockResolvedValue({ id: 'guide', path: 'docs/guide.md', content: '# Guide', language: 'markdown', hash: 'hash', kind: 'markdown', sizeBytes: 7, editable: true, truncated: false });
    apiMock.workspaceFileDiff.mockResolvedValue({ diff: '' });

    function Harness() {
      const [location, setLocation] = useState<{ workspaceId?: string; path?: string; mode?: 'all' | 'sources' }>({ workspaceId: 'ws', path: 'docs/guide.md', mode: 'all' });
      return <WorkstreamExplorer workspaces={[workspace]} location={location} onLocationChange={(next) => setLocation(next ?? { workspaceId: 'ws', mode: 'all' })} onOpenWorkstream={vi.fn()} />;
    }

    const { container } = render(<Harness />);
    await waitFor(() => expect(container.querySelector('.file-tab-button')).toHaveTextContent('guide.md'));
    fireEvent.click(screen.getByRole('button', { name: /Source/i }));
    await waitFor(() => expect((container.querySelector('.raw-editor') as HTMLTextAreaElement).value).toContain('# Guide'));

    const folderButton = screen.getByRole('button', { name: 'docs' });
    fireEvent.doubleClick(folderButton);
    await waitFor(() => expect(apiMock.workspaceTree).toHaveBeenCalledWith('ws', 'docs', false));
    fireEvent.click(folderButton);

    await waitFor(() => expect((container.querySelector('.raw-editor') as HTMLTextAreaElement).value).toContain('# Guide'));
    expect(container.querySelector('.file-tab-button')).toHaveTextContent('guide.md');
  });

  it('does not request file content for a directory path during initial resolution', async () => {
    apiMock.workspaceTree.mockResolvedValue({ workspaceId: 'ws', path: '', hiddenCount: 0, entries: [
      { id: 'docs', name: 'docs', path: 'docs', type: 'directory', hasChildren: true, ignored: false, hidden: false, editable: false }
    ] });

    render(<WorkstreamExplorer workspaces={[workspace]} location={{ workspaceId: 'ws', path: 'docs', mode: 'all' }} onLocationChange={vi.fn()} onOpenWorkstream={vi.fn()} />);

    await screen.findByRole('button', { name: 'docs' });
    expect(apiMock.workspaceFile).not.toHaveBeenCalled();
  });

  it('keeps Open Workstream explicit for a selected workspace root', async () => {
    const onOpenWorkstream = vi.fn();
    render(<WorkstreamExplorer workspaces={[workspace]} location={{ workspaceId: 'ws' }} onLocationChange={vi.fn()} onOpenWorkstream={onOpenWorkstream} />);
    expect(screen.queryByRole('button', { name: /Open Workstream/i })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Expand inspector' }));
    fireEvent.click(await screen.findByRole('button', { name: /Open Workstream/i }));
    expect(onOpenWorkstream).toHaveBeenCalledWith(workspace, undefined);
  });

  it('searches unloaded paths and opens a result', async () => {
    const onLocationChange = vi.fn();
    apiMock.searchWorkspacePaths.mockResolvedValue({ results: [{ id: 'result', workspaceId: 'ws', workspaceName: 'Workspace', name: 'guide.md', path: 'docs/guide.md', type: 'file', ignored: false, context: 'docs' }], truncated: false });
    render(<WorkstreamExplorer workspaces={[workspace]} location={{ workspaceId: 'ws' }} onLocationChange={onLocationChange} onOpenWorkstream={vi.fn()} />);
		fireEvent.change(screen.getByRole('textbox', { name: 'Search files' }), { target: { value: 'guide' } });
		const pathResult = await screen.findByRole('option', { name: /guide.md/i });
		expect(pathResult).toHaveClass('content-search-result');
		expect(pathResult).toHaveTextContent('File');
		fireEvent.click(pathResult);
    await waitFor(() => expect(onLocationChange).toHaveBeenCalledWith({ workspaceId: 'ws', path: 'docs/guide.md' }));
  });

  it('creates a Markdown file from the selected workspace root', async () => {
    apiMock.createWorkspaceFile.mockResolvedValue({ workspaceId: 'ws', path: 'notes.md', type: 'file', invalidatedPaths: [''], refreshed: false });
    render(<WorkstreamExplorer workspaces={[workspace]} location={{ workspaceId: 'ws' }} onLocationChange={vi.fn()} onOpenWorkstream={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /New file/i }));
    fireEvent.change(screen.getByRole('textbox', { name: 'Name' }), { target: { value: 'notes.md' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));
    await waitFor(() => expect(apiMock.createWorkspaceFile).toHaveBeenCalledWith('ws', { parentPath: '', name: 'notes.md', content: '' }));
  });

  it('collapses and expands the embedded files panel without changing tree expansion', async () => {
    apiMock.workspaceTree.mockImplementation((_workspaceId: string, path: string) => Promise.resolve({
      workspaceId: 'ws', path, hiddenCount: 0, entries: path === ''
        ? [{ id: 'docs', name: 'docs', path: 'docs', type: 'directory', hasChildren: true, ignored: false, hidden: false, editable: false }]
        : [{ id: 'guide', name: 'guide.md', path: 'docs/guide.md', type: 'file', hasChildren: false, ignored: false, hidden: false, editable: true, kind: 'markdown' }]
    }));
    render(
      <WorkstreamExplorer
        embedded
        showModeSelector={false}
        treeRootPath="docs"
        workspaces={[workspace]}
        location={{ workspaceId: 'ws', path: 'docs', mode: 'all' }}
        onLocationChange={vi.fn()}
        onOpenWorkstream={vi.fn()}
      />
    );

    expect(await screen.findByRole('button', { name: 'docs' })).toBeInTheDocument();
    fireEvent.doubleClick(screen.getByRole('button', { name: 'docs' }));
    await waitFor(() => expect(apiMock.workspaceTree).toHaveBeenCalledWith('ws', 'docs', false));
    expect(await screen.findByText('guide.md')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Collapse files panel' }));
    expect(screen.queryByText('guide.md')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'docs' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Expand files panel' }));
    expect(screen.getByRole('button', { name: 'docs' })).toBeInTheDocument();
    expect(screen.getByText('guide.md')).toBeInTheDocument();
  });

	it('switches tree mode and opens a highlighted content match', async () => {
		const onLocationChange = vi.fn();
		apiMock.searchWorkspaceContent.mockResolvedValue({
			results: [{ id: 'match', workspaceId: 'ws', workspaceName: 'Workspace', path: 'docs/guide.md', name: 'guide.md', kind: 'markdown', language: 'markdown', lineNumber: 7, columnStart: 3, columnEnd: 9, snippet: 'A needle here', ignored: false }],
			truncated: false, filesVisited: 1, bytesRead: 20, skippedFiles: 0
		});
		render(<WorkstreamExplorer workspaces={[{ ...workspace, sources: ['docs'] }]} location={{ workspaceId: 'ws' }} onLocationChange={onLocationChange} onOpenWorkstream={vi.fn()} />);
		fireEvent.change(screen.getByRole('combobox', { name: 'Explorer tree mode' }), { target: { value: 'all' } });
		expect(onLocationChange).toHaveBeenCalledWith({ workspaceId: 'ws', mode: 'all' });
		fireEvent.change(screen.getByRole('textbox', { name: 'Search files' }), { target: { value: 'needle' } });
		const result = await screen.findByRole('option', { name: /guide.md/i });
		expect(result).toHaveClass('content-search-result');
		expect(result.querySelector('mark')).toHaveTextContent('needle');
		fireEvent.click(result);
		await waitFor(() => expect(onLocationChange).toHaveBeenCalledWith({ workspaceId: 'ws', path: 'docs/guide.md' }));
		expect(document.querySelector('.content-match-context')).toHaveTextContent('Line 7, columns 3–9');
		expect(screen.getByRole('textbox', { name: 'Search files' })).toHaveValue('');
		expect(screen.queryByRole('option', { name: /guide.md/i })).not.toBeInTheDocument();
		expect(screen.queryByText('Paths')).not.toBeInTheDocument();
		expect(screen.queryByText('Content')).not.toBeInTheDocument();
		expect(screen.queryByLabelText('Search options')).not.toBeInTheDocument();
		expect(screen.getAllByRole('combobox')).toHaveLength(2);
	});
});

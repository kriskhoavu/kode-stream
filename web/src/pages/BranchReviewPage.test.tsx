import { useState } from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ReviewLocation } from '../app/router';
import type { FileContent, WorkspaceConfig, WorkstreamBranchLoadResult } from '../lib/types';
import { BranchReviewPage } from './BranchReviewPage';

const mocks = vi.hoisted(() => ({
  loadBranchReview: vi.fn(), files: vi.fn(), file: vi.fn(), importReviewedPlan: vi.fn(), switchBranch: vi.fn()
}));
vi.mock('../shared/api', () => ({ api: { loadBranchReview: mocks.loadBranchReview, files: mocks.files, file: mocks.file, importReviewedPlan: mocks.importReviewedPlan, switchBranch: mocks.switchBranch } }));
vi.mock('../features/workstream-explorer/useWorkspaceBranches', () => ({
  useWorkspaceBranches: () => ({ states: { 'ws-1': { workspaceId: 'ws-1', current: 'main', branches: ['main', 'feature/review', 'feature/slow', 'feature/fast'], loading: false, switching: false, error: '', recoveryHint: '' } }, switchBranch: mocks.switchBranch })
}));
vi.mock('../features/content-viewer/ContentViewer', () => ({ ContentViewer: ({ file }: { file: FileContent }) => <><div>Previewing {file.path}</div><div>{file.content}</div></> }));

describe('BranchReviewPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.loadBranchReview.mockResolvedValue(reviewResult());
    mocks.files.mockResolvedValue([{ id: 'README_md', name: 'README.md', path: 'README.md', type: 'file' }]);
    mocks.file.mockResolvedValue(file());
    mocks.importReviewedPlan.mockResolvedValue({ item: { id: 'imported-1' }, scannedAt: '' });
    mocks.switchBranch.mockResolvedValue(true);
  });

  it('renders a pinned read-only plan and committed file without operational controls', async () => {
    renderReview();
    expect(await screen.findByText('PM-038')).toBeInTheDocument();
    expect(screen.getByText('Read-only committed snapshot. Operational pages remain on the current checkout.')).toBeInTheDocument();
    expect(await screen.findByText('Previewing README.md')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /new work item/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /AI session/i })).not.toBeInTheDocument();
  });

  it('shows an explicit empty state when the reviewed branch has no plans', async () => {
    mocks.loadBranchReview.mockResolvedValue(reviewResult({ items: [], itemCount: 0 }));
    renderReview();
    expect(await screen.findByText('No plans on this branch')).toBeInTheDocument();
  });

  it('reports import conflicts without leaving review', async () => {
    mocks.importReviewedPlan.mockRejectedValue(new Error('Target plan already exists'));
    vi.stubGlobal('confirm', vi.fn(() => true));
    renderReview();
    fireEvent.click(await screen.findByRole('button', { name: 'Import selected plan' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Target plan already exists');
    expect(mocks.importReviewedPlan).toHaveBeenCalledWith('ws-1', { sourceBranch: 'feature/review', expectedCommit: 'abcdef123456', expectedCheckoutBranch: 'main', itemId: 'snapshot-1' });
  });

  it('uses the checkout picker for the reviewed branch', async () => {
		mocks.switchBranch.mockResolvedValue({ ok: true, status: { branch: 'feature/review' } });
    renderReview();
		fireEvent.click(await screen.findByRole('button', { name: 'Select checkout branch' }));
		fireEvent.click(screen.getByRole('listbox', { name: 'Checkout branches' }).querySelector('[role="option"]:nth-of-type(3)') as HTMLElement);
    await waitFor(() => expect(mocks.switchBranch).toHaveBeenCalledWith('ws-1', { name: 'feature/review', strategy: undefined, stashMessage: undefined }));
  });

  it('disables checkout switching while refreshing the snapshot', async () => {
    let finishRefresh!: (result: WorkstreamBranchLoadResult) => void;
    mocks.loadBranchReview
      .mockResolvedValueOnce(reviewResult())
      .mockImplementationOnce(() => new Promise((resolve) => { finishRefresh = resolve; }));
    renderReview();
		const switchButton = await screen.findByRole('button', { name: 'Select checkout branch' });

    fireEvent.click(screen.getByRole('button', { name: 'Refresh snapshot' }));

    await waitFor(() => expect(switchButton).toBeDisabled());
    fireEvent.click(switchButton);
    expect(mocks.switchBranch).not.toHaveBeenCalled();
    finishRefresh(reviewResult());
    await waitFor(() => expect(switchButton).toBeEnabled());
  });

  it('reloads the same item files when refresh advances the reviewed commit', async () => {
    mocks.loadBranchReview
      .mockResolvedValueOnce(reviewResult({ commit: 'oldcommit123456' }))
      .mockResolvedValueOnce(reviewResult({ commit: 'newcommit654321' }));
    mocks.file
      .mockResolvedValueOnce(file('# Content from old commit'))
      .mockResolvedValueOnce(file('# Content from new commit'));
    renderReview();
    expect(await screen.findByText('# Content from old commit')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Refresh snapshot' }));

    expect(await screen.findByText('newcommi')).toBeInTheDocument();
    expect(await screen.findByText('# Content from new commit')).toBeInTheDocument();
    expect(screen.queryByText('# Content from old commit')).not.toBeInTheDocument();
    expect(mocks.files).toHaveBeenCalledTimes(2);
    expect(mocks.files).toHaveBeenNthCalledWith(1, 'snapshot-1', 'oldcommit123456');
    expect(mocks.files).toHaveBeenNthCalledWith(2, 'snapshot-1', 'newcommit654321');
    expect(mocks.file).toHaveBeenCalledTimes(2);
  });

  it('ignores an older manual file response after refresh publishes a new commit', async () => {
    let finishManual!: (content: FileContent) => void;
    let oldCommitReads = 0;
    mocks.loadBranchReview
      .mockResolvedValueOnce(reviewResult({ commit: 'oldcommit123456' }))
      .mockResolvedValueOnce(reviewResult({ commit: 'newcommit654321' }));
    mocks.file.mockImplementation((_itemId: string, _fileId: string, expectedCommit: string) => {
      if (expectedCommit === 'oldcommit123456' && oldCommitReads++ === 0) {
        return Promise.resolve(file('# Initial old content'));
      }
      if (expectedCommit === 'oldcommit123456') {
        return new Promise((resolve) => { finishManual = resolve; });
      }
      return Promise.resolve(file('# Refreshed new content'));
    });
    renderReview();
    expect(await screen.findByText('# Initial old content')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'README.md' }));
    await waitFor(() => expect(mocks.file).toHaveBeenCalledTimes(2));
    fireEvent.click(screen.getByRole('button', { name: 'Refresh snapshot' }));
    expect(await screen.findByText('# Refreshed new content')).toBeInTheDocument();

    await act(async () => { finishManual(file('# Late old manual content')); });
    expect(screen.getByText('# Refreshed new content')).toBeInTheDocument();
    expect(screen.queryByText('# Late old manual content')).not.toBeInTheDocument();
    expect(mocks.file).toHaveBeenCalledWith('snapshot-1', 'README_md', 'oldcommit123456');
    expect(mocks.file).toHaveBeenCalledWith('snapshot-1', 'README_md', 'newcommit654321');
  });

  it('ignores a superseded branch response that resolves after the current request', async () => {
    let finishSlow!: (result: WorkstreamBranchLoadResult) => void;
    let finishFast!: (result: WorkstreamBranchLoadResult) => void;
    mocks.loadBranchReview.mockImplementation((_workspaceId: string, input: { branch: string }) => new Promise((resolve) => {
      if (input.branch === 'feature/slow') finishSlow = resolve;
      if (input.branch === 'feature/fast') finishFast = resolve;
    }));
    vi.stubGlobal('confirm', vi.fn(() => true));
    renderReview('feature/slow');
    await waitFor(() => expect(mocks.loadBranchReview).toHaveBeenCalledWith('ws-1', { branch: 'feature/slow', force: false }));

    fireEvent.change(screen.getByRole('combobox', { name: 'Review branch' }), { target: { value: 'feature/fast' } });
    await waitFor(() => expect(mocks.loadBranchReview).toHaveBeenCalledWith('ws-1', { branch: 'feature/fast', force: false }));
    await act(async () => { finishFast(reviewResultFor('feature/fast', 'FAST-001')); });
    expect(await screen.findAllByText('FAST-001')).not.toHaveLength(0);

    await act(async () => { finishSlow(reviewResultFor('feature/slow', 'SLOW-001')); });
    expect(screen.queryByText('SLOW-001')).not.toBeInTheDocument();
    expect(screen.getByText('feature/fast', { selector: '.branch-context-chip.review strong' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Import selected plan' }));
    await waitFor(() => expect(mocks.importReviewedPlan).toHaveBeenCalledWith('ws-1', expect.objectContaining({ sourceBranch: 'feature/fast', itemId: 'feature-fast-item' })));
  });

  it('keeps loading owned by the newest request when a superseded request settles first', async () => {
    let finishSlow!: (result: WorkstreamBranchLoadResult) => void;
    let finishFast!: (result: WorkstreamBranchLoadResult) => void;
    mocks.loadBranchReview.mockImplementation((_workspaceId: string, input: { branch: string }) => new Promise((resolve) => {
      if (input.branch === 'feature/slow') finishSlow = resolve;
      if (input.branch === 'feature/fast') finishFast = resolve;
    }));
    renderReview('feature/slow');
    await waitFor(() => expect(mocks.loadBranchReview).toHaveBeenCalledWith('ws-1', { branch: 'feature/slow', force: false }));
    fireEvent.change(screen.getByRole('combobox', { name: 'Review branch' }), { target: { value: 'feature/fast' } });
    await waitFor(() => expect(mocks.loadBranchReview).toHaveBeenCalledWith('ws-1', { branch: 'feature/fast', force: false }));

    await act(async () => { finishSlow(reviewResultFor('feature/slow', 'SLOW-001')); });
    expect(screen.getByRole('status')).toHaveTextContent('Loading committed snapshot…');
    expect(screen.queryByText('SLOW-001')).not.toBeInTheDocument();

    await act(async () => { finishFast(reviewResultFor('feature/fast', 'FAST-001')); });
    expect(screen.queryByText('Loading committed snapshot…')).not.toBeInTheDocument();
    expect(screen.getByText('feature/fast', { selector: '.branch-context-chip.review strong' })).toBeInTheDocument();
  });
});

const workspace: WorkspaceConfig = { id: 'ws-1', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '' };

function renderReview(initialBranch = 'feature/review') {
  function Harness() {
    const [location, setLocation] = useState<ReviewLocation>({ workspaceId: workspace.id, branch: initialBranch });
    return <BranchReviewPage workspace={workspace} location={location} onLocationChange={setLocation} onExit={vi.fn()} onImported={vi.fn()} onCheckoutSwitched={vi.fn()} />;
  }
  return render(<Harness />);
}

function reviewResult(overrides: Partial<WorkstreamBranchLoadResult> = {}): WorkstreamBranchLoadResult {
  return {
    workspaceId: workspace.id, branch: 'feature/review', selectedBranch: 'feature/review', branchRef: 'refs/heads/feature/review', commit: 'abcdef123456', currentCheckoutBranch: 'main', sourceMode: 'snapshot', mode: 'snapshot', editable: false, scannedAt: '', itemCount: 1, warnings: [],
    items: [{ id: 'snapshot-1', workspaceId: workspace.id, workspaceName: workspace.name, scope: 'platform', branch: 'feature/review', identifier: 'PM-038', title: 'Checkout-first review', status: 'review', tags: [], metadataSource: 'plan.yaml', itemPath: 'plans/platform/PM-038', sourceMode: 'snapshot', editable: false }],
    ...overrides
  };
}

function reviewResultFor(branch: string, identifier: string): WorkstreamBranchLoadResult {
  const slug = branch.replace('/', '-');
  return reviewResult({
    branch,
    selectedBranch: branch,
    branchRef: `refs/heads/${branch}`,
    items: [{ id: `${slug}-item`, workspaceId: workspace.id, workspaceName: workspace.name, scope: 'platform', branch, identifier, title: identifier, status: 'review', tags: [], metadataSource: 'plan.yaml', itemPath: `plans/platform/${identifier}`, sourceMode: 'snapshot', editable: false }]
  });
}

function file(content = '# PM-038'): FileContent {
  return { id: 'README_md', path: 'README.md', content, language: 'markdown', hash: 'hash', kind: 'markdown', sizeBytes: content.length, editable: false };
}

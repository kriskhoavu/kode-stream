import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { FileContent, WorkspaceConfig, WorkstreamBranchLoadResult } from '../lib/types';
import { BranchReviewPage } from './BranchReviewPage';

const mocks = vi.hoisted(() => ({
  loadBranchReview: vi.fn(), files: vi.fn(), file: vi.fn(), importReviewedPlan: vi.fn(), switchBranch: vi.fn()
}));
vi.mock('../lib/api', () => ({ api: { loadBranchReview: mocks.loadBranchReview, files: mocks.files, file: mocks.file, importReviewedPlan: mocks.importReviewedPlan } }));
vi.mock('../features/workstream-explorer/useWorkspaceBranches', () => ({
  useWorkspaceBranches: () => ({ states: { 'ws-1': { workspaceId: 'ws-1', current: 'main', branches: ['main', 'feature/review'], loading: false, switching: false, error: '', recoveryHint: '' } }, switchBranch: mocks.switchBranch })
}));
vi.mock('../features/content-viewer/ContentViewer', () => ({ ContentViewer: ({ file }: { file: FileContent }) => <div>Previewing {file.path}</div> }));

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
  });

  it('uses the guarded workspace switch action for the reviewed branch', async () => {
    renderReview();
    fireEvent.click(await screen.findByRole('button', { name: 'Switch workspace to this branch' }));
    await waitFor(() => expect(mocks.switchBranch).toHaveBeenCalledWith(workspace, 'feature/review'));
  });
});

const workspace: WorkspaceConfig = { id: 'ws-1', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '' };

function renderReview() {
  return render(<BranchReviewPage workspace={workspace} location={{ workspaceId: workspace.id, branch: 'feature/review' }} onLocationChange={vi.fn()} onExit={vi.fn()} onImported={vi.fn()} onCheckoutSwitched={vi.fn()} />);
}

function reviewResult(overrides: Partial<WorkstreamBranchLoadResult> = {}): WorkstreamBranchLoadResult {
  return {
    workspaceId: workspace.id, branch: 'feature/review', selectedBranch: 'feature/review', branchRef: 'refs/heads/feature/review', commit: 'abcdef123456', currentCheckoutBranch: 'main', sourceMode: 'snapshot', mode: 'snapshot', editable: false, scannedAt: '', itemCount: 1, warnings: [],
    items: [{ id: 'snapshot-1', workspaceId: workspace.id, workspaceName: workspace.name, scope: 'platform', branch: 'feature/review', identifier: 'PM-038', title: 'Checkout-first review', status: 'review', tags: [], metadataSource: 'plan.yaml', itemPath: 'plans/platform/PM-038', sourceMode: 'snapshot', editable: false }],
    ...overrides
  };
}

function file(): FileContent {
  return { id: 'README_md', path: 'README.md', content: '# PM-038', language: 'markdown', hash: 'hash', kind: 'markdown', sizeBytes: 8, editable: false };
}

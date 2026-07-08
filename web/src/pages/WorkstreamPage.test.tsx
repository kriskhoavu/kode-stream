import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { filterPlans, WorkstreamPage } from './WorkstreamPage';
import type { WorkstreamBranchLoadResult, ItemSummary, SourceMode } from '../lib/types';

const workspace = { id: 'r1', name: 'Discovery', path: '/repo', baselineBranch: 'main', sources: ['items'], createdAt: new Date().toISOString() };
const draftItem: ItemSummary = {
  id: 'p1',
  workspaceId: 'r1',
  workspaceName: 'Discovery',
  branch: 'main',
  scope: 'platform',
  identifier: 'PM-012',
  title: 'Drag cards',
  status: 'draft',
  tags: [],
  metadataSource: 'plan.yaml',
  itemPath: 'items/platform/PM-012'
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('WorkstreamPage', () => {
  it('renders status columns from cached plan summaries', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [
        {
          id: 'p1',
          workspaceId: 'r1',
          workspaceName: 'Discovery',
          branch: 'main',
          scope: 'platform',
          identifier: 'PM-001',
          title: 'Item Manager',
          status: 'draft',
          tags: ['readonly'],
          metadataSource: 'plan.yaml',
          itemPath: 'items/platform/PM-001'
        }
      ]
    }));

    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    expect(screen.getByRole('heading', { name: 'Unsorted' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Draft' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText('Item Manager')).toBeInTheDocument());
  });

  it('creates a work item from Jira context', async () => {
    const onOpenPlan = vi.fn();
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/branch') return Promise.resolve(response(workstreamBranchLoadResult([], 'main')));
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main'] }));
      if (url === '/api/workspaces/r1/jira/issues/PM-025') return Promise.resolve(response({
        state: 'available',
        issue: {
          key: 'PM-025',
          summary: 'Jira First Workspace',
          status: 'In Progress',
          description: 'Create from Jira first.',
          issueType: 'Story',
          assignee: { displayName: 'Kim' },
          reporter: { displayName: 'BA' },
          priority: 'High',
          labels: ['planning'],
          browserUrl: 'https://jira.example/browse/PM-025',
          attachments: [{ id: 'a1', filename: 'spec.pdf', mediaType: 'application/pdf', sizeBytes: 120, author: { displayName: 'BA' } }]
        }
      }));
      if (url === '/api/items' && init?.method === 'POST') return Promise.resolve(response({
        item: { ...draftItem, id: 'created', identifier: 'PM-025', title: 'Jira First Workspace', status: 'draft', documents: [], metadata: {}, counts: { files: 2 } },
        scannedAt: '2026-06-23T00:00:00Z'
      }));
      return Promise.resolve(response([]));
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={onOpenPlan} onWorkspacesChanged={() => undefined} />);

    fireEvent.click(await screen.findByRole('button', { name: /\+ New Work Item/i }));
    fireEvent.click(screen.getByRole('button', { name: 'From Jira' }));
    fireEvent.change(screen.getByLabelText('Jira key'), { target: { value: 'pm-025' } });
    fireEvent.keyDown(screen.getByLabelText('Jira key'), { key: 'Enter' });

    expect(await screen.findByText('PM-025: Jira First Workspace')).toBeInTheDocument();
    expect(screen.getByLabelText('Item name')).toHaveValue('PM-025');
    expect(screen.getByLabelText('Title')).toHaveValue('Jira First Workspace');
    expect(screen.getByLabelText('Owner')).toHaveValue('Kim');

    fireEvent.click(screen.getByRole('button', { name: 'Create Item' }));

    await waitFor(() => expect(onOpenPlan).toHaveBeenCalledWith('created'));
    const createCall = fetchMock.mock.calls.find(([url, init]) => String(url) === '/api/items' && init?.method === 'POST');
    const body = JSON.parse(String(createCall?.[1]?.body ?? '{}')) as Record<string, unknown>;
    expect(body).toMatchObject({ workspaceId: 'r1', source: 'items', scope: 'items', identifier: 'PM-025', title: 'Jira First Workspace', owner: 'Kim', jiraKey: 'PM-025' });
    expect(body.tags).toEqual(['priority-high', 'story', 'planning']);
    expect(String(body.initialReadme)).toContain('## Jira Context');
    expect(String(body.initialReadme)).toContain('spec.pdf');
  });

  it('does not create files when Jira lookup fails', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, _init?: RequestInit) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/branch') return Promise.resolve(response(workstreamBranchLoadResult([], 'main')));
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main'] }));
      if (url === '/api/workspaces/r1/jira/issues/PM-404') return Promise.resolve(response({ state: 'not_found', message: 'No Jira ticket exists for this item' }));
      return Promise.resolve(response([]));
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    fireEvent.click(await screen.findByRole('button', { name: /\+ New Work Item/i }));
    fireEvent.click(screen.getByRole('button', { name: 'From Jira' }));
    fireEvent.change(screen.getByLabelText('Jira key'), { target: { value: 'PM-404' } });
    fireEvent.click(screen.getByRole('button', { name: /Fetch Jira/i }));

    expect(await screen.findByText('No Jira ticket exists for this item')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create Item' })).toBeDisabled();
    expect(fetchMock.mock.calls.some(([url, init]) => String(url) === '/api/items' && init?.method === 'POST')).toBe(false);

    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByRole('dialog', { name: 'Create new work item' })).not.toBeInTheDocument();
  });

  it('does not repeat placeholder docs metadata on docs cards', async () => {
    const docsItem: ItemSummary = {
      id: 'docs',
      workspaceId: 'r1',
      workspaceName: 'Discovery',
      branch: 'main',
      scope: 'docs',
      identifier: 'docs',
      title: 'Docs',
      status: 'unsorted',
      author: 'Khoa Đăng Vũ',
      tags: ['docs'],
      updatedAt: '2026-05-28T00:00:00Z',
      metadataSource: 'docs',
      itemPath: 'docs'
    };
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/branch') return Promise.resolve(response(workstreamBranchLoadResult([docsItem], 'main')));
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main'] }));
      return Promise.resolve(response({}));
    }));

    render(<WorkstreamPage workspace={{ ...workspace, sources: ['items', 'docs'] }} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    await screen.findByRole('button', { name: 'Docs' });
    const card = document.querySelector('.docs-plan');
    expect(card).toBeInstanceOf(HTMLElement);
    expect(within(card as HTMLElement).getAllByText('Docs')).toHaveLength(1);
    expect(within(card as HTMLElement).getByText('docs')).toHaveClass('source-badge', 'docs');
    expect(within(card as HTMLElement).queryByText('No date')).not.toBeInTheDocument();
  });

  it('shows only configured Workstream status columns', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/branch') return Promise.resolve(response(workstreamBranchLoadResult([draftItem], 'main')));
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main'] }));
      return Promise.resolve(response({}));
    }));

    render(<WorkstreamPage workspace={workspace} refreshKey={0} visibleStatuses={['draft', 'review']} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    expect(await screen.findByRole('heading', { name: 'Draft' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Review' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Unsorted' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Done' })).not.toBeInTheDocument();
    expect(screen.getByText('Drag cards')).toBeInTheDocument();
  });

  it('filters to and opens a focused card from the route', async () => {
    const detail = { ...draftItem, documents: [], metadata: {}, counts: { files: 0 } };
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/branch') return Promise.resolve(response(workstreamBranchLoadResult([draftItem], 'main')));
      if (url === '/api/items/p1') return Promise.resolve(response(detail));
      if (url === '/api/items/p1/files') return Promise.resolve(response([]));
      if (url === '/api/items/p1/diff') return Promise.resolve(response({ diff: '' }));
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main'] }));
      return Promise.resolve(response({}));
    }));

    render(<WorkstreamPage workspace={workspace} refreshKey={0} focusedItemId="p1" onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    await waitFor(() => expect(screen.getByPlaceholderText('Search items...')).toHaveValue('PM-012'));
    expect(await screen.findByLabelText('Item preview')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Drag cards' })).toBeInTheDocument();
  });

  it('shows Jira next to Git in the item preview drawer', async () => {
    const detail = { ...draftItem, documents: [], metadata: {}, counts: { files: 0 } };
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/branch') return Promise.resolve(response(workstreamBranchLoadResult([draftItem], 'main')));
      if (url === '/api/items/p1') return Promise.resolve(response(detail));
      if (url === '/api/items/p1/files') return Promise.resolve(response([]));
      if (url === '/api/items/p1/diff') return Promise.resolve(response({ diff: '' }));
      if (url === '/api/items/p1/jira') return Promise.resolve(response({
        state: 'available',
        issue: {
          key: 'PM-012',
          summary: 'Drag cards Jira ticket',
          status: 'Done',
          description: 'Jira context for this item.',
          issueType: 'Story',
          assignee: { displayName: 'Kim' },
          reporter: { displayName: 'BA' },
          priority: 'Medium',
          labels: ['frontend'],
          browserUrl: 'https://jira.example/browse/PM-012',
          attachments: []
        }
      }));
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main'] }));
      return Promise.resolve(response({}));
    }));

    render(<WorkstreamPage workspace={workspace} refreshKey={0} focusedItemId="p1" onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    const drawer = await screen.findByLabelText('Item preview');
    const tablist = within(drawer).getByRole('tablist', { name: 'Work item side panel' });
    expect(within(tablist).getAllByRole('button').map((button) => button.textContent?.trim())).toEqual(['Info', 'Git', 'Jira']);

    fireEvent.click(within(tablist).getByRole('button', { name: 'Jira' }));

    expect(await within(drawer).findByText('Drag cards Jira ticket')).toBeInTheDocument();
  });

  it('shows the active branch context and switches the loaded branch', async () => {
    const mainItems = [draftItem];
    const featureItems = [{ ...draftItem, id: 'p2', title: 'Feature item', branch: 'feature/pm-012' }];
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/branch') {
        return Promise.resolve(response(workstreamBranchLoadResult(mainItems, 'main')));
      }
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['feature/pm-012', 'release/old', 'master', 'main'] }));
      return Promise.resolve(response({}));
    }));

    const { container } = render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    const branchSelect = await screen.findByRole('button', { name: 'Select board branch' });
    await waitFor(() => expect(screen.queryByText('Feature item')).not.toBeInTheDocument());
    expect(screen.getByText('Drag cards')).toBeInTheDocument();
    expect(screen.queryByText('working tree')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Clear' })).not.toBeInTheDocument();

    fireEvent.click(screen.getAllByRole('button', { name: 'Source' })[0]);
    expect(container.querySelector('.facet-popover')).not.toBeNull();
    fireEvent.keyDown(document, { key: 'Escape' });
    await waitFor(() => expect(container.querySelector('.facet-popover')).toBeNull());

    fireEvent.click(branchSelect);
    const searchInput = screen.getByRole('textbox', { name: 'Search branches' });
    const branchMenu = screen.getByRole('listbox', { name: 'Board branches' });
    expect(within(branchMenu).getAllByRole('option').map((option) => option.textContent)).toEqual(['main', 'master', 'feature/pm-012', 'release/old']);
    expect(within(branchMenu).getByRole('option', { name: 'main' }).querySelector('.branch-option-check')).not.toBeNull();
    expect(within(branchMenu).getByRole('option', { name: 'main' }).querySelector('.branch-option-checkout')).not.toBeNull();
    expect(within(branchMenu).getByRole('option', { name: 'feature/pm-012' }).querySelector('.branch-option-checkout')).toBeNull();
    fireEvent.change(searchInput, { target: { value: 'release' } });
    expect(within(branchMenu).getAllByRole('option').map((option) => option.textContent)).toEqual(['main', 'master', 'release/old']);
    expect(within(branchMenu).queryByRole('option', { name: 'feature/pm-012' })).not.toBeInTheDocument();
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/branch') {
        return Promise.resolve(response(workstreamBranchLoadResult(featureItems, 'feature/pm-012', 'snapshot')));
      }
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['feature/pm-012', 'release/old', 'master', 'main'] }));
      return Promise.resolve(response({}));
    });
    fireEvent.change(searchInput, { target: { value: 'feature' } });
    fireEvent.click(within(branchMenu).getByRole('option', { name: 'feature/pm-012' }));

    await waitFor(() => expect(screen.getByText('Feature item')).toBeInTheDocument());
    expect(screen.queryByText('Drag cards')).not.toBeInTheDocument();
    expect(screen.getByText('snapshot -> main')).toBeInTheDocument();
  });

  it('closes the branch selector when clicking outside', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/branch') return Promise.resolve(response(workstreamBranchLoadResult([draftItem], 'main')));
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main', 'feature/pm-012'] }));
      return Promise.resolve(response({}));
    }));

    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    fireEvent.click(await screen.findByRole('button', { name: 'Select board branch' }));
    expect(screen.getByRole('listbox', { name: 'Board branches' })).toBeInTheDocument();

    fireEvent.pointerDown(document.body);

    await waitFor(() => expect(screen.queryByRole('listbox', { name: 'Board branches' })).not.toBeInTheDocument());
  });

  it('offers the current branch selector option even when no indexed item is on it', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/branch') return Promise.resolve(response(workstreamBranchLoadResult([], 'main')));
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main', 'feature/pm-012'] }));
      return Promise.resolve(response({}));
    }));

    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    await waitFor(() => expect(screen.queryByText('Drag cards')).not.toBeInTheDocument());
    const branchSelect = await screen.findByRole('button', { name: 'Select board branch' });

    fireEvent.click(branchSelect);
    const branchMenu = screen.getByRole('listbox', { name: 'Board branches' });
    expect(within(branchMenu).getByRole('option', { name: 'main' })).toBeInTheDocument();
    expect(within(branchMenu).getByRole('option', { name: 'feature/pm-012' })).toBeInTheDocument();
  });

  it('moves status optimistically and reconciles the returned item', async () => {
    const fetchMock = statusFetchMock(async () => response({
      item: { ...draftItem, status: 'review', title: 'Persisted title', documents: [], metadata: {}, counts: { files: 1 } },
      scannedAt: '2026-06-23T00:00:00Z'
    }));
    const onWorkspacesChanged = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={onWorkspacesChanged} />);
    await screen.findByText('Drag cards');

    selectCardStatus('Review');

    expect(within(column('Review')).getByText('Drag cards')).toBeInTheDocument();
    await waitFor(() => expect(within(column('Review')).getByText('Persisted title')).toBeInTheDocument());
    expect(fetchMock).toHaveBeenCalledWith('/api/items/p1/status', expect.objectContaining({ method: 'PATCH' }));
    expect(onWorkspacesChanged).toHaveBeenCalledOnce();
  });

  it('rolls back the item when status persistence fails', async () => {
    vi.stubGlobal('fetch', statusFetchMock(async () => response({ error: 'Status update failed' }, false, 500)));
    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
    await screen.findByText('Drag cards');

    selectCardStatus('Review');

    expect(within(column('Review')).getByText('Drag cards')).toBeInTheDocument();
    await waitFor(() => expect(within(column('Draft')).getByText('Drag cards')).toBeInTheDocument());
    expect(screen.getByText('Status update failed')).toBeInTheDocument();
  });

  it('ignores another move while the item status request is pending', async () => {
    let resolveUpdate!: (value: Response) => void;
    const update = new Promise<Response>((resolve) => { resolveUpdate = resolve; });
    const fetchMock = statusFetchMock(() => update);
    vi.stubGlobal('fetch', fetchMock);
    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
    await screen.findByText('Drag cards');

    selectCardStatus('Review');
    selectCardStatus('Done');

    expect(fetchMock.mock.calls.filter(([url]) => isItemStatusUrl(url))).toHaveLength(1);
    await act(async () => resolveUpdate(response({
      item: { ...draftItem, status: 'review', documents: [], metadata: {}, counts: { files: 1 } },
      scannedAt: '2026-06-23T00:00:00Z'
    })));
    await waitFor(() => expect(within(column('Review')).getByText('Drag cards')).toBeInTheDocument());
  });

  it('confirms before materializing a snapshot status move', async () => {
    const snapshotItem = { ...draftItem, sourceMode: 'snapshot' as const, editable: false };
    const confirm = vi.fn(() => true);
    vi.stubGlobal('confirm', confirm);
    const fetchMock = statusFetchMock(async () => response({
      item: { ...draftItem, status: 'review', sourceMode: 'working_tree', editable: true, documents: [], metadata: {}, counts: { files: 1 } },
      scannedAt: '2026-06-23T00:00:00Z'
    }), snapshotItem);
    vi.stubGlobal('fetch', fetchMock);
    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
    await screen.findByText('Drag cards');

    selectCardStatus('Review');

    await waitFor(() => expect(fetchMock.mock.calls.filter(([url]) => isItemStatusUrl(url))).toHaveLength(1));
    expect(confirm).toHaveBeenCalledWith(expect.stringContaining('copy the whole plan at items/platform/PM-012 into the current checkout branch'));
    expect(statusRequestBody(fetchMock)).toMatchObject({ status: 'review', materializeConfirmed: true });
  });

  it('cancels snapshot status moves when materialization is declined', async () => {
    const snapshotItem = { ...draftItem, sourceMode: 'snapshot' as const, editable: false };
    vi.stubGlobal('confirm', vi.fn(() => false));
    const fetchMock = statusFetchMock(async () => response({}), snapshotItem);
    vi.stubGlobal('fetch', fetchMock);
    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
    await screen.findByText('Drag cards');

    selectCardStatus('Review');

    await waitFor(() => expect(fetchMock.mock.calls.filter(([url]) => isItemStatusUrl(url))).toHaveLength(0));
    expect(within(column('Draft')).getByText('Drag cards')).toBeInTheDocument();
  });

  it('shows materialization conflict errors and rolls status back', async () => {
    const snapshotItem = { ...draftItem, sourceMode: 'snapshot' as const, editable: false };
    vi.stubGlobal('confirm', vi.fn(() => true));
    vi.stubGlobal('fetch', statusFetchMock(async () => response({ error: 'Target file already exists' }, false, 409), snapshotItem));
    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
    await screen.findByText('Drag cards');

    selectCardStatus('Review');

    await waitFor(() => expect(within(column('Draft')).getByText('Drag cards')).toBeInTheDocument());
    expect(screen.getByText('Target file already exists')).toBeInTheDocument();
  });
});

function column(name: string): HTMLElement {
  const element = screen.getByRole('heading', { name }).closest('.workstream-column');
  if (!element) throw new Error(`Missing ${name} column`);
  return element as HTMLElement;
}

function selectCardStatus(status: string): void {
  fireEvent.click(screen.getByRole('button', { name: 'Move item status' }));
  fireEvent.click(screen.getByRole('button', { name: status }));
}

function statusFetchMock(updateStatus: () => Promise<Response>, item: ItemSummary = draftItem) {
  return vi.fn((input: RequestInfo | URL) => {
    const url = String(input);
    if (url === '/api/workspaces/r1/workstream/branch') return Promise.resolve(response(workstreamBranchLoadResult([item], item.branch, item.sourceMode)));
    if (url.startsWith('/api/items?')) return Promise.resolve(response([item]));
    if (url === '/api/saved-filters') return Promise.resolve(response([]));
    if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
    if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main'] }));
    if (isItemStatusUrl(url)) return updateStatus();
    return Promise.resolve(response({}));
  });
}

function statusRequestBody(fetchMock: ReturnType<typeof vi.fn>): Record<string, unknown> {
  const call = fetchMock.mock.calls.find(([url]) => isItemStatusUrl(url));
  return JSON.parse(String(call?.[1]?.body ?? '{}')) as Record<string, unknown>;
}

function workstreamBranchLoadResult(items: ItemSummary[], branch: string, sourceMode: SourceMode = 'working_tree'): WorkstreamBranchLoadResult {
  return {
    workspaceId: 'r1',
    branch,
    selectedBranch: branch,
    branchRef: `refs/heads/${branch}`,
    commit: sourceMode === 'snapshot' ? 'abc123' : '',
    currentCheckoutBranch: 'main',
    sourceMode,
    mode: sourceMode,
    editable: sourceMode !== 'snapshot',
    scannedAt: '2026-06-23T00:00:00Z',
    itemCount: items.length,
    warnings: [],
    items
  };
}

function isItemStatusUrl(input: RequestInfo | URL): boolean {
  const url = String(input);
  return url.startsWith('/api/items/') && url.endsWith('/status');
}

function response(body: unknown, ok = true, status = 200): Response {
  return { ok, status, json: async () => body } as Response;
}

describe('filterPlans', () => {
  const items: ItemSummary[] = [
    {
      id: 'p1',
      workspaceId: 'r1',
      workspaceName: 'Discovery',
      branch: 'main',
      scope: 'api',
      identifier: 'DI-1',
      title: 'API Item',
      status: 'draft',
      author: 'Khoa',
      tags: [],
      metadataSource: 'plan.yaml',
      itemPath: 'items/api/DI-1'
    },
    {
      id: 'p2',
      workspaceId: 'r2',
      workspaceName: 'Docs',
      branch: 'feature/docs',
      scope: 'docs',
      identifier: 'docs',
      title: 'Docs',
      status: 'unsorted',
      author: 'Giang',
      tags: ['docs'],
      metadataSource: 'docs',
      itemPath: 'docs'
    }
  ];
  const workspace = { id: 'r1', name: 'Discovery', path: '/repo', baselineBranch: 'main', sources: ['items', 'docs'], createdAt: new Date().toISOString() };

  it('uses OR within a facet', () => {
    const result = filterPlans(items, { sources: ['items', 'docs'], scopes: [], statuses: [], branches: [], authors: [] }, '', workspace);
    expect(result.map((plan) => plan.id)).toEqual(['p1', 'p2']);
  });

  it('filters by scope', () => {
    const result = filterPlans(items, { sources: [], scopes: ['api'], statuses: [], branches: [], authors: [] }, '', workspace);
    expect(result.map((plan) => plan.id)).toEqual(['p1']);
  });

  it('uses AND across facets', () => {
    const result = filterPlans(items, { sources: ['docs'], scopes: ['docs'], statuses: ['unsorted'], branches: [], authors: ['Giang'] }, '', workspace);
    expect(result.map((plan) => plan.id)).toEqual(['p2']);
  });
});

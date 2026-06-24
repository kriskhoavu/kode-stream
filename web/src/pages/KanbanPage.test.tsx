import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { filterPlans, KanbanPage } from './KanbanPage';
import type { BranchLoadResult, ItemSummary, SourceMode } from '../lib/types';

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

describe('KanbanPage', () => {
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

    render(<KanbanPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    expect(screen.getByRole('heading', { name: 'Unsorted' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Ideas' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Draft' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText('Item Manager')).toBeInTheDocument());
  });

  it('shows the active branch context and switches the loaded branch', async () => {
    const mainItems = [draftItem];
    const featureItems = [{ ...draftItem, id: 'p2', title: 'Feature item', branch: 'feature/pm-012' }];
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/kanban/branch') {
        return Promise.resolve(response(branchLoadResult(mainItems, 'main')));
      }
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main', 'feature/pm-012', 'release/old'] }));
      return Promise.resolve(response({}));
    }));

    render(<KanbanPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    const branchSelect = await screen.findByRole('combobox', { name: 'Select Kanban branch' });
    await waitFor(() => expect(screen.queryByText('Feature item')).not.toBeInTheDocument());
    expect(screen.getByText('Drag cards')).toBeInTheDocument();
    expect(screen.getByText('working tree')).toBeInTheDocument();

    expect(within(branchSelect).getByRole('option', { name: 'main' })).toBeInTheDocument();
    expect(within(branchSelect).getByRole('option', { name: 'feature/pm-012' })).toBeInTheDocument();
    expect(within(branchSelect).getByRole('option', { name: 'release/old' })).toBeInTheDocument();
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/kanban/branch') {
        return Promise.resolve(response(branchLoadResult(featureItems, 'feature/pm-012', 'snapshot')));
      }
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main', 'feature/pm-012', 'release/old'] }));
      return Promise.resolve(response({}));
    });
    fireEvent.change(branchSelect, { target: { value: 'feature/pm-012' } });

    await waitFor(() => expect(screen.getByText('Feature item')).toBeInTheDocument());
    expect(screen.queryByText('Drag cards')).not.toBeInTheDocument();
    expect(screen.getByText('snapshot; writes copy into main')).toBeInTheDocument();
  });

  it('offers the current branch selector option even when no indexed item is on it', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/kanban/branch') return Promise.resolve(response(branchLoadResult([], 'main')));
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main', 'feature/pm-012'] }));
      return Promise.resolve(response({}));
    }));

    render(<KanbanPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    await waitFor(() => expect(screen.queryByText('Drag cards')).not.toBeInTheDocument());
    const branchSelect = await screen.findByRole('combobox', { name: 'Select Kanban branch' });

    expect(within(branchSelect).getByRole('option', { name: 'main' })).toBeInTheDocument();
    expect(within(branchSelect).getByRole('option', { name: 'feature/pm-012' })).toBeInTheDocument();
  });

  it('moves status optimistically and reconciles the returned item', async () => {
    const fetchMock = statusFetchMock(async () => response({
      item: { ...draftItem, status: 'review', title: 'Persisted title', documents: [], metadata: {}, counts: { files: 1 } },
      scannedAt: '2026-06-23T00:00:00Z'
    }));
    const onWorkspacesChanged = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    render(<KanbanPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={onWorkspacesChanged} />);
    await screen.findByText('Drag cards');

    selectCardStatus('Review');

    expect(within(column('Review')).getByText('Drag cards')).toBeInTheDocument();
    await waitFor(() => expect(within(column('Review')).getByText('Persisted title')).toBeInTheDocument());
    expect(fetchMock).toHaveBeenCalledWith('/api/items/p1/status', expect.objectContaining({ method: 'PATCH' }));
    expect(onWorkspacesChanged).toHaveBeenCalledOnce();
  });

  it('rolls back the item when status persistence fails', async () => {
    vi.stubGlobal('fetch', statusFetchMock(async () => response({ error: 'Status update failed' }, false, 500)));
    render(<KanbanPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
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
    render(<KanbanPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
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
    render(<KanbanPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
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
    render(<KanbanPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
    await screen.findByText('Drag cards');

    selectCardStatus('Review');

    await waitFor(() => expect(fetchMock.mock.calls.filter(([url]) => isItemStatusUrl(url))).toHaveLength(0));
    expect(within(column('Draft')).getByText('Drag cards')).toBeInTheDocument();
  });

  it('shows materialization conflict errors and rolls status back', async () => {
    const snapshotItem = { ...draftItem, sourceMode: 'snapshot' as const, editable: false };
    vi.stubGlobal('confirm', vi.fn(() => true));
    vi.stubGlobal('fetch', statusFetchMock(async () => response({ error: 'Target file already exists' }, false, 409), snapshotItem));
    render(<KanbanPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
    await screen.findByText('Drag cards');

    selectCardStatus('Review');

    await waitFor(() => expect(within(column('Draft')).getByText('Drag cards')).toBeInTheDocument());
    expect(screen.getByText('Target file already exists')).toBeInTheDocument();
  });
});

function column(name: string): HTMLElement {
  const element = screen.getByRole('heading', { name }).closest('.kanban-column');
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
    if (url === '/api/workspaces/r1/kanban/branch') return Promise.resolve(response(branchLoadResult([item], item.branch, item.sourceMode)));
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

function branchLoadResult(items: ItemSummary[], branch: string, sourceMode: SourceMode = 'working_tree'): BranchLoadResult {
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

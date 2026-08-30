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
  itemPath: 'items/platform/PM-012',
  metadataRevision: 'revision-1'
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
      if (url === '/api/workspaces/r1/workstream/checkout') return Promise.resolve(response(workstreamBranchLoadResult([], 'main')));
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

    // The new item lands on the board, highlighted; it does not navigate away.
    await waitFor(() => expect(screen.queryByLabelText('Item name')).not.toBeInTheDocument());
    const card = await screen.findByText('Jira First Workspace');
    expect(card.closest('.plan-card')).toHaveClass('just-created');
    expect(onOpenPlan).not.toHaveBeenCalled();
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
      if (url === '/api/workspaces/r1/workstream/checkout') return Promise.resolve(response(workstreamBranchLoadResult([], 'main')));
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

  it('aborts a pending Jira intake lookup when the dialog closes', async () => {
    let jiraInit: RequestInit | undefined;
    let resolveJira: ((value: Response) => void) | undefined;
    const pendingJira = new Promise<Response>((resolve) => { resolveJira = resolve; });
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/checkout') return Promise.resolve(response(workstreamBranchLoadResult([], 'main')));
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main'] }));
      if (url === '/api/workspaces/r1/jira/issues/PM-025') { jiraInit = init; return pendingJira; }
      return Promise.resolve(response([]));
    }));
    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
    fireEvent.click(await screen.findByRole('button', { name: /\+ New Work Item/i }));
    fireEvent.click(screen.getByRole('button', { name: 'From Jira' }));
    fireEvent.change(screen.getByLabelText('Jira key'), { target: { value: 'PM-025' } });
    fireEvent.click(screen.getByRole('button', { name: /Fetch Jira/i }));
    await waitFor(() => expect(jiraInit?.signal).toBeDefined());
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(jiraInit?.signal?.aborted).toBe(true);
    await act(async () => { resolveJira?.(response({ state: 'available', issue: { key: 'PM-025', summary: 'late', status: 'Open', description: '', issueType: 'Story', labels: [], browserUrl: '', attachments: [] } })); });
    expect(screen.queryByText('PM-025: late')).not.toBeInTheDocument();
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
      if (url === '/api/workspaces/r1/workstream/checkout') return Promise.resolve(response(workstreamBranchLoadResult([docsItem], 'main')));
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

  // Wiki folders are indexed through source-structure settings, whose default card
  // tags every item with its own source name. The card already shows that name as
  // the source badge, so rendering the tag too printed "wiki" twice.
  it('does not repeat the source badge as a tag on settings-derived cards', async () => {
    const wikiItem: ItemSummary = {
      id: 'wiki-master-data',
      workspaceId: 'r1',
      workspaceName: 'Discovery',
      branch: 'main',
      scope: 'wiki',
      identifier: 'master-data',
      title: 'Master Data',
      status: 'unsorted',
      author: 'Khoa Đăng Vũ',
      tags: ['wiki', 'reference'],
      updatedAt: '2026-05-28T00:00:00Z',
      metadataSource: 'workspace-settings',
      itemPath: 'wiki/master-data'
    };
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/checkout') return Promise.resolve(response(workstreamBranchLoadResult([wikiItem], 'main')));
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'main', branches: ['main'] }));
      return Promise.resolve(response({}));
    }));

    render(<WorkstreamPage workspace={{ ...workspace, sources: ['items', 'wiki'] }} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    await screen.findByRole('button', { name: 'Master Data' });
    const card = document.querySelector('.plan-card');
    expect(card).toBeInstanceOf(HTMLElement);
    expect(within(card as HTMLElement).getAllByText('wiki')).toHaveLength(1);
    expect(within(card as HTMLElement).getByText('wiki')).toHaveClass('source-badge');
    // Tags that say something the card does not already show must survive.
    expect(within(card as HTMLElement).getByText('reference')).toBeInTheDocument();
  });

  it('shows only configured Workstream status columns', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/checkout') return Promise.resolve(response(workstreamBranchLoadResult([draftItem], 'main')));
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
      if (url === '/api/workspaces/r1/workstream/checkout') return Promise.resolve(response(workstreamBranchLoadResult([draftItem], 'main')));
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
      if (url === '/api/workspaces/r1/workstream/checkout') return Promise.resolve(response(workstreamBranchLoadResult([draftItem], 'main')));
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

  it('shows the checkout branch without an operational branch selector', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/checkout') return Promise.resolve(response(workstreamBranchLoadResult([draftItem], 'checkout/current', 'working_tree', 'checkout/current')));
      if (url === '/api/saved-filters') return Promise.resolve(response([]));
      if (url === '/api/workspaces/r1/git/status') return Promise.resolve(response({ workspaceId: 'r1', branch: 'checkout/current', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/r1/git/branches') return Promise.resolve(response({ workspaceId: 'r1', current: 'checkout/current', branches: ['main', 'checkout/current'] }));
      return Promise.resolve(response({}));
    }));

    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    expect(await screen.findByText('Drag cards')).toBeInTheDocument();
    expect(screen.getByLabelText('Workspace context')).toHaveTextContent('Checkoutcheckout/current');
    expect(screen.queryByRole('button', { name: 'Select board branch' })).not.toBeInTheDocument();
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
		const statusCall = fetchMock.mock.calls.find(([url]) => String(url) === '/api/items/p1/status');
		expect(JSON.parse(String(statusCall?.[1]?.body))).toMatchObject({ expectedRevision: 'revision-1' });
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

});

function column(name: string): HTMLElement {
  const element = screen.getByRole('heading', { name }).closest('.workstream-column');
  if (!element) throw new Error(`Missing ${name} column`);
  return element as HTMLElement;
}

function selectCardStatus(status: string): void {
  fireEvent.click(screen.getByRole('button', { name: 'Move item status' }));
  fireEvent.click(screen.getByRole('menuitemradio', { name: status }));
}

function statusFetchMock(updateStatus: () => Promise<Response>, item: ItemSummary = draftItem) {
	return vi.fn((input: RequestInfo | URL, _init?: RequestInit) => {
    const url = String(input);
    if (url === '/api/workspaces/r1/workstream/checkout') return Promise.resolve(response(workstreamBranchLoadResult([item], item.branch, item.sourceMode)));
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

function workstreamBranchLoadResult(items: ItemSummary[], branch: string, sourceMode: SourceMode = 'working_tree', currentCheckoutBranch = 'main'): WorkstreamBranchLoadResult {
  return {
    workspaceId: 'r1',
    branch,
    selectedBranch: branch,
    branchRef: `refs/heads/${branch}`,
    commit: sourceMode === 'snapshot' ? 'abc123' : '',
    currentCheckoutBranch,
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

describe('Jira intake refetch', () => {
  const issue = (key: string, summary: string, assignee: string, priority: string) => ({
    state: 'available',
    issue: {
      key, summary, status: 'QA', description: `${key} description`, issueType: 'User-Story',
      assignee: { displayName: assignee }, reporter: { displayName: 'BA' }, priority,
      labels: [], browserUrl: `https://jira.example/browse/${key}`, attachments: []
    }
  });

  it('replaces the imported details when a different ticket is fetched', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/checkout') return Promise.resolve(response(workstreamBranchLoadResult([], 'main')));
      if (url === '/api/workspaces/r1/jira/issues/DI-512') return Promise.resolve(response(issue('DI-512', 'Team meetings', 'Thu Ho Linh Pham', 'Low')));
      if (url === '/api/workspaces/r1/jira/issues/DI-510') return Promise.resolve(response(issue('DI-510', 'Remove legacy ArticleIndex', 'Viet Van Nguyen', 'Medium')));
      return Promise.resolve(response([]));
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    fireEvent.click(await screen.findByRole('button', { name: /\+ New Work Item/i }));
    fireEvent.click(screen.getByRole('button', { name: 'From Jira' }));

    fireEvent.change(screen.getByLabelText('Jira key'), { target: { value: 'DI-512' } });
    fireEvent.click(screen.getByRole('button', { name: /Fetch Jira/i }));
    await waitFor(() => expect(screen.getByLabelText('Item name')).toHaveValue('DI-512'));
    expect(screen.getByLabelText('Title')).toHaveValue('Team meetings');
    expect(screen.getByLabelText('Owner')).toHaveValue('Thu Ho Linh Pham');

    fireEvent.change(screen.getByLabelText('Jira key'), { target: { value: 'DI-510' } });
    fireEvent.click(screen.getByRole('button', { name: /Fetch Jira/i }));

    await waitFor(() => expect(screen.getByLabelText('Item name')).toHaveValue('DI-510'));
    expect(screen.getByLabelText('Title')).toHaveValue('Remove legacy ArticleIndex');
    expect(screen.getByLabelText('Owner')).toHaveValue('Viet Van Nguyen');
  });
});

describe('Jira description formatting', () => {
  it('renders the description as formatted Markdown', async () => {
    const markdown = '## User Story\n\nRemove the obsolete Solr integration.\n\n## Acceptance Criteria\n\n- No connection to Solr.\n- `ArticleIndex` stays a DTO.';
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/workspaces/r1/workstream/checkout') return Promise.resolve(response(workstreamBranchLoadResult([], 'main')));
      if (url === '/api/workspaces/r1/jira/issues/DI-510') return Promise.resolve(response({
        state: 'available',
        issue: {
          key: 'DI-510', summary: 'Remove legacy ArticleIndex', status: 'QA', description: markdown,
          issueType: 'User-Story', assignee: { displayName: 'Viet Van Nguyen' }, priority: 'Medium',
          labels: [], browserUrl: 'https://jira.example/browse/DI-510', attachments: []
        }
      }));
      return Promise.resolve(response([]));
    }));

    render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    fireEvent.click(await screen.findByRole('button', { name: /\+ New Work Item/i }));
    fireEvent.click(screen.getByRole('button', { name: 'From Jira' }));
    fireEvent.change(screen.getByLabelText('Jira key'), { target: { value: 'DI-510' } });
    fireEvent.click(screen.getByRole('button', { name: /Fetch Jira/i }));

    const preview = await screen.findByLabelText('Jira issue preview');
    // Headings, list items and inline code survive as real elements rather
    // than being run together into one paragraph.
    await waitFor(() => expect(within(preview).getByRole('heading', { name: 'User Story' })).toBeInTheDocument());
    expect(within(preview).getByRole('heading', { name: 'Acceptance Criteria' })).toBeInTheDocument();
    expect(within(preview).getAllByRole('listitem')).toHaveLength(2);
    expect(preview.querySelector('code')?.textContent).toBe('ArticleIndex');
  });
});

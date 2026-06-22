import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { filterPlans, KanbanPage } from './KanbanPage';
import type { ItemSummary } from '../lib/types';

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

    expect(fetchMock.mock.calls.filter(([url]) => String(url).endsWith('/status'))).toHaveLength(1);
    await act(async () => resolveUpdate(response({
      item: { ...draftItem, status: 'review', documents: [], metadata: {}, counts: { files: 1 } },
      scannedAt: '2026-06-23T00:00:00Z'
    })));
    await waitFor(() => expect(within(column('Review')).getByText('Drag cards')).toBeInTheDocument());
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

function statusFetchMock(updateStatus: () => Promise<Response>) {
  return vi.fn((input: RequestInfo | URL) => {
    const url = String(input);
    if (url.startsWith('/api/items?')) return Promise.resolve(response([draftItem]));
    if (url === '/api/saved-filters') return Promise.resolve(response([]));
    if (url.endsWith('/status')) return updateStatus();
    return Promise.resolve(response({}));
  });
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

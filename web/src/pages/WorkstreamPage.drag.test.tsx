import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { WorkstreamBranchLoadResult, ItemSummary } from '../lib/types';
import { WorkstreamPage } from './WorkstreamPage';

const workspace = { id: 'workspace-1', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['plans', 'docs'], createdAt: '2026-06-23T00:00:00Z' };
const items: ItemSummary[] = [
  {
    id: 'item-1', workspaceId: workspace.id, workspaceName: workspace.name, branch: 'main', scope: 'platform', identifier: 'PM-012',
    title: 'Draggable item', status: 'draft', tags: [], metadataSource: 'plan.yaml', itemPath: 'plans/platform/PM-012'
  },
  {
    id: 'item-2', workspaceId: workspace.id, workspaceName: workspace.name, branch: 'main', scope: 'docs', identifier: 'docs',
    title: 'Unsorted docs', status: 'unsorted', tags: [], metadataSource: 'docs', itemPath: 'docs'
  },
  {
    id: 'item-3', workspaceId: workspace.id, workspaceName: workspace.name, branch: 'main', scope: 'docs', identifier: 'guide',
    title: 'Protected docs', status: 'draft', tags: [], metadataSource: 'docs', itemPath: 'docs/guide'
  }
];

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('Workstream card drag and drop', () => {
  it('marks only editable cards as draggable without rendering a drag control', async () => {
    vi.stubGlobal('fetch', boardFetchMock());
    renderPage();
    await screen.findByText('Draggable item');

    expect(cardFor('Draggable item')).toHaveAttribute('draggable', 'true');
    expect(cardFor('Unsorted docs')).toHaveAttribute('draggable', 'false');
    expect(cardFor('Protected docs')).toHaveAttribute('draggable', 'false');
    expect(within(cardFor('Draggable item')).queryByRole('button', { name: 'Drag card to another status' })).not.toBeInTheDocument();
    expect(within(cardFor('Protected docs')).queryByLabelText('Card cannot be dragged')).not.toBeInTheDocument();
  });

  it('moves a card through the shared optimistic status path after a valid native drop', async () => {
    const fetchMock = boardFetchMock();
    vi.stubGlobal('fetch', fetchMock);
    renderPage();
    await screen.findByText('Draggable item');

    const dataTransfer = createDataTransfer();
    act(() => {
      fireEvent.dragStart(cardFor('Draggable item'), { dataTransfer });
      fireEvent.dragOver(column('Review'), { dataTransfer });
      fireEvent.drop(column('Review'), { dataTransfer });
      fireEvent.dragEnd(cardFor('Draggable item'), { dataTransfer });
    });

    expect(within(column('Review')).getByText('Draggable item')).toBeInTheDocument();
    await waitFor(() => expect(fetchMock.mock.calls.filter(([url]) => isItemStatusUrl(url))).toHaveLength(1));
  });

  it('asks before materializing a snapshot card dropped onto another status', async () => {
    const snapshotItems = items.map((item) => item.id === 'item-1' ? { ...item, sourceMode: 'snapshot' as const, editable: false } : item);
    const confirm = vi.fn(() => true);
    vi.stubGlobal('confirm', confirm);
    const fetchMock = boardFetchMock(snapshotItems);
    vi.stubGlobal('fetch', fetchMock);
    renderPage();
    await screen.findByText('Draggable item');

    const dataTransfer = createDataTransfer();
    act(() => {
      fireEvent.dragStart(cardFor('Draggable item'), { dataTransfer });
      fireEvent.drop(column('Review'), { dataTransfer });
    });

    expect(confirm).toHaveBeenCalledWith(expect.stringContaining('copy the whole plan at plans/platform/PM-012 into the current checkout branch'));
    expect(within(column('Review')).getByText('Draggable item')).toBeInTheDocument();
    await waitFor(() => expect(fetchMock.mock.calls.filter(([url]) => isItemStatusUrl(url))).toHaveLength(1));
    expect(statusRequestBody(fetchMock)).toMatchObject({ status: 'review', materializeConfirmed: true });
  });

  it('leaves a snapshot card in place when drag/drop materialization is declined', async () => {
    const snapshotItems = items.map((item) => item.id === 'item-1' ? { ...item, sourceMode: 'snapshot' as const, editable: false } : item);
    vi.stubGlobal('confirm', vi.fn(() => false));
    const fetchMock = boardFetchMock(snapshotItems);
    vi.stubGlobal('fetch', fetchMock);
    renderPage();
    await screen.findByText('Draggable item');

    const dataTransfer = createDataTransfer();
    act(() => {
      fireEvent.dragStart(cardFor('Draggable item'), { dataTransfer });
      fireEvent.drop(column('Review'), { dataTransfer });
    });

    expect(within(column('Draft')).getByText('Draggable item')).toBeInTheDocument();
    expect(fetchMock.mock.calls.filter(([url]) => isItemStatusUrl(url))).toHaveLength(0);
  });

  it('treats same-column, outside, and protected drops as no-ops', async () => {
    const fetchMock = boardFetchMock();
    vi.stubGlobal('fetch', fetchMock);
    renderPage();
    await screen.findByText('Draggable item');

    const sameColumnTransfer = createDataTransfer();
    act(() => {
      fireEvent.dragStart(cardFor('Draggable item'), { dataTransfer: sameColumnTransfer });
      fireEvent.drop(column('Draft'), { dataTransfer: sameColumnTransfer });
      fireEvent.dragEnd(cardFor('Draggable item'), { dataTransfer: sameColumnTransfer });
    });

    const protectedTransfer = createDataTransfer();
    act(() => {
      fireEvent.dragStart(cardFor('Protected docs'), { dataTransfer: protectedTransfer });
      fireEvent.drop(column('Review'), { dataTransfer: protectedTransfer });
    });

    expect(fetchMock.mock.calls.filter(([url]) => isItemStatusUrl(url))).toHaveLength(0);
    expect(within(column('Draft')).getByText('Draggable item')).toBeInTheDocument();
  });

  it('clears the active drag state without writing when the interaction is cancelled', async () => {
    const fetchMock = boardFetchMock();
    vi.stubGlobal('fetch', fetchMock);
    renderPage();
    await screen.findByText('Draggable item');

    const dataTransfer = createDataTransfer();
    act(() => {
      fireEvent.dragStart(cardFor('Draggable item'), { dataTransfer });
    });
    expect(cardFor('Draggable item')).toHaveClass('dragging');

    act(() => {
      fireEvent.dragEnd(cardFor('Draggable item'), { dataTransfer });
    });

    expect(cardFor('Draggable item')).not.toHaveClass('dragging');
    expect(fetchMock.mock.calls.filter(([url]) => isItemStatusUrl(url))).toHaveLength(0);
  });

  it('suppresses the click emitted immediately after a completed drag', async () => {
    const fetchMock = boardFetchMock();
    vi.stubGlobal('fetch', fetchMock);
    renderPage();
    await screen.findByText('Draggable item');

    const dataTransfer = createDataTransfer();
    const card = cardFor('Draggable item');
    act(() => {
      fireEvent.dragStart(card, { dataTransfer });
      fireEvent.drop(column('Review'), { dataTransfer });
      fireEvent.dragEnd(card, { dataTransfer });
      fireEvent.click(card);
    });

    expect(fetchMock.mock.calls.filter(([url]) => String(url) === '/api/items/item-1')).toHaveLength(0);
    await waitFor(() => expect(within(column('Review')).getByText('Draggable item')).toBeInTheDocument());
  });

  it('allows normal preview selection after the post-drag suppression window expires', async () => {
    const fetchMock = boardFetchMock();
    vi.stubGlobal('fetch', fetchMock);
    vi.spyOn(window.performance, 'now').mockReturnValue(1000);
    renderPage();
    await screen.findByText('Draggable item');

    const dataTransfer = createDataTransfer();
    const card = cardFor('Draggable item');
    act(() => {
      fireEvent.dragStart(card, { dataTransfer });
      fireEvent.dragEnd(card, { dataTransfer });
    });

    vi.mocked(window.performance.now).mockReturnValue(1100);
    act(() => {
      fireEvent.click(card);
    });
    expect(fetchMock.mock.calls.filter(([url]) => String(url) === '/api/items/item-1')).toHaveLength(0);

    vi.mocked(window.performance.now).mockReturnValue(1400);
    act(() => {
      fireEvent.click(card);
    });
    await waitFor(() => expect(fetchMock.mock.calls.filter(([url]) => String(url) === '/api/items/item-1')).toHaveLength(1));
  });
});

function renderPage() {
  return render(<WorkstreamPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
}

function column(name: string): HTMLElement {
  const element = screen.getByRole('heading', { name }).closest('.workstream-column');
  if (!element) throw new Error(`Missing ${name} column`);
  return element as HTMLElement;
}

function cardFor(title: string): HTMLElement {
  const element = screen.getByText(title).closest('.plan-card');
  if (!element) throw new Error(`Missing ${title} card`);
  return element as HTMLElement;
}

function createDataTransfer(): DataTransfer {
  const data = new Map<string, string>();
  return {
    dropEffect: 'none',
    effectAllowed: 'all',
    files: [] as unknown as FileList,
    items: [] as unknown as DataTransferItemList,
    types: [],
    clearData: vi.fn((format?: string) => {
      if (format) data.delete(format);
      else data.clear();
    }),
    getData: vi.fn((format: string) => data.get(format) ?? ''),
    setData: vi.fn((format: string, value: string) => {
      data.set(format, value);
    }),
    setDragImage: vi.fn()
  };
}

function boardFetchMock(boardItems: ItemSummary[] = items) {
  return vi.fn((input: RequestInfo | URL) => {
    const url = String(input);
    if (url === '/api/workspaces/workspace-1/workstream/branch') return Promise.resolve(response(workstreamBranchLoadResult(boardItems)));
    if (url.startsWith('/api/items?')) return Promise.resolve(response(boardItems));
    if (url === '/api/saved-filters') return Promise.resolve(response([]));
    if (url === '/api/items/item-1') return Promise.resolve(response({ ...boardItems[0], documents: [], metadata: {}, counts: { files: 0 } }));
    if (url === '/api/items/item-1/files') return Promise.resolve(response([]));
    if (url === '/api/workspaces/workspace-1/git/status') return Promise.resolve(response({ branch: 'main', changes: [] }));
    if (url === '/api/workspaces/workspace-1/git/branches') return Promise.resolve(response({ workspaceId: 'workspace-1', current: 'main', branches: ['main'] }));
    if (isItemStatusUrl(url)) {
      return Promise.resolve(response({
        item: { ...boardItems[0], status: 'review', sourceMode: 'working_tree', editable: true, documents: [], metadata: {}, counts: { files: 1 } },
        scannedAt: '2026-06-23T00:00:00Z'
      }));
    }
    return Promise.resolve(response({}));
  });
}

function statusRequestBody(fetchMock: ReturnType<typeof vi.fn>): Record<string, unknown> {
  const call = fetchMock.mock.calls.find(([url]) => isItemStatusUrl(url));
  return JSON.parse(String(call?.[1]?.body ?? '{}')) as Record<string, unknown>;
}

function workstreamBranchLoadResult(items: ItemSummary[]): WorkstreamBranchLoadResult {
  return {
    workspaceId: workspace.id,
    branch: 'main',
    selectedBranch: 'main',
    branchRef: 'refs/heads/main',
    commit: '',
    currentCheckoutBranch: 'main',
    sourceMode: 'working_tree',
    mode: 'working_tree',
    editable: true,
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

function response(body: unknown): Response {
  return { ok: true, status: 200, json: async () => body } as Response;
}

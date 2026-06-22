import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ItemSummary } from '../lib/types';
import { KanbanPage } from './KanbanPage';

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
  vi.unstubAllGlobals();
});

describe('Kanban card drag and drop', () => {
  it('marks only editable cards as draggable and shows clear affordances', async () => {
    vi.stubGlobal('fetch', boardFetchMock());
    renderPage();
    await screen.findByText('Draggable item');

    expect(cardFor('Draggable item')).toHaveAttribute('draggable', 'true');
    expect(cardFor('Unsorted docs')).toHaveAttribute('draggable', 'false');
    expect(cardFor('Protected docs')).toHaveAttribute('draggable', 'false');
    expect(within(cardFor('Draggable item')).getByRole('button', { name: 'Drag card to another status' })).toHaveTextContent('Drag');
    expect(within(cardFor('Protected docs')).getByLabelText('Card cannot be dragged')).toHaveTextContent('Fixed');
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
    await waitFor(() => expect(fetchMock.mock.calls.filter(([url]) => String(url).endsWith('/status'))).toHaveLength(1));
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

    expect(fetchMock.mock.calls.filter(([url]) => String(url).endsWith('/status'))).toHaveLength(0);
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
    expect(fetchMock.mock.calls.filter(([url]) => String(url).endsWith('/status'))).toHaveLength(0);
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
});

function renderPage() {
  return render(<KanbanPage workspace={workspace} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);
}

function column(name: string): HTMLElement {
  const element = screen.getByRole('heading', { name }).closest('.kanban-column');
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

function boardFetchMock() {
  return vi.fn((input: RequestInfo | URL) => {
    const url = String(input);
    if (url.startsWith('/api/items?')) return Promise.resolve(response(items));
    if (url === '/api/saved-filters') return Promise.resolve(response([]));
    if (url.endsWith('/status')) {
      return Promise.resolve(response({
        item: { ...items[0], status: 'review', documents: [], metadata: {}, counts: { files: 1 } },
        scannedAt: '2026-06-23T00:00:00Z'
      }));
    }
    return Promise.resolve(response({}));
  });
}

function response(body: unknown): Response {
  return { ok: true, status: 200, json: async () => body } as Response;
}

import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { filterPlans, KanbanPage } from './KanbanPage';
import type { ItemSummary } from '../lib/types';

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

    render(<KanbanPage workspace={{ id: 'r1', name: 'Discovery', path: '/repo', baselineBranch: 'main', sources: ['items'], createdAt: new Date().toISOString() }} refreshKey={0} onOpenPlan={() => undefined} onWorkspacesChanged={() => undefined} />);

    expect(screen.getByRole('heading', { name: 'Unsorted' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Ideas' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Draft' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText('Item Manager')).toBeInTheDocument());
  });
});

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

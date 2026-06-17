import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { filterPlans, KanbanPage } from './KanbanPage';
import type { PlanSummary } from '../lib/types';

describe('KanbanPage', () => {
  it('renders status columns from cached plan summaries', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [
        {
          id: 'p1',
          repositoryId: 'r1',
          repositoryName: 'Discovery',
          branch: 'main',
          service: 'platform',
          ticket: 'PM-001',
          title: 'Plan Manager',
          status: 'draft',
          tags: ['readonly'],
          metadataSource: 'plan.yaml'
        }
      ]
    }));

    render(<KanbanPage repository={{ id: 'r1', name: 'Discovery', path: '/repo', baselineBranch: 'main', planDirectories: ['plans'], createdAt: new Date().toISOString() }} refreshKey={0} onOpenPlan={() => undefined} onRepositoriesChanged={() => undefined} />);

    expect(screen.getByRole('heading', { name: 'Ideas' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Draft' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText('Plan Manager')).toBeInTheDocument());
  });
});

describe('filterPlans', () => {
  const plans: PlanSummary[] = [
    {
      id: 'p1',
      repositoryId: 'r1',
      repositoryName: 'Discovery',
      branch: 'main',
      service: 'api',
      ticket: 'DI-1',
      title: 'API Plan',
      status: 'draft',
      author: 'Khoa',
      tags: [],
      metadataSource: 'plan.yaml'
    },
    {
      id: 'p2',
      repositoryId: 'r2',
      repositoryName: 'Docs',
      branch: 'feature/docs',
      service: 'docs',
      ticket: 'docs',
      title: 'Docs',
      status: 'done',
      author: 'Giang',
      tags: ['docs'],
      metadataSource: 'docs'
    }
  ];

  it('uses OR within a facet', () => {
    const result = filterPlans(plans, { statuses: [], branches: [], authors: [] }, '');
    expect(result.map((plan) => plan.id)).toEqual(['p1', 'p2']);
  });

  it('uses AND across facets', () => {
    const result = filterPlans(plans, { statuses: ['done'], branches: [], authors: ['Giang'] }, '');
    expect(result.map((plan) => plan.id)).toEqual(['p2']);
  });
});

import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { KanbanPage } from './KanbanPage';

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

    render(<KanbanPage repositories={[{ id: 'r1', name: 'Discovery', path: '/repo', baselineBranch: 'main', planDirectories: ['plans'], createdAt: new Date().toISOString() }]} query="" onOpenPlan={() => undefined} onRepositoriesChanged={() => undefined} />);

    expect(screen.getByRole('heading', { name: 'Ideas' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Draft' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText('Plan Manager')).toBeInTheDocument());
  });
});

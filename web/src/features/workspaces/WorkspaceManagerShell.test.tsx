import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { WorkspaceConfig } from '../../lib/types';
import { filterWorkspaces, WorkspaceList } from './WorkspaceManagerShell';

const workspaces: WorkspaceConfig[] = [
  { id: 'one', name: 'Discovery', path: '/repos/discovery', baselineBranch: 'main', sources: ['plans', 'docs'], createdAt: '', lastScannedAt: '2026-01-01' },
  { id: 'two', name: 'Payments', path: '/repos/payments', baselineBranch: 'main', sources: ['plans'], createdAt: '' }
];

describe('WorkspaceList', () => {
  it('filters by workspace name and path', () => {
    expect(filterWorkspaces(workspaces, 'payment')).toEqual([workspaces[1]]);
    expect(filterWorkspaces(workspaces, '/repos/discovery')).toEqual([workspaces[0]]);
  });

  it('selects a workspace without entering edit mode', () => {
    const onSelect = vi.fn();
    render(<WorkspaceList workspaces={workspaces} selectedWorkspaceId="one" selectedWorkspaceIds={[]} query="" bulkMode={false} onQueryChange={vi.fn()} onSelect={onSelect} onToggleBulkMode={vi.fn()} onToggleSelection={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: /Payments/ }));

    expect(onSelect).toHaveBeenCalledWith('two');
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('shows selection controls only in explicit bulk mode', () => {
    const onToggleSelection = vi.fn();
    const { rerender } = render(<WorkspaceList workspaces={workspaces} selectedWorkspaceId="one" selectedWorkspaceIds={[]} query="" bulkMode={false} onQueryChange={vi.fn()} onSelect={vi.fn()} onToggleBulkMode={vi.fn()} onToggleSelection={onToggleSelection} />);
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();

    rerender(<WorkspaceList workspaces={workspaces} selectedWorkspaceId="one" selectedWorkspaceIds={['two']} query="" bulkMode onQueryChange={vi.fn()} onSelect={vi.fn()} onToggleBulkMode={vi.fn()} onToggleSelection={onToggleSelection} />);
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select Discovery' }));

    expect(onToggleSelection).toHaveBeenCalledWith('one');
  });
});

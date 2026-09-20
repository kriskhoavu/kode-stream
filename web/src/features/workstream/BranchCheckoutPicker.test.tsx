import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { GitStatus } from '../../lib/types';
import { BranchCheckoutPicker } from './BranchCheckoutPicker';

describe('BranchCheckoutPicker Git status', () => {
  it('shows a green icon without visible status text when the checkout is in sync', () => {
    renderPicker(gitStatus({ upstream: 'origin/main', fetchedAt: new Date().toISOString() }));

    expect(screen.getByRole('img', { name: 'Git in sync with origin/main' })).toHaveClass('is-synced');
    expect(screen.queryByText('in sync')).not.toBeInTheDocument();
  });

  it('shows a warning icon without visible status text when the checkout has drifted', () => {
    renderPicker(gitStatus({ upstream: 'origin/main', behind: 2, fetchedAt: new Date().toISOString() }));

    expect(screen.getByRole('img', { name: 'Git not in sync with origin/main: 2 behind' })).toHaveClass('is-not-synced');
    expect(screen.queryByText('2 behind')).not.toBeInTheDocument();
  });
});

function renderPicker(status: GitStatus) {
  render(<BranchCheckoutPicker workspaceId="workspace-1" currentCheckoutBranch="main" branches={['main']} gitStatus={status} ariaLabel="Select checkout branch" listboxLabel="Checkout branches" onSwitched={vi.fn()} />);
}

function gitStatus(overrides: Partial<GitStatus>): GitStatus {
  return { workspaceId: 'workspace-1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [], ...overrides };
}

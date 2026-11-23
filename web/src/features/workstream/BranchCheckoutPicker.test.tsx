import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { GitStatus } from '../../lib/types';
import { BranchCheckoutPicker } from './BranchCheckoutPicker';

describe('BranchCheckoutPicker Git status', () => {
  it.each([
    ['in sync', 0, 'Git in sync with origin/main', 'is-synced', 'in sync'],
    ['drifted', 2, 'Git not in sync with origin/main: 2 behind', 'is-not-synced', '2 behind']
  ])('shows an icon without visible status text when the checkout is %s', (_state, behind, name, className, text) => {
    renderPicker(gitStatus({ upstream: 'origin/main', behind, fetchedAt: new Date().toISOString() }));

    expect(screen.getByRole('img', { name })).toHaveClass(className);
    expect(screen.queryByText(text)).not.toBeInTheDocument();
  });
});

function renderPicker(status: GitStatus) {
  render(<BranchCheckoutPicker workspaceId="workspace-1" currentCheckoutBranch="main" branches={['main']} gitStatus={status} ariaLabel="Select checkout branch" listboxLabel="Checkout branches" onSwitched={vi.fn()} />);
}

function gitStatus(overrides: Partial<GitStatus>): GitStatus {
  return { workspaceId: 'workspace-1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [], ...overrides };
}

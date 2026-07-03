import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { WorkspacesPage } from '../../pages/WorkspacesPage';

vi.mock('../../lib/api', () => ({
  api: { systemConfigPaths: vi.fn(), workspaceHealth: vi.fn() },
  ApiError: class ApiError extends Error { recoveryHint?: string }
}));

const workspace = {
  id: 'workspace-1',
  name: 'Discovery',
  path: '/repos/discovery',
  baselineBranch: 'main',
  sources: ['plans', 'docs'],
  createdAt: '2026-01-01T00:00:00Z',
  jira: { deploymentType: 'cloud' as const, baseUrl: 'https://example.atlassian.net', projectKey: 'DI', accountEmail: 'user@example.com', tokenEnvVar: 'JIRA_TOKEN' }
};

describe('workspace detail settings', () => {
  beforeEach(() => {
    vi.mocked(api.workspaceHealth).mockImplementation(() => new Promise(() => undefined));
  });
  afterEach(() => vi.restoreAllMocks());

  it('groups general, health, and sources into collapsible Overview sections', () => {
    vi.mocked(api.systemConfigPaths).mockImplementation(() => new Promise(() => undefined));
    render(<WorkspacesPage workspaces={[workspace]} onChanged={vi.fn()} />);

    expect(screen.queryByRole('tab', { name: 'Health' })).not.toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Sources' })).not.toBeInTheDocument();
    expect(screen.getByLabelText('Workspace health')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'General' })).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('button', { name: 'Health' })).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('button', { name: 'Sources' })).toHaveAttribute('aria-expanded', 'true');
    const heading = screen.getByRole('button', { name: 'Remove' }).closest('header');
    expect(heading).toHaveClass('workspace-detail-heading');
    expect(heading?.querySelectorAll('button')).toHaveLength(2);
    expect(heading?.querySelectorAll('button')[0]).toHaveTextContent('Scan workspace');
    expect(heading?.querySelectorAll('button')[1]).toHaveTextContent('Remove');
    expect(screen.queryByText('Connect Jira')).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Configure structure' })).toHaveLength(2);
    expect(screen.getByText('docs')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Sources' }));
    expect(screen.queryByText('docs')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('tab', { name: 'Integrations' }));
    expect(screen.getByText('DI · Cloud')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Configure' })).toBeInTheDocument();
  });

  it('guards tab navigation while a settings draft is open', () => {
    vi.mocked(api.systemConfigPaths).mockImplementation(() => new Promise(() => undefined));
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    render(<WorkspacesPage workspaces={[workspace]} onChanged={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: 'Edit general' }));
    fireEvent.change(screen.getByLabelText('Workspace Name'), { target: { value: 'Changed' } });
    fireEvent.click(screen.getByRole('tab', { name: 'Integrations' }));

    expect(window.confirm).toHaveBeenCalledWith('Discard unsaved workspace changes?');
    expect(screen.getByRole('tab', { name: 'Overview' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByDisplayValue('Changed')).toBeInTheDocument();
  });

  it('supports arrow-key navigation between settings tabs', () => {
    vi.mocked(api.systemConfigPaths).mockImplementation(() => new Promise(() => undefined));
    render(<WorkspacesPage workspaces={[workspace]} onChanged={vi.fn()} />);

    fireEvent.keyDown(screen.getByRole('tab', { name: 'Overview' }), { key: 'ArrowRight' });

    expect(screen.getByRole('tab', { name: 'Integrations' })).toHaveAttribute('aria-selected', 'true');
  });

  it('reveals repository settings after location and keeps Jira in optional step two', () => {
    vi.mocked(api.systemConfigPaths).mockImplementation(() => new Promise(() => undefined));
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    render(<WorkspacesPage workspaces={[workspace]} onChanged={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: 'Add workspace' }));
    expect(screen.queryByLabelText('Workspace Name')).not.toBeInTheDocument();
    expect(screen.queryByText('Base Branch')).not.toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: 'Jira integration' })).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Local Path'), { target: { value: '/repos/new-repo' } });
    expect(screen.getByLabelText('Workspace Name')).toHaveValue('new-repo');
    fireEvent.change(screen.getByLabelText('Workspace Name'), { target: { value: 'Editable name' } });
    fireEvent.click(screen.getByRole('button', { name: /Next: Jira/ }));
    expect(screen.getByRole('checkbox', { name: 'Jira integration' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Register workspace' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Close add workspace' }));

    expect(window.confirm).toHaveBeenCalledWith('Discard this workspace registration draft?');
    expect(screen.getByRole('dialog', { name: 'Add workspace' })).toBeInTheDocument();
  });

  it('reveals clone settings only after a remote URL is provided', () => {
    vi.mocked(api.systemConfigPaths).mockImplementation(() => new Promise(() => undefined));
    render(<WorkspacesPage workspaces={[workspace]} onChanged={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: 'Add workspace' }));
    fireEvent.click(screen.getByRole('radio', { name: 'Remote Git URL' }));
    expect(screen.queryByText('Clone Root')).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Remote Git URL'), { target: { value: 'git@bitbucket.org:team/remote-repo.git' } });

    expect(screen.getByLabelText('Workspace Name')).toHaveValue('remote-repo');
    expect(screen.getByText('Clone Root')).toBeInTheDocument();
  });
});

import { fireEvent, render, screen, within } from '@testing-library/react';
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

  it('keeps overview focused on general and sources while health has its own tab', () => {
    vi.mocked(api.systemConfigPaths).mockImplementation(() => new Promise(() => undefined));
    render(<WorkspacesPage workspaces={[workspace]} onChanged={vi.fn()} />);

    expect(screen.getByRole('tab', { name: 'Health' })).toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Sources' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'General' })).toHaveAttribute('aria-expanded', 'true');
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

    fireEvent.click(screen.getByRole('tab', { name: 'Health' }));
    expect(screen.getByLabelText('Workspace health')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('tab', { name: 'Integrations' }));
    expect(screen.getByText('DI · Cloud')).toBeInTheDocument();
    const jiraCard = screen.getByText('Jira').closest('.workspace-integration-card');
    expect(jiraCard).not.toBeNull();
    expect(within(jiraCard as HTMLElement).getByRole('button', { name: 'Configure' })).toBeInTheDocument();
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

    expect(screen.getByRole('tab', { name: 'Health' })).toHaveAttribute('aria-selected', 'true');
    fireEvent.keyDown(screen.getByRole('tab', { name: 'Health' }), { key: 'ArrowRight' });

    expect(screen.getByRole('tab', { name: 'Integrations' })).toHaveAttribute('aria-selected', 'true');
  });

  it('opens only the selected integration settings and provides a back action', () => {
    vi.mocked(api.systemConfigPaths).mockImplementation(() => new Promise(() => undefined));
    render(<WorkspacesPage workspaces={[workspace]} onChanged={vi.fn()} />);

    fireEvent.click(screen.getByRole('tab', { name: 'Integrations' }));
    const knowledgeCard = screen.getByText('Knowledge').closest('.workspace-integration-card');
    expect(knowledgeCard).not.toBeNull();
    fireEvent.click(within(knowledgeCard as HTMLElement).getByRole('button', { name: 'Configure' }));

    expect(screen.getByRole('button', { name: 'Back' })).toBeInTheDocument();
    expect(screen.getByLabelText('Knowledge Wiki detection')).toBeInTheDocument();
    expect(screen.queryByLabelText('Jira integration')).not.toBeInTheDocument();
    expect(screen.queryByText('Runtime verification')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Back' }));
    const restoredKnowledgeCard = screen.getByText('Knowledge').closest('.workspace-integration-card');
    expect(restoredKnowledgeCard).not.toBeNull();
    expect(within(restoredKnowledgeCard as HTMLElement).getByRole('button', { name: 'Configure' })).toBeInTheDocument();
  });

  it('shows runtime verification and automation as sibling tabs', () => {
    vi.mocked(api.systemConfigPaths).mockImplementation(() => new Promise(() => undefined));
    render(<WorkspacesPage workspaces={[workspace]} onChanged={vi.fn()} />);

    fireEvent.click(screen.getByRole('tab', { name: 'Integrations' }));
    fireEvent.click(screen.getByRole('button', { name: 'Set runtime' }));

    expect(screen.getByRole('tab', { name: 'Runtime verification' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Automation tests' })).toHaveAttribute('aria-selected', 'false');
    fireEvent.click(screen.getByRole('checkbox', { name: 'Runtime verification' }));
    expect(screen.getByRole('radiogroup', { name: 'Runtime type' })).toBeInTheDocument();
    expect(screen.queryByText('Automation repository')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('tab', { name: 'Automation tests' }));

    expect(screen.getByRole('tab', { name: 'Automation tests' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByText('Automation repository')).toBeInTheDocument();
    expect(screen.queryByRole('radiogroup', { name: 'Runtime type' })).not.toBeInTheDocument();
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

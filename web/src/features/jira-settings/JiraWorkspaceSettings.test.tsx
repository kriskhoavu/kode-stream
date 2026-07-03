import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { WorkspacesPage } from '../../pages/WorkspacesPage';

vi.mock('../../lib/api', () => ({ api: {
  systemConfigPaths: vi.fn(), workspaceHealth: vi.fn(), testJiraConnection: vi.fn(), updateWorkspace: vi.fn()
}, ApiError: class ApiError extends Error { recoveryHint?: string } }));

describe('workspace Jira settings', () => {
  afterEach(() => vi.clearAllMocks());

  it('tests an existing workspace connection using an environment reference', async () => {
    vi.mocked(api.systemConfigPaths).mockResolvedValue({ dataDir: '/data', defaultDataDir: '/data', cloneRootDir: '/clone' });
    vi.mocked(api.workspaceHealth).mockResolvedValue({ workspaceId: 'w1', checkedAt: '', summary: 'ok', checks: [] });
    vi.mocked(api.testJiraConnection).mockResolvedValue({ ok: true, deploymentType: 'server', projectKey: 'DI', message: 'Jira connection succeeded' });
    const { container } = render(<WorkspacesPage workspaces={[{ id: 'w1', name: 'Repo', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '', jira: { deploymentType: 'server', baseUrl: 'https://jira.example.com', projectKey: 'DI', tokenEnvVar: 'JIRA_PAT' } }]} onChanged={vi.fn()} />);
    expect(screen.queryByText('Token Environment Variable')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('tab', { name: 'Integrations' }));
    fireEvent.click(screen.getByRole('button', { name: 'Configure' }));
    fireEvent.click(screen.getByRole('button', { name: 'Test Jira connection' }));
    await waitFor(() => expect(api.testJiraConnection).toHaveBeenCalledWith('w1', expect.objectContaining({ tokenEnvVar: 'JIRA_PAT' })));
    expect(await screen.findByText('Jira connection succeeded')).toBeInTheDocument();
    expect(container.querySelector('.jira-connection-status.success .jira-connection-status-dot')).toBeTruthy();
  });

  it('shows a failure dot when Jira test fails', async () => {
    vi.mocked(api.systemConfigPaths).mockResolvedValue({ dataDir: '/data', defaultDataDir: '/data', cloneRootDir: '/clone' });
    vi.mocked(api.workspaceHealth).mockResolvedValue({ workspaceId: 'w1', checkedAt: '', summary: 'ok', checks: [] });
    vi.mocked(api.testJiraConnection).mockRejectedValue(new Error('Jira unavailable'));
    const { container } = render(<WorkspacesPage workspaces={[{ id: 'w1', name: 'Repo', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '', jira: { deploymentType: 'server', baseUrl: 'https://jira.example.com', projectKey: 'DI', tokenEnvVar: 'JIRA_PAT' } }]} onChanged={vi.fn()} />);
    fireEvent.click(screen.getByRole('tab', { name: 'Integrations' }));
    fireEvent.click(screen.getByRole('button', { name: 'Configure' }));
    fireEvent.click(screen.getByRole('button', { name: 'Test Jira connection' }));
    expect(await screen.findByText('Jira unavailable')).toBeInTheDocument();
    expect(container.querySelector('.jira-connection-status.error .jira-connection-status-dot')).toBeTruthy();
  });
});

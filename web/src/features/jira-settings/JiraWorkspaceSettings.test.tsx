import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import { WorkspacesPage } from '../../pages/WorkspacesPage';

vi.mock('../../shared/api', () => ({ api: {
  systemConfigPaths: vi.fn(), workspaceHealth: vi.fn(), testJiraConnection: vi.fn(), updateWorkspace: vi.fn()
}, ApiError: class ApiError extends Error { recoveryHint?: string } }));

describe('workspace Jira settings', () => {
  afterEach(() => vi.clearAllMocks());

  it.each([
    ['succeeds', true, 'Jira connection succeeded', 'success'],
    ['fails', false, 'Jira unavailable', 'error']
  ])('tests an existing workspace connection by its environment reference, and shows when it %s', async (_result, ok, message, dot) => {
    vi.mocked(api.systemConfigPaths).mockResolvedValue({ dataDir: '/data', defaultDataDir: '/data', cloneRootDir: '/clone' });
    vi.mocked(api.workspaceHealth).mockResolvedValue({ workspaceId: 'w1', checkedAt: '', summary: 'ok', checks: [] });
    if (ok) vi.mocked(api.testJiraConnection).mockResolvedValue({ ok: true, deploymentType: 'server', projectKey: 'DI', message });
    else vi.mocked(api.testJiraConnection).mockRejectedValue(new Error(message));
    const { container } = render(<WorkspacesPage workspaces={[{ id: 'w1', name: 'Repo', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '', jira: { deploymentType: 'server', baseUrl: 'https://jira.example.com', projectKey: 'DI', tokenEnvVar: 'JIRA_PAT' } }]} onChanged={vi.fn()} showHarness={false} />);
    expect(screen.queryByText('Token Environment Variable')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('tab', { name: 'Integrations' }));
    fireEvent.click(within(screen.getByText('Jira').closest('.workspace-integration-card') as HTMLElement).getByRole('button', { name: 'Configure' }));
    fireEvent.click(screen.getByRole('button', { name: 'Test Jira connection' }));
    await waitFor(() => expect(api.testJiraConnection).toHaveBeenCalledWith('w1', expect.objectContaining({ tokenEnvVar: 'JIRA_PAT' })));
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(container.querySelector(`.jira-connection-status.${dot} .jira-connection-status-dot`)).toBeTruthy();
  });
});

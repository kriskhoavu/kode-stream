import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../lib/api';
import { WorkspacesPage } from './WorkspacesPage';

afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
});

describe('existing workspace import state', () => {
	it('renders Cloud Agent registration controls in Cloud mode', async () => {
		vi.spyOn(api, 'systemConfigPaths').mockResolvedValue({ dataDir: '/data', defaultDataDir: '/default', cloneRootDir: '/data/clones', registryFile: '/data/workspaces.yaml' });
		vi.spyOn(api, 'createAgentConnectToken').mockResolvedValue({ token: 'token', expiresAt: '2026-07-15T00:00:00Z', deepLink: 'kodestream://connect?token=token' });
		render(<WorkspacesPage workspaces={[]} runtimeContext={{ mode: 'cloud', role: 'editor', capabilities: { read: true, write: true, workspace_registration: true, git: true, system: false, terminal: true, ai: true, runtime: true, verification: true }, agent: { available: false, status: 'offline' } }} onChanged={vi.fn()} />);

		fireEvent.click(screen.getByRole('button', { name: 'Add workspace' }));

		expect(screen.getByLabelText('Cloud Agent connection')).toBeInTheDocument();
		expect(screen.queryByRole('radio', { name: 'Local Path' })).not.toBeInTheDocument();
		expect(screen.queryByRole('radio', { name: 'Remote Git URL' })).not.toBeInTheDocument();
		expect(screen.queryByRole('radio', { name: 'Existing Workspaces' })).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Reconnect agent' }));
		await waitFor(() => expect(api.createAgentConnectToken).toHaveBeenCalledWith(expect.objectContaining({ name: 'Cloud Agent' })));
		expect(await screen.findByRole('link', { name: 'Open deep link again' })).toHaveAttribute('href', 'kodestream://connect?token=token');
	});

	it('renders Cloud Agent workspace metadata without direct path reveal', () => {
		vi.spyOn(api, 'systemConfigPaths').mockResolvedValue({ dataDir: '/data', defaultDataDir: '/default', cloneRootDir: '/data/clones', registryFile: '/data/workspaces.yaml' });
		render(<WorkspacesPage workspaces={[{ id: 'cloud', name: 'Cloud Repo', path: '', location: 'cloud_agent', localRootLabel: '.../repo', remoteUrl: 'git@example.com:repo.git', agentId: 'agent-1', scanStatus: 'published', baselineBranch: 'main', sources: ['plans'], createdAt: '' }]} runtimeContext={{ mode: 'cloud', role: 'editor', capabilities: { read: true, write: true, workspace_registration: true, git: true, system: false, terminal: true, ai: true, runtime: true, verification: true }, agent: { available: true, status: 'connected' } }} onChanged={vi.fn()} />);

		expect(screen.getByText('Cloud Agent')).toBeInTheDocument();
		expect(screen.getAllByText('.../repo')).toHaveLength(2);
		expect(screen.getByText('agent-1')).toBeInTheDocument();
		expect(screen.getByText('published')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /Reveal folder/ })).not.toBeInTheDocument();
	});

	it('gates Cloud workspace commands when role or agent state disallows them', () => {
		vi.spyOn(api, 'systemConfigPaths').mockResolvedValue({ dataDir: '/data', defaultDataDir: '/default', cloneRootDir: '/data/clones', registryFile: '/data/workspaces.yaml' });
		render(<WorkspacesPage workspaces={[{ id: 'cloud', name: 'Cloud Repo', path: '', location: 'cloud_agent', localRootLabel: '.../repo', agentId: 'agent-1', scanStatus: 'offline', baselineBranch: 'main', sources: ['plans'], createdAt: '' }]} runtimeContext={{ mode: 'cloud', role: 'viewer', capabilities: { read: true, write: false, workspace_registration: false, git: true, system: false, terminal: false, ai: false, runtime: false, verification: false }, agent: { available: false, status: 'offline' } }} onChanged={vi.fn()} />);

		expect(screen.getByRole('button', { name: 'Add workspace' })).toBeDisabled();
		expect(screen.getByRole('button', { name: 'Scan all' })).toBeDisabled();
		expect(screen.getByRole('button', { name: 'Scan workspace' })).toBeDisabled();
		expect(screen.getByRole('button', { name: 'Remove' })).toBeDisabled();
		expect(screen.getByRole('button', { name: 'Configure structure' })).toBeDisabled();
		expect(screen.getByRole('button', { name: 'Edit sources' })).toBeDisabled();
	});

	it('keeps manual input on picker cancellation and retries a failed preview', async () => {
		vi.spyOn(api, 'systemConfigPaths').mockResolvedValue({ dataDir: '/data', defaultDataDir: '/default', cloneRootDir: '/data/clones', registryFile: '/data/workspaces.yaml' });
		vi.spyOn(api, 'selectYAMLFile').mockResolvedValue({ path: '' });
		vi.spyOn(api, 'previewWorkspaceImport')
			.mockRejectedValueOnce(new Error('source is unreadable'))
			.mockResolvedValueOnce({
				sourcePath: '/source/workspaces.yaml', destinationPath: '/data/workspaces.yaml', sourceFingerprint: 'hash',
				summary: { valid: 1, invalid: 1, duplicate: 0, alreadyRegistered: 0 },
				candidates: [
					{ candidateKey: 'valid', position: 1, status: 'valid', selected: true, issues: [], workspace: { name: 'Valid', path: '/valid', baselineBranch: 'main', sources: ['plans'] } },
					{ candidateKey: 'invalid', position: 2, status: 'invalid', selected: false, issues: [{ field: 'path', code: 'invalid_path', message: 'invalid' }], workspace: { name: 'Invalid', path: '/invalid', baselineBranch: 'main', sources: ['plans'] } }
				]
			});

		render(<WorkspacesPage workspaces={[]} onChanged={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Add workspace' }));
		fireEvent.click(screen.getByRole('radio', { name: 'Existing Workspaces' }));
		const sourceInput = screen.getByLabelText('Import source path');
		fireEvent.change(sourceInput, { target: { value: '/manual/workspaces.yaml' } });
		fireEvent.click(screen.getByTitle('Select YAML file'));
		await waitFor(() => expect(api.selectYAMLFile).toHaveBeenCalled());
		expect(sourceInput).toHaveValue('/manual/workspaces.yaml');

		fireEvent.click(screen.getByRole('button', { name: 'Preview workspaces' }));
		expect(await screen.findByRole('alert')).toHaveTextContent('source is unreadable');
		fireEvent.click(screen.getByRole('button', { name: 'Preview workspaces' }));
		expect(await screen.findByText('1 workspace ready to import')).toBeInTheDocument();
		expect(screen.getByText('1 selected from 2 candidates.')).toBeInTheDocument();
	});

	it('clears stale preview when the source path changes', async () => {
		vi.spyOn(api, 'systemConfigPaths').mockResolvedValue({ dataDir: '/data', defaultDataDir: '/default', cloneRootDir: '/data/clones', registryFile: '/data/workspaces.yaml' });
		vi.spyOn(api, 'previewWorkspaceImport').mockResolvedValue({
			sourcePath: '/one.yaml', destinationPath: '/data/workspaces.yaml', sourceFingerprint: 'hash',
			summary: { valid: 1, invalid: 0, duplicate: 0, alreadyRegistered: 0 },
			candidates: [{ candidateKey: 'one', position: 1, status: 'valid', selected: true, issues: [], workspace: { name: 'One', path: '/one', baselineBranch: 'main', sources: ['plans'] } }]
		});
		render(<WorkspacesPage workspaces={[]} onChanged={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Add workspace' }));
		fireEvent.click(screen.getByRole('radio', { name: 'Existing Workspaces' }));
		const sourceInput = screen.getByLabelText('Import source path');
		fireEvent.change(sourceInput, { target: { value: '/one.yaml' } });
		fireEvent.click(screen.getByRole('button', { name: 'Preview workspaces' }));
		expect(await screen.findByText('1 workspace ready to import')).toBeInTheDocument();
		fireEvent.change(sourceInput, { target: { value: '/two.yaml' } });
		expect(screen.queryByText('1 workspace ready to import')).not.toBeInTheDocument();
	});

	it('does not repeat the path issue for already registered candidates', async () => {
		vi.spyOn(api, 'systemConfigPaths').mockResolvedValue({ dataDir: '/data', defaultDataDir: '/default', cloneRootDir: '/data/clones', registryFile: '/data/workspaces.yaml' });
		vi.spyOn(api, 'previewWorkspaceImport').mockResolvedValue({
			sourcePath: '/data/workspaces.yaml', destinationPath: '/data/workspaces.yaml', sourceFingerprint: 'hash',
			summary: { valid: 0, invalid: 0, duplicate: 0, alreadyRegistered: 1 },
			candidates: [{ candidateKey: 'existing', position: 1, status: 'already_registered', selected: false, issues: [{ field: 'path', code: 'already_registered', message: 'workspace path is already registered' }], workspace: { name: 'Existing', path: '/existing', baselineBranch: 'main', sources: ['plans'] } }]
		});
		render(<WorkspacesPage workspaces={[]} onChanged={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Add workspace' }));
		fireEvent.click(screen.getByRole('radio', { name: 'Existing Workspaces' }));
		fireEvent.change(screen.getByLabelText('Import source path'), { target: { value: '/data/workspaces.yaml' } });
		fireEvent.click(screen.getByRole('button', { name: 'Preview workspaces' }));

		expect(await screen.findByText('Already registered')).toBeInTheDocument();
		expect(screen.queryByText('workspace path is already registered')).not.toBeInTheDocument();
	});

	it('reviews mixed candidates, confirms selection, and renders mixed results', async () => {
		const onChanged = vi.fn();
		vi.spyOn(api, 'systemConfigPaths').mockResolvedValue({ dataDir: '/data', defaultDataDir: '/default', cloneRootDir: '/data/clones', registryFile: '/data/workspaces.yaml' });
		vi.spyOn(api, 'previewWorkspaceImport').mockResolvedValue({
			sourcePath: '/source/workspaces.yaml', destinationPath: '/effective/workspaces.yaml', sourceFingerprint: 'hash',
			summary: { valid: 2, invalid: 1, duplicate: 1, alreadyRegistered: 0 },
			candidates: [
				{ candidateKey: 'one', position: 1, status: 'valid', selected: true, issues: [], workspace: { name: 'One', path: '/one', baselineBranch: 'main', sources: ['plans'], jira: { deploymentType: 'cloud', baseUrl: 'https://jira.example.com', projectKey: 'ONE', accountEmail: 'one@example.com', tokenEnvVar: 'JIRA_TOKEN' } } },
				{ candidateKey: 'two', position: 2, status: 'valid', selected: false, issues: [], workspace: { name: 'Two', path: '/two', baselineBranch: 'develop', sources: ['docs'] } },
				{ candidateKey: 'bad', position: 3, status: 'invalid', selected: false, issues: [{ field: 'baselineBranch', code: 'invalid_branch', message: 'branch does not exist' }], workspace: { name: 'Bad', path: '/bad', baselineBranch: 'missing', sources: ['plans'] } },
				{ candidateKey: 'duplicate', position: 4, status: 'duplicate', selected: false, issues: [{ field: 'path', code: 'duplicate_source', message: 'path is repeated' }], workspace: { name: 'Duplicate', path: '/one', baselineBranch: 'main', sources: ['plans'] } }
			]
		});
		vi.spyOn(api, 'importWorkspaces').mockResolvedValue([
			{ candidateKey: 'one', status: 'indexed', workspace: { id: 'one', name: 'One', path: '/one', baselineBranch: 'main', sources: ['plans'], createdAt: '' }, message: 'workspace imported and indexed' },
			{ candidateKey: 'two', status: 'scan_failed', workspace: { id: 'two', name: 'Two', path: '/two', baselineBranch: 'develop', sources: ['docs'], createdAt: '' }, message: 'workspace was registered but indexing failed' },
			{ candidateKey: 'missing', status: 'skipped', message: 'candidate changed' },
			{ candidateKey: 'failed', status: 'failed', message: 'registry write failed' }
		]);

		render(<WorkspacesPage workspaces={[]} onChanged={onChanged} />);
		fireEvent.click(screen.getByRole('button', { name: 'Add workspace' }));
		fireEvent.click(screen.getByRole('radio', { name: 'Existing Workspaces' }));
		fireEvent.change(screen.getByLabelText('Import source path'), { target: { value: '/source/workspaces.yaml' } });
		fireEvent.click(screen.getByRole('button', { name: 'Preview workspaces' }));

		expect(await screen.findByText('2 workspaces ready to import')).toBeInTheDocument();
		expect(screen.getByText('branch does not exist')).toBeInTheDocument();
		expect(screen.getByText('Token env')).toBeInTheDocument();
		expect(screen.getByText('JIRA_TOKEN')).toBeInTheDocument();
		expect(screen.getByRole('checkbox', { name: /Bad/ })).toBeDisabled();
		fireEvent.click(screen.getByRole('button', { name: 'Select all valid' }));
		expect(screen.getByRole('button', { name: 'Import 2 selected' })).toBeEnabled();
		fireEvent.click(screen.getByRole('button', { name: 'Import 2 selected' }));
		expect(screen.getByRole('dialog', { name: 'Import selected workspaces' })).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Import 2 workspaces' }));

		expect(await screen.findByText('Import complete')).toBeInTheDocument();
		for (const label of ['Indexed', 'Scan failed', 'Skipped', 'Failed']) {
			expect(screen.getByText(label)).toBeInTheDocument();
		}
		expect(api.importWorkspaces).toHaveBeenCalledWith({ sourcePath: '/source/workspaces.yaml', candidateKeys: ['one', 'two'] });
		expect(onChanged).toHaveBeenCalledOnce();
	});
});

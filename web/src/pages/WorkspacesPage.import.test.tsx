import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../lib/api';
import { WorkspacesPage } from './WorkspacesPage';

afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
});

describe('existing workspace import state', () => {
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
});

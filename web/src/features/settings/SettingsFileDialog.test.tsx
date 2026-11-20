import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { SettingsFileView } from '../../lib/types';
import { SettingsFileDialog } from './SettingsFileDialog';

const apiMock = vi.hoisted(() => ({ settingsFile: vi.fn(), exportSettingsFile: vi.fn(), importSettingsFile: vi.fn() }));
vi.mock('../../shared/api', () => ({ api: apiMock }));

afterEach(cleanup);

const view: SettingsFileView = {
	path: '.kode-stream/settings.yaml', exists: true, has: ['tests', 'models'],
	file: { version: 1, harness: { tests: ['make verify'], models: { coder: { model: 'haiku' } } } },
	current: { version: 1, ai: { provider: 'claude' }, harness: { tests: ['go test ./...'], models: { coder: { engine: 'claudecode', model: 'sonnet' } } } },
	parts: ['provider', 'pipeline', 'tests', 'scope', 'models', 'procedure', 'sources']
};

describe('SettingsFileDialog', () => {
	it('exports the chosen parts', async () => {
		apiMock.settingsFile.mockResolvedValue(view);
		apiMock.exportSettingsFile.mockResolvedValue({ path: view.path, has: ['tests', 'models', 'provider'] });
		render(<SettingsFileDialog mode="export" workspaceId="sandbox" workspaceName="sandbox" onClose={vi.fn()} onImported={vi.fn()} />);
		expect(await screen.findByText('The file holds Test command, Models.')).toBeInTheDocument();

		const exportButton = screen.getByRole('button', { name: /Export to the file/ });
		expect(exportButton).toBeDisabled();
		fireEvent.click(within(screen.getByRole('group', { name: 'Parts to export' })).getByRole('checkbox', { name: /AI provider/ }));
		fireEvent.click(exportButton);
		await waitFor(() => expect(apiMock.exportSettingsFile).toHaveBeenCalledWith('sandbox', ['provider']));
		expect(await screen.findByText(/Commit it to share it/)).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Done' })).toBeInTheDocument();
	});

	it('imports the chosen parts, showing the file beside what the workspace has now', async () => {
		apiMock.settingsFile.mockResolvedValue(view);
		apiMock.importSettingsFile.mockResolvedValue({ imported: ['tests'] });
		const onImported = vi.fn();
		render(<SettingsFileDialog mode="import" workspaceId="sandbox" workspaceName="sandbox" onClose={vi.fn()} onImported={onImported} />);
		const importing = await screen.findByRole('group', { name: 'Parts to import' });
		expect(within(importing).getByText('File: make verify')).toBeInTheDocument();
		expect(within(importing).getByText('Now: go test ./...')).toBeInTheDocument();
		fireEvent.click(within(importing).getByRole('checkbox', { name: /Test command/ }));
		fireEvent.click(screen.getByRole('button', { name: /Import the chosen parts/ }));
		await waitFor(() => expect(apiMock.importSettingsFile).toHaveBeenCalledWith('sandbox', ['tests']));
		expect(onImported).toHaveBeenCalled();
	});

	it('refuses to import from a file that is not valid', async () => {
		apiMock.settingsFile.mockResolvedValue({ ...view, has: [], file: undefined, error: 'harness.models names an unknown role "boss"' });
		render(<SettingsFileDialog mode="import" workspaceId="sandbox" workspaceName="sandbox" onClose={vi.fn()} onImported={vi.fn()} />);
		expect(await screen.findByText(/The file is not valid, so nothing can be imported from it/)).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /Import the chosen parts/ })).not.toBeInTheDocument();
	});
});

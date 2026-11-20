import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { WorkspaceConfig } from '../../lib/types';
import { api } from '../../shared/api';
import { WorkspaceOwnSettings } from './WorkspaceOwnSettings';

vi.mock('../workstream/HarnessSettingsSection', () => ({ HarnessSettingsSection: ({ workspaceId }: { workspaceId?: string }) => <section aria-label={`Harness settings for ${workspaceId}`} /> }));
vi.mock('../settings/SettingsFilePanel', () => ({ SettingsFilePanel: ({ workspaceName }: { workspaceName: string }) => <section aria-label={`Settings file for ${workspaceName}`} /> }));
vi.mock('../../shared/api', async (importOriginal) => {
	const original = await importOriginal<typeof import('../../shared/api')>();
	return { ...original, api: { ...original.api, aiSettings: vi.fn(), aiCapabilities: vi.fn(), saveAISettings: vi.fn() } };
});

const workspace = { id: 'ws-1', name: 'Sandbox' } as WorkspaceConfig;

describe('WorkspaceOwnSettings', () => {
	afterEach(() => vi.clearAllMocks());

	it('sets the workspace its own provider and shows its harness settings and file', async () => {
		vi.mocked(api.aiSettings).mockResolvedValue({
			defaultProvider: 'claude', defaultTerminal: 'terminal',
			providers: { claude: { enabled: true, executable: 'claude', args: [] }, codex: { enabled: true, executable: 'codex', args: [] } },
			terminals: {}, workspaceProviders: {}
		});
		vi.mocked(api.aiCapabilities).mockResolvedValue([]);
		vi.mocked(api.saveAISettings).mockImplementation(async (value) => value);
		render(<WorkspaceOwnSettings workspace={workspace} />);

		const provider = await screen.findByRole('combobox', { name: 'Provider here' });
		expect(provider).toHaveDisplayValue('Default (claude)');
		expect(screen.getByRole('region', { name: 'Harness settings for ws-1' })).toBeInTheDocument();
		expect(screen.getByRole('region', { name: 'Settings file for Sandbox' })).toBeInTheDocument();
		fireEvent.change(provider, { target: { value: 'codex' } });
		fireEvent.click(screen.getByRole('button', { name: /Save provider/ }));
		await waitFor(() => expect(api.saveAISettings).toHaveBeenCalledWith(expect.objectContaining({ defaultProvider: 'claude', workspaceProviders: { 'ws-1': 'codex' } })));
	});

	it('leaves the harness out where it does not run', async () => {
		vi.mocked(api.aiSettings).mockResolvedValue({ defaultProvider: 'claude', defaultTerminal: '', providers: {}, terminals: {} });
		vi.mocked(api.aiCapabilities).mockResolvedValue([]);
		render(<WorkspaceOwnSettings workspace={workspace} showHarness={false} />);
		await screen.findByRole('combobox', { name: 'Provider here' });
		expect(screen.queryByRole('region', { name: /Harness settings/ })).not.toBeInTheDocument();
	});
});

import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { WorkspaceConfig } from '../../lib/types';
import { api } from '../../shared/api';
import { WorkspaceOwnSettings } from './WorkspaceOwnSettings';

vi.mock('../workstream/HarnessSettingsSection', () => ({ HarnessSettingsSection: ({ workspaceId, view }: { workspaceId?: string; view?: string }) => <section aria-label={`Harness settings for ${workspaceId}`} data-view={view} /> }));
vi.mock('../../shared/api', async (importOriginal) => {
	const original = await importOriginal<typeof import('../../shared/api')>();
	return { ...original, api: { ...original.api, aiSettings: vi.fn(), aiCapabilities: vi.fn(), saveAISettings: vi.fn() } };
});

const workspace = { id: 'ws-1', name: 'Sandbox' } as WorkspaceConfig;

describe('WorkspaceOwnSettings', () => {
	afterEach(() => vi.clearAllMocks());

	it('starts a workspace with Claude, sets it its own provider, and shows its harness settings', async () => {
		vi.mocked(api.aiSettings).mockResolvedValue({
			defaultProvider: 'codex', defaultTerminal: 'terminal',
			providers: { claude: { enabled: true, executable: 'claude', args: [] }, codex: { enabled: true, executable: 'codex', args: [] } },
			terminals: {}, workspaceProviders: {}
		});
		vi.mocked(api.aiCapabilities).mockResolvedValue([]);
		vi.mocked(api.saveAISettings).mockImplementation(async (value) => value);
		render(<WorkspaceOwnSettings workspace={workspace} />);

		const provider = await screen.findByRole('combobox', { name: 'Provider here' });
		expect(provider).toHaveDisplayValue('claude');
		expect(screen.getByRole('region', { name: 'Harness settings for ws-1' })).toBeInTheDocument();
		fireEvent.change(provider, { target: { value: 'codex' } });
		fireEvent.click(screen.getByRole('button', { name: /Save provider/ }));
		await waitFor(() => expect(api.saveAISettings).toHaveBeenCalledWith(expect.objectContaining({ defaultProvider: 'codex', workspaceProviders: { 'ws-1': 'codex' } })));
	});

	it('holds the provider and harness choices in one tab, the models and role skills in the other', async () => {
		vi.mocked(api.aiSettings).mockResolvedValue({ defaultProvider: 'claude', defaultTerminal: '', providers: { claude: { enabled: true, executable: 'claude', args: [] } }, terminals: {} });
		vi.mocked(api.aiCapabilities).mockResolvedValue([]);
		render(<WorkspaceOwnSettings workspace={workspace} />);
		await screen.findByRole('combobox', { name: 'Provider here' });
		expect(screen.getByRole('tab', { name: 'Tools and harness' })).toHaveAttribute('aria-selected', 'true');
		expect(screen.getByRole('region', { name: 'Harness settings for ws-1' })).toHaveAttribute('data-view', 'choices');
		fireEvent.click(screen.getByRole('tab', { name: 'Models and skills' }));
		expect(screen.queryByRole('combobox', { name: 'Provider here' })).not.toBeInTheDocument();
		expect(screen.getByRole('region', { name: 'Harness settings for ws-1' })).toHaveAttribute('data-view', 'models');
	});

	it('leaves the harness out where it does not run', async () => {
		vi.mocked(api.aiSettings).mockResolvedValue({ defaultProvider: 'claude', defaultTerminal: '', providers: {}, terminals: {} });
		vi.mocked(api.aiCapabilities).mockResolvedValue([]);
		render(<WorkspaceOwnSettings workspace={workspace} showHarness={false} />);
		await screen.findByRole('combobox', { name: 'Provider here' });
		expect(screen.queryByRole('region', { name: /Harness settings/ })).not.toBeInTheDocument();
		expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
	});
});

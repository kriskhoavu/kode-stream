import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { KnowledgeActions } from './KnowledgeActions';

vi.mock('../../lib/api', () => ({ api: { gitStatus: vi.fn() } }));

describe('KnowledgeActions', () => {
	beforeEach(() => { vi.clearAllMocks(); vi.mocked(api.gitStatus).mockResolvedValue({ workspaceId: 'ws', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }); });

	it('rescans and syncs a clean workspace without confirmation', async () => {
		const onRun = vi.fn().mockResolvedValue(undefined);
		render(<KnowledgeActions workspaceId="ws" root="docs" busy={false} result={null} onRun={onRun} />);
		fireEvent.click(screen.getByRole('button', { name: 'Rescan' }));
		fireEvent.click(screen.getByRole('button', { name: 'Sync' }));
		await waitFor(() => expect(onRun).toHaveBeenCalledWith('sync', false));
		expect(onRun).toHaveBeenCalledWith('rescan');
	});

	it('confirms dirty Sync and configured Enrich and shows bounded-log status', async () => {
		vi.mocked(api.gitStatus).mockResolvedValue({ workspaceId: 'ws', branch: 'main', ahead: 0, behind: 0, dirty: true, conflicted: false, changes: [] });
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		const onRun = vi.fn().mockResolvedValue(undefined);
		render(<KnowledgeActions workspaceId="ws" root="docs" settings={{ enrichExecutable: 'wiki-enrich', enrichArgs: ['--source', 'docs'] }} busy={false} result={{ ok: true, operation: 'enrich', wikis: [], warnings: [], log: 'done', logTruncated: true, completedAt: '' }} onRun={onRun} />);
		fireEvent.click(screen.getByRole('button', { name: 'Sync' }));
		fireEvent.click(screen.getByRole('button', { name: 'Enrich' }));
		await waitFor(() => expect(onRun).toHaveBeenCalledWith('sync', true));
		expect(onRun).toHaveBeenCalledWith('enrich', true);
		expect(screen.getByText('enrich completed')).toBeInTheDocument();
		expect(screen.getByText(/Action log \(truncated\)/)).toBeInTheDocument();
	});
});

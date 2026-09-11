import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import type { SafeSessionRecord, SessionChanges } from '../../lib/types';
import { SessionChangesPanel } from './SessionChangesPanel';

vi.mock('../../shared/api', async () => {
	const actual = await vi.importActual<typeof import('../../shared/api')>('../../shared/api');
	return { ...actual, api: { sessionChanges: vi.fn(), sessionFileDiff: vi.fn() } };
});

function record(overrides: Partial<SafeSessionRecord> = {}): SafeSessionRecord {
	return { id: 'session-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', requestedBranch: 'main', state: 'exited', startedAt: '', lastKnownAt: '', live: false, ...overrides };
}

function changes(overrides: Partial<SessionChanges> = {}): SessionChanges {
	return { sessionId: 'session-1', launchCommit: 'a', headCommit: 'b', headMoved: 2, files: [{ path: 'src/a.ts', status: 'M' }, { path: 'src/new.ts', status: 'A' }, { path: 'src/gone.ts', status: 'D' }, { path: 'src/scratch.ts', status: '?' }], additions: 5, deletions: 1, truncated: false, availability: 'available', ...overrides };
}

describe('SessionChangesPanel', () => {
	it('renders the header line, grouped files, and the shared-checkout note', async () => {
		vi.mocked(api.sessionChanges).mockResolvedValue(changes());
		render(<SessionChangesPanel session={record()} />);
		expect(await screen.findByText('+5 −1 · 4 files · HEAD moved 2 commits')).toBeInTheDocument();
		expect(screen.getByText('Changes since launch include work by any session in this checkout.')).toBeInTheDocument();
		expect(screen.getByText('Added')).toBeInTheDocument();
		expect(screen.getByText('Modified')).toBeInTheDocument();
		expect(screen.getByText('Deleted')).toBeInTheDocument();
		expect(screen.getByText('Untracked')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'src/scratch.ts' })).toBeInTheDocument();
	});

	it('uses singular wording for one file and one commit', async () => {
		vi.mocked(api.sessionChanges).mockResolvedValue(changes({ headMoved: 1, files: [{ path: 'a.ts', status: 'M' }] }));
		render(<SessionChangesPanel session={record()} />);
		expect(await screen.findByText('+5 −1 · 1 file · HEAD moved 1 commit')).toBeInTheDocument();
	});

	it('shows the truncation note when the file list is truncated', async () => {
		vi.mocked(api.sessionChanges).mockResolvedValue(changes({ truncated: true }));
		render(<SessionChangesPanel session={record()} />);
		expect(await screen.findByText(/Only the first files are shown/)).toBeInTheDocument();
	});

	it.each([
		['unsupported', undefined, "Changes are not available for this workspace's content provider."],
		['unavailable', 'workspace_missing', 'The workspace checkout is missing, so changes cannot be computed.'],
		['unavailable', 'launch_commit_unknown', "The session's launch commit is unknown, so changes cannot be computed."],
		['unavailable', 'launch_commit_missing', "The session's launch commit is no longer in the checkout's history, so changes cannot be computed."],
	] as const)('renders availability %s / %s as text', async (availability, reason, expectedText) => {
		vi.mocked(api.sessionChanges).mockResolvedValue({ sessionId: 'session-1', launchCommit: '', availability, reason, files: [], additions: 0, deletions: 0, headMoved: 0, headCommit: '', truncated: false } as unknown as SessionChanges);
		render(<SessionChangesPanel session={record()} />);
		expect(await screen.findByText(expectedText)).toBeInTheDocument();
	});

	it('renders an available-but-empty result whose optional fields are absent from the payload', async () => {
		vi.mocked(api.sessionChanges).mockResolvedValue({ sessionId: 'session-1', launchCommit: 'a', headCommit: 'b', availability: 'available' } as unknown as SessionChanges);
		render(<SessionChangesPanel session={record()} />);
		expect(await screen.findByText('+0 −0 · 0 files · HEAD moved 0 commits')).toBeInTheDocument();
	});

	it('shows a transient poll error as a banner above the still-rendered panel', async () => {
		vi.useFakeTimers();
		try {
			vi.mocked(api.sessionChanges).mockResolvedValueOnce(changes()).mockRejectedValueOnce(new Error('poll failed'));
			render(<SessionChangesPanel session={record({ live: true, state: 'running' })} />);
			await act(async () => { await vi.advanceTimersByTimeAsync(0); });
			expect(screen.getByText('+5 −1 · 4 files · HEAD moved 2 commits')).toBeInTheDocument();
			await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
			expect(screen.getByRole('alert')).toHaveTextContent('poll failed');
			expect(screen.getByText('+5 −1 · 4 files · HEAD moved 2 commits')).toBeInTheDocument();
		} finally {
			vi.useRealTimers();
		}
	});

	it('loads a unified diff for the selected file, guarded by a request token', async () => {
		vi.mocked(api.sessionChanges).mockResolvedValue(changes());
		let resolveFirst: (value: { path: string; diff: string; truncated: boolean }) => void = () => {};
		vi.mocked(api.sessionFileDiff)
			.mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve; }))
			.mockResolvedValueOnce({ path: 'src/new.ts', diff: '+new file', truncated: false });
		render(<SessionChangesPanel session={record()} />);
		fireEvent.click(await screen.findByRole('button', { name: 'src/a.ts' }));
		fireEvent.click(screen.getByRole('button', { name: 'src/new.ts' }));
		await waitFor(() => expect(screen.getByText('+new file')).toBeInTheDocument());
		resolveFirst({ path: 'src/a.ts', diff: 'stale', truncated: false });
		await new Promise((resolve) => setTimeout(resolve, 0));
		expect(screen.queryByText('stale')).not.toBeInTheDocument();
		expect(screen.getByText('+new file')).toBeInTheDocument();
	});
});

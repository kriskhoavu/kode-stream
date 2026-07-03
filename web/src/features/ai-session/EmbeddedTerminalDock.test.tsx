import { act, fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { EmbeddedTerminalDock } from './EmbeddedTerminalDock';
import { openEmbeddedSession } from './terminalSessions';

vi.mock('./EmbeddedTerminal', () => ({ EmbeddedTerminal: ({ title, visible, mode, onToggleMinimize, onToggleMaximize }: { title: string; visible: boolean; mode: string; onToggleMinimize: () => void; onToggleMaximize: () => void }) => <section aria-label={title} data-mode={mode} hidden={!visible}><button onClick={onToggleMinimize}>{mode === 'minimized' ? 'Restore test terminal' : 'Minimize test terminal'}</button><button onClick={onToggleMaximize}>Maximize test terminal</button><div>Live terminal output</div></section> }));

function session(id: string, workspaceId: string, itemId: string) {
	return { session: { id, itemId, workspaceId, provider: 'codex', intent: 'card_context' as const, state: 'running' as const, startedAt: '2026-07-03T00:00:00Z' }, grant: { sessionId: id, token: `token-${id}`, expiresAt: '2026-07-03T00:01:00Z' } };
}

describe('EmbeddedTerminalDock', () => {
	it('keeps and switches sessions from multiple workspaces', () => {
		render(<EmbeddedTerminalDock workspaces={[{ id: 'ws-1', name: 'Discovery', path: '/one', baselineBranch: 'main', sources: ['plans'], createdAt: '2026-07-03T00:00:00Z' }, { id: 'ws-2', name: 'Platform', path: '/two', baselineBranch: 'main', sources: ['plans'], createdAt: '2026-07-03T00:00:00Z' }]} />);
		act(() => { openEmbeddedSession(session('one', 'ws-1', 'PM-020')); openEmbeddedSession(session('two', 'ws-2', 'PM-021')); });
		expect(screen.getByRole('button', { name: /Discovery · codex · PM-020/ })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: /Platform · codex · PM-021/ })).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: /Discovery · codex · PM-020/ }));
		expect(screen.getByRole('region', { name: 'Discovery · codex · PM-020' })).toBeVisible();
	});

	it('fully collapses to a corner chip and restores the connected terminal', () => {
		render(<EmbeddedTerminalDock workspaces={[]} />);
		act(() => { openEmbeddedSession(session('one', 'ws-1', 'item-1')); });
		fireEvent.click(screen.getByRole('button', { name: 'Minimize test terminal' }));
		const restore = screen.getByRole('button', { name: 'Restore embedded terminal, 1 open session' });
		expect(restore).toBeVisible();
		expect(screen.getByText('Live terminal output')).not.toBeVisible();
		fireEvent.click(restore);
		expect(screen.getByRole('region', { name: 'Embedded terminal dock' }).parentElement).toHaveClass('terminal-mode-normal');
		expect(screen.getByText('Live terminal output')).toBeVisible();
	});
});

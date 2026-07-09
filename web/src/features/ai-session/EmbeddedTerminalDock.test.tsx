import { act, fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { EmbeddedTerminalDock } from './EmbeddedTerminalDock';
import { openEmbeddedSession } from './terminalSessions';

Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1440 });
Object.defineProperty(window, 'innerHeight', { configurable: true, value: 900 });

vi.mock('./EmbeddedTerminal', () => ({ EmbeddedTerminal: ({ title, subtitle, visible, mode, onToggleMinimize, onToggleMaximize, onToggleDockMode }: { title: string; subtitle: string; visible: boolean; mode: string; onToggleMinimize: () => void; onToggleMaximize: () => void; onToggleDockMode: () => void }) => <section aria-label={title} data-mode={mode} hidden={!visible}><span>{subtitle}</span><button onClick={onToggleMinimize}>Minimize test terminal</button><button onClick={onToggleMaximize}>Maximize test terminal</button><button onClick={onToggleDockMode}>Toggle dock mode</button><div>Live terminal output</div></section> }));

function session(id: string, workspaceId: string, itemId: string) {
	return { session: { id, itemId, itemIdentifier: itemId, itemTitle: `Title ${itemId}`, workspaceId, provider: 'codex', intent: 'card_context' as const, state: 'running' as const, startedAt: '2026-07-03T00:00:00Z' }, grant: { sessionId: id, token: `token-${id}`, expiresAt: '2026-07-03T00:01:00Z' } };
}

function pointerEvent(type: string, clientX: number) {
	const event = new Event(type, { bubbles: true }) as Event & { clientX: number };
	event.clientX = clientX;
	return event;
}

describe('EmbeddedTerminalDock', () => {
	it('keeps and switches sessions from multiple workspaces', () => {
		render(<EmbeddedTerminalDock workspaces={[{ id: 'ws-1', name: 'Discovery', path: '/one', baselineBranch: 'main', sources: ['plans'], createdAt: '2026-07-03T00:00:00Z' }, { id: 'ws-2', name: 'Platform', path: '/two', baselineBranch: 'main', sources: ['plans'], createdAt: '2026-07-03T00:00:00Z' }]} />);
		act(() => { openEmbeddedSession(session('one', 'ws-1', 'PM-020')); openEmbeddedSession(session('two', 'ws-2', 'PM-021')); });
		expect(screen.getByRole('button', { name: /Codex · PM-020/ })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: /Codex · PM-021/ })).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: /Codex · PM-020/ }));
		expect(screen.getByRole('region', { name: 'Codex terminal' })).toBeVisible();
		expect(screen.getByRole('region', { name: 'Codex terminal' })).toHaveTextContent('Discovery · PM-020');
	});

	it('fully collapses to a corner chip and restores the connected terminal', () => {
		render(<EmbeddedTerminalDock workspaces={[]} />);
		act(() => { openEmbeddedSession(session('one', 'ws-1', 'item-1')); });
		fireEvent.click(screen.getByRole('button', { name: 'Minimize test terminal' }));
		const restore = screen.getByRole('button', { name: 'Restore embedded terminal, 1 open session' });
		expect(restore).toBeVisible();
		expect(screen.getByText('Live terminal output')).not.toBeVisible();
		fireEvent.click(restore);
		expect(screen.getByRole('region', { name: 'Embedded terminal dock' })).toHaveClass('embedded-terminal-shell-floating');
		expect(screen.getByText('Live terminal output')).toBeVisible();
	});

	it('switches between floating and right-side panel modes without closing the session', () => {
		render(<EmbeddedTerminalDock workspaces={[]} />);
		act(() => { openEmbeddedSession(session('one', 'ws-1', 'item-1')); });
		fireEvent.click(screen.getByRole('button', { name: 'Toggle dock mode' }));
		expect(screen.getByRole('region', { name: 'Embedded terminal dock' })).toHaveClass('embedded-terminal-shell-side_panel');
		expect(screen.getByText('Live terminal output')).toBeVisible();
		fireEvent.click(screen.getByRole('button', { name: 'Toggle dock mode' }));
		expect(screen.getByRole('region', { name: 'Embedded terminal dock' })).toHaveClass('embedded-terminal-shell-floating');
	});

	it('resizes the right-side panel from its left edge', () => {
		render(<EmbeddedTerminalDock workspaces={[]} />);
		act(() => { openEmbeddedSession(session('one', 'ws-1', 'item-1')); });
		fireEvent.click(screen.getByRole('button', { name: 'Toggle dock mode' }));
		const dock = screen.getByRole('region', { name: 'Embedded terminal dock' });
		expect(dock).toHaveStyle({ width: '518px' });
		act(() => {
			screen.getByRole('button', { name: 'Resize right-side terminal panel' }).dispatchEvent(pointerEvent('pointerdown', 1000));
			window.dispatchEvent(pointerEvent('pointermove', 900));
			window.dispatchEvent(pointerEvent('pointerup', 900));
		});
		expect(dock).toHaveStyle({ width: '618px' });
	});

	it('falls back to maximized rendering when the viewport is too narrow for the side panel', () => {
		Object.defineProperty(window, 'innerWidth', { configurable: true, value: 640 });
		render(<EmbeddedTerminalDock workspaces={[]} />);
		act(() => { openEmbeddedSession(session('one', 'ws-1', 'item-1')); });
		fireEvent.click(screen.getByRole('button', { name: 'Toggle dock mode' }));
		expect(screen.getByRole('region', { name: 'Embedded terminal dock' })).toHaveClass('embedded-terminal-shell-maximized');
	});
});

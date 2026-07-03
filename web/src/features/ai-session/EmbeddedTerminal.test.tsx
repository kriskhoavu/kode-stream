import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { EmbeddedTerminal } from './EmbeddedTerminal';

const write = vi.fn();
const fit = vi.fn();
vi.mock('@xterm/xterm', () => ({ Terminal: class {
	cols = 80; rows = 24;
	loadAddon() {} open() {} focus() {} dispose() {}
	write = write;
	onData() { return { dispose() {} }; }
	onResize() { return { dispose() {} }; }
} }));
vi.mock('@xterm/addon-fit', () => ({ FitAddon: class { fit = fit; } }));
vi.mock('../../lib/api', () => ({ api: { cancelEmbeddedAISession: vi.fn() } }));

class TestSocket {
	static OPEN = 1;
	static instances: TestSocket[] = [];
	readyState = 1;
	onopen?: () => void;
	onmessage?: (event: { data: string }) => void;
	onclose?: () => void;
	send = vi.fn(); close = vi.fn();
	constructor(public url: string) { TestSocket.instances.push(this); }
}

const initial = { session: { id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context' as const, state: 'running' as const, startedAt: '2026-07-03T00:00:00Z' }, grant: { sessionId: 'session-1', token: 'secret', expiresAt: '2026-07-03T00:01:00Z' } };
const terminalProps = { visible: true, mode: 'normal' as const, title: 'Workspace · codex · item-1', onToggleMinimize: vi.fn(), onToggleMaximize: vi.fn() };

describe('EmbeddedTerminal', () => {
	afterEach(() => { TestSocket.instances = []; vi.clearAllMocks(); });

	it('connects, handles output and presents exit state', async () => {
		vi.stubGlobal('WebSocket', TestSocket);
		vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
		render(<EmbeddedTerminal initial={initial} {...terminalProps} onClose={vi.fn()} />);
		await waitFor(() => expect(TestSocket.instances).toHaveLength(1));
		const socket = TestSocket.instances[0];
		act(() => { socket.onopen?.(); socket.onmessage?.({ data: JSON.stringify({ type: 'output', data: btoa('hello'), encoding: 'base64' }) }); });
		expect(write).toHaveBeenCalled();
		act(() => { socket.onmessage?.({ data: JSON.stringify({ type: 'state', state: 'exited', exitCode: 0 }) }); });
		expect(await screen.findByRole('status')).toHaveTextContent('exited with code 0');
		expect(screen.getByRole('button', { name: 'Cancel session' })).toBeDisabled();
	});

	it('cancels explicitly and closes the socket during cleanup', async () => {
		vi.stubGlobal('WebSocket', TestSocket);
		vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
		vi.mocked(api.cancelEmbeddedAISession).mockResolvedValue({ ...initial.session, state: 'cancelled' });
		const view = render(<EmbeddedTerminal initial={initial} {...terminalProps} onClose={vi.fn()} />);
		await waitFor(() => expect(TestSocket.instances).toHaveLength(1));
		fireEvent.click(screen.getByRole('button', { name: 'Cancel session' }));
		await waitFor(() => expect(api.cancelEmbeddedAISession).toHaveBeenCalledWith('session-1'));
		view.unmount(); expect(TestSocket.instances[0].close).toHaveBeenCalled();
	});

	it('offers minimize and maximize controls and refits on mode changes', async () => {
		vi.stubGlobal('WebSocket', TestSocket);
		vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
		const onToggleMinimize = vi.fn(); const onToggleMaximize = vi.fn();
		const view = render(<EmbeddedTerminal initial={initial} {...terminalProps} onToggleMinimize={onToggleMinimize} onToggleMaximize={onToggleMaximize} onClose={vi.fn()} />);
		await waitFor(() => expect(fit).toHaveBeenCalled());
		fireEvent.click(screen.getByRole('button', { name: 'Minimize embedded terminal' }));
		fireEvent.click(screen.getByRole('button', { name: 'Maximize embedded terminal' }));
		expect(onToggleMinimize).toHaveBeenCalled(); expect(onToggleMaximize).toHaveBeenCalled();
		const calls = fit.mock.calls.length;
		view.rerender(<EmbeddedTerminal initial={initial} {...terminalProps} mode="maximized" onClose={vi.fn()} />);
		await waitFor(() => expect(fit.mock.calls.length).toBeGreaterThan(calls));
		expect(screen.getByRole('button', { name: 'Restore embedded terminal size' })).toBeInTheDocument();
	});

	it('becomes a non-modal live terminal in minimized mode', async () => {
		vi.stubGlobal('WebSocket', TestSocket);
		vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
		render(<EmbeddedTerminal initial={initial} {...terminalProps} mode="minimized" onClose={vi.fn()} />);
		await waitFor(() => expect(fit).toHaveBeenCalled());
		const dialog = screen.getByRole('dialog');
		expect(dialog).not.toHaveAttribute('aria-modal');
		expect(screen.getByRole('button', { name: 'Restore embedded terminal' })).toBeInTheDocument();
		expect(screen.getByLabelText('AI terminal output')).toBeVisible();
	});
});

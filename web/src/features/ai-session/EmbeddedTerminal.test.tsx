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

const initial = { session: { id: 'session-1', itemId: 'item-1', itemIdentifier: 'PM-020', itemTitle: 'Embedded terminal', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context' as const, state: 'running' as const, startedAt: '2026-07-03T00:00:00Z' }, grant: { sessionId: 'session-1', token: 'secret', expiresAt: '2026-07-03T00:01:00Z' } };
const terminalProps = { visible: true, mode: 'normal' as const, title: 'Codex terminal', subtitle: 'Discovery · PM-020', onToggleMinimize: vi.fn(), onToggleMaximize: vi.fn() };

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
		expect(screen.getByRole('heading', { name: 'Codex terminal' })).toBeInTheDocument();
		expect(screen.getByText(/Discovery · PM-020/)).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Cancel session' })).not.toBeInTheDocument();
	});

	it('confirms close, cancels the running session, and closes the socket during cleanup', async () => {
		vi.stubGlobal('WebSocket', TestSocket);
		vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		vi.mocked(api.cancelEmbeddedAISession).mockResolvedValue({ ...initial.session, state: 'cancelled' });
		const onClose = vi.fn();
		const view = render(<EmbeddedTerminal initial={initial} {...terminalProps} onClose={onClose} />);
		await waitFor(() => expect(TestSocket.instances).toHaveLength(1));
		fireEvent.click(screen.getByRole('button', { name: 'Close embedded terminal' }));
		expect(window.confirm).toHaveBeenCalled(); expect(onClose).toHaveBeenCalled();
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

});

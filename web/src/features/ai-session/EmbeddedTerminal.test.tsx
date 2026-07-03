import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { EmbeddedTerminal } from './EmbeddedTerminal';

const write = vi.fn();
vi.mock('@xterm/xterm', () => ({ Terminal: class {
	cols = 80; rows = 24;
	loadAddon() {} open() {} focus() {} dispose() {}
	write = write;
	onData() { return { dispose() {} }; }
	onResize() { return { dispose() {} }; }
} }));
vi.mock('@xterm/addon-fit', () => ({ FitAddon: class { fit() {} } }));
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

describe('EmbeddedTerminal', () => {
	afterEach(() => { TestSocket.instances = []; vi.clearAllMocks(); });

	it('connects, handles output and presents exit state', async () => {
		vi.stubGlobal('WebSocket', TestSocket);
		vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
		render(<EmbeddedTerminal initial={initial} onClose={vi.fn()} />);
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
		const view = render(<EmbeddedTerminal initial={initial} onClose={vi.fn()} />);
		await waitFor(() => expect(TestSocket.instances).toHaveLength(1));
		fireEvent.click(screen.getByRole('button', { name: 'Cancel session' }));
		await waitFor(() => expect(api.cancelEmbeddedAISession).toHaveBeenCalledWith('session-1'));
		view.unmount(); expect(TestSocket.instances[0].close).toHaveBeenCalled();
	});
});

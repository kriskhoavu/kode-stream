import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import type { GlobalSessionRow, GlobalSessionsResponse, SessionScope } from '../../lib/types';
import { useGlobalSessions } from './useGlobalSessions';

function row(overrides: Partial<GlobalSessionRow['record']> = {}): GlobalSessionRow {
	return {
		workspaceName: 'Workspace',
		checkoutBranch: 'main',
		planTitle: 'Plan One',
		record: {
			id: 'session-1',
			workspaceId: 'workspace-1',
			provider: 'codex',
			intent: 'card_context',
			requestedBranch: 'main',
			state: 'running',
			startedAt: '2026-01-01T00:00:00.000Z',
			lastKnownAt: '2026-01-01T00:00:00.000Z',
			live: true,
			...overrides
		}
	};
}

function response(rows: GlobalSessionRow[], truncated = false): GlobalSessionsResponse {
	return { rows, truncated };
}

describe('useGlobalSessions', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});
	afterEach(() => {
		vi.useRealTimers();
		vi.restoreAllMocks();
	});

	it('loads sessions for the given scope', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([row()]));
		const { result } = renderHook(() => useGlobalSessions('active'));
		await act(async () => { await Promise.resolve(); });
		expect(api.listSessions).toHaveBeenCalledWith('active');
		expect(result.current.rows).toHaveLength(1);
		expect(result.current.truncated).toBe(false);
	});

	it('polls every 5 seconds while a row is live, and stops once nothing is', async () => {
		vi.spyOn(api, 'listSessions')
			.mockResolvedValueOnce(response([row({ state: 'running', live: true })]))
			.mockResolvedValueOnce(response([row({ state: 'exited', live: false })]));
		renderHook(() => useGlobalSessions('active'));
		await act(async () => { await Promise.resolve(); });
		expect(api.listSessions).toHaveBeenCalledTimes(1);

		await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
		expect(api.listSessions).toHaveBeenCalledTimes(2);

		// The second response has nothing live, so no further poll should be scheduled.
		await act(async () => { await vi.advanceTimersByTimeAsync(10000); });
		expect(api.listSessions).toHaveBeenCalledTimes(2);
	});

	it('cancels polling on unmount', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([row({ state: 'running', live: true })]));
		const { unmount } = renderHook(() => useGlobalSessions('active'));
		await act(async () => { await Promise.resolve(); });
		expect(api.listSessions).toHaveBeenCalledTimes(1);
		unmount();
		await act(async () => { await vi.advanceTimersByTimeAsync(10000); });
		expect(api.listSessions).toHaveBeenCalledTimes(1);
	});

	it('drops a stale response when the scope changes mid-flight', async () => {
		let resolveActive: (value: GlobalSessionsResponse) => void = () => {};
		const activePromise = new Promise<GlobalSessionsResponse>((resolve) => { resolveActive = resolve; });
		vi.spyOn(api, 'listSessions').mockImplementation((scope) => scope === 'active' ? activePromise : Promise.resolve(response([row({ state: 'exited', live: false })])));
		const { result, rerender } = renderHook(({ scope }: { scope: SessionScope }) => useGlobalSessions(scope), { initialProps: { scope: 'active' } });
		rerender({ scope: 'all' });
		await act(async () => { await Promise.resolve(); });
		resolveActive(response([row({ state: 'running', live: true })]));
		await act(async () => { await Promise.resolve(); });
		// The stale "active" response must not overwrite the "all" scope's result.
		expect(result.current.rows[0]?.record.state).toBe('exited');
	});

	it('retries a failed poll with backoff while the last known rows had a live one', async () => {
		vi.spyOn(api, 'listSessions')
			.mockResolvedValueOnce(response([row({ state: 'running', live: true })]))
			.mockRejectedValueOnce(new Error('network down'))
			.mockRejectedValueOnce(new Error('network down'))
			.mockResolvedValueOnce(response([row({ state: 'exited', live: false })]));
		const { result } = renderHook(() => useGlobalSessions('active'));
		await act(async () => { await Promise.resolve(); });
		expect(api.listSessions).toHaveBeenCalledTimes(1);

		// The regular 5s poll fires next and fails; a live row is still the last
		// known state, so the first retry is scheduled 5s out.
		await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
		expect(api.listSessions).toHaveBeenCalledTimes(2);
		expect(result.current.error).toBe('network down');

		// Not yet -- the first retry needs the full 5s.
		await act(async () => { await vi.advanceTimersByTimeAsync(4000); });
		expect(api.listSessions).toHaveBeenCalledTimes(2);

		// The first retry fires and fails again; the second retry backs off to 10s.
		await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
		expect(api.listSessions).toHaveBeenCalledTimes(3);
		await act(async () => { await vi.advanceTimersByTimeAsync(9000); });
		expect(api.listSessions).toHaveBeenCalledTimes(3);

		// The second retry succeeds with nothing live, clearing the error and
		// stopping the retry loop.
		await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
		expect(api.listSessions).toHaveBeenCalledTimes(4);
		expect(result.current.error).toBe('');
		await act(async () => { await vi.advanceTimersByTimeAsync(20000); });
		expect(api.listSessions).toHaveBeenCalledTimes(4);
	});

	it('stops a session and reloads', async () => {
		vi.spyOn(api, 'listSessions').mockResolvedValue(response([row({ state: 'running', live: true })]));
		vi.spyOn(api, 'cancelEmbeddedAISession').mockResolvedValue({} as never);
		const { result } = renderHook(() => useGlobalSessions('active'));
		await act(async () => { await Promise.resolve(); });
		await act(async () => { await result.current.stop('session-1'); });
		expect(api.cancelEmbeddedAISession).toHaveBeenCalledWith('session-1');
		expect(api.listSessions).toHaveBeenCalledTimes(2);
	});
});

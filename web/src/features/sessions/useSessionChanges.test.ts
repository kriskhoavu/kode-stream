import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import type { SessionChanges } from '../../lib/types';
import { useSessionChanges } from './useSessionChanges';

vi.mock('../../shared/api', async () => {
	const actual = await vi.importActual<typeof import('../../shared/api')>('../../shared/api');
	return { ...actual, api: { sessionChanges: vi.fn() } };
});

function changes(overrides: Partial<SessionChanges> = {}): SessionChanges {
	return { sessionId: 'session-1', launchCommit: 'a', headCommit: 'b', headMoved: 1, files: [{ path: 'a.ts', status: 'M' }], additions: 1, deletions: 0, truncated: false, availability: 'available', ...overrides };
}

const flush = () => act(async () => { await vi.advanceTimersByTimeAsync(0); });

describe('useSessionChanges', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.clearAllMocks();
	});
	afterEach(() => vi.useRealTimers());

	it('loads changes once and does not poll when the session is not live', async () => {
		vi.mocked(api.sessionChanges).mockResolvedValue(changes());
		const { result } = renderHook(() => useSessionChanges('session-1', false));
		await flush();
		expect(result.current.changes).toBeDefined();
		expect(api.sessionChanges).toHaveBeenCalledTimes(1);
		await act(async () => { await vi.advanceTimersByTimeAsync(20000); });
		expect(api.sessionChanges).toHaveBeenCalledTimes(1);
	});

	it('polls every 5s while the session is live and stops on unmount', async () => {
		vi.mocked(api.sessionChanges).mockResolvedValue(changes());
		const { result, unmount } = renderHook(() => useSessionChanges('session-1', true));
		await flush();
		expect(result.current.changes).toBeDefined();
		expect(api.sessionChanges).toHaveBeenCalledTimes(1);
		await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
		expect(api.sessionChanges).toHaveBeenCalledTimes(2);
		await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
		expect(api.sessionChanges).toHaveBeenCalledTimes(3);
		unmount();
		await act(async () => { await vi.advanceTimersByTimeAsync(15000); });
		expect(api.sessionChanges).toHaveBeenCalledTimes(3);
	});

	it('stops polling once liveness turns false', async () => {
		vi.mocked(api.sessionChanges).mockResolvedValue(changes());
		const { result, rerender } = renderHook(({ live }) => useSessionChanges('session-1', live), { initialProps: { live: true } });
		await flush();
		expect(result.current.changes).toBeDefined();
		expect(api.sessionChanges).toHaveBeenCalledTimes(1);
		rerender({ live: false });
		await flush();
		expect(api.sessionChanges).toHaveBeenCalledTimes(2);
		await act(async () => { await vi.advanceTimersByTimeAsync(20000); });
		expect(api.sessionChanges).toHaveBeenCalledTimes(2);
	});

	it('keeps polling after a failed poll while the session stays live', async () => {
		vi.mocked(api.sessionChanges)
			.mockResolvedValueOnce(changes())
			.mockRejectedValueOnce(new Error('transient'))
			.mockResolvedValueOnce(changes({ headMoved: 2 }));
		const { result } = renderHook(() => useSessionChanges('session-1', true));
		await flush();
		expect(result.current.changes?.headMoved).toBe(1);
		await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
		expect(api.sessionChanges).toHaveBeenCalledTimes(2);
		expect(result.current.error).toBe('transient');
		// Even though the previous poll failed, the interval must still be
		// running for the next tick.
		await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
		expect(api.sessionChanges).toHaveBeenCalledTimes(3);
		expect(result.current.changes?.headMoved).toBe(2);
		expect(result.current.error).toBe('');
	});

	it('waits for a slow changes request before starting another poll', async () => {
		let finishFirst: (value: SessionChanges) => void = () => {};
		vi.mocked(api.sessionChanges)
			.mockImplementationOnce(() => new Promise((resolve) => { finishFirst = resolve; }))
			.mockResolvedValue(changes({ headMoved: 2 }));
		const { result } = renderHook(() => useSessionChanges('session-1', true));
		await act(async () => { await vi.advanceTimersByTimeAsync(15000); });
		expect(api.sessionChanges).toHaveBeenCalledTimes(1);
		await act(async () => finishFirst(changes()));
		expect(result.current.changes?.headMoved).toBe(1);
		await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
		expect(api.sessionChanges).toHaveBeenCalledTimes(2);
		expect(result.current.changes?.headMoved).toBe(2);
	});

	it('drops an in-flight response when the session id changes before it resolves', async () => {
		let resolveFirst: (value: SessionChanges) => void = () => {};
		vi.mocked(api.sessionChanges).mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve; }));
		vi.mocked(api.sessionChanges).mockResolvedValueOnce(changes({ sessionId: 'session-2', headMoved: 9 }));
		const { result, rerender } = renderHook(({ sessionId }) => useSessionChanges(sessionId, false), { initialProps: { sessionId: 'session-1' } });
		rerender({ sessionId: 'session-2' });
		await flush();
		expect(result.current.changes?.sessionId).toBe('session-2');
		expect(result.current.changes?.headMoved).toBe(9);
		resolveFirst(changes({ sessionId: 'session-1', headMoved: 1 }));
		await flush();
		expect(result.current.changes?.sessionId).toBe('session-2');
		expect(result.current.changes?.headMoved).toBe(9);
	});
});

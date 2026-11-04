import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiHarnessClient } from './harnessClient';
import { useHarness } from './useHarness';
import { pollMs, reconnectGraceMs, retryFeedMs, useHarnessEvents } from './useHarnessEvents';
import { status, task } from './testData';

class FakeEventSource {
	static instances: FakeEventSource[] = [];
	onopen: (() => void) | null = null;
	onmessage: ((message: { data: string }) => void) | null = null;
	onerror: (() => void) | null = null;
	listeners = new Map<string, () => void>();
	closed = false;
	constructor(public url: string, public init?: EventSourceInit) { FakeEventSource.instances.push(this); }
	addEventListener(name: string, listener: () => void) { this.listeners.set(name, listener); }
	close() { this.closed = true; }
	static latest() { return FakeEventSource.instances.at(-1) as FakeEventSource; }
}

function json(body: unknown) {
	return Promise.resolve(new Response(JSON.stringify(body), { headers: { 'content-type': 'application/json' } }));
}

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
	vi.useFakeTimers();
	FakeEventSource.instances = [];
	vi.stubGlobal('EventSource', FakeEventSource);
	fetchMock = vi.fn((path: string) => {
		const route = path.split('?')[0];
		if (route === '/api/harness/status') return json(status({ lastEventSeq: 7 }));
		if (route === '/api/harness/tasks') return json({ tasks: [task('A')] });
		if (route === '/api/harness/runs') return json({ runs: [] });
		if (route === '/api/harness/approvals') return json({ approvals: [] });
		return json({ groupBy: 'role', totalUsd: 0, agentRuns: 0, groups: [] });
	});
	vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

function statusCalls() {
	return fetchMock.mock.calls.filter(([path]) => String(path) === '/api/harness/status').length;
}

async function flush() {
	await act(async () => { await vi.advanceTimersByTimeAsync(0); });
}

describe('useHarnessEvents', () => {
	it('goes live on open, refreshes once, and folds each event into the state', async () => {
		const { result } = renderHook(() => { const harness = useHarness(apiHarnessClient); useHarnessEvents(harness); return harness; });
		await flush();
		const source = FakeEventSource.latest();
		expect(source.url).toBe('/api/harness/events');
		expect(source.init).toEqual({ withCredentials: true });
		expect(result.current.state.connection).toBe('connecting');
		const before = statusCalls();
		act(() => source.onopen?.());
		await flush();
		expect(result.current.state.connection).toBe('live');
		expect(statusCalls()).toBe(before + 1);
		act(() => source.onmessage?.({ data: JSON.stringify({ seq: 8, at: '', type: 'task.updated', task: task('A', { title: 'Renamed' }) }) }));
		expect(result.current.state.tasks['task-A'].title).toBe('Renamed');
		act(() => source.onmessage?.({ data: 'not json' }));
		expect(result.current.state.lastSeq).toBe(8);
	});

	it('says Reconnecting…, then polls every 10 s, then tries the feed again after the last event', async () => {
		const { result } = renderHook(() => { const harness = useHarness(apiHarnessClient); useHarnessEvents(harness); return harness; });
		await flush();
		act(() => FakeEventSource.latest().onopen?.());
		await flush();
		act(() => FakeEventSource.latest().onmessage?.({ data: JSON.stringify({ seq: 9, at: '', type: 'task.updated', task: task('A') }) }));
		act(() => FakeEventSource.latest().onerror?.());
		expect(result.current.state.connection).toBe('reconnecting');
		expect(FakeEventSource.latest().closed).toBe(true);
		await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
		FakeEventSource.latest().onerror?.();
		await act(async () => { await vi.advanceTimersByTimeAsync(reconnectGraceMs); });
		expect(result.current.state.connection).toBe('polling');
		const polled = statusCalls();
		await act(async () => { await vi.advanceTimersByTimeAsync(pollMs); });
		expect(statusCalls()).toBe(polled + 1);
		const opened = FakeEventSource.instances.length;
		await act(async () => { await vi.advanceTimersByTimeAsync(retryFeedMs); });
		expect(FakeEventSource.instances.length).toBeGreaterThan(opened);
		expect(FakeEventSource.latest().url).toBe('/api/harness/events?after=9');
		act(() => FakeEventSource.latest().onopen?.());
		await flush();
		expect(result.current.state.connection).toBe('live');
	});

	it('refetches a snapshot when the server says this subscriber fell behind', async () => {
		renderHook(() => { const harness = useHarness(apiHarnessClient); useHarnessEvents(harness); return harness; });
		await flush();
		const before = statusCalls();
		act(() => FakeEventSource.latest().listeners.get('reset')?.());
		await flush();
		expect(statusCalls()).toBe(before + 1);
	});
});

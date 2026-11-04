import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, api } from '.';

function json(body: unknown, status = 200) {
	return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
}

describe('harness api', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('sends a decision with its key and the revision it was made against', async () => {
		const fetchMock = vi.fn().mockResolvedValue(json({ id: 'a1', state: 'approved', revision: 2 }));
		vi.stubGlobal('fetch', fetchMock);
		await expect(api.approveHarnessGate({ id: 'a1', revision: 1 }, 'key-1', '  ')).resolves.toMatchObject({ state: 'approved' });
		const [path, init] = fetchMock.mock.calls[0];
		expect(path).toBe('/api/harness/approvals/a1/decide');
		expect(init.method).toBe('POST');
		expect(JSON.parse(init.body)).toEqual({ decision: 'approve', idempotencyKey: 'key-1', expectedRevision: 1 });
	});

	it('treats a decision someone else already made as done, and other conflicts as errors', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(json({ error: 'Already decided', code: 'already_decided' }, 409)));
		await expect(api.rejectHarnessGate({ id: 'a1', revision: 1 }, 'key-2', 'No')).resolves.toBeUndefined();
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(json({ error: 'Moved', code: 'conflict' }, 409)));
		await expect(api.approveHarnessGate({ id: 'a1', revision: 1 }, 'key-3')).rejects.toBeInstanceOf(ApiError);
	});

	it('never merges two identical decisions in flight', async () => {
		const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(json({ id: 'a1', state: 'approved', revision: 2 })));
		vi.stubGlobal('fetch', fetchMock);
		await Promise.all([api.approveHarnessGate({ id: 'a1', revision: 1 }, 'k'), api.approveHarnessGate({ id: 'a1', revision: 1 }, 'k')]);
		expect(fetchMock).toHaveBeenCalledTimes(2);
	});

	it('fills missing task lists so the screens never see null', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(json({ tasks: [{ id: 't1', source: { kind: 'jira', key: 'PM-1' }, title: 'One', state: 'ranked' }] })));
		const [task] = await api.harnessTasks();
		expect(task.dependsOn).toEqual([]);
		expect(task.scoreReasons).toEqual([]);
		expect(task.source.labels).toEqual([]);
	});

	it('builds the cost and event addresses', async () => {
		const fetchMock = vi.fn().mockResolvedValue(json({ groupBy: 'role', totalUsd: 1, agentRuns: 1 }));
		vi.stubGlobal('fetch', fetchMock);
		await expect(api.harnessCostSummary('role', { from: '2026-11-04T00:00:00.000Z' })).resolves.toMatchObject({ groups: [] });
		expect(fetchMock.mock.calls[0][0]).toBe('/api/harness/cost?groupBy=role&from=2026-11-04T00%3A00%3A00.000Z');
		expect(api.harnessEventsURL(42)).toBe('/api/harness/events?after=42');
		expect(api.harnessEventsURL()).toBe('/api/harness/events');
	});
});

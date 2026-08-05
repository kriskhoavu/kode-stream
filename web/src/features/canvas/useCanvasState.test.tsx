import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, api } from '../../lib/api';
import type { CanvasProjection } from '../../lib/types';
import { useCanvasState } from './useCanvasState';

vi.mock('../../lib/api', async () => {
	const actual = await vi.importActual<typeof import('../../lib/api')>('../../lib/api');
	return { ...actual, api: { resolveDefaultCanvas: vi.fn(), canvasLayout: vi.fn(), patchCanvasPlacements: vi.fn(), patchCanvasViewport: vi.fn(), removeCanvasPlacement: vi.fn() } };
});

const projection = (revision = 1): CanvasProjection => ({
	layout: { id: 'layout-1', workspaceId: 'workspace-1', branchKey: 'main', viewport: { x: 0, y: 0, zoom: 1 }, version: 1, createdAt: '', updatedAt: '' },
	nodes: [{ id: 'plan:item-1', kind: 'plan', state: 'resolved', entityRef: { kind: 'plan', workspaceId: 'workspace-1', itemId: 'item-1', itemPath: 'plans/platform/one', branchKey: 'main' }, position: { x: 10, y: 20 }, collapsed: false, revision, plan: { itemId: 'item-1', title: 'One', service: 'platform', status: 'draft', branch: 'main', editable: true, actions: {} } }],
	connections: [], unplaced: []
});

describe('useCanvasState', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.clearAllMocks();
		vi.mocked(api.resolveDefaultCanvas).mockResolvedValue(projection());
		vi.mocked(api.patchCanvasPlacements).mockResolvedValue(projection(2));
		vi.mocked(api.patchCanvasViewport).mockResolvedValue({ ...projection(), layout: { ...projection().layout, version: 2, viewport: { x: 5, y: 6, zoom: 1.2 } } });
		vi.mocked(api.canvasLayout).mockResolvedValue(projection(2));
		vi.mocked(api.removeCanvasPlacement).mockResolvedValue({ ...projection(), nodes: [], unplaced: [] });
	});
	afterEach(() => vi.useRealTimers());

	it('loads branch context, moves optimistically, and saves a bounded patch', async () => {
		const { result } = renderHook(() => useCanvasState('workspace-1'));
		await act(async () => { await vi.runAllTimersAsync(); });
		expect(api.resolveDefaultCanvas).toHaveBeenCalledWith('workspace-1');
		act(() => result.current.moveNode('plan:item-1', { x: 90, y: 80 }));
		expect(result.current.projection?.nodes[0].position).toEqual({ x: 90, y: 80 });
		expect(result.current.dirtyCount).toBe(1);
		expect(result.current.saveStatus).toBe('saving');
		await act(async () => { await vi.advanceTimersByTimeAsync(351); });
		expect(api.patchCanvasPlacements).toHaveBeenCalledWith('layout-1', [expect.objectContaining({ nodeId: 'plan:item-1', position: { x: 90, y: 80 }, expectedRevision: 1 })]);
		await act(async () => { await Promise.resolve(); });
		expect(result.current.dirtyCount).toBe(0);
		expect(result.current.saveStatus).toBe('saved');
	});

	it('silently refreshes a conflicting position without exposing a warning', async () => {
		vi.mocked(api.patchCanvasPlacements).mockRejectedValue(new ApiError('conflict', undefined, undefined, { code: 'placement_conflict', nodeIds: ['plan:item-1'], status: 409 }));
		const { result } = renderHook(() => useCanvasState('workspace-1'));
		await act(async () => { await vi.runAllTimersAsync(); });
		act(() => result.current.moveNode('plan:item-1', { x: 50, y: 60 }));
		await act(async () => { await vi.advanceTimersByTimeAsync(351); });
		expect(result.current.conflicts).toEqual([]);
		expect(result.current.dirtyCount).toBe(0);
		expect(result.current.error).toBe('');
		expect(api.canvasLayout).toHaveBeenCalledWith('layout-1');
		expect(result.current.projection?.nodes[0].revision).toBe(2);
	});

	it('saves viewport separately and reloads checkout context on demand', async () => {
		const { result } = renderHook(() => useCanvasState('workspace-1'));
		await act(async () => { await vi.runAllTimersAsync(); });
		act(() => result.current.saveViewport({ x: 5, y: 6, zoom: 1.2 }));
		await act(async () => { await vi.advanceTimersByTimeAsync(501); });
		expect(api.patchCanvasViewport).toHaveBeenCalledWith('layout-1', 1, { x: 5, y: 6, zoom: 1.2 });
		await act(async () => { await result.current.reload(); });
		await act(async () => { await vi.runAllTimersAsync(); });
		expect(api.resolveDefaultCanvas).toHaveBeenLastCalledWith('workspace-1');
	});

	it('preserves an optimistic move through a transient failure and retries twice at most', async () => {
		vi.mocked(api.patchCanvasPlacements).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(projection(2));
		const { result } = renderHook(() => useCanvasState('workspace-1'));
		await act(async () => { await vi.runAllTimersAsync(); });
		act(() => result.current.moveNode('plan:item-1', { x: 70, y: 75 }));
		await act(async () => { await vi.advanceTimersByTimeAsync(351); });
		expect(result.current.projection?.nodes[0].position).toEqual({ x: 70, y: 75 });
		expect(result.current.dirtyCount).toBe(1);
		await act(async () => { await vi.advanceTimersByTimeAsync(601); });
		await act(async () => { await vi.advanceTimersByTimeAsync(351); });
		expect(api.patchCanvasPlacements).toHaveBeenCalledTimes(2);
		expect(result.current.dirtyCount).toBe(0);
	});

	it('places newly discovered entities automatically, resets deterministically, and preserves removal', async () => {
		const unplaced = { ...projection(), nodes: [], unplaced: [projection().nodes[0].entityRef] };
		vi.mocked(api.resolveDefaultCanvas).mockResolvedValue(unplaced);
		vi.mocked(api.patchCanvasPlacements).mockResolvedValue(projection());
		const { result } = renderHook(() => useCanvasState('workspace-1'));
		await act(async () => { await vi.runAllTimersAsync(); });
		expect(api.patchCanvasPlacements).toHaveBeenCalledWith('layout-1', [expect.objectContaining({ nodeId: 'plan:item-1', expectedRevision: 0 })]);
		expect(result.current.projection?.unplaced).toHaveLength(0);
		act(() => result.current.resetPositions());
		expect(result.current.projection?.nodes[0].position).toEqual({ x: 360, y: 0 });
		await act(async () => { await result.current.removeNode('plan:item-1'); });
		expect(api.removeCanvasPlacement).toHaveBeenCalledWith('layout-1', 'plan:item-1', 1);
		expect(result.current.projection?.nodes).toHaveLength(0);
		expect(result.current.projection?.unplaced).toHaveLength(0);
	});

	it('places newly discovered entities silently during refresh', async () => {
		const unplaced = { ...projection(2), unplaced: [{ ...projection().nodes[0].entityRef, itemId: 'item-2' }] };
		vi.mocked(api.resolveDefaultCanvas).mockResolvedValueOnce(projection()).mockResolvedValueOnce(unplaced);
		vi.mocked(api.patchCanvasPlacements).mockResolvedValue({ ...projection(3), nodes: [...projection().nodes, { ...projection().nodes[0], id: 'plan:item-2', entityRef: unplaced.unplaced[0], revision: 1 }], unplaced: [] });
		const { result } = renderHook(() => useCanvasState('workspace-1'));
		await act(async () => { await vi.runAllTimersAsync(); });
		await act(async () => { await result.current.refresh(); });
		expect(api.resolveDefaultCanvas).toHaveBeenLastCalledWith('workspace-1');
		expect(api.patchCanvasPlacements).toHaveBeenCalledWith('layout-1', [expect.objectContaining({ nodeId: 'plan:item-2', expectedRevision: 0 })]);
		expect(result.current.projection?.nodes).toHaveLength(2);
	});

	it('discards unsaved moves when refresh resolves a different checkout layout', async () => {
		const feature = { ...projection(1), layout: { ...projection().layout, id: 'layout-feature', branchKey: 'feature' }, nodes: [{ ...projection().nodes[0], position: { x: 30, y: 40 }, entityRef: { ...projection().nodes[0].entityRef, branchKey: 'feature' }, plan: { ...projection().nodes[0].plan!, branch: 'feature' } }] };
		vi.mocked(api.resolveDefaultCanvas).mockResolvedValueOnce(projection()).mockResolvedValueOnce(feature);
		const { result } = renderHook(() => useCanvasState('workspace-1'));
		await act(async () => { await vi.runAllTimersAsync(); });
		act(() => result.current.moveNode('plan:item-1', { x: 90, y: 80 }));
		await act(async () => { await result.current.refresh(); });
		expect(result.current.projection?.layout.id).toBe('layout-feature');
		expect(result.current.projection?.nodes[0].position).toEqual({ x: 30, y: 40 });
		expect(result.current.error).toMatch(/Unsaved Canvas moves were discarded/);
		await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
		expect(api.patchCanvasPlacements).not.toHaveBeenCalled();
	});

	it('persists session disclosure independently from selection and position', async () => {
		const session = { id: 'session:session-1', kind: 'session' as const, state: 'resolved' as const, entityRef: { kind: 'session' as const, workspaceId: 'workspace-1', sessionId: 'session-1', branchKey: 'main' }, position: { x: 30, y: 40 }, collapsed: true, revision: 1, session: { record: { id: 'session-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', requestedBranch: 'main', state: 'running' as const, startedAt: '', lastKnownAt: '', live: true } } };
		vi.mocked(api.resolveDefaultCanvas).mockResolvedValue({ ...projection(), nodes: [session] });
		vi.mocked(api.patchCanvasPlacements).mockResolvedValue({ ...projection(2), nodes: [{ ...session, collapsed: false, revision: 2 }] });
		const { result } = renderHook(() => useCanvasState('workspace-1'));
		await act(async () => { await vi.runAllTimersAsync(); });
		act(() => result.current.setNodeCollapsed(session.id, false));
		expect(result.current.projection?.nodes[0]).toMatchObject({ collapsed: false, position: { x: 30, y: 40 } });
		await act(async () => { await vi.advanceTimersByTimeAsync(351); });
		expect(api.patchCanvasPlacements).toHaveBeenCalledWith('layout-1', [expect.objectContaining({ nodeId: session.id, collapsed: false, position: { x: 30, y: 40 }, expectedRevision: 1 })]);
	});

	it('opens a newly launched session when automatic placement observed it first', async () => {
		const sessionRef = { kind: 'session' as const, workspaceId: 'workspace-1', sessionId: 'session-1', branchKey: 'main' };
		vi.mocked(api.resolveDefaultCanvas).mockResolvedValue({ ...projection(), unplaced: [sessionRef, { ...projection().nodes[0].entityRef, itemId: 'item-2' }] });
		const sessionNode = { id: 'session:session-1', kind: 'session' as const, state: 'resolved' as const, entityRef: sessionRef, position: { x: 680, y: 420 }, collapsed: true, revision: 1, session: { record: { id: 'session-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', requestedBranch: 'main', state: 'running' as const, startedAt: '', lastKnownAt: '', live: true } } };
		const automaticallyPlaced = { ...projection(), nodes: [...projection().nodes, sessionNode], unplaced: [] };
		vi.mocked(api.patchCanvasPlacements).mockResolvedValueOnce(automaticallyPlaced).mockResolvedValueOnce({ ...automaticallyPlaced, nodes: [projection().nodes[0], { ...sessionNode, collapsed: false, revision: 2 }] });
		const { result } = renderHook(() => useCanvasState('workspace-1'));
		await act(async () => { await vi.runAllTimersAsync(); });
		expect(vi.mocked(api.patchCanvasPlacements).mock.calls[0][1]).toHaveLength(2);
		await act(async () => { await result.current.placeSession('session-1'); });
		expect(api.patchCanvasPlacements).toHaveBeenLastCalledWith('layout-1', [expect.objectContaining({ nodeId: 'session:session-1', entityRef: sessionRef, collapsed: false, expectedRevision: 1 })]);
		expect(result.current.projection?.nodes.find((node) => node.id === sessionNode.id)?.collapsed).toBe(false);
	});
});

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
	nodes: [{ id: 'plan:item-1', kind: 'plan', state: 'resolved', entityRef: { kind: 'plan', workspaceId: 'workspace-1', itemId: 'item-1', itemPath: 'plans/one', branchKey: 'main' }, position: { x: 10, y: 20 }, collapsed: false, revision, plan: { itemId: 'item-1', title: 'One', branch: 'main', editable: true, actions: {} } }],
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
		vi.mocked(api.removeCanvasPlacement).mockResolvedValue({ ...projection(), nodes: [], unplaced: [projection().nodes[0].entityRef] });
	});
	afterEach(() => vi.useRealTimers());

	it('loads branch context, moves optimistically, and saves a bounded patch', async () => {
		const { result } = renderHook(() => useCanvasState('workspace-1', 'main'));
		await act(async () => { await vi.runAllTimersAsync(); });
		expect(api.resolveDefaultCanvas).toHaveBeenCalledWith('workspace-1', 'main');
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

	it('keeps a conflicting position dirty and supports reload recovery', async () => {
		vi.mocked(api.patchCanvasPlacements).mockRejectedValue(new ApiError('conflict', undefined, undefined, { code: 'placement_conflict', nodeIds: ['plan:item-1'], status: 409 }));
		const { result } = renderHook(() => useCanvasState('workspace-1', 'main'));
		await act(async () => { await vi.runAllTimersAsync(); });
		act(() => result.current.moveNode('plan:item-1', { x: 50, y: 60 }));
		await act(async () => { await vi.advanceTimersByTimeAsync(351); });
		expect(result.current.conflicts).toEqual(['plan:item-1']);
		expect(result.current.dirtyCount).toBe(1);
		await act(async () => { await result.current.reloadPosition('plan:item-1'); });
		expect(result.current.conflicts).toEqual([]);
		expect(result.current.dirtyCount).toBe(0);
		expect(result.current.projection?.nodes[0].revision).toBe(2);
	});

	it('saves viewport separately and reloads when branch context changes', async () => {
		const { result, rerender } = renderHook(({ branch }) => useCanvasState('workspace-1', branch), { initialProps: { branch: 'main' } });
		await act(async () => { await vi.runAllTimersAsync(); });
		act(() => result.current.saveViewport({ x: 5, y: 6, zoom: 1.2 }));
		await act(async () => { await vi.advanceTimersByTimeAsync(501); });
		expect(api.patchCanvasViewport).toHaveBeenCalledWith('layout-1', 1, { x: 5, y: 6, zoom: 1.2 });
		rerender({ branch: 'feature' });
		await act(async () => { await vi.runAllTimersAsync(); });
		expect(api.resolveDefaultCanvas).toHaveBeenLastCalledWith('workspace-1', 'feature');
	});

	it('preserves an optimistic move through a transient failure and retries twice at most', async () => {
		vi.mocked(api.patchCanvasPlacements).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(projection(2));
		const { result } = renderHook(() => useCanvasState('workspace-1', 'main'));
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

	it('places unplaced entities, resets deterministically, and removes only the placement', async () => {
		const unplaced = { ...projection(), nodes: [], unplaced: [projection().nodes[0].entityRef] };
		vi.mocked(api.resolveDefaultCanvas).mockResolvedValue(unplaced);
		vi.mocked(api.patchCanvasPlacements).mockResolvedValue(projection());
		const { result } = renderHook(() => useCanvasState('workspace-1', 'main'));
		await act(async () => { await vi.runAllTimersAsync(); });
		await act(async () => { await result.current.placeUnplaced(); });
		expect(api.patchCanvasPlacements).toHaveBeenCalledWith('layout-1', [expect.objectContaining({ nodeId: 'plan:item-1', expectedRevision: 0 })]);
		act(() => result.current.resetPositions());
		expect(result.current.projection?.nodes[0].position).toEqual({ x: 360, y: 0 });
		await act(async () => { await result.current.removeNode('plan:item-1'); });
		expect(api.removeCanvasPlacement).toHaveBeenCalledWith('layout-1', 'plan:item-1', 1);
		expect(result.current.projection?.nodes).toHaveLength(0);
	});
});

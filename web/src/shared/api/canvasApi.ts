import type { CanvasPlacementPatch, CanvasProjection, CanvasViewport } from '../../lib/types';
import { request } from './http';

export const canvasApi = {
	resolveDefaultCanvas: (workspaceId: string, signal?: AbortSignal) => request<CanvasProjection>('/api/canvas/default', { method: 'POST', body: JSON.stringify({ workspaceId }), signal }, !signal),
	canvasLayout: (layoutId: string, signal?: AbortSignal) => request<CanvasProjection>(`/api/canvas/layouts/${encodeURIComponent(layoutId)}`, { signal }, !signal),
	patchCanvasPlacements: (layoutId: string, patches: CanvasPlacementPatch[], signal?: AbortSignal) => request<CanvasProjection>(`/api/canvas/layouts/${encodeURIComponent(layoutId)}/placements`, { method: 'PATCH', body: JSON.stringify({ patches }), signal }, !signal),
	patchCanvasViewport: (layoutId: string, expectedVersion: number, viewport: CanvasViewport, signal?: AbortSignal) => request<CanvasProjection>(`/api/canvas/layouts/${encodeURIComponent(layoutId)}/viewport`, { method: 'PATCH', body: JSON.stringify({ expectedVersion, viewport }), signal }, !signal),
	removeCanvasPlacement: (layoutId: string, nodeId: string, expectedRevision: number, signal?: AbortSignal) => request<CanvasProjection>(`/api/canvas/layouts/${encodeURIComponent(layoutId)}/placements/${encodeURIComponent(nodeId)}?expectedRevision=${expectedRevision}`, { method: 'DELETE', signal }, !signal)
};

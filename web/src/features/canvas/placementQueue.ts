import type { CanvasPlacementPatch, CanvasProjection } from '../../lib/types';

export interface DirtyPlacement {
	patch: CanvasPlacementPatch;
	mutation: number;
	retries: number;
}

export const MAX_PLACEMENT_RETRIES = 2;

export type PlacementQueue = Map<string, DirtyPlacement>;

export function acceptSaved(queue: PlacementQueue, batch: DirtyPlacement[]) {
	for (const entry of batch) {
		if (queue.get(entry.patch.nodeId)?.mutation === entry.mutation) queue.delete(entry.patch.nodeId);
	}
}

export function rebaseConflicts(queue: PlacementQueue, affected: string[], latest: CanvasProjection | undefined) {
	let rebased = false;
	let exhausted = false;
	for (const nodeId of affected) {
		const entry = queue.get(nodeId);
		if (!entry) continue;
		if (entry.retries >= MAX_PLACEMENT_RETRIES) {
			queue.delete(nodeId);
			exhausted = true;
			continue;
		}
		entry.retries += 1;
		if (!latest) {
			rebased = true;
			continue;
		}
		const revision = latest.nodes.find((node) => node.id === nodeId)?.revision;
		if (revision === undefined) {
			queue.delete(nodeId);
			continue;
		}
		entry.patch = { ...entry.patch, expectedRevision: revision };
		rebased = true;
	}
	return { rebased, exhausted };
}

export function countRetries(queue: PlacementQueue, batch: DirtyPlacement[]): boolean {
	let retry = false;
	for (const entry of batch) {
		const current = queue.get(entry.patch.nodeId);
		if (!current || current.mutation !== entry.mutation || current.retries >= MAX_PLACEMENT_RETRIES) continue;
		current.retries += 1;
		retry = true;
	}
	return retry;
}

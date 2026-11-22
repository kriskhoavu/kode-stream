import type { CanvasPlacementPatch, CanvasProjection } from '../../lib/types';

/** A node's position or size change waiting to be saved, with the edit that made it. */
export interface DirtyPlacement {
	patch: CanvasPlacementPatch;
	mutation: number;
	retries: number;
}

/** How often one change is retried after a conflict or a failed save before it is dropped. */
export const MAX_PLACEMENT_RETRIES = 2;

export type PlacementQueue = Map<string, DirtyPlacement>;

/** Drops the saved changes that no later edit replaced while the save was in flight. */
export function acceptSaved(queue: PlacementQueue, batch: DirtyPlacement[]) {
	for (const entry of batch) {
		if (queue.get(entry.patch.nodeId)?.mutation === entry.mutation) queue.delete(entry.patch.nodeId);
	}
}

/**
 * Rebases the conflicted changes onto the revisions of `latest`, the layout the server now holds.
 * A change past its retries, or for a node that is gone, is dropped. Without `latest` a change is
 * kept on its current revision rather than lost. `rebased` says a kept change needs another save;
 * `exhausted` says a change was dropped for its retries.
 */
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

/** Counts a retry on each unchanged change of a failed batch; true when any may be retried. */
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

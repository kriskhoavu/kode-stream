import type { HarnessTask } from '../../lib/types';

/**
 * Waves and the critical path, from the tasks' own dependency lists.
 *
 * A task's wave is the length of the longest chain of dependencies in front
 * of it (Kahn's algorithm over the shown tasks), so a card only ever waits for
 * cards to its left. The critical path is the longest chain by estimated
 * minutes among unfinished work: the order in which delay hurts most.
 */

export interface WavePlan {
	/** Task ids per wave, left to right. */
	waves: string[][];
	waveOf: Map<string, number>;
	/** Every dependency between two shown tasks, as [waitedFor, waiting]. */
	edges: [string, string][];
	critical: Set<string>;
	criticalEdges: Set<string>;
	/** Tasks caught in a dependency loop; placed after every wave so they still show. */
	cyclic: Set<string>;
}

export const defaultEstimateMinutes = 30;

export function edgeKey(from: string, to: string): string {
	return `${from}>${to}`;
}

function finished(task: HarnessTask): boolean {
	return task.state === 'done' || Boolean(task.finishedOutside) || task.state === 'dismissed' || task.state === 'stale' || task.state === 'cancelled';
}

export function planWaves(tasks: HarnessTask[]): WavePlan {
	const byId = new Map(tasks.map((task) => [task.id, task]));
	const edges: [string, string][] = [];
	const indegree = new Map<string, number>();
	const next = new Map<string, string[]>();
	for (const task of tasks) {
		indegree.set(task.id, 0);
		next.set(task.id, []);
	}
	for (const task of tasks) {
		for (const dependency of new Set(task.dependsOn)) {
			if (!byId.has(dependency) || dependency === task.id) continue;
			edges.push([dependency, task.id]);
			next.get(dependency)?.push(task.id);
			indegree.set(task.id, (indegree.get(task.id) ?? 0) + 1);
		}
	}

	// Kahn: a task's wave is one more than the deepest task it waits for.
	const waveOf = new Map<string, number>();
	const queue = tasks.filter((task) => indegree.get(task.id) === 0).map((task) => task.id);
	const order: string[] = [];
	queue.forEach((id) => waveOf.set(id, 0));
	for (let index = 0; index < queue.length; index++) {
		const id = queue[index];
		order.push(id);
		for (const child of next.get(id) ?? []) {
			waveOf.set(child, Math.max(waveOf.get(child) ?? 0, (waveOf.get(id) ?? 0) + 1));
			const remaining = (indegree.get(child) ?? 0) - 1;
			indegree.set(child, remaining);
			if (remaining === 0) queue.push(child);
		}
	}

	const cyclic = new Set(tasks.filter((task) => !waveOf.has(task.id)).map((task) => task.id));
	const lastWave = Math.max(-1, ...waveOf.values());
	cyclic.forEach((id) => waveOf.set(id, lastWave + 1));

	const waves: string[][] = [];
	for (const task of tasks) {
		const wave = waveOf.get(task.id) ?? 0;
		(waves[wave] ??= []).push(task.id);
	}
	for (let index = 0; index < waves.length; index++) waves[index] ??= [];

	// Longest path by estimated minutes over unfinished work, in topological order.
	const weight = (id: string) => {
		const task = byId.get(id);
		return !task || finished(task) ? 0 : task.estimatedMinutes ?? defaultEstimateMinutes;
	};
	const best = new Map<string, number>();
	const previous = new Map<string, string>();
	for (const id of order) {
		const here = (best.get(id) ?? 0) + weight(id);
		best.set(id, here);
		for (const child of next.get(id) ?? []) {
			if (here > (best.get(child) ?? 0)) {
				best.set(child, here);
				previous.set(child, id);
			}
		}
	}
	// After the pass, `best` holds each task's whole chain including its own minutes.
	let end: string | undefined;
	let longest = 0;
	for (const id of order) {
		const total = best.get(id) ?? 0;
		if (total > longest) {
			longest = total;
			end = id;
		}
	}
	const critical = new Set<string>();
	const criticalEdges = new Set<string>();
	for (let id = end; id; id = previous.get(id)) {
		if (weight(id) > 0) critical.add(id);
		const before = previous.get(id);
		if (before && weight(before) > 0) criticalEdges.add(edgeKey(before, id));
	}

	return { waves, waveOf, edges, critical, criticalEdges, cyclic };
}

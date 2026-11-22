import { describe, expect, it } from 'vitest';
import type { CanvasProjection } from '../../lib/types';
import { acceptSaved, countRetries, rebaseConflicts, type DirtyPlacement, type PlacementQueue } from './placementQueue';

const entry = (nodeId: string, mutation = 1, retries = 0): DirtyPlacement => ({ patch: { nodeId, expectedRevision: 1 } as DirtyPlacement['patch'], mutation, retries });
const queueOf = (...entries: DirtyPlacement[]): PlacementQueue => new Map(entries.map((item) => [item.patch.nodeId, item]));
const layoutWith = (...nodes: Array<{ id: string; revision: number }>) => ({ nodes }) as unknown as CanvasProjection;

describe('placement queue', () => {
	it('keeps a change edited again while its save was in flight', () => {
		const sent = [entry('a', 1), entry('b', 1)];
		const queue = queueOf(entry('a', 1), entry('b', 2));
		acceptSaved(queue, sent);
		expect([...queue.keys()]).toEqual(['b']);
	});

	it('rebases conflicted changes, drops gone nodes and changes past their retries', () => {
		const queue = queueOf(entry('a'), entry('gone'), entry('tired', 1, 2));
		const result = rebaseConflicts(queue, ['a', 'gone', 'tired'], layoutWith({ id: 'a', revision: 7 }));
		expect(result).toEqual({ rebased: true, exhausted: true });
		expect(queue.get('a')?.patch.expectedRevision).toBe(7);
		expect(queue.get('a')?.retries).toBe(1);
		expect(queue.has('gone')).toBe(false);
		expect(queue.has('tired')).toBe(false);
	});

	it('keeps a conflicted change on its revision when the layout could not be fetched', () => {
		const queue = queueOf(entry('a'));
		expect(rebaseConflicts(queue, ['a'], undefined)).toEqual({ rebased: true, exhausted: false });
		expect(queue.get('a')?.patch.expectedRevision).toBe(1);
	});

	it('retries only unchanged changes that have retries left', () => {
		const queue = queueOf(entry('a', 1), entry('b', 2), entry('c', 1, 2));
		expect(countRetries(queue, [entry('a', 1), entry('b', 1), entry('c', 1)])).toBe(true);
		expect(queue.get('a')?.retries).toBe(1);
		expect(queue.get('b')?.retries).toBe(0);
		expect(countRetries(queue, [entry('c', 1)])).toBe(false);
	});
});

import { describe, expect, it } from 'vitest';
import { edgeKey, planWaves } from './waves';
import { task } from './testData';

describe('planWaves', () => {
	it('puts each task one wave after the deepest task it waits for', () => {
		const plan = planWaves([
			task('A'), task('B'),
			task('C', { dependsOn: ['task-A'] }),
			task('D', { dependsOn: ['task-C', 'task-B'] }),
			task('E', { dependsOn: ['task-A', 'task-D'] })
		]);
		expect(plan.waves).toEqual([['task-A', 'task-B'], ['task-C'], ['task-D'], ['task-E']]);
		expect(plan.edges).toContainEqual(['task-C', 'task-D']);
	});

	it('ignores dependencies on tasks that are not shown and on itself', () => {
		const plan = planWaves([task('A', { dependsOn: ['task-gone', 'task-A'] })]);
		expect(plan.waves).toEqual([['task-A']]);
		expect(plan.edges).toEqual([]);
	});

	it('still shows tasks caught in a loop, after every wave', () => {
		const plan = planWaves([task('A'), task('B', { dependsOn: ['task-C'] }), task('C', { dependsOn: ['task-B'] })]);
		expect([...plan.cyclic].sort()).toEqual(['task-B', 'task-C']);
		expect(plan.waves[1].sort()).toEqual(['task-B', 'task-C']);
	});

	it('takes the longest chain of unfinished minutes as the critical path', () => {
		const plan = planWaves([
			task('A', { estimatedMinutes: 10 }),
			task('B', { estimatedMinutes: 100 }),
			task('C', { dependsOn: ['task-A'], estimatedMinutes: 20 }),
			task('D', { dependsOn: ['task-B'], estimatedMinutes: 5 }),
			task('E', { dependsOn: ['task-C', 'task-D'], estimatedMinutes: 10 }),
			task('F', { dependsOn: ['task-A'], state: 'done', estimatedMinutes: 500 })
		]);
		expect([...plan.critical]).toEqual(['task-E', 'task-D', 'task-B']);
		expect([...plan.criticalEdges].sort()).toEqual([edgeKey('task-B', 'task-D'), edgeKey('task-D', 'task-E')].sort());
	});
});

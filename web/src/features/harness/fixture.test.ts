import { describe, expect, it } from 'vitest';
import { createFixtureClient } from './fixture';
import { waitsFor } from './harnessState';

describe('harness fixture', () => {
	it('holds a consistent demo: every dependency exists and the waiting approvals match their tasks', async () => {
		const client = createFixtureClient({ tickMs: 0, arrivalMs: 0 });
		const tasks = await client.tasks();
		const byId = Object.fromEntries(tasks.map((task) => [task.id, task]));
		expect(tasks).toHaveLength(48);
		expect(tasks.every((task) => task.dependsOn.every((id) => byId[id]))).toBe(true);
		expect(waitsFor(byId['task-srv-02'], byId)).toEqual(['SRV-01']);
		const waiting = (await client.approvals()).filter((approval) => approval.state === 'requested');
		expect(waiting.map((approval) => byId[approval.taskId].source.key).sort()).toEqual(['SRV-04', 'UX-11']);
	});

	it('moves a run on when its plan is approved', async () => {
		const client = createFixtureClient({ tickMs: 0, arrivalMs: 0 });
		const events: string[] = [];
		const close = client.subscribe?.(0, { onOpen: () => undefined, onEvent: (event) => events.push(event.type), onReset: () => undefined, onError: () => undefined });
		const approval = (await client.approvals()).find((item) => item.id === 'appr-ux11-plan');
		const decided = await client.approve(approval!, 'key');
		expect(decided).toMatchObject({ state: 'approved', channel: 'dashboard' });
		expect(events).toEqual(expect.arrayContaining(['approval.decided', 'stage.finished', 'stage.started', 'agent.started']));
		const detail = await client.run('run-ux-11');
		expect(detail.stages.find((stage) => stage.kind === 'implement')?.state).toBe('running');
		await expect(client.approve(approval!, 'key')).resolves.toBeUndefined();
		close?.();
	});
});

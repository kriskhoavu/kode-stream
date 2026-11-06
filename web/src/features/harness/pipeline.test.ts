import { describe, expect, it } from 'vitest';
import { createFixtureClient } from './fixture';
import { pipelineModel } from './pipeline';
import { ledgerGroups } from './RunLedger';

async function detail(runId: string) {
	return createFixtureClient({ tickMs: 0, arrivalMs: 0 }).run(runId);
}

describe('pipeline model', () => {
	it('reads the building run as the mockup does', async () => {
		const model = pipelineModel(await detail('run-pm-02'));
		expect(model.groups.map((group) => group.label)).toEqual(['Start', 'Plan · round 2 of 3', 'You', 'Build · round 1 of 3', 'Test', 'Hand-off']);
		const cards = Object.fromEntries(model.groups.flatMap((group) => group.cards).map((card) => [card.id, card]));
		expect(cards.start).toMatchObject({ status: 'done', meta: ['Khoa · here', '31 min ago'] });
		expect(cards.plan).toMatchObject({ status: 'done', meta: ['2 rounds', '8 min', '$3.03'] });
		expect(cards.plan_check).toMatchObject({ status: 'done', meta: ['approved', '3 min', '$1.44'] });
		expect(cards.approval.meta[0]).toBe('Anna · via Webex');
		expect(cards.build).toMatchObject({ status: 'working', current: true });
		expect(cards.code_check).toMatchObject({ status: 'not_started', meta: ['after Build'] });
		expect(cards.fix).toMatchObject({ status: 'not_started', meta: ['only if a test fails'] });
		expect(cards.draft_pr.meta).toEqual(['you approve first']);
		expect(model.groups[model.activeGroup].id).toBe('build');
	});

	it('makes the waiting approval the current step', async () => {
		const model = pipelineModel(await detail('run-ux-11'));
		expect(model.current).toMatchObject({ id: 'approval', status: 'waiting' });
		expect(model.activeGroup).toBe(-1);
	});

	it('shows a stuck plan as needing a fix with three rounds behind it', async () => {
		const model = pipelineModel(await detail('run-srv-04'));
		const plan = model.groups.find((group) => group.id === 'plan');
		expect(plan?.label).toBe('Plan · round 3 of 3');
		expect(plan?.cards[1]).toMatchObject({ status: 'needs_fix', meta: ['1 must fix', '9 min', '$1.80'] });
	});

	it('shows the stage a resume started, not the one it replaced', async () => {
		const run = await detail('run-srv-04');
		const stuck = run.stages.find((stage) => stage.kind === 'plan');
		if (!stuck) throw new Error('fixture has no plan stage');
		run.stages.push({ ...stuck, id: 'stg-resumed', state: 'running', round: 1, turns: 0, durationMs: 0, costUsd: 0, blocking: 0, failureReason: undefined });
		const plan = pipelineModel(run).groups.find((group) => group.id === 'plan');
		expect(plan?.label).toBe('Plan · round 1 of 3');
		expect(plan?.cards[0].status).toBe('working');
	});

	it('groups the ledger by stage and names each round', async () => {
		const groups = ledgerGroups(await detail('run-pm-02'));
		expect(groups.map((group) => group.title)).toEqual(['Plan', 'Build']);
		expect(groups[0].note).toBe('Anna approved via Webex');
		expect(groups[0].rows.map((row) => row.step)).toEqual(['Round 1 · plan', 'Round 1 · plan check', 'Round 2 · plan again', 'Round 2 · plan check']);
	});

	it('names a retried try of a round so three rows are not three rounds', async () => {
		const run = await detail('run-pm-02');
		const planner = run.agentRuns.find((agentRun) => agentRun.role === 'planner' && agentRun.round === 2);
		if (!planner) throw new Error('fixture has no round-2 planner');
		run.agentRuns.push({ ...planner, id: 'agr-retry', attempt: 2, startedAt: '2099-01-01T00:00:00Z' });
		const groups = ledgerGroups(run);
		expect(groups[0].rows.map((row) => row.step)).toContain('Round 2 · plan again · try 2');
	});

	it('opens only the latest stage, and numbers a stage that ran again after a stop', async () => {
		const run = await detail('run-pm-02');
		expect(groups_open(ledgerGroups(run))).toEqual([false, true]);
		const plan = run.stages.find((stage) => stage.kind === 'plan');
		const planner = run.agentRuns.find((agentRun) => agentRun.role === 'planner');
		if (!plan || !planner) throw new Error('fixture has no plan stage');
		run.stages.push({ ...plan, id: 'stage-plan-2', ordinal: plan.ordinal + 0.5, state: 'failed' });
		run.agentRuns.push({ ...planner, id: 'agr-plan-2', stageId: 'stage-plan-2', state: 'cancelled' });
		const groups = ledgerGroups(run);
		expect(groups.map((group) => group.title)).toEqual(['Plan 1', 'Plan 2', 'Build']);
		expect(groups_open(groups)).toEqual([false, false, true]);
	});
});

const groups_open = (groups: ReturnType<typeof ledgerGroups>) => groups.map((group) => group.open);

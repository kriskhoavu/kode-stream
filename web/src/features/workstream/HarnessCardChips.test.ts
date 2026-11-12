import { describe, expect, it } from 'vitest';
import { badgeLabel, planNote } from './HarnessCardChips';
import { run, task } from '../harness/testData';
import type { CardWork } from './harnessJoin';
import type { HarnessRun, HarnessStageKind } from '../../lib/types';

const work = (patch: Partial<CardWork>): CardWork => ({ task: task('PM-1'), waiting: [], status: 'working', waitsFor: [], stageLabel: 'Plan', synthetic: false, ...patch });

const atStage = (kind: HarnessStageKind) => {
	const base = run('run-1', 'PM-1');
	return { ...base, currentStage: { ...base.currentStage!, kind } } as HarnessRun;
};

describe('badgeLabel', () => {
	it('names the stage while working and the status beside it otherwise', () => {
		expect(badgeLabel(work({ status: 'working', stageLabel: 'Build · round 2 of 3' }))).toBe('Build · round 2 of 3');
		expect(badgeLabel(work({ status: 'failed', stageLabel: 'Test', run: atStage('qa') }))).toBe("Didn't pass while testing");
		expect(badgeLabel(work({ status: 'stopped', stageLabel: 'Plan', run: atStage('plan') }))).toBe('Stopped while planning');
		expect(badgeLabel(work({ status: 'waiting', stageLabel: 'Your approval', run: atStage('plan_approval') }))).toBe('Waiting on you · Your approval');
		expect(badgeLabel(work({ status: 'done', stageLabel: '' }))).toBeUndefined();
	});

	it('says a ticket has no plan until a run has written one', () => {
		expect(planNote(work({ synthetic: true }))).toBe('no plan yet');
		expect(planNote(work({ synthetic: true, run: atStage('plan') }))).toBe('no plan yet');
		expect(planNote(work({ synthetic: true, run: atStage('implement') }))).toBe('plan not merged yet');
	});
});

import { describe, expect, it } from 'vitest';
import { badgeLabel } from './HarnessCardChips';
import { task } from '../harness/testData';
import type { CardWork } from './harnessJoin';

const work = (patch: Partial<CardWork>): CardWork => ({ task: task('PM-1'), waiting: [], status: 'working', waitsFor: [], stageLabel: 'Plan', synthetic: false, ...patch });

describe('badgeLabel', () => {
	it('names the stage while working and the status beside it otherwise', () => {
		expect(badgeLabel(work({ status: 'working', stageLabel: 'Build · round 2 of 3' }))).toBe('Build · round 2 of 3');
		expect(badgeLabel(work({ status: 'failed', stageLabel: 'Plan' }))).toBe("Didn't pass · Plan");
		expect(badgeLabel(work({ status: 'stopped', stageLabel: 'Plan' }))).toBe('Stopped · Plan');
		expect(badgeLabel(work({ status: 'done', stageLabel: '' }))).toBeUndefined();
	});
});

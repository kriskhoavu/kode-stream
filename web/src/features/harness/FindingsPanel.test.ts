import { describe, expect, it } from 'vitest';
import type { HarnessAgentRun } from '../../lib/types';
import { earlierFixes, featuredReview } from './FindingsPanel';

function review(round: number, blocking: number, extra: Partial<HarnessAgentRun> = {}): HarnessAgentRun {
	return { id: `ar-${round}`, role: 'plan_reviewer', stageId: 'plan', round, blocking, advisory: 0, verdict: blocking ? 'changes_requested' : 'approve', startedAt: new Date(Date.UTC(2026, 0, 1, 0, round)).toISOString(), ...extra } as HarnessAgentRun;
}

describe('featuredReview', () => {
	it('shows the latest finished review, not an earlier one with must-fix items', () => {
		const runs = [review(1, 2), review(2, 0)];
		expect(featuredReview(runs)?.round).toBe(2);
		expect(earlierFixes(runs, runs[1])).toBe('Round 1 asked for 2 must-fix items. Round 2 found them fixed.');
	});

	it('skips a review still running and says nothing of earlier rounds while fixes stay open', () => {
		const runs = [review(1, 1), review(2, 1), review(3, 0, { verdict: undefined })];
		expect(featuredReview(runs)?.round).toBe(2);
		expect(earlierFixes(runs, runs[1])).toBe('');
	});
});

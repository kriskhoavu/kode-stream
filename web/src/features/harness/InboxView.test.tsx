import { cleanup, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { InboxView } from './InboxView';
import { harnessReducer, initialHarnessState } from './harnessState';
import { createFixtureStore } from './fixture';

// Noon local time, so "today" holds every fixture run whatever hour the suite runs.
beforeEach(() => vi.useFakeTimers({ now: new Date(2026, 10, 4, 12), toFake: ['Date'] }));
afterEach(() => {
	cleanup();
	vi.useRealTimers();
});

function fixtureState() {
	const store = createFixtureStore();
	return harnessReducer(initialHarnessState, {
		type: 'snapshot',
		status: store.status,
		tasks: [...store.tasks.values()],
		runs: [...store.runs.values()],
		approvals: [...store.approvals.values()],
		cost: { groupBy: 'role', from: '', to: '', totalUsd: 17.62, agentRuns: 16, groups: [{ key: 'planner', costUsd: 6.21, agentRuns: 5, turns: 1, durationMs: 1 }, { key: 'coder', costUsd: 8.44, agentRuns: 6, turns: 1, durationMs: 1 }, { key: 'plan_reviewer', costUsd: 1.5, agentRuns: 3, turns: 1, durationMs: 1 }, { key: 'code_reviewer', costUsd: 1.47, agentRuns: 2, turns: 1, durationMs: 1 }] }
	});
}

describe('InboxView', () => {
	it('shows what needs you, what is working, what is done, spend and team health', () => {
		render(<InboxView state={fixtureState()} onApprove={vi.fn()} onReject={vi.fn()} onOpenRun={vi.fn()} />);
		expect(screen.getByRole('heading', { name: 'Needs you · 2' })).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: /approve the plan for UX-11/ })).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: /SRV-04 · WebSocket gateway is stuck/ })).toBeInTheDocument();

		const working = screen.getByRole('heading', { name: 'Working now · 4' }).parentElement as HTMLElement;
		expect(within(working).getAllByText('Building · round 1 of 3')).toHaveLength(2);
		expect(within(working).getByText('Planning · round 1 of 3')).toBeInTheDocument();
		expect(within(working).getByText('Testing · run 2 of 2')).toBeInTheDocument();
		expect(within(working).getAllByText('Claude · Sonnet 5.5').length).toBeGreaterThan(0);
		expect(within(working).getByText('Tester · no model')).toBeInTheDocument();

		const done = screen.getByRole('heading', { name: 'Done today · 2' }).parentElement as HTMLElement;
		expect(within(done).getByText('Draft pull request airship#3103')).toBeInTheDocument();
		expect(within(done).getByText('Done by hand')).toBeInTheDocument();

		expect(screen.getByText('$17.62', { selector: '.sr-only' })).toBeInTheDocument();
		expect(screen.getByText('Checker $2.97')).toBeInTheDocument();
		expect(screen.getByText(/looked 12 min ago · 3 new/)).toBeInTheDocument();
	});

	it('has plain copy when nothing needs you', () => {
		render(<InboxView state={{ ...initialHarnessState, loaded: true }} onApprove={vi.fn()} onReject={vi.fn()} onOpenRun={vi.fn()} />);
		expect(screen.getByText("Nothing needs you right now. We'll let you know here and in Webex.")).toBeInTheDocument();
		expect(screen.getByText('Nothing finished yet today.')).toBeInTheDocument();
	});
});

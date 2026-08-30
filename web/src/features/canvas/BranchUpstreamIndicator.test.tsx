import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { GitStatus } from '../../lib/types';
import { BranchUpstreamIndicator } from './BranchUpstreamIndicator';

describe('BranchUpstreamIndicator', () => {
	it('reports how far the branch has drifted from its upstream', () => {
		render(<BranchUpstreamIndicator git={gitStatus({ upstream: 'origin/main', ahead: 2, behind: 5, fetchedAt: hoursAgo(2) })} />);
		expect(screen.getByText('2 ahead, 5 behind')).toBeInTheDocument();
		expect(screen.getByText('origin/main')).toBeInTheDocument();
	});

	it('says a branch level with its upstream is in sync rather than counting to zero', () => {
		render(<BranchUpstreamIndicator git={gitStatus({ upstream: 'origin/main', fetchedAt: hoursAgo(0) })} />);
		expect(screen.getByText('in sync')).toBeInTheDocument();
	});

	it('drops the side that is at zero so a one-way drift reads cleanly', () => {
		render(<BranchUpstreamIndicator git={gitStatus({ upstream: 'origin/main', behind: 3, fetchedAt: hoursAgo(1) })} />);
		expect(screen.getByText('3 behind')).toBeInTheDocument();
	});

	// The counts are measured against the last-fetched remote ref, so their age is
	// part of the reading: a stale "in sync" is not evidence of being up to date.
	it('dates the measurement so a stale count is not read as current', () => {
		render(<BranchUpstreamIndicator git={gitStatus({ upstream: 'origin/main', fetchedAt: hoursAgo(2) })} />);
		expect(screen.getByTitle(/fetched 2h ago/i)).toBeInTheDocument();
	});

	it('says so when nothing has fetched, rather than implying a fresh zero', () => {
		render(<BranchUpstreamIndicator git={gitStatus({ upstream: 'origin/main' })} />);
		expect(screen.getByTitle(/never fetched/i)).toBeInTheDocument();
	});

	// A Go zero time can still reach the client from an older server build, and it
	// parses as a real date rather than as absent.
	it('treats a zero timestamp as never fetched rather than as an ancient fetch', () => {
		render(<BranchUpstreamIndicator git={gitStatus({ upstream: 'origin/main', fetchedAt: '0001-01-01T00:00:00Z' })} />);
		expect(screen.getByTitle(/never fetched/i)).toBeInTheDocument();
	});

	// Ahead/behind are measured against the last-fetched ref. With no fetch at all
	// there is no comparison to report, so claiming "in sync" states as fact
	// something the data cannot support.
	it('does not claim to be in sync when nothing has ever been fetched', () => {
		render(<BranchUpstreamIndicator git={gitStatus({ upstream: 'origin/main' })} />);
		expect(screen.queryByText('in sync')).not.toBeInTheDocument();
		expect(screen.getByText('not fetched')).toBeInTheDocument();
	});

	it('reports a fetch timestamp slightly ahead of the local clock as recent', () => {
		render(<BranchUpstreamIndicator git={gitStatus({ upstream: 'origin/main', fetchedAt: new Date(Date.now() + 2000).toISOString() })} />);
		expect(screen.getByTitle(/fetched just now/i)).toBeInTheDocument();
	});

	it('hides the bare drift numerals from assistive technology', () => {
		const { container } = render(<BranchUpstreamIndicator git={gitStatus({ upstream: 'origin/main', ahead: 1, behind: 2, fetchedAt: hoursAgo(1) })} />);
		expect(container.querySelector('.branch-upstream-ahead')).toHaveAttribute('aria-hidden', 'true');
		expect(container.querySelector('.branch-upstream-behind')).toHaveAttribute('aria-hidden', 'true');
		expect(screen.getByText('1 ahead, 2 behind')).toHaveClass('sr-only');
	});

	it('renders nothing for a branch with no upstream', () => {
		const { container } = render(<BranchUpstreamIndicator git={gitStatus({})} />);
		expect(container).toBeEmptyDOMElement();
	});

	it('renders nothing when git status is unavailable', () => {
		const { container } = render(<BranchUpstreamIndicator />);
		expect(container).toBeEmptyDOMElement();
	});
});

function hoursAgo(hours: number) {
	return new Date(Date.now() - hours * 60 * 60 * 1000).toISOString();
}

function gitStatus(overrides: Partial<GitStatus>): GitStatus {
	return { workspaceId: 'workspace-1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [], ...overrides };
}

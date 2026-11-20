import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ItemSummary } from '../../lib/types';
import { PlanListView } from './PlanListView';

afterEach(cleanup);

function plan(patch: Partial<ItemSummary>): ItemSummary {
	return { id: 'p1', workspaceId: 'ws', workspaceName: 'Sandbox', branch: 'main', scope: 'web', identifier: 'PM-7', title: 'Sort the queue', status: 'draft', tags: [], itemPath: 'plans/PM-7', ...patch } as ItemSummary;
}

describe('PlanListView', () => {
	it('opens a plan from its ID, while its title is plain text and the row previews it', () => {
		const onOpen = vi.fn();
		const onPreview = vi.fn();
		render(<PlanListView items={[plan({})]} loading={false} pendingIds={new Set()} workOf={() => undefined} onPreview={onPreview} onOpen={onOpen} onMove={vi.fn()} />);
		expect(screen.queryByRole('button', { name: 'Sort the queue' })).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'PM-7' }));
		expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: 'p1' }));
		expect(onPreview).not.toHaveBeenCalled();
		fireEvent.click(screen.getByText('Sort the queue'));
		expect(onPreview).toHaveBeenCalledTimes(1);
	});

	it('opens a plan from its title when its ID only repeats the title', () => {
		const onOpen = vi.fn();
		render(<PlanListView items={[plan({ identifier: 'sort the queue' })]} loading={false} pendingIds={new Set()} workOf={() => undefined} onPreview={vi.fn()} onOpen={onOpen} onMove={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Sort the queue' }));
		expect(onOpen).toHaveBeenCalledTimes(1);
	});

	it('shows what the card shows, and offers the harness on a plan it does not hold', () => {
		const onRunWithHarness = vi.fn();
		render(<PlanListView items={[plan({ description: 'Rank the work by score.', tags: ['harness', 'web'], author: 'Anna', updatedAt: new Date().toISOString() })]} loading={false} pendingIds={new Set()} workOf={() => undefined} harnessOn onPreview={vi.fn()} onOpen={vi.fn()} onMove={vi.fn()} onRunWithHarness={onRunWithHarness} />);
		expect(screen.getByText('Rank the work by score.')).toBeInTheDocument();
		expect(screen.getByText('#harness')).toBeInTheDocument();
		expect(screen.queryByText('#web')).not.toBeInTheDocument();
		expect(screen.getByText('Anna')).toBeInTheDocument();
		expect(screen.getByText('Updated just now')).toBeInTheDocument();
		expect(screen.getByText('By hand')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Run' }));
		expect(onRunWithHarness).toHaveBeenCalledWith(expect.objectContaining({ id: 'p1' }));
		expect(screen.getByRole('button', { name: 'Move Sort the queue' }).closest('.status-move-control')).toHaveClass('toned', 'plan-status-draft');
	});
});

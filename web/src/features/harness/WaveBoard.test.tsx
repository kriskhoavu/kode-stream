import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { WaveBoard } from './WaveBoard';
import { createFixtureClient } from './fixture';
import type { HarnessRun, HarnessTask } from '../../lib/types';

class FakeResizeObserver {
	static instances: FakeResizeObserver[] = [];
	constructor(public callback: ResizeObserverCallback) { FakeResizeObserver.instances.push(this); }
	observe() {}
	disconnect() {}
	unobserve() {}
}

beforeEach(() => {
	vi.stubGlobal('ResizeObserver', FakeResizeObserver);
	vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => { callback(0); return 1; });
	vi.stubGlobal('cancelAnimationFrame', () => undefined);
	// Lay cards out on a grid so the curves have somewhere to go.
	vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
		const id = this.dataset.taskId;
		if (!id) return new DOMRect(0, 0, 1200, 800);
		const index = Number(id.length);
		return new DOMRect(index * 10, index * 5, 186, 74);
	});
});
afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
	localStorage.clear();
});

async function fixture() {
	const client = createFixtureClient({ tickMs: 0, arrivalMs: 0 });
	const tasks = await client.tasks();
	const runs = await client.runs();
	return { tasks, allTasks: Object.fromEntries(tasks.map((task) => [task.id, task])) as Record<string, HarnessTask>, runs: Object.fromEntries(runs.map((run) => [run.id, run])) as Record<string, HarnessRun> };
}

function renderBoard(data: Awaited<ReturnType<typeof fixture>>, handlers: Partial<{ onMove: () => void; onOpenTask: () => void; onPin: () => void }> = {}) {
	return render(<WaveBoard tasks={data.tasks} allTasks={data.allTasks} runs={data.runs} pendingIds={new Set()} onOpenTask={handlers.onOpenTask ?? vi.fn()} onPin={handlers.onPin ?? vi.fn()} onMove={handlers.onMove ?? vi.fn()} onRefresh={vi.fn()} />);
}

describe('WaveBoard', () => {
	it('lays the queue out in waves with plain statuses and counts', async () => {
		const data = await fixture();
		renderBoard(data);
		const wave0 = screen.getByRole('region', { name: /^Wave 0, 2 of 8 done/ });
		expect(within(wave0).getByRole('button', { name: /^PM-02, Add login form, Working, on the critical path/ })).toBeInTheDocument();
		expect(within(wave0).getByText('Builder · Sonnet 5.5')).toBeInTheDocument();
		expect(screen.getByText('48 of 48')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: /Waiting on you 2/ })).toHaveAttribute('aria-pressed', 'false');
		const blocked = screen.getByRole('button', { name: /^SRV-02, Local PostgreSQL, Not started, on the critical path, waits for SRV-01/ });
		expect(blocked).toHaveTextContent('Critical');
	});

	it('draws one curve per dependency between shown cards, hidden from assistive technology', async () => {
		const data = await fixture();
		const { container } = renderBoard(data);
		const svg = container.querySelector('svg.hx-edges') as SVGSVGElement;
		expect(svg).toHaveAttribute('aria-hidden', 'true');
		const expected = data.tasks.reduce((sum, task) => sum + task.dependsOn.length, 0);
		await act(async () => undefined);
		expect(svg.querySelectorAll('path')).toHaveLength(expected);
		expect(svg.querySelectorAll('path.crit').length).toBeGreaterThan(0);

		fireEvent.mouseEnter(screen.getByRole('button', { name: /^SRV-01,/ }).closest('article') as HTMLElement);
		const related = [...svg.querySelectorAll('path.on')].map((path) => path.getAttribute('data-edge'));
		expect(related).toContain('task-pm-02>task-srv-01');
		expect(related.every((key) => key?.includes('task-srv-01'))).toBe(true);
	});

	it('filters by status and by text, and the curves follow', async () => {
		const data = await fixture();
		renderBoard(data);
		fireEvent.click(screen.getByRole('button', { name: /Waiting on you 2/ }));
		expect(screen.getByText('2 of 48')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: /Waiting on you 2/ }));
		fireEvent.change(screen.getByRole('textbox', { name: 'Find a task' }), { target: { value: 'add login' } });
		expect(screen.getByText('1 of 48')).toBeInTheDocument();
	});

	it('moves a card within its wave and refuses another wave with a hint', async () => {
		const onMove = vi.fn();
		const data = await fixture();
		renderBoard(data, { onMove });
		const card = (name: RegExp) => screen.getByRole('button', { name }).closest('article') as HTMLElement;
		const dataTransfer = { setData: vi.fn(), effectAllowed: '', dropEffect: '' };
		fireEvent.dragStart(card(/^PM-04,/), { dataTransfer });
		const wave1 = screen.getByRole('region', { name: /^Wave 1,/ });
		fireEvent.dragOver(wave1, { dataTransfer });
		expect(screen.getByRole('status')).toHaveTextContent('A card only moves up or down its own wave');
		const wave0 = screen.getByRole('region', { name: /^Wave 0,/ });
		fireEvent.dragOver(card(/^ART-01,/), { dataTransfer });
		fireEvent.drop(within(wave0).getByRole('button', { name: /^ART-01,/ }), { dataTransfer });
		expect(onMove).toHaveBeenCalledWith(expect.objectContaining({ id: 'task-pm-04' }), expect.objectContaining({ id: 'task-art-01' }));
	});

	it('collapses a wave that is all done by default and remembers the choice', async () => {
		const data = await fixture();
		for (const task of data.tasks) if (!task.dependsOn.length) Object.assign(task, { state: 'done', needsYou: false });
		const { unmount } = renderBoard(data);
		expect(screen.getByRole('region', { name: /^Wave 0, 8 of 8 done/ })).toHaveTextContent('All 8 done');
		fireEvent.click(within(screen.getByRole('group', { name: 'Done waves' })).getByRole('button', { name: 'Hide' }));
		expect(screen.queryByRole('region', { name: /^Wave 0,/ })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Wave 0 · Show' })).toBeInTheDocument();
		unmount();
		renderBoard(data);
		expect(screen.queryByRole('region', { name: /^Wave 0,/ })).not.toBeInTheDocument();
	});

	it('shows the ranked list when grouped by priority', async () => {
		const data = await fixture();
		renderBoard(data);
		fireEvent.click(screen.getByRole('button', { name: 'Priority' }));
		expect(screen.getByRole('list', { name: 'Queue in order' })).toBeInTheDocument();
	});
});

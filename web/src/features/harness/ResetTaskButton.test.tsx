import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ResetTaskButton, canResetTask } from './ResetTaskButton';
import { task } from './testData';

describe('ResetTaskButton', () => {
	it('is offered only for a task in the queue that has had a run', () => {
		expect(canResetTask(task('PM-1'))).toBe(false);
		expect(canResetTask(task('PM-1', { state: 'cancelled', runId: 'run-1', attempts: 1 }))).toBe(true);
		expect(canResetTask(task('PM-1', { state: 'running', runId: 'run-1' }))).toBe(true);
		expect(canResetTask(task('PM-1', { state: 'dismissed', attempts: 2 }))).toBe(false);
	});

	it('resets only after a person confirms, and says what is deleted', async () => {
		const onReset = vi.fn().mockResolvedValue(undefined);
		const stopped = task('PM-4', { state: 'cancelled', runId: 'run-4', attempts: 1 });
		render(<ResetTaskButton task={stopped} onReset={onReset} />);
		fireEvent.click(screen.getByRole('button', { name: 'Reset' }));
		expect(screen.getByText(/working copy and local branch/)).toBeInTheDocument();
		expect(onReset).not.toHaveBeenCalled();
		fireEvent.click(screen.getByRole('button', { name: 'Reset this task' }));
		await waitFor(() => expect(onReset).toHaveBeenCalledWith(stopped));
	});

	it('shows why a reset failed', async () => {
		const onReset = vi.fn().mockRejectedValue(new Error('PM-4 moved'));
		render(<ResetTaskButton task={task('PM-4', { state: 'cancelled', runId: 'run-4' })} onReset={onReset} />);
		fireEvent.click(screen.getByRole('button', { name: 'Reset' }));
		fireEvent.click(screen.getByRole('button', { name: 'Reset this task' }));
		expect(await screen.findByRole('alert')).toHaveTextContent('PM-4 could not be reset.');
	});
});

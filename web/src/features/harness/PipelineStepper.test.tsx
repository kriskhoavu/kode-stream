import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { createFixtureClient } from './fixture';
import { pipelineModel } from './pipeline';
import { PipelineStepper } from './PipelineStepper';

afterEach(cleanup);

describe('PipelineStepper', () => {
	it('is one tab stop that arrow keys move through, starting at the current step', async () => {
		const model = pipelineModel(await createFixtureClient({ tickMs: 0, arrivalMs: 0 }).run('run-pm-02'));
		render(<PipelineStepper model={model} label="Progress for PM-02" />);
		expect(screen.getByRole('list', { name: 'Progress for PM-02' })).toBeInTheDocument();
		expect(screen.getByRole('group', { name: 'Build · round 1 of 3' })).toBeInTheDocument();
		const current = screen.getByRole('button', { current: 'step' });
		expect(current).toHaveTextContent('WorkingBuild');
		expect(current).toHaveAttribute('tabindex', '0');
		expect(screen.getAllByRole('button').filter((button) => button.getAttribute('tabindex') === '0')).toHaveLength(1);

		current.focus();
		fireEvent.keyDown(current, { key: 'ArrowRight' });
		const next = screen.getByRole('button', { name: /Code check/ });
		expect(next).toHaveFocus();
		expect(next).toHaveAttribute('tabindex', '0');
		fireEvent.keyDown(next, { key: 'Home' });
		expect(screen.getAllByRole('button')[0]).toHaveFocus();
		expect(screen.getAllByRole('button')[0]).toHaveTextContent(/^DoneStart/);
	});

	it('explains a step in two plain sentences and closes on Escape', async () => {
		const model = pipelineModel(await createFixtureClient({ tickMs: 0, arrivalMs: 0 }).run('run-pm-02'));
		render(<PipelineStepper model={model} label="Progress" />);
		fireEvent.click(screen.getByRole('button', { name: /Code check/ }));
		expect(screen.getByRole('note')).toHaveTextContent('The Checker reads the change and lists must-fix items. The Builder fixes them, up to 3 rounds. It has not started yet.');
		fireEvent.keyDown(window, { key: 'Escape' });
		expect(screen.queryByRole('note')).not.toBeInTheDocument();
	});
});

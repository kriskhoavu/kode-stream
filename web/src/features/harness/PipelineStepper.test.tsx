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
		// Start is a small circle: its words are its name, not its text.
		expect(screen.getAllByRole('button')[0]).toHaveAccessibleName(/^Start, Done/);
		expect(screen.getAllByRole('button')[0]).toHaveClass('hx-stage-dot');
	});

	it('keeps Start a full step in the drawer', async () => {
		const model = pipelineModel(await createFixtureClient({ tickMs: 0, arrivalMs: 0 }).run('run-pm-02'));
		render(<PipelineStepper model={model} label="Progress" orientation="vertical" />);
		expect(screen.getAllByRole('button')[0]).toHaveTextContent(/^DoneStart/);
	});

	it('names what a step does on hover and opens nothing over the pipeline', async () => {
		const model = pipelineModel(await createFixtureClient({ tickMs: 0, arrivalMs: 0 }).run('run-pm-02'));
		render(<PipelineStepper model={model} label="Progress" />);
		const step = screen.getByRole('button', { name: /Code check/ });
		expect(step).toHaveAttribute('title', 'The Checker reads the change and lists must-fix items. The Builder fixes them, up to 3 rounds. It has not started yet.');
		fireEvent.click(step);
		expect(screen.queryByRole('note')).not.toBeInTheDocument();
	});

	it('marks the step clicked when the page opens what it made', async () => {
		const model = pipelineModel(await createFixtureClient({ tickMs: 0, arrivalMs: 0 }).run('run-pm-02'));
		const opened: string[] = [];
		render(<PipelineStepper model={model} label="Progress" onSelect={(card) => opened.push(card.step)} />);
		fireEvent.click(screen.getByRole('button', { name: /Plan check/ }));
		expect(opened).toEqual(['plan_check']);
		expect(screen.getByRole('button', { name: /Plan check/ })).toHaveAttribute('aria-pressed', 'true');
		expect(screen.getByText('Click a step to open what it made.')).toBeInTheDocument();
	});
});

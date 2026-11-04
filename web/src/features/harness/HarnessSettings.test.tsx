import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { HarnessSettings } from './HarnessSettings';
import { createFixtureClient } from './fixture';

afterEach(cleanup);

async function setup() {
	const client = createFixtureClient({ tickMs: 0, arrivalMs: 0 });
	const status = await client.status();
	const onPause = vi.fn().mockResolvedValue(undefined);
	const onResume = vi.fn().mockResolvedValue(undefined);
	const onSaved = vi.fn();
	render(<HarnessSettings client={client} status={status} onPause={onPause} onResume={onResume} onSaved={onSaved} />);
	await screen.findByRole('heading', { name: 'Who does what' });
	return { client, onPause, onResume, onSaved };
}

describe('HarnessSettings', () => {
	it('shows who does what in plain words, with an easy and a hard model for the Builder', async () => {
		await setup();
		expect(screen.getByRole('combobox', { name: 'Planner tool for “Writes the plan”' })).toHaveDisplayValue('Claude');
		expect(screen.getAllByRole('option', { name: 'Codex (not installed)' })[0]).toBeDisabled();
		expect(screen.getByRole('combobox', { name: /^Easy/ })).toHaveValue('claude-sonnet-5-5');
		expect(screen.getByRole('combobox', { name: /^Hard/ })).toHaveValue('claude-opus-5-5');
		expect(screen.getByText('What counts as hard?')).toBeInTheDocument();
		expect(screen.getByText('No model.')).toBeInTheDocument();
		expect(screen.getByText(/Cannot limit which tools an agent may use/)).toBeInTheDocument();
		expect(screen.queryByText(/worktree|SSE/i)).not.toBeInTheDocument();
	});

	it('keeps the source query folded under Advanced', async () => {
		await setup();
		const advanced = screen.getAllByText('Advanced')[0].closest('details') as HTMLDetailsElement;
		expect(advanced.open).toBe(false);
		expect(screen.getByText('Looked 12 min ago · 3 new')).toBeInTheDocument();
	});

	it('saves a changed model and says so', async () => {
		const { client, onSaved } = await setup();
		expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
		fireEvent.change(screen.getByRole('combobox', { name: /^Hard/ }), { target: { value: 'opus' } });
		fireEvent.click(screen.getByRole('button', { name: 'Save' }));
		await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Saved.'));
		expect(onSaved).toHaveBeenCalled();
		expect((await client.settings()).modelPolicy.coder.hardModel).toBe('opus');
	});

	it('asks before pausing a role with work in flight, and resumes without asking', async () => {
		const { onPause, onResume } = await setup();
		const builder = screen.getByText('Builder', { selector: 'strong' }).closest('.hx-role-row') as HTMLElement;
		fireEvent.click(within(builder).getByRole('switch'));
		const dialog = screen.getByRole('dialog', { name: 'Pause the Builder?' });
		expect(dialog).toHaveTextContent('1 task is with the Builder now.');
		fireEvent.click(within(dialog).getByRole('button', { name: 'Pause' }));
		expect(onPause).toHaveBeenCalledWith('coder');

		const finder = screen.getByText('Finder', { selector: 'strong' }).closest('.hx-role-row') as HTMLElement;
		fireEvent.click(within(finder).getByRole('switch'));
		expect(onPause).toHaveBeenCalledWith('scout');
		expect(onResume).not.toHaveBeenCalled();
	});
});

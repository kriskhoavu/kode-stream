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

	it('adds a Jira source for a workspace with its project as the first query, and saves it', async () => {
		const { client } = await setup();
		const form = await screen.findByRole('form', { name: 'Add a source' });
		fireEvent.change(within(form).getByRole('combobox', { name: 'Workspace' }), { target: { value: 'airship-docs' } });
		fireEvent.change(within(form).getByRole('combobox', { name: 'Look in' }), { target: { value: 'jira' } });
		fireEvent.click(within(form).getByRole('button', { name: 'Add source' }));
		expect(screen.getByRole('status')).toHaveTextContent('Added. Save to let the Finder look there.');
		expect(screen.getByDisplayValue('project = DOC AND sprint in openSprints() AND statusCategory != Done ORDER BY updated DESC')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Save' }));
		await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Saved.'));
		const sources = (await client.settings()).sources;
		expect(sources).toHaveLength(4);
		expect(sources[3]).toMatchObject({ workspaceId: 'airship-docs', kind: 'jira', enabled: true });
	});

	it('refuses a second source of the same kind for one workspace, and offers Jira, GitHub and a local folder', async () => {
		await setup();
		const form = await screen.findByRole('form', { name: 'Add a source' });
		expect(within(form).getAllByRole('option').map((option) => option.textContent)).toEqual(['Airship', 'Airship client', 'Airship docs', 'Jira', 'GitHub', 'Local folder']);
		fireEvent.change(within(form).getByRole('combobox', { name: 'Workspace' }), { target: { value: 'airship' } });
		expect(within(form).getByRole('button', { name: 'Add source' })).toBeDisabled();
		expect(form).toHaveTextContent('Jira is already set for this workspace.');
	});

	it('clears a workspace queue after a confirmation and says how many tasks went', async () => {
		const { client, onSaved } = await setup();
		const before = (await client.tasks()).filter((task) => task.workspaceId === 'airship' && (task.state === 'ranked' || task.state === 'discovered' || task.state === 'stale' || task.state === 'dismissed')).length;
		expect(before).toBeGreaterThan(0);
		fireEvent.click(screen.getAllByRole('button', { name: 'Clear the Task Queue for Airship' })[0]);
		const dialog = await screen.findByRole('dialog');
		expect(within(dialog).getByText(/Work that is running, waiting on you or done stays/)).toBeInTheDocument();
		fireEvent.click(within(dialog).getByRole('button', { name: 'Clear the Task Queue' }));
		await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent(`Cleared ${before} tasks from the Task Queue`));
		expect(onSaved).toHaveBeenCalled();
		const after = await client.tasks();
		expect(after.filter((task) => task.workspaceId === 'airship' && task.state === 'ranked')).toHaveLength(0);
		expect(after.some((task) => task.workspaceId === 'airship' && task.state === 'running')).toBe(true);
	});

	it('turns stop-and-ask off everywhere and back on for one workspace', async () => {
		const { client } = await setup();
		const everywhere = screen.getByRole('switch', { name: /Let an agent stop and ask you/ });
		expect(everywhere).toBeChecked();
		fireEvent.click(everywhere);
		const airship = screen.getByRole('combobox', { name: 'May an agent stop and ask in Airship' });
		expect(airship).toHaveDisplayValue('Same as everywhere (no)');
		fireEvent.change(airship, { target: { value: 'yes' } });
		fireEvent.click(screen.getByRole('button', { name: 'Save' }));
		await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Saved'));
		const saved = await client.settings();
		expect(saved.askQuestions).toBe(false);
		expect(saved.sources.filter((source) => source.workspaceId === 'airship').every((source) => source.askQuestions === true)).toBe(true);
	});

	it('sets one test command for every source of a workspace', async () => {
		const { client } = await setup();
		fireEvent.change(screen.getByRole('textbox', { name: 'Test command for Airship' }), { target: { value: 'go test ./...' } });
		fireEvent.click(screen.getByRole('button', { name: 'Save' }));
		await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Saved'));
		const saved = await client.settings();
		expect(saved.sources.filter((source) => source.workspaceId === 'airship').every((source) => source.verificationCommands?.[0] === 'go test ./...')).toBe(true);
	});

	it('removes a source and names workspaces by their name', async () => {
		await setup();
		expect(screen.getByText('· Airship client')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Remove Bitbucket for Airship client' }));
		expect(screen.queryByText('· Airship client')).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled();
	});
});

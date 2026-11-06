import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AgentsTab } from './AgentsPanel';

vi.mock('../ai-session/AISessionLaunchControl', () => ({ AISessionLaunchControl: ({ buttonLabel }: { buttonLabel: string }) => <button type="button">{buttonLabel}</button> }));

afterEach(cleanup);

describe('AgentsTab modes', () => {
	it('is the hand-driven tab alone without a harness run', () => {
		render(<AgentsTab itemId="p1" onFocus={vi.fn()} onLaunched={vi.fn()} onError={vi.fn()} />);
		expect(screen.queryByRole('tablist', { name: 'Agents mode' })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'New session' })).toBeInTheDocument();
	});

	it('opens on the harness run and switches to the sessions', () => {
		const onModeChange = vi.fn();
		render(<AgentsTab itemId="p1" onFocus={vi.fn()} onLaunched={vi.fn()} onError={vi.fn()} harnessRun={<p>The run</p>} onModeChange={onModeChange} />);
		expect(screen.getByRole('tab', { name: 'Harness run' })).toHaveAttribute('aria-selected', 'true');
		expect(screen.getByText('The run')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'New session' })).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('tab', { name: 'Sessions' }));
		expect(onModeChange).toHaveBeenCalledWith('sessions');
		expect(screen.getByRole('button', { name: 'New session' })).toBeInTheDocument();
		expect(screen.queryByText('The run')).not.toBeInTheDocument();
	});
});

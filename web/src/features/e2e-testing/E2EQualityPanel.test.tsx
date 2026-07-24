import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { E2EQualityPanel } from './E2EQualityPanel';

vi.mock('../ai-session/AISessionLaunchDialog', () => ({ AISessionLaunchDialog: () => <div>AI dialog</div> }));

describe('E2EQualityPanel', () => {
	it('shows a runbook latest result and opens the E2E handoff', async () => {
		render(<E2EQualityPanel workspaceId="workspace-1" runbooks={[{ title: 'Review offer PDF', path: 'plans/api/DI-365/automation/scenario-01.md', source: 'plan', latestResult: { status: 'failed', provider: 'playwright', environment: 'staging', failedStep: 'Review PDF', evidence: [] } }]} />);
		expect(screen.getByText('Review offer PDF')).toBeInTheDocument();
		expect(screen.getByText('failed · playwright · staging · failed: Review PDF')).toBeInTheDocument();
		screen.getByRole('button', { name: 'Run E2E test' }).click();
		expect(await screen.findByText('AI dialog')).toBeInTheDocument();
	});

	it('explains when no E2E coverage exists', () => {
		render(<E2EQualityPanel workspaceId="workspace-1" diagnostic="No E2E coverage." />);
		expect(screen.getByText('No E2E coverage.')).toBeInTheDocument();
	});
});

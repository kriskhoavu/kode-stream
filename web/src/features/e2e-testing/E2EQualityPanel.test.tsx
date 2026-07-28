import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { E2EQualityPanel } from './E2EQualityPanel';

vi.mock('../../lib/api', () => ({ api: { workspaceFile: vi.fn() } }));
vi.mock('../content-viewer/ContentViewer', () => ({ ContentViewer: ({ file }: { file: { path: string } }) => <div>Previewing {file.path}</div> }));
vi.mock('../ai-session/AISessionLaunchDialog', () => ({ AISessionLaunchDialog: () => <div>AI dialog</div> }));

const planRunbook = {
	title: 'Review offer PDF',
	path: 'plans/api/DI-365/automation/scenario-01.md',
	resultPath: 'plans/api/DI-365/automation/results/latest.md',
	source: 'plan',
	latestResult: {
		status: 'failed',
		provider: 'playwright',
		environment: 'staging',
		failedStep: 'Review PDF',
		evidence: ['plans/api/DI-365/automation/artifacts/failure.png']
	}
};

describe('E2EQualityPanel', () => {
	it('shows plan coverage before canonical coverage and opens the E2E handoff', async () => {
		render(<E2EQualityPanel workspaceId="workspace-1" runbooks={[
			{ title: 'Canonical offer journey', path: 'wiki/e2e-testing/offer.md', resultPath: 'plans/api/DI-365/automation/results/latest.md', source: 'wiki' },
			planRunbook
		]} />);
		const groupButtons = screen.getAllByRole('button', { name: /runbooks|journeys/i });
		expect(groupButtons[0]).toHaveTextContent('Plan runbooks');
		expect(groupButtons[1]).toHaveTextContent('Canonical wiki journeys');
		expect(screen.getByText('Review offer PDF')).toBeInTheDocument();
		expect(screen.getByText('failed · playwright · staging · failed: Review PDF')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Run E2E test' }));
		expect(await screen.findByText('AI dialog')).toBeInTheDocument();
	});

	it('keeps a canonical coverage diagnostic visible beside local coverage', () => {
		render(<E2EQualityPanel workspaceId="workspace-1" runbooks={[planRunbook]} diagnostic="Canonical E2E coverage could not be loaded." />);
		expect(screen.getByRole('status')).toHaveTextContent('Canonical E2E coverage could not be loaded.');
		expect(screen.getByText('Review offer PDF')).toBeInTheDocument();
	});

	it('opens evidence through the safe workspace file reader', async () => {
		vi.mocked(api.workspaceFile).mockResolvedValue({
			id: 'failure_png',
			path: 'plans/api/DI-365/automation/artifacts/failure.png',
			content: 'data:image/png;base64,abc',
			language: 'image/png',
			hash: 'hash',
			kind: 'image',
			sizeBytes: 3,
			editable: false
		});
		render(<E2EQualityPanel workspaceId="workspace-1" runbooks={[planRunbook]} />);
		const evidence = screen.getByLabelText('Evidence for Review offer PDF');
		fireEvent.click(within(evidence).getByRole('button'));
		await waitFor(() => expect(api.workspaceFile).toHaveBeenCalledWith('workspace-1', 'plans/api/DI-365/automation/artifacts/failure.png'));
		expect(await screen.findByText('Previewing plans/api/DI-365/automation/artifacts/failure.png')).toBeInTheDocument();
	});

	it('explains when no E2E coverage exists', () => {
		render(<E2EQualityPanel workspaceId="workspace-1" diagnostic="No E2E coverage." />);
		expect(screen.getByText('No E2E coverage.')).toBeInTheDocument();
	});
});

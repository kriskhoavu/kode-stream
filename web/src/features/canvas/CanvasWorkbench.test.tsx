import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, api } from '../../lib/api';
import type { CanvasNode, CanvasProjection, EmbeddedAISessionResult, SafeSessionRecord } from '../../lib/types';
import { CanvasWorkbench } from './CanvasWorkbench';

vi.mock('../../lib/api', async () => {
	const actual = await vi.importActual<typeof import('../../lib/api')>('../../lib/api');
	return { ...actual, api: { aiSettings: vi.fn(), startEmbeddedAISession: vi.fn(), embeddedAISession: vi.fn(), embeddedAISessionGrant: vi.fn(), cancelEmbeddedAISession: vi.fn(), createVerificationJob: vi.fn(), verificationJob: vi.fn() } };
});
describe('CanvasWorkbench', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.aiSettings).mockResolvedValue({ defaultProvider: 'codex', defaultTerminal: '', providers: {}, terminals: {} });
		vi.mocked(api.startEmbeddedAISession).mockResolvedValue(sessionResult());
		vi.mocked(api.embeddedAISession).mockResolvedValue(sessionResult().session);
		vi.mocked(api.embeddedAISessionGrant).mockResolvedValue(sessionResult().grant);
		vi.mocked(api.cancelEmbeddedAISession).mockResolvedValue({ ...sessionResult().session, state: 'cancelled' });
		vi.spyOn(window, 'confirm').mockReturnValue(true);
	});

	it('does not expose a closed Workbench when no workspace or plan is selected', () => {
		render(<CanvasWorkbench {...defaultProps(baseProjection(), undefined)} />);
		expect(screen.queryByLabelText('Canvas Workbench')).not.toBeInTheDocument();
	});

	it('launches one branch-safe process for a double submission', async () => {
		let resolveSettings!: (value: Awaited<ReturnType<typeof api.aiSettings>>) => void;
		vi.mocked(api.aiSettings).mockReturnValue(new Promise((resolve) => { resolveSettings = resolve; }));
		const reload = vi.fn();
		const placeSession = vi.fn();
		const selectNode = vi.fn();
		renderWorkbench(planNode(), { onReload: reload, onPlaceSession: placeSession, onSelectNode: selectNode });
		const launch = screen.getByRole('button', { name: 'Launch terminal' });
		fireEvent.click(launch);
		fireEvent.click(launch);
		expect(api.aiSettings).toHaveBeenCalledTimes(1);
		resolveSettings({ defaultProvider: 'codex', defaultTerminal: '', providers: {}, terminals: {} });
		await waitFor(() => expect(api.startEmbeddedAISession).toHaveBeenCalledTimes(1));
		expect(api.startEmbeddedAISession).toHaveBeenCalledWith('item-1', expect.objectContaining({ expectedWorkspaceId: 'workspace-1', expectedBranch: 'main', observedCommit: 'abc123', idempotencyKey: expect.any(String) }));
		await waitFor(() => expect(placeSession).toHaveBeenCalledWith('session-1'));
		expect(selectNode).toHaveBeenCalledWith('session:session-1');
		expect(reload).toHaveBeenCalled();
	});

	it('shows expected and current branch recovery without starting a terminal', async () => {
		vi.mocked(api.startEmbeddedAISession).mockRejectedValue(new ApiError('checkout changed', undefined, undefined, { code: 'terminal_branch_mismatch', details: { expectedBranch: 'main', currentBranch: 'other', dirty: 'true' }, status: 409 }));
		const reload = vi.fn();
		renderWorkbench(planNode(), { onReload: reload });
		fireEvent.click(screen.getByRole('button', { name: 'Launch terminal' }));
		await screen.findByText('Checkout changed');
		expect(screen.getByText('main')).toBeInTheDocument();
		expect(screen.getByText('other')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: /Refresh Canvas/ }));
		expect(reload).toHaveBeenCalled();
	});

	it('separates verification result from freshness and removes stale success emphasis', async () => {
		const workspace = workspaceNode();
		renderWorkbench(workspace);
		expect(screen.getByText('Passed (historical)')).toBeInTheDocument();
		expect(screen.getByText('stale')).toBeInTheDocument();
		expect(screen.getByText('11111111')).toBeInTheDocument();
		expect(screen.getAllByText('22222222')).toHaveLength(2);
		expect(screen.getByRole('button', { name: /Run smoke verification/ })).toBeDisabled();
	});

	it('runs verification and opens the full view from a plan', async () => {
		const plan = planNode();
		plan.plan!.actions['verification.run'] = { action: 'verification.run', state: 'available', recoveryActions: [] };
		const projection = baseProjection([workspaceNode(), plan]);
		const openFullView = vi.fn();
		vi.mocked(api.createVerificationJob).mockResolvedValue({ id: 'verify-2', workspaceId: 'workspace-1', profile: 'smoke', status: 'passed', exitCode: 0, steps: [], artifacts: [] });
		render(<CanvasWorkbench {...defaultProps(projection, plan)} onOpenFullView={openFullView} />);
		fireEvent.click(screen.getByRole('button', { name: 'Run smoke verification' }));
		await waitFor(() => expect(api.createVerificationJob).toHaveBeenCalledWith('workspace-1', { profile: 'smoke', trigger: 'canvas' }));
		fireEvent.click(screen.getByRole('button', { name: 'Open full view' }));
		expect(openFullView).toHaveBeenCalledWith(plan);
	});

	it('hides cached details for forbidden references and offers safe refresh', async () => {
		const forbidden: CanvasNode = { id: 'plan:gone', kind: 'plan', state: 'forbidden', entityRef: { kind: 'plan', workspaceId: 'workspace-1', itemId: 'gone', itemPath: 'private/Secret', identifier: 'SECRET-TITLE', branchKey: 'main' }, position: { x: 0, y: 0 }, collapsed: false, revision: 1 };
		const reload = vi.fn();
		renderWorkbench(forbidden, { onReload: reload });
		expect(screen.getByText('Access restricted')).toBeInTheDocument();
		expect(screen.queryByText('SECRET-TITLE')).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: /Refresh reference/ }));
		expect(reload).toHaveBeenCalled();
	});
});

function renderWorkbench(selectedNode: CanvasNode, overrides: Partial<React.ComponentProps<typeof CanvasWorkbench>> = {}) {
	const projection = baseProjection([planNode(), selectedNode]);
	return render(<CanvasWorkbench {...defaultProps(projection, selectedNode)} {...overrides} />);
}

function defaultProps(projection: CanvasProjection, selectedNode?: CanvasNode): React.ComponentProps<typeof CanvasWorkbench> {
	return { projection, selectedNode, onClose: vi.fn(), onReload: vi.fn(), onSelectNode: vi.fn(), onPlaceSession: vi.fn() };
}

function baseProjection(nodes: CanvasNode[] = [planNode()]): CanvasProjection {
	return { layout: { id: 'layout', workspaceId: 'workspace-1', branchKey: 'main', viewport: { x: 0, y: 0, zoom: 1 }, version: 1, createdAt: '', updatedAt: '' }, nodes, connections: [], unplaced: [] };
}

function planNode(): CanvasNode {
	return { id: 'plan:item-1', kind: 'plan', state: 'resolved', entityRef: { kind: 'plan', workspaceId: 'workspace-1', itemId: 'item-1', itemPath: 'plans/platform/PM-037', branchKey: 'main', observedCommit: 'abc123' }, position: { x: 0, y: 0 }, collapsed: false, revision: 1, plan: { itemId: 'item-1', identifier: 'PM-037', title: 'Canvas', service: 'platform', status: 'in_progress', branch: 'main', commit: 'abc123', editable: true, actions: { 'terminal.launch': { action: 'terminal.launch', state: 'available', recoveryActions: [] } } } };
}

function sessionRecord(): SafeSessionRecord {
	return { id: 'session-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', requestedBranch: 'main', state: 'running', startedAt: '', lastKnownAt: '', live: true };
}

function sessionResult(): EmbeddedAISessionResult {
	return { session: { id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'running', startedAt: '' }, grant: { sessionId: 'session-1', token: 'grant', expiresAt: '' }, record: sessionRecord() };
}

function workspaceNode(): CanvasNode {
	return { id: 'workspace:workspace-1', kind: 'workspace', state: 'resolved', entityRef: { kind: 'workspace', workspaceId: 'workspace-1' }, position: { x: 0, y: 0 }, collapsed: false, revision: 1, workspace: { id: 'workspace-1', name: 'Workspace', branch: 'main', commit: '222222223333', git: { workspaceId: 'workspace-1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }, providerAxes: { topology: 'local_application', contentProvider: 'local_checkout', executionProvider: 'local_process' }, actions: { 'verification.run': { action: 'verification.run', state: 'unsupported', message: 'No runtime', recoveryActions: [] } }, verification: { id: 'verify-1', workspaceId: 'workspace-1', profile: 'smoke', status: 'passed', exitCode: 0, steps: [], artifacts: [], freshness: 'stale', finishFingerprint: { value: 'one', branch: 'main', commit: '111111112222' }, currentFingerprint: { value: 'two', branch: 'main', commit: '222222223333' } } } };
}

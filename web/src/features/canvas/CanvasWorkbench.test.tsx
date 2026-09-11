import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import type { CanvasNode, CanvasProjection, EmbeddedAISessionResult, SafeSessionRecord } from '../../lib/types';
import { CanvasWorkbench } from './CanvasWorkbench';

vi.mock('../../shared/api', async () => {
	const actual = await vi.importActual<typeof import('../../shared/api')>('../../shared/api');
	return { ...actual, api: { embeddedAISession: vi.fn(), embeddedAISessionGrant: vi.fn(), cancelEmbeddedAISession: vi.fn(), createVerificationJob: vi.fn(), verificationJob: vi.fn(), itemE2ERunbooks: vi.fn(), item: vi.fn(), saveMetadata: vi.fn(), aiSessionRecords: vi.fn(), sessionChanges: vi.fn(), sessionFileDiff: vi.fn() } };
});
vi.mock('../ai-session/AISessionLaunchDialog', () => ({
	AISessionLaunchDialog: ({ onLaunched }: { onLaunched: (result: EmbeddedAISessionResult, input: { provider: string; terminal: string; contextMode: 'card_context'; surface: 'embedded' }) => void }) => <button type="button" aria-label="Complete embedded AI session" onClick={() => onLaunched({ session: { id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'running', startedAt: '' }, grant: { sessionId: 'session-1', token: 'grant', expiresAt: '' } }, { provider: 'codex', terminal: 'terminal', contextMode: 'card_context', surface: 'embedded' })}>Complete embedded AI session</button>
}));
vi.mock('../ai-session/EmbeddedTerminal', () => ({
	EmbeddedTerminal: ({ initial, visible }: { initial: EmbeddedAISessionResult; visible: boolean }) => <section data-testid={`workbench-terminal-${initial.session.id}`} data-visible={String(visible)} />
}));
describe('CanvasWorkbench', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.embeddedAISession).mockResolvedValue(sessionResult().session);
		vi.mocked(api.embeddedAISessionGrant).mockResolvedValue(sessionResult().grant);
		vi.mocked(api.cancelEmbeddedAISession).mockResolvedValue({ ...sessionResult().session, state: 'cancelled' });
		vi.mocked(api.itemE2ERunbooks).mockResolvedValue({ runbooks: [] });
		vi.mocked(api.item).mockRejectedValue(new Error('Item details unavailable in this test.'));
		vi.mocked(api.aiSessionRecords).mockResolvedValue([]);
		vi.mocked(api.sessionChanges).mockResolvedValue({ sessionId: 'session-1', launchCommit: 'a', headCommit: 'b', headMoved: 0, files: [], additions: 0, deletions: 0, truncated: false, availability: 'available' });
		vi.spyOn(window, 'confirm').mockReturnValue(true);
	});

	it('does not expose a closed Workbench when no workspace or plan is selected', () => {
		render(<CanvasWorkbench {...defaultProps(baseProjection(), undefined)} />);
		expect(screen.queryByLabelText('Canvas Workbench')).not.toBeInTheDocument();
	});

	it('opens an AI session dialog from a workspace node when terminal launch is available', () => {
		const workspace = workspaceNode();
		workspace.workspace!.actions['terminal.launch'] = { action: 'terminal.launch', state: 'available', recoveryActions: [] };
		renderWorkbench(workspace, { aiSessionDialogOpen: true });
		expect(screen.getByRole('button', { name: 'Complete embedded AI session' })).toBeInTheDocument();
	});

	it('opens the AI session dialog and adds its embedded session to the canvas', async () => {
		const reload = vi.fn();
		const placeSession = vi.fn();
		const selectNode = vi.fn();
		renderWorkbench(planNode(), { aiSessionDialogOpen: true, onReload: reload, onPlaceSession: placeSession, onSelectNode: selectNode });
		expect(screen.getByRole('button', { name: 'Complete embedded AI session' })).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Complete embedded AI session' }));
		await waitFor(() => expect(placeSession).toHaveBeenCalledWith('session-1'));
		expect(selectNode).toHaveBeenCalledWith('session:session-1');
		expect(reload).toHaveBeenCalled();
	});

	it('reports a canvas placement failure after an embedded session opens', async () => {
		const placeSession = vi.fn().mockRejectedValue(new Error('Canvas unavailable'));
		renderWorkbench(planNode(), { aiSessionDialogOpen: true, onPlaceSession: placeSession });
		fireEvent.click(screen.getByRole('button', { name: 'Complete embedded AI session' }));
		expect(await screen.findByText('Canvas unavailable')).toHaveAttribute('role', 'alert');
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
		fireEvent.click(screen.getByRole('button', { name: 'Quality' }));
		fireEvent.click(screen.getByRole('button', { name: 'Run smoke verify' }));
		await waitFor(() => expect(api.createVerificationJob).toHaveBeenCalledWith('workspace-1', { profile: 'smoke', trigger: 'manual_checkpoint', terminalMode: 'embedded' }));
		fireEvent.click(screen.getByRole('button', { name: 'View details' }));
		expect(openFullView).toHaveBeenCalledWith(plan);
	});

	it('provides the same left-edge resize control as the Workstream item panel', async () => {
		renderWorkbench(planNode());
		expect(screen.getByRole('button', { name: 'Resize Workbench panel' })).toBeInTheDocument();
		await screen.findByText('Item details unavailable in this test.');
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

	it('aborts a superseded E2E runbook request without painting stale coverage', async () => {
		let signal: AbortSignal | undefined;
		vi.mocked(api.itemE2ERunbooks).mockImplementation((_id, nextSignal) => {
			signal = nextSignal;
			return new Promise(() => {});
		});
		const { unmount } = renderWorkbench(planNode());
		await waitFor(() => expect(signal).toBeDefined());
		unmount();
		expect(signal?.aborted).toBe(true);
	});

	it('shows Info and Sessions tabs for a workspace node, and lists sessions under Sessions', async () => {
		vi.mocked(api.aiSessionRecords).mockResolvedValue([sessionRecord()]);
		const workspace = workspaceNode();
		renderWorkbench(workspace);
		expect(screen.getByRole('button', { name: 'Info' })).toHaveClass('active');
		expect(screen.getByRole('button', { name: 'Sessions' })).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Sessions' }));
		expect(await screen.findByRole('button', { name: 'card_context, Running' })).toBeInTheDocument();
		await screen.findByTestId('workbench-terminal-session-1');
	});

	it('widens the Workbench panel while the Sessions tab is active, and reverts once another tab is selected', async () => {
		vi.mocked(api.aiSessionRecords).mockResolvedValue([]);
		const workspace = workspaceNode();
		renderWorkbench(workspace);
		expect(screen.getByLabelText('Canvas Workbench')).not.toHaveClass('sessions-wide');
		fireEvent.click(screen.getByRole('button', { name: 'Sessions' }));
		await screen.findByText('No sessions for this workspace and branch.');
		expect(screen.getByLabelText('Canvas Workbench')).toHaveClass('sessions-wide');
		fireEvent.click(screen.getByRole('button', { name: 'Info' }));
		expect(screen.getByLabelText('Canvas Workbench')).not.toHaveClass('sessions-wide');
	});

	it('lets the resize handle widen the panel past the usual pixel cap while the Sessions tab is active', async () => {
		vi.mocked(api.aiSessionRecords).mockResolvedValue([]);
		const workspace = workspaceNode();
		renderWorkbench(workspace);
		fireEvent.click(screen.getByRole('button', { name: 'Sessions' }));
		await screen.findByText('No sessions for this workspace and branch.');
		const aside = screen.getByLabelText('Canvas Workbench');
		const defaultWidth = parseInt(aside.style.getPropertyValue('--canvas-workbench-width'), 10);
		expect(defaultWidth).toBeGreaterThan(520); // the Sessions tab already defaults wider than the old fixed cap.
		const handle = screen.getByRole('button', { name: 'Resize Workbench panel' });
		fireEvent(handle, pointerEventWithClientX('pointerdown', 500));
		act(() => { window.dispatchEvent(pointerEventWithClientX('pointermove', 50)); });
		act(() => { window.dispatchEvent(pointerEventWithClientX('pointerup', 50)); });
		const widenedWidth = parseInt(aside.style.getPropertyValue('--canvas-workbench-width'), 10);
		expect(widenedWidth).toBeGreaterThan(defaultWidth);
		expect(widenedWidth).toBeGreaterThan(520);
	});

	it('opens the session node with Terminal selected by default and Changes available', async () => {
		renderWorkbench(sessionNode());
		expect(screen.getByLabelText('Canvas Workbench')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Terminal' })).toHaveClass('active');
		expect(screen.getByText('codex')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Changes' }));
		expect(await screen.findByText('+0 −0 · 0 files · HEAD moved 0 commits')).toBeInTheDocument();
	});

	it('shows a stop failure inside the Terminal tab confirm dialog', async () => {
		vi.mocked(api.cancelEmbeddedAISession).mockRejectedValue(new Error('The session could not be stopped.'));
		renderWorkbench(sessionNode());
		fireEvent.click(screen.getByRole('button', { name: 'Stop codex session' }));
		const dialog = await screen.findByRole('dialog');
		fireEvent.click(within(dialog).getByRole('button', { name: 'Stop session' }));
		expect(await within(dialog).findByRole('alert')).toHaveTextContent('The session could not be stopped.');
		expect(screen.getAllByRole('alert')).toHaveLength(1);
	});

	it('applies an initial tab hint once and reports it back', async () => {
		const onInitialTabApplied = vi.fn();
		renderWorkbench(sessionNode(), { initialTab: 'changes', onInitialTabApplied });
		await waitFor(() => expect(screen.getByRole('button', { name: 'Changes' })).toHaveClass('active'));
		expect(onInitialTabApplied).toHaveBeenCalled();
	});

	it('refetches the Sessions tab after a session is stopped', async () => {
		vi.mocked(api.aiSessionRecords).mockResolvedValueOnce([sessionRecord()]).mockResolvedValueOnce([]);
		const workspace = workspaceNode();
		renderWorkbench(workspace);
		fireEvent.click(screen.getByRole('button', { name: 'Sessions' }));
		expect(await screen.findByRole('button', { name: 'card_context, Running' })).toBeInTheDocument();
		await screen.findByTestId('workbench-terminal-session-1');
		fireEvent.click(screen.getByRole('button', { name: 'Stop card_context session' }));
		const dialog = await screen.findByRole('dialog');
		fireEvent.click(within(dialog).getByRole('button', { name: 'Stop session' }));
		await waitFor(() => expect(api.cancelEmbeddedAISession).toHaveBeenCalledWith('session-1'));
		await waitFor(() => expect(api.aiSessionRecords).toHaveBeenCalledTimes(2));
		expect(screen.queryByRole('button', { name: 'card_context, Running' })).not.toBeInTheDocument();
	});

	it('opens a session on the board from the Sessions tab', async () => {
		vi.mocked(api.aiSessionRecords).mockResolvedValue([sessionRecord()]);
		const onOpenSession = vi.fn();
		const workspace = workspaceNode();
		renderWorkbench(workspace, { onOpenSession });
		fireEvent.click(screen.getByRole('button', { name: 'Sessions' }));
		await screen.findByRole('button', { name: 'card_context, Running' });
		await screen.findByTestId('workbench-terminal-session-1');
		fireEvent.click(screen.getByRole('button', { name: 'Open on board' }));
		expect(onOpenSession).toHaveBeenCalledWith('session-1');
	});
});

// jsdom has no PointerEvent constructor, so the resize drag (a plain
// window pointermove/pointerup listener pair, not a React handler) is
// exercised with a bare Event carrying a manually attached clientX.
function pointerEventWithClientX(type: string, clientX: number) {
	const event = new Event(type, { bubbles: true, cancelable: true });
	Object.defineProperty(event, 'clientX', { value: clientX });
	return event;
}

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

function sessionNode(): CanvasNode {
	return { id: 'session:session-1', kind: 'session', state: 'resolved', entityRef: { kind: 'session', workspaceId: 'workspace-1', sessionId: 'session-1', branchKey: 'main' }, position: { x: 0, y: 0 }, collapsed: false, revision: 1, session: { record: sessionRecord() } };
}

function workspaceNode(): CanvasNode {
	return { id: 'workspace:workspace-1', kind: 'workspace', state: 'resolved', entityRef: { kind: 'workspace', workspaceId: 'workspace-1' }, position: { x: 0, y: 0 }, collapsed: false, revision: 1, workspace: { id: 'workspace-1', name: 'Workspace', branch: 'main', commit: '222222223333', git: { workspaceId: 'workspace-1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }, providerAxes: { topology: 'local_application', contentProvider: 'local_checkout', executionProvider: 'local_process' }, actions: { 'verification.run': { action: 'verification.run', state: 'unsupported', message: 'No runtime', recoveryActions: [] } }, verification: { id: 'verify-1', workspaceId: 'workspace-1', profile: 'smoke', status: 'passed', exitCode: 0, steps: [], artifacts: [], freshness: 'stale', finishFingerprint: { value: 'one', branch: 'main', commit: '111111112222' }, currentFingerprint: { value: 'two', branch: 'main', commit: '222222223333' } } } };
}

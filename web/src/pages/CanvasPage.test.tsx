import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { CanvasProjection, WorkspaceConfig } from '../lib/types';
import { CanvasPage } from './CanvasPage';

const canvasState = vi.hoisted(() => ({ projection: undefined as CanvasProjection | undefined, loading: false, removeStalePlans: vi.fn(), resetPositions: vi.fn(), moveNode: vi.fn() }));
vi.mock('../features/canvas/useCanvasState', () => ({ useCanvasState: () => ({ projection: canvasState.projection, loading: canvasState.loading, error: '', conflicts: [], dirtyCount: 0, saveStatus: 'saved', hasUnsavedChanges: false, moveNode: canvasState.moveNode, setNodeCollapsed: vi.fn(), saveViewport: vi.fn(), reloadPosition: vi.fn(), reapplyPosition: vi.fn(), placeSession: vi.fn(), removeNode: vi.fn(), removeStalePlans: canvasState.removeStalePlans, resetPositions: canvasState.resetPositions, reload: vi.fn(), refresh: vi.fn() }) }));
vi.mock('../features/canvas/CanvasBoard', () => ({ CanvasBoard: ({ projection, onSelect, onReset }: { projection: CanvasProjection; onSelect: (id?: string) => void; onReset: () => void }) => <><button type="button" onClick={onReset}>Reset board</button>{projection.nodes.map((node) => <button key={node.id} data-canvas-node-id={node.id} aria-label={node.session ? `Session: ${node.session.record.provider}` : 'Plan: PM-037 Canvas main'} type="button" onClick={() => onSelect(node.id)}>{node.session ? 'Session node' : 'Plan node'}</button>)}</> }));
vi.mock('../features/canvas/CanvasSessionsView', () => ({ CanvasSessionsView: ({ onOpenOnBoard }: { onOpenOnBoard: (sessionId: string) => void }) => <section aria-label="Sessions view"><button type="button" onClick={() => onOpenOnBoard('session-1')}>Open session on Canvas</button></section> }));
vi.mock('../features/workstream-explorer/useWorkspaceBranches', () => ({ useWorkspaceBranches: () => ({ states: { 'workspace-1': { current: 'main', branches: ['main'] } } }) }));
vi.mock('../features/canvas/CanvasWorkbench', () => ({ CanvasWorkbench: ({ selectedNode, onClose }: { selectedNode?: unknown; onClose: () => void }) => selectedNode ? <aside aria-label="Canvas Workbench"><button type="button" onClick={onClose}>Close Workbench</button></aside> : null }));

describe('CanvasPage', () => {
	it('continues rendering when browser preference storage is unavailable', async () => {
		const brokenStorage = { getItem: () => { throw new Error('blocked'); }, setItem: () => { throw new Error('blocked'); }, removeItem: () => { throw new Error('blocked'); } };
		vi.stubGlobal('localStorage', brokenStorage);
		canvasState.projection = projection();
		render(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id, view: 'canvas' }} onLocationChange={vi.fn()} />);
		expect(await screen.findByLabelText('Workspace Canvas')).toBeInTheDocument();
		vi.unstubAllGlobals();
	});

	it('returns keyboard focus to the selected node after closing the Workbench', async () => {
		canvasState.projection = projection();
		render(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id, view: 'canvas' }} onLocationChange={vi.fn()} />);
		const node = screen.getByRole('button', { name: 'Plan: PM-037 Canvas main' });
		fireEvent.click(node);
		fireEvent.click(screen.getByRole('button', { name: 'Close Workbench' }));
		await waitFor(() => expect(node).toHaveFocus());
	});

	it('closes the Workbench on Escape, but not while typing in a field', async () => {
		canvasState.projection = projection();
		render(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id, view: 'canvas' }} onLocationChange={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Plan: PM-037 Canvas main' }));
		expect(screen.getByLabelText('Canvas Workbench')).toBeInTheDocument();
		const input = document.createElement('input');
		document.body.appendChild(input);
		fireEvent.keyDown(input, { key: 'Escape' });
		expect(screen.getByLabelText('Canvas Workbench')).toBeInTheDocument();
		input.remove();
		fireEvent.keyDown(window, { key: 'Escape' });
		await waitFor(() => expect(screen.queryByLabelText('Canvas Workbench')).not.toBeInTheDocument());
	});

	it('shows checkout context and exposes a polite save-status region', async () => {
		canvasState.projection = projection();
		render(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id, view: 'canvas' }} onLocationChange={vi.fn()} />);
		await waitFor(() => expect(screen.getByLabelText('Checkout: main')).toBeInTheDocument());
		expect(screen.queryByTestId('canvas-ready')).not.toBeInTheDocument();
		expect(screen.getByRole('status')).toHaveTextContent('Saved');
	});

	it('confirms stale plan cleanup before resetting the remaining layout', async () => {
		canvasState.removeStalePlans.mockClear();
		canvasState.resetPositions.mockClear();
		const stale = { ...projection().nodes[0], state: 'stale' as const, plan: undefined };
		canvasState.projection = { ...projection(), nodes: [stale] };
		canvasState.removeStalePlans.mockResolvedValueOnce({ ...projection(), nodes: [] });
		render(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id, view: 'canvas' }} onLocationChange={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Reset board' }));
		expect(screen.getByRole('dialog')).toHaveTextContent('Remove 1 stale plan placement');
		fireEvent.click(screen.getByRole('button', { name: 'Reset layout' }));
		await waitFor(() => expect(canvasState.removeStalePlans).toHaveBeenCalledTimes(1));
		await waitFor(() => expect(canvasState.resetPositions).toHaveBeenCalledTimes(1));
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('shows upstream drift on the Git icon inside the checkout picker', async () => {
		canvasState.projection = upstreamProjection();
		render(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id, view: 'canvas' }} onLocationChange={vi.fn()} />);
		expect(await screen.findByRole('img', { name: 'Git not in sync with origin/main: 4 behind' })).toHaveClass('is-not-synced');
		expect(screen.queryByText('origin/main')).not.toBeInTheDocument();
	});

	it('opens the Workbench for a selected terminal session, alongside the terminal kept in the Canvas node', async () => {
		canvasState.projection = sessionProjection();
		render(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id, view: 'canvas' }} onLocationChange={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Session: codex' }));
		await waitFor(() => expect(screen.getByLabelText('Canvas Workbench')).toBeInTheDocument());
	});

	it('shows Sessions while the canvas is still loading, and says so only on the Canvas view', () => {
		canvasState.projection = undefined;
		canvasState.loading = true;
		try {
			const view = render(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id }} onLocationChange={vi.fn()} />);
			expect(screen.getByLabelText('Sessions view')).toBeInTheDocument();
			expect(screen.queryByText('Loading Canvas…')).not.toBeInTheDocument();
			view.rerender(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id, view: 'canvas' }} onLocationChange={vi.fn()} />);
			expect(screen.getByText('Loading Canvas…')).toBeInTheDocument();
		} finally {
			canvasState.loading = false;
		}
	});

	it('opens on Sessions, switches to Canvas and back through the route', async () => {
		canvasState.projection = sessionProjection();
		const onLocationChange = vi.fn();
		const view = render(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id }} onLocationChange={onLocationChange} />);
		expect(screen.getByLabelText('Sessions view')).toBeInTheDocument();
		// The switcher sits in the title row, Sessions first, as on every page with views.
		const switcher = screen.getByRole('group', { name: 'Agent Space view' });
		expect(switcher.closest('.page-title')).not.toBeNull();
		expect(within(switcher).getAllByRole('button').map((button) => button.textContent?.trim())).toEqual(['Sessions (1)', 'Canvas']);
		fireEvent.click(within(switcher).getByRole('button', { name: 'Canvas' }));
		await waitFor(() => expect(onLocationChange).toHaveBeenCalledWith({ workspaceId: workspace.id, view: 'canvas' }));
		view.rerender(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id, view: 'canvas' }} onLocationChange={onLocationChange} />);
		fireEvent.click(within(screen.getByRole('group', { name: 'Agent Space view' })).getByRole('button', { name: /Sessions/ }));
		await waitFor(() => expect(onLocationChange).toHaveBeenLastCalledWith({ workspaceId: workspace.id, view: undefined }));
	});

	// A session link hands off a node id through the route so Open can select
	// the session there; it must apply once and then clear so a later refresh or
	// checkout switch never re-selects it.
	it('honours a selectedNodeId handed off through the route, once, then clears it', async () => {
		canvasState.projection = sessionProjection();
		const onLocationChange = vi.fn();
		render(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id, selectedNodeId: 'session:session-1' }} onLocationChange={onLocationChange} />);
		await waitFor(() => expect(onLocationChange).toHaveBeenCalledWith({ workspaceId: workspace.id, view: 'canvas', selectedNodeId: undefined }));
	});
});

const workspace: WorkspaceConfig = { id: 'workspace-1', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '' };
function projection(): CanvasProjection {
	return { layout: { id: 'layout', workspaceId: workspace.id, branchKey: 'main', viewport: { x: 0, y: 0, zoom: 1 }, version: 1, createdAt: '', updatedAt: '' }, nodes: [{ id: 'plan:item-1', kind: 'plan', state: 'resolved', entityRef: { kind: 'plan', workspaceId: workspace.id, itemId: 'item-1', itemPath: 'plans/platform/PM-037', identifier: 'PM-037', branchKey: 'main' }, position: { x: 0, y: 0 }, collapsed: false, revision: 1, plan: { itemId: 'item-1', identifier: 'PM-037', title: 'Canvas', service: 'platform', status: 'in_progress', branch: 'main', editable: true, actions: {} } }], connections: [], unplaced: [] };
}

function sessionProjection(): CanvasProjection {
	return { ...projection(), nodes: [{ id: 'session:session-1', kind: 'session', state: 'resolved', entityRef: { kind: 'session', workspaceId: workspace.id, sessionId: 'session-1', branchKey: 'main' }, position: { x: 0, y: 0 }, collapsed: false, revision: 1, session: { record: { id: 'session-1', workspaceId: workspace.id, provider: 'codex', intent: 'card_context', requestedBranch: 'main', state: 'running', startedAt: '', lastKnownAt: '', live: true } } }] };
}

function upstreamProjection(): CanvasProjection {
	const base = projection();
	base.nodes = [...base.nodes, { id: `workspace:${workspace.id}`, kind: 'workspace', state: 'resolved', entityRef: { kind: 'workspace', workspaceId: workspace.id }, position: { x: 0, y: 0 }, collapsed: false, revision: 1, workspace: { id: workspace.id, name: workspace.name, branch: 'main', providerAxes: { topology: 'local_application', contentProvider: 'local_checkout', executionProvider: 'local_process' }, actions: {}, git: { workspaceId: workspace.id, branch: 'main', upstream: 'origin/main', ahead: 0, behind: 4, fetchedAt: new Date().toISOString(), dirty: false, conflicted: false, changes: [] } } }];
	return base;
}

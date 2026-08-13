import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { CanvasProjection, WorkspaceConfig } from '../lib/types';
import { CanvasPage } from './CanvasPage';

const canvasState = vi.hoisted(() => ({ projection: undefined as CanvasProjection | undefined }));
vi.mock('../features/canvas/useCanvasState', () => ({ useCanvasState: () => ({ projection: canvasState.projection, loading: false, error: '', conflicts: [], dirtyCount: 0, saveStatus: 'saved', hasUnsavedChanges: false, moveNode: vi.fn(), setNodeCollapsed: vi.fn(), saveViewport: vi.fn(), reloadPosition: vi.fn(), reapplyPosition: vi.fn(), placeSession: vi.fn(), removeNode: vi.fn(), resetPositions: vi.fn(), reload: vi.fn(), refresh: vi.fn() }) }));
vi.mock('../features/canvas/CanvasBoard', () => ({ CanvasBoard: ({ projection, onSelect }: { projection: CanvasProjection; onSelect: (id?: string) => void }) => <>{projection.nodes.map((node) => <button key={node.id} data-canvas-node-id={node.id} aria-label={node.session ? `Session: ${node.session.record.provider}` : 'Plan: PM-037 Canvas main'} type="button" onClick={() => onSelect(node.id)}>{node.session ? 'Session node' : 'Plan node'}</button>)}</> }));
vi.mock('../features/canvas/CanvasWorkbench', () => ({ CanvasWorkbench: ({ selectedNode, onClose }: { selectedNode?: unknown; onClose: () => void }) => selectedNode ? <aside aria-label="Canvas Workbench"><button type="button" onClick={onClose}>Close Workbench</button></aside> : null }));

describe('CanvasPage', () => {
	it('returns keyboard focus to the selected node after closing the Workbench', async () => {
		canvasState.projection = projection();
		render(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id }} onLocationChange={vi.fn()} />);
		const node = screen.getByRole('button', { name: 'Plan: PM-037 Canvas main' });
		fireEvent.click(node);
		fireEvent.click(screen.getByRole('button', { name: 'Close Workbench' }));
		await waitFor(() => expect(node).toHaveFocus());
	});

	it('shows checkout context and exposes a polite save-status region', async () => {
		canvasState.projection = projection();
		render(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id }} onLocationChange={vi.fn()} />);
		await waitFor(() => expect(screen.getByLabelText('Checkout: main')).toBeInTheDocument());
		expect(screen.queryByTestId('canvas-ready')).not.toBeInTheDocument();
		expect(screen.getByRole('status')).toHaveTextContent('Saved');
	});

	it('keeps a selected terminal session in the Canvas instead of opening the Workbench', async () => {
		canvasState.projection = sessionProjection();
		render(<CanvasPage workspace={workspace} location={{ workspaceId: workspace.id }} onLocationChange={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Session: codex' }));
		await waitFor(() => expect(screen.queryByLabelText('Canvas Workbench')).not.toBeInTheDocument());
	});
});

const workspace: WorkspaceConfig = { id: 'workspace-1', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '' };
function projection(): CanvasProjection {
	return { layout: { id: 'layout', workspaceId: workspace.id, branchKey: 'main', viewport: { x: 0, y: 0, zoom: 1 }, version: 1, createdAt: '', updatedAt: '' }, nodes: [{ id: 'plan:item-1', kind: 'plan', state: 'resolved', entityRef: { kind: 'plan', workspaceId: workspace.id, itemId: 'item-1', itemPath: 'plans/platform/PM-037', identifier: 'PM-037', branchKey: 'main' }, position: { x: 0, y: 0 }, collapsed: false, revision: 1, plan: { itemId: 'item-1', identifier: 'PM-037', title: 'Canvas', service: 'platform', status: 'in_progress', branch: 'main', editable: true, actions: {} } }], connections: [], unplaced: [] };
}

function sessionProjection(): CanvasProjection {
	return { ...projection(), nodes: [{ id: 'session:session-1', kind: 'session', state: 'resolved', entityRef: { kind: 'session', workspaceId: workspace.id, sessionId: 'session-1', branchKey: 'main' }, position: { x: 0, y: 0 }, collapsed: false, revision: 1, session: { record: { id: 'session-1', workspaceId: workspace.id, provider: 'codex', intent: 'card_context', requestedBranch: 'main', state: 'running', startedAt: '', lastKnownAt: '', live: true } } }] };
}

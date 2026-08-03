import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { CanvasProjection, WorkspaceConfig } from '../lib/types';
import { CanvasPage } from './CanvasPage';

const canvasState = vi.hoisted(() => ({ projection: undefined as CanvasProjection | undefined }));
vi.mock('../lib/api', () => ({ api: { workspaceBranches: vi.fn().mockResolvedValue({ current: 'main', branches: ['main', 'feature'] }) } }));
vi.mock('../features/canvas/useCanvasState', () => ({ useCanvasState: () => ({ projection: canvasState.projection, loading: false, error: '', conflicts: [], dirtyCount: 0, saveStatus: 'saved', hasUnsavedChanges: false, moveNode: vi.fn(), saveViewport: vi.fn(), reloadPosition: vi.fn(), reapplyPosition: vi.fn(), placeUnplaced: vi.fn(), removeNode: vi.fn(), resetPositions: vi.fn(), reload: vi.fn(), refresh: vi.fn() }) }));
vi.mock('../features/canvas/CanvasBoard', () => ({ CanvasBoard: ({ onSelect }: { onSelect: (id?: string) => void }) => <button data-canvas-node-id="plan:item-1" aria-label="Plan: PM-037 Canvas main" type="button" onClick={() => onSelect('plan:item-1')}>Plan node</button> }));
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
		expect(screen.getByTestId('canvas-ready')).toHaveTextContent('1 placed');
		expect(screen.getByRole('status')).toHaveTextContent('Saved');
	});
});

const workspace: WorkspaceConfig = { id: 'workspace-1', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '' };
function projection(): CanvasProjection {
	return { layout: { id: 'layout', workspaceId: workspace.id, branchKey: 'main', viewport: { x: 0, y: 0, zoom: 1 }, version: 1, createdAt: '', updatedAt: '' }, nodes: [{ id: 'plan:item-1', kind: 'plan', state: 'resolved', entityRef: { kind: 'plan', workspaceId: workspace.id, itemId: 'item-1', itemPath: 'plans/item-1', identifier: 'PM-037', branchKey: 'main' }, position: { x: 0, y: 0 }, collapsed: false, revision: 1, plan: { itemId: 'item-1', identifier: 'PM-037', title: 'Canvas', branch: 'main', editable: true, actions: {} } }], connections: [], unplaced: [] };
}

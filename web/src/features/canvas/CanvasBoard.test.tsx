import { fireEvent, render, screen } from '@testing-library/react';
import type { ComponentType, ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import type { CanvasNode, CanvasProjection } from '../../lib/types';
import { CanvasBoard } from './CanvasBoard';

const fitView = vi.fn();
vi.mock('@xyflow/react', async () => {
	const React = await import('react');
	const FlowContext = React.createContext(false);
	return {
		ReactFlowProvider: ({ children }: { children: ReactNode }) => <FlowContext.Provider value>{children}</FlowContext.Provider>,
		ReactFlow: ({ nodes, edges, nodeTypes, onNodeDragStop, onNodeClick, onlyRenderVisibleElements, nodesDraggable, children }: { nodes: Array<{ id: string; type: string; data: Record<string, unknown>; position: { x: number; y: number }; draggable?: boolean }>; edges: Array<{ id: string }>; nodeTypes: Record<string, ComponentType<{ data: Record<string, unknown> }>>; onNodeDragStop: (event: unknown, node: unknown) => void; onNodeClick: (event: unknown, node: { id: string }) => void; onlyRenderVisibleElements: boolean; nodesDraggable: boolean; children: ReactNode }) => <div data-testid="react-flow" data-nodes={nodes.length} data-edges={edges.length} data-visible-only={String(onlyRenderVisibleElements)} data-draggable={String(nodesDraggable)}>{nodes.map((node) => { const Component = nodeTypes[node.type]; return <div key={node.id} data-draggable-node={String(node.draggable)}><Component data={node.data} /><button type="button" onClick={() => onNodeDragStop({}, { ...node, position: { x: 999, y: 888 } })}>drag {node.id}</button><button type="button" onClick={() => onNodeClick({}, node)}>select {node.id}</button></div>; })}{children}</div>,
		Background: () => null,
		Controls: () => <div data-testid="flow-controls" />,
		Handle: () => null,
		Position: { Left: 'left', Right: 'right' },
		useNodesState: <T,>(initial: T[]) => [initial, vi.fn(), vi.fn()],
		useReactFlow: () => {
			if (!React.useContext(FlowContext)) throw new Error('ReactFlowProvider is required');
			return { fitView, getNode: (id: string) => ({ id }) };
		}
	};
});

describe('CanvasBoard', () => {
	it('provides React Flow context to toolbar search', () => {
		expect(() => renderBoard(baseProjection())).not.toThrow();
		expect(screen.getByLabelText('Search Canvas nodes')).toBeInTheDocument();
	});

	it('renders all semantic nodes as independent draggable placements with derived edges', () => {
		const onMoveNode = vi.fn();
		renderBoard(baseProjection(), { onMoveNode });
		expect(screen.getByLabelText('Workspace: Workspace main')).toBeInTheDocument();
		expect(screen.getByLabelText('Plan: PM-037 Focused Canvas main')).toBeInTheDocument();
		expect(screen.getByLabelText('Session: codex main running')).toBeInTheDocument();
		expect(screen.getByTestId('react-flow')).toHaveAttribute('data-edges', '2');
		expect(screen.getByTestId('react-flow')).toHaveAttribute('data-visible-only', 'true');
		expect(document.querySelectorAll('[data-draggable-node="true"]')).toHaveLength(3);
		expect(screen.getAllByText(/^drag /)).toHaveLength(3);
		fireEvent.click(screen.getByText('drag workspace:workspace-1'));
		expect(onMoveNode).toHaveBeenCalledWith('workspace:workspace-1', { x: 999, y: 888 });
		expect(onMoveNode).toHaveBeenCalledTimes(1);
	});

	it('searches and focuses by identifier, exposes unplaced work, reset, and isolated removal', () => {
		const onSelect = vi.fn();
		const onPlaceUnplaced = vi.fn();
		const onReset = vi.fn();
		const onRemove = vi.fn();
		renderBoard({ ...baseProjection(), unplaced: [{ kind: 'plan', workspaceId: 'workspace-1', itemId: 'new', itemPath: 'plans/new', identifier: 'PM-038', branchKey: 'main' }] }, { selectedId: 'plan:item-1', onSelect, onPlaceUnplaced, onReset, onRemove });
		fireEvent.change(screen.getByLabelText('Search Canvas nodes'), { target: { value: 'PM-037' } });
		fireEvent.click(screen.getByRole('option', { name: /PM-037/ }));
		expect(onSelect).toHaveBeenCalledWith('plan:item-1');
		expect(fitView).toHaveBeenCalled();
		fireEvent.click(screen.getByRole('button', { name: /Place new items/ }));
		fireEvent.click(screen.getByRole('button', { name: /Reset layout/ }));
		fireEvent.click(screen.getByRole('button', { name: /Remove from Canvas/ }));
		expect(onPlaceUnplaced).toHaveBeenCalled();
		expect(onReset).toHaveBeenCalled();
		expect(onRemove).toHaveBeenCalledWith(expect.objectContaining({ id: 'plan:item-1' }));
	});

	it('filters plans without removing other node kinds and can persist a grouped arrangement', () => {
		const onArrange = vi.fn();
		const projection = baseProjection();
		const plan = projection.nodes[1];
		projection.nodes.push({ ...plan, id: 'plan:item-2', entityRef: { ...plan.entityRef, itemId: 'item-2' }, plan: { ...plan.plan!, itemId: 'item-2', identifier: 'PM-038', status: 'draft' } });
		renderBoard(projection, { onArrange });
		fireEvent.click(screen.getByRole('button', { name: 'Status' }));
		fireEvent.click(screen.getByLabelText('Draft'));
		expect(screen.queryByLabelText('Plan: PM-037 Focused Canvas main')).not.toBeInTheDocument();
		expect(screen.getByLabelText('Workspace: Workspace main')).toBeInTheDocument();
		expect(screen.getByLabelText('Session: codex main running')).toBeInTheDocument();
		expect(screen.getByText(/1 hidden/)).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Group' }));
		fireEvent.click(screen.getByRole('button', { name: 'Service + status' }));
		expect(onArrange).toHaveBeenCalledWith('service_status');
	});

	it('moves nodes by keyboard and disables every layout action from capabilities', () => {
		const onMoveNode = vi.fn();
		const projection = baseProjection();
		const first = renderBoard(projection, { onMoveNode });
		fireEvent.keyDown(screen.getByLabelText('Plan: PM-037 Focused Canvas main'), { key: 'ArrowRight' });
		expect(onMoveNode).toHaveBeenCalledWith('plan:item-1', { x: 372, y: 0 });
		first.unmount();

		const unavailable = baseProjection();
		unavailable.nodes = unavailable.nodes.map((node) => node.workspace ? { ...node, workspace: { ...node.workspace, actions: { 'layout.move': { action: 'layout.move', state: 'forbidden', message: 'No layout access', recoveryActions: [] } } } } : node.plan ? { ...node, plan: { ...node.plan, actions: { 'layout.move': { action: 'layout.move', state: 'forbidden', message: 'No layout access', recoveryActions: [] } } } } : node);
		const second = renderBoard(unavailable, { selectedId: 'plan:item-1' });
		expect(second.getByRole('button', { name: /Reset layout/ })).toBeDisabled();
		expect(second.getByRole('button', { name: /Remove from Canvas/ })).toBeDisabled();
	});

	it('shows Git HEAD and verification result separately from stale freshness', () => {
		const projection = baseProjection();
		projection.nodes[0] = { ...projection.nodes[0], workspace: { ...projection.nodes[0].workspace!, commit: 'abcdef123456', verification: { id: 'verify-1', workspaceId: 'workspace-1', profile: 'smoke', status: 'passed', exitCode: 0, steps: [], artifacts: [], freshness: 'stale' } } };
		renderBoard(projection);
		expect(screen.getByText('HEAD abcdef12')).toBeInTheDocument();
		expect(screen.getByText('passed')).toBeInTheDocument();
		expect(screen.getByText('Freshness: stale')).toBeInTheDocument();
	});

	it.each([25, 100, 300])('keeps %i placements virtualized', (count) => {
		const template = baseProjection().nodes[1];
		const nodes = Array.from({ length: count }, (_, index): CanvasNode => ({ ...template, id: `plan:${index}`, entityRef: { ...template.entityRef, itemId: String(index) }, position: { x: index * 10, y: index * 5 }, plan: { ...template.plan!, itemId: String(index), identifier: `PM-${index}` } }));
		renderBoard({ ...baseProjection(), nodes, connections: [] });
		expect(screen.getByTestId('react-flow')).toHaveAttribute('data-nodes', String(count));
		expect(screen.getByTestId('react-flow')).toHaveAttribute('data-visible-only', 'true');
	});
});

function renderBoard(projection: CanvasProjection, overrides: Partial<React.ComponentProps<typeof CanvasBoard>> = {}) {
	return render(<CanvasBoard projection={projection} conflicts={[]} onSelect={vi.fn()} onMoveNode={vi.fn()} onArrange={vi.fn()} onSaveViewport={vi.fn()} onReloadPosition={vi.fn()} onReapplyPosition={vi.fn()} onPlaceUnplaced={vi.fn()} onReset={vi.fn()} onRemove={vi.fn()} {...overrides} />);
}

function baseProjection(): CanvasProjection {
	const actions = { 'layout.move': { action: 'layout.move' as const, state: 'available' as const, recoveryActions: [] } };
	return {
		layout: { id: 'layout-1', workspaceId: 'workspace-1', branchKey: 'main', viewport: { x: 0, y: 0, zoom: 1 }, version: 1, createdAt: '', updatedAt: '' },
		nodes: [
			{ id: 'workspace:workspace-1', kind: 'workspace', state: 'resolved', entityRef: { kind: 'workspace', workspaceId: 'workspace-1' }, position: { x: 0, y: 0 }, collapsed: false, revision: 1, workspace: { id: 'workspace-1', name: 'Workspace', branch: 'main', providerAxes: { topology: 'local_application', contentProvider: 'local_checkout', executionProvider: 'local_process' }, actions } },
			{ id: 'plan:item-1', kind: 'plan', state: 'resolved', entityRef: { kind: 'plan', workspaceId: 'workspace-1', itemId: 'item-1', itemPath: 'plans/platform/PM-037', identifier: 'PM-037', branchKey: 'main' }, position: { x: 360, y: 0 }, collapsed: false, revision: 1, plan: { itemId: 'item-1', identifier: 'PM-037', title: 'Focused Canvas', service: 'platform', status: 'in_progress', branch: 'main', editable: true, actions } },
			{ id: 'session:session-1', kind: 'session', state: 'resolved', entityRef: { kind: 'session', workspaceId: 'workspace-1', sessionId: 'session-1', branchKey: 'main' }, position: { x: 680, y: 420 }, collapsed: false, revision: 1, session: { record: { id: 'session-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', requestedBranch: 'main', state: 'running', startedAt: '', lastKnownAt: '', live: true } } }
		],
		connections: [{ id: 'one', source: 'workspace:workspace-1', target: 'plan:item-1', kind: 'repository_contains', sourceOfTruth: 'derived' }, { id: 'two', source: 'plan:item-1', target: 'session:session-1', kind: 'session_launched_from', sourceOfTruth: 'derived' }],
		unplaced: []
	};
}

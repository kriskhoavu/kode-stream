import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ComponentType, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import type { CanvasNode, CanvasProjection, EmbeddedAISessionResult } from '../../lib/types';
import { CanvasBoard } from './CanvasBoard';

const fitView = vi.fn();
vi.mock('../../shared/api', async () => {
	const actual = await vi.importActual<typeof import('../../shared/api')>('../../shared/api');
	return { ...actual, api: { ...actual.api, embeddedAISession: vi.fn(), embeddedAISessionGrant: vi.fn(), cancelEmbeddedAISession: vi.fn() } };
});
vi.mock('../ai-session/EmbeddedTerminal', () => ({ EmbeddedTerminal: ({ initial, visible, onClose }: { initial: EmbeddedAISessionResult; visible: boolean; onClose: () => void }) => <section data-testid={`canvas-terminal-${initial.session.id}`} data-visible={String(visible)}><button type="button" onClick={onClose}>Hide terminal</button><input aria-label="Terminal prompt" /></section> }));
vi.mock('@xyflow/react', async () => {
	const React = await import('react');
	const FlowContext = React.createContext(false);
	return {
		ReactFlowProvider: ({ children }: { children: ReactNode }) => <FlowContext.Provider value>{children}</FlowContext.Provider>,
		ReactFlow: ({ nodes, edges, nodeTypes, onNodeDragStop, onNodeClick, onlyRenderVisibleElements, nodesDraggable, children }: { nodes: Array<{ id: string; type: string; data: Record<string, unknown>; position: { x: number; y: number }; draggable?: boolean }>; edges: Array<{ id: string }>; nodeTypes: Record<string, ComponentType<{ data: Record<string, unknown> }>>; onNodeDragStop: (event: unknown, node: unknown) => void; onNodeClick: (event: unknown, node: { id: string }) => void; onlyRenderVisibleElements: boolean; nodesDraggable: boolean; children: ReactNode }) => <div data-testid="react-flow" data-nodes={nodes.length} data-edges={edges.length} data-visible-only={String(onlyRenderVisibleElements)} data-draggable={String(nodesDraggable)}>{nodes.map((node) => { const Component = nodeTypes[node.type]; return <div key={node.id} data-draggable-node={String(node.draggable)}><Component data={node.data} /><button type="button" onClick={() => onNodeDragStop({}, { ...node, position: { x: 999, y: 888 } })}>drag {node.id}</button><button type="button" onClick={() => onNodeClick({}, node)}>select {node.id}</button></div>; })}{children}</div>,
		Background: () => null,
		Controls: () => <div data-testid="flow-controls" />,
		Handle: () => null,
		NodeResizer: ({ handleClassName }: { handleClassName?: string }) => <div data-testid="node-resizer" className={handleClassName} />,
		Position: { Left: 'left', Right: 'right' },
		useNodesState: <T,>(initial: T[]) => [initial, vi.fn(), vi.fn()],
		useReactFlow: () => {
			if (!React.useContext(FlowContext)) throw new Error('ReactFlowProvider is required');
			return { fitView, getNode: (id: string) => ({ id }) };
		}
	};
});

describe('CanvasBoard', () => {
	beforeEach(() => vi.clearAllMocks());

	it('uses an explicit disclosure action and keeps expansion independent from selection', async () => {
		vi.mocked(api.embeddedAISession).mockResolvedValue({ id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'running', startedAt: '' });
		vi.mocked(api.embeddedAISessionGrant).mockResolvedValue({ sessionId: 'session-1', token: 'grant', expiresAt: '' });
		const onSelect = vi.fn();
		const onSetCollapsed = vi.fn();
		const view = renderBoard(baseProjection(), { selectedId: 'session:session-1', onSelect, onSetCollapsed });
		expect(screen.queryByTestId('canvas-terminal-session-1')).not.toBeInTheDocument();
		expect(api.embeddedAISession).not.toHaveBeenCalled();
		fireEvent.click(screen.getByRole('button', { name: 'Expand session terminal' }));
		expect(onSetCollapsed).toHaveBeenCalledWith('session:session-1', false);
		const expanded = baseProjection();
		expanded.nodes[2] = { ...expanded.nodes[2], collapsed: false };
		view.rerender(<CanvasBoard projection={expanded} conflicts={[]} selectedId="session:session-1" onSelect={onSelect} onMoveNode={vi.fn()} onSetCollapsed={onSetCollapsed} onArrange={vi.fn()} onSaveViewport={vi.fn()} onReload={vi.fn()} onReloadPosition={vi.fn()} onReapplyPosition={vi.fn()} onReset={vi.fn()} onRemove={vi.fn()} />);
		expect(await screen.findByTestId('canvas-terminal-session-1')).toHaveAttribute('data-visible', 'true');
		expect(screen.getByTestId('node-resizer')).toHaveClass('canvas-session-resize-handle');
		fireEvent.change(screen.getByLabelText('Terminal prompt'), { target: { value: 'review this change' } });
		view.rerender(<CanvasBoard projection={expanded} conflicts={[]} onSelect={onSelect} onMoveNode={vi.fn()} onSetCollapsed={onSetCollapsed} onArrange={vi.fn()} onSaveViewport={vi.fn()} onReload={vi.fn()} onReloadPosition={vi.fn()} onReapplyPosition={vi.fn()} onReset={vi.fn()} onRemove={vi.fn()} />);
		expect(screen.getByTestId('canvas-terminal-session-1')).toHaveAttribute('data-visible', 'true');
		expect(api.embeddedAISessionGrant).toHaveBeenCalledTimes(1);
		fireEvent.click(screen.getByRole('button', { name: 'Hide terminal' }));
		expect(onSetCollapsed).toHaveBeenLastCalledWith('session:session-1', true);
	});

	it('shows interrupted lifecycle details and cancels a live process separately from Canvas placement', async () => {
		const interrupted = baseProjection();
		interrupted.nodes[2] = { ...interrupted.nodes[2], collapsed: false, session: { record: { ...interrupted.nodes[2].session!.record, state: 'interrupted', live: false, exitCode: 130 } } };
		const first = renderBoard(interrupted, { selectedId: 'session:session-1' });
		expect(screen.getByText(/application restarted without this process/i)).toBeInTheDocument();
		expect(screen.getByText('Exit code 130')).toBeInTheDocument();
		first.unmount();

		vi.spyOn(window, 'confirm').mockReturnValue(true);
		vi.mocked(api.embeddedAISession).mockResolvedValue({ id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'running', startedAt: '' });
		vi.mocked(api.embeddedAISessionGrant).mockResolvedValue({ sessionId: 'session-1', token: 'grant', expiresAt: '' });
		vi.mocked(api.cancelEmbeddedAISession).mockResolvedValue({ id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'cancelled', startedAt: '' });
		const onReload = vi.fn();
		const onRemove = vi.fn();
		const expanded = baseProjection();
		expanded.nodes[2] = { ...expanded.nodes[2], collapsed: false };
		renderBoard(expanded, { selectedId: 'session:session-1', onReload, onRemove });
		fireEvent.click(screen.getByRole('button', { name: 'Cancel process' }));
		await waitFor(() => expect(api.cancelEmbeddedAISession).toHaveBeenCalledWith('session-1'));
		expect(onReload).toHaveBeenCalled();
		expect(onRemove).not.toHaveBeenCalled();
	});

	it('provides React Flow context to toolbar search', () => {
		expect(() => renderBoard(baseProjection())).not.toThrow();
		expect(screen.getByLabelText('Search Canvas nodes')).toBeInTheDocument();
	});

	it('renders plans and terminal sessions without a separate workspace card', () => {
		const onMoveNode = vi.fn();
		renderBoard(baseProjection(), { onMoveNode });
		expect(screen.queryByLabelText('Workspace: Workspace main')).not.toBeInTheDocument();
		expect(screen.getByLabelText('Plan: PM-037 Focused Canvas main')).toBeInTheDocument();
		expect(screen.getByLabelText('Session: codex main running')).toBeInTheDocument();
		expect(screen.getByTestId('react-flow')).toHaveAttribute('data-edges', '1');
		expect(screen.getByTestId('react-flow')).toHaveAttribute('data-visible-only', 'true');
		expect(document.querySelectorAll('[data-draggable-node="true"]')).toHaveLength(2);
		expect(screen.getAllByText(/^drag /)).toHaveLength(2);
		fireEvent.click(screen.getByText('drag plan:item-1'));
		expect(onMoveNode).toHaveBeenCalledWith('plan:item-1', { x: 999, y: 888 });
		expect(screen.getByLabelText('Node type legend')).toHaveTextContent('Workspace group');
		expect(screen.getByLabelText('Node type legend')).toHaveTextContent('Service group');
		expect(screen.getByLabelText('Node type legend')).toHaveTextContent('Plan');
		expect(screen.getByLabelText('Node type legend')).toHaveTextContent('Terminal session');
	});

	it('filters Canvas nodes by identifier without showing autocomplete results', () => {
		const onSelect = vi.fn();
		const onReset = vi.fn();
		const onRemove = vi.fn();
		renderBoard(baseProjection(), { selectedId: 'plan:item-1', onSelect, onReset, onRemove });
		fireEvent.change(screen.getByLabelText('Search Canvas nodes'), { target: { value: 'PM-037' } });
		expect(screen.getByTestId('react-flow')).toHaveAttribute('data-nodes', '2');
		expect(screen.getByLabelText('Session: codex main running')).toBeInTheDocument();
		expect(screen.queryByRole('option')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /Add new nodes/ })).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: /Reset layout/ }));
		fireEvent.click(screen.getByRole('button', { name: /Remove from Canvas/ }));
		expect(onReset).toHaveBeenCalled();
		expect(onRemove).toHaveBeenCalledWith(expect.objectContaining({ id: 'plan:item-1' }));
	});

	it('filters plans and terminal sessions by node type', () => {
		renderBoard(baseProjection());
		fireEvent.click(screen.getByRole('button', { name: 'Node type' }));
		fireEvent.click(screen.getByLabelText('Plans'));
		expect(screen.getByTestId('react-flow')).toHaveAttribute('data-nodes', '1');
		expect(screen.queryByLabelText('Session: codex main running')).not.toBeInTheDocument();
	});

	it('renders a movable section boundary and lets it be removed independently of nodes', () => {
		const onRemoveSection = vi.fn();
		renderBoard(baseProjection(), { sections: [{ id: 'section-1', title: 'Assistant work', position: { x: 320, y: -30 }, width: 620, height: 360, nodeIds: ['plan:item-1', 'session:session-1'] }], onRemoveSection });
		expect(screen.getByLabelText('Section: Assistant work')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Remove section Assistant work' }));
		expect(onRemoveSection).toHaveBeenCalledWith('section-1');
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
		expect(screen.queryByLabelText('Workspace: Workspace main')).not.toBeInTheDocument();
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

	it('moves sections by keyboard', () => {
		const onMoveSection = vi.fn();
		renderBoard(baseProjection(), { sections: [{ id: 'section-1', title: 'Assistant work', position: { x: 320, y: -30 }, width: 620, height: 360, nodeIds: ['plan:item-1'] }], onMoveSection });
		fireEvent.keyDown(screen.getByRole('button', { name: 'Move section Assistant work' }), { key: 'ArrowRight' });
		expect(onMoveSection).toHaveBeenCalledWith('section-1', { x: 332, y: -30 }, { x: 12, y: 0 });
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
	return render(<CanvasBoard projection={projection} conflicts={[]} onSelect={vi.fn()} onMoveNode={vi.fn()} onSetCollapsed={vi.fn()} onArrange={vi.fn()} onSaveViewport={vi.fn()} onReload={vi.fn()} onReloadPosition={vi.fn()} onReapplyPosition={vi.fn()} onReset={vi.fn()} onRemove={vi.fn()} {...overrides} />);
}

function baseProjection(): CanvasProjection {
	const actions = { 'layout.move': { action: 'layout.move' as const, state: 'available' as const, recoveryActions: [] } };
	return {
		layout: { id: 'layout-1', workspaceId: 'workspace-1', branchKey: 'main', viewport: { x: 0, y: 0, zoom: 1 }, version: 1, createdAt: '', updatedAt: '' },
		nodes: [
			{ id: 'workspace:workspace-1', kind: 'workspace', state: 'resolved', entityRef: { kind: 'workspace', workspaceId: 'workspace-1' }, position: { x: 0, y: 0 }, collapsed: false, revision: 1, workspace: { id: 'workspace-1', name: 'Workspace', branch: 'main', providerAxes: { topology: 'local_application', contentProvider: 'local_checkout', executionProvider: 'local_process' }, actions } },
			{ id: 'plan:item-1', kind: 'plan', state: 'resolved', entityRef: { kind: 'plan', workspaceId: 'workspace-1', itemId: 'item-1', itemPath: 'plans/platform/PM-037', identifier: 'PM-037', branchKey: 'main' }, position: { x: 360, y: 0 }, collapsed: false, revision: 1, plan: { itemId: 'item-1', identifier: 'PM-037', title: 'Focused Canvas', service: 'platform', status: 'in_progress', branch: 'main', editable: true, actions } },
			{ id: 'session:session-1', kind: 'session', state: 'resolved', entityRef: { kind: 'session', workspaceId: 'workspace-1', sessionId: 'session-1', branchKey: 'main' }, position: { x: 680, y: 420 }, collapsed: true, revision: 1, session: { record: { id: 'session-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', requestedBranch: 'main', state: 'running', startedAt: '', lastKnownAt: '', live: true } } }
		],
		connections: [{ id: 'one', source: 'workspace:workspace-1', target: 'plan:item-1', kind: 'repository_contains', sourceOfTruth: 'derived' }, { id: 'two', source: 'plan:item-1', target: 'session:session-1', kind: 'session_launched_from', sourceOfTruth: 'derived' }],
		unplaced: []
	};
}

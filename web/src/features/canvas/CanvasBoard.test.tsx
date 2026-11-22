import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ComponentType, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import type { CanvasNode, CanvasProjection, EmbeddedAISessionResult } from '../../lib/types';
import { CanvasBoard } from './CanvasBoard';
import { SessionTerminal } from '../ai-session/SessionTerminal';

const fitView = vi.fn();
const setViewport = vi.fn();
const getViewport = vi.fn(() => ({ x: 11, y: 22, zoom: 1.5 }));
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
		ReactFlow: ({ nodes, edges, nodeTypes, onNodeDragStop, onNodeClick, onMoveEnd, onlyRenderVisibleElements, nodesDraggable, children }: { nodes: Array<{ id: string; type: string; data: Record<string, unknown>; position: { x: number; y: number }; draggable?: boolean }>; edges: Array<{ id: string }>; nodeTypes: Record<string, ComponentType<{ data: Record<string, unknown> }>>; onNodeDragStop: (event: unknown, node: unknown) => void; onNodeClick: (event: unknown, node: { id: string }) => void; onMoveEnd: (event: unknown, viewport: { x: number; y: number; zoom: number }) => void; onlyRenderVisibleElements: boolean; nodesDraggable: boolean; children: ReactNode }) => <div data-testid="react-flow" data-nodes={nodes.length} data-edges={edges.length} data-visible-only={String(onlyRenderVisibleElements)} data-draggable={String(nodesDraggable)}>{nodes.map((node) => { const Component = nodeTypes[node.type]; return <div key={node.id} data-draggable-node={String(node.draggable)} data-position={JSON.stringify(node.position)}><Component data={node.data} /><button type="button" onClick={() => onNodeDragStop({}, { ...node, position: { x: 999, y: 888 } })}>drag {node.id}</button><button type="button" onClick={() => onNodeClick({}, node)}>select {node.id}</button></div>; })}<button type="button" onClick={() => onMoveEnd({}, { x: 5, y: 5, zoom: 1 })}>move viewport</button>{children}</div>,
		Background: () => null,
		Controls: () => <div data-testid="flow-controls" />,
		Handle: () => null,
		NodeResizer: ({ handleClassName, onResizeEnd }: { handleClassName?: string; onResizeEnd?: (event: unknown, size: { width: number; height: number }) => void }) => <button type="button" data-testid="node-resizer" className={handleClassName} onClick={() => onResizeEnd?.({}, { width: 840, height: 620 })}>Resize terminal</button>,
		Position: { Left: 'left', Right: 'right' },
		useNodesState: <T,>(initial: T[]) => [initial, vi.fn(), vi.fn()],
		useReactFlow: () => {
			if (!React.useContext(FlowContext)) throw new Error('ReactFlowProvider is required');
			return { fitView, getViewport, setViewport, getNode: (id: string) => ({ id }) };
		}
	};
});

describe('CanvasBoard', () => {
	beforeEach(() => { vi.clearAllMocks(); localStorage.clear(); });

	it('ignores a stale terminal attachment after the selected session changes', async () => {
		let resolveFirst: (session: EmbeddedAISessionResult['session']) => void = () => {};
		const firstSession = new Promise<EmbeddedAISessionResult['session']>((resolve) => { resolveFirst = resolve; });
		vi.mocked(api.embeddedAISession).mockImplementation((id) => id === 'session-1'
			? firstSession
			: Promise.resolve({ id, itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'running', startedAt: '' }));
		vi.mocked(api.embeddedAISessionGrant).mockImplementation((id) => Promise.resolve({ sessionId: id, token: `grant-${id}`, expiresAt: '' }));
		const firstRecord = baseProjection().nodes[2].session!.record;
		const secondRecord = { ...firstRecord, id: 'session-2' };
		const view = render(<SessionTerminal record={firstRecord} visible onReload={vi.fn()} />);
		await waitFor(() => expect(api.embeddedAISession).toHaveBeenCalledWith('session-1'));

		view.rerender(<SessionTerminal record={secondRecord} visible onReload={vi.fn()} />);
		expect(await screen.findByTestId('canvas-terminal-session-2')).toBeInTheDocument();
		await act(async () => { resolveFirst({ id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'running', startedAt: '' }); });

		expect(screen.queryByTestId('canvas-terminal-session-1')).not.toBeInTheDocument();
		expect(screen.getByTestId('canvas-terminal-session-2')).toBeInTheDocument();
	});

	it('uses an explicit disclosure action and keeps expansion independent from selection', async () => {
		vi.mocked(api.embeddedAISession).mockResolvedValue({ id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'running', startedAt: '' });
		vi.mocked(api.embeddedAISessionGrant).mockResolvedValue({ sessionId: 'session-1', token: 'grant', expiresAt: '' });
		const onSelect = vi.fn();
		const onSetCollapsed = vi.fn();
		const onResizeNode = vi.fn();
		const view = renderBoard(baseProjection(), { selectedId: 'session:session-1', onSelect, onSetCollapsed, onResizeNode });
		expect(screen.queryByTestId('canvas-terminal-session-1')).not.toBeInTheDocument();
		expect(api.embeddedAISession).not.toHaveBeenCalled();
		fireEvent.click(screen.getByRole('button', { name: 'Expand session terminal' }));
		expect(onSetCollapsed).toHaveBeenCalledWith('session:session-1', false);
		const expanded = baseProjection();
		expanded.nodes[2] = { ...expanded.nodes[2], collapsed: false };
		view.rerender(<CanvasBoard projection={expanded} conflicts={[]} selectedId="session:session-1" onSelect={onSelect} onMoveNode={vi.fn()} onResizeNode={onResizeNode} onSetCollapsed={onSetCollapsed} onArrange={vi.fn()} onSaveViewport={vi.fn()} onReload={vi.fn()} onReloadPosition={vi.fn()} onReapplyPosition={vi.fn()} onReset={vi.fn()} onRemove={vi.fn()} />);
		expect(await screen.findByTestId('canvas-terminal-session-1')).toHaveAttribute('data-visible', 'true');
		expect(screen.getByTestId('node-resizer')).toHaveClass('canvas-session-resize-handle');
		fireEvent.click(screen.getByTestId('node-resizer'));
		expect(onResizeNode).toHaveBeenCalledWith('session:session-1', { width: 840, height: 620 });
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

	it('keeps section and removal actions together in a right-aligned row when a node is selected', () => {
		renderBoard(baseProjection(), { selectedId: 'plan:item-1', onCreateSection: vi.fn() });
		const actions = screen.getByRole('group', { name: 'Canvas actions' });
		expect(actions).toHaveClass('with-selection');
		expect(actions).toContainElement(screen.getByRole('button', { name: 'Create section' }));
		expect(actions).toContainElement(screen.getByRole('button', { name: 'Remove from Canvas' }));
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

	it('keeps the Focus label concise for more than one active plan and session', () => {
		const projection = focusProjection();
		const activeSession2 = projection.nodes.find((node) => node.id === 'session:session-2')!;
		activeSession2.session = { record: { ...activeSession2.session!.record, state: 'running', live: true } };
		const plan1 = projection.nodes.find((node) => node.id === 'plan:item-1')!;
		const session3: CanvasNode = { ...activeSession2, id: 'session:session-3', entityRef: { ...activeSession2.entityRef, sessionId: 'session-3' }, session: { record: { ...activeSession2.session!.record, id: 'session-3' } } };
		projection.nodes.push(session3);
		projection.connections.push({ id: 'four', source: plan1.id, target: session3.id, kind: 'session_launched_from', sourceOfTruth: 'derived' });
		renderBoard(projection);
		fireEvent.click(screen.getByRole('button', { name: 'Focus' }));
		expect(screen.getByRole('button', { name: 'Focus' })).toHaveTextContent('Focus');
	});

	it('removes inactive nodes and their edges from the graph when Focus is on', () => {
		renderBoard(focusProjection());
		expect(screen.getByTestId('react-flow')).toHaveAttribute('data-nodes', '4');
		expect(screen.getByTestId('react-flow')).toHaveAttribute('data-edges', '2');
		fireEvent.click(screen.getByRole('button', { name: 'Focus' }));
		expect(screen.getByRole('button', { name: 'Focus' })).toHaveAttribute('aria-pressed', 'true');
		expect(screen.queryByLabelText('Plan: PM-038 Focused Canvas main')).not.toBeInTheDocument();
		expect(screen.queryByLabelText('Session: codex main exited')).not.toBeInTheDocument();
		expect(screen.getByLabelText('Plan: PM-037 Focused Canvas main')).toBeInTheDocument();
		expect(screen.getByLabelText('Session: codex main running')).toBeInTheDocument();
		// No sections are rendered while Focus is on; one plan and one session left.
		expect(screen.getByTestId('react-flow')).toHaveAttribute('data-nodes', '2');
		expect(screen.getByTestId('react-flow')).toHaveAttribute('data-edges', '1');
	});

	it('lays out active plans in a left column and their sessions in a row to the right', () => {
		renderBoard(focusLayoutProjection());
		fireEvent.click(screen.getByRole('button', { name: 'Focus' }));
		const positionOf = (label: string) => JSON.parse(screen.getByLabelText(label).closest('[data-draggable-node]')!.getAttribute('data-position')!);
		expect(positionOf('Plan: PM-037 Focused Canvas main')).toEqual({ x: 0, y: 0 });
		expect(positionOf('Session: codex main running')).toEqual({ x: 285, y: 0 });
		expect(positionOf('Plan: PM-038 Focused Canvas main')).toEqual({ x: 0, y: 540 });
		expect(positionOf('Session: claude main running')).toEqual({ x: 285, y: 540 });
		expect(positionOf('Session: gemini main running')).toEqual({ x: 985, y: 540 });
	});

	it('sets nodes not draggable while Focus is on and restores draggability and projection positions when it turns off', () => {
		renderBoard(focusLayoutProjection());
		const before = screen.getByLabelText('Plan: PM-037 Focused Canvas main').closest('[data-draggable-node]')?.getAttribute('data-position');
		fireEvent.click(screen.getByRole('button', { name: 'Focus' }));
		expect(document.querySelectorAll('[data-draggable-node="true"]')).toHaveLength(0);
		fireEvent.click(screen.getByRole('button', { name: /^Focus/ }));
		const after = screen.getByLabelText('Plan: PM-037 Focused Canvas main').closest('[data-draggable-node]')?.getAttribute('data-position');
		expect(after).toEqual(before);
		expect(document.querySelectorAll('[data-draggable-node="true"]').length).toBeGreaterThan(0);
	});

	it('fits the view to the focus layout on toggle-on and restores the prior viewport on toggle-off', async () => {
		renderBoard(focusLayoutProjection());
		fireEvent.click(screen.getByRole('button', { name: 'Focus' }));
		await waitFor(() => expect(fitView).toHaveBeenCalled());
		fireEvent.click(screen.getByRole('button', { name: /^Focus/ }));
		await waitFor(() => expect(setViewport).toHaveBeenCalledWith({ x: 11, y: 22, zoom: 1.5 }, expect.anything()));
	});

	it('hides the disclosure and ignores the terminal close action while Focus is on', async () => {
		vi.mocked(api.embeddedAISession).mockResolvedValue({ id: 'session-1', itemId: 'item-1', workspaceId: 'workspace-1', provider: 'codex', intent: 'card_context', state: 'running', startedAt: '' });
		vi.mocked(api.embeddedAISessionGrant).mockResolvedValue({ sessionId: 'session-1', token: 'grant', expiresAt: '' });
		const onSetCollapsed = vi.fn();
		renderBoard(focusProjection(), { onSetCollapsed });
		fireEvent.click(screen.getByRole('button', { name: 'Focus' }));
		expect(screen.queryByRole('button', { name: /Expand session terminal|Collapse session terminal/ })).not.toBeInTheDocument();
		expect(await screen.findByTestId('canvas-terminal-session-1')).toHaveAttribute('data-visible', 'true');
		fireEvent.click(screen.getByRole('button', { name: 'Hide terminal' }));
		expect(onSetCollapsed).not.toHaveBeenCalled();
	});

	it('renders no sections while Focus is on and never calls onMoveSection, onMoveNode or onSaveViewport', () => {
		const onMoveSection = vi.fn();
		const onMoveNode = vi.fn();
		const onSaveViewport = vi.fn();
		renderBoard(focusProjection(), { sections: [{ id: 'section-1', title: 'Assistant work', position: { x: 320, y: -30 }, width: 620, height: 360, nodeIds: ['plan:item-1', 'session:session-1'] }], onMoveSection, onMoveNode, onSaveViewport });
		expect(screen.getByLabelText('Section: Assistant work')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Focus' }));
		expect(screen.queryByLabelText('Section: Assistant work')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /Move section/ })).not.toBeInTheDocument();
		fireEvent.click(screen.getByText('drag plan:item-1'));
		fireEvent.click(screen.getByRole('button', { name: 'move viewport' }));
		expect(onMoveSection).not.toHaveBeenCalled();
		expect(onMoveNode).not.toHaveBeenCalled();
		expect(onSaveViewport).not.toHaveBeenCalled();
	});

	it('falls back to Focus off when the stored preference value is invalid', () => {
		localStorage.setItem('kodeStream.canvas-focus', JSON.stringify('yes'));
		renderBoard(baseProjection());
		expect(screen.getByRole('button', { name: 'Focus' })).toHaveAttribute('aria-pressed', 'false');
	});

	it('keeps only the Focus toggle when no session is active', () => {
		const projection = baseProjection();
		projection.nodes[2] = { ...projection.nodes[2], session: { record: { ...projection.nodes[2].session!.record, state: 'exited', live: false } } };
		renderBoard(projection);
		const button = screen.getByRole('button', { name: 'Focus' });
		expect(button.querySelector('.lucide-eye-off')).toBeInTheDocument();
		fireEvent.click(button);
		expect(button).toHaveAttribute('aria-pressed', 'true');
		expect(button.querySelector('.lucide-eye')).toBeInTheDocument();
		expect(screen.queryByText('nothing running')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Turn off' })).not.toBeInTheDocument();
		expect(screen.queryByText('Layout is automatic while Focus is on')).not.toBeInTheDocument();
	});

	it('exposes aria-pressed on the Focus toggle and persists the preference across renders', () => {
		const first = renderBoard(baseProjection());
		const button = screen.getByRole('button', { name: 'Focus' });
		expect(button).toHaveAttribute('aria-pressed', 'false');
		fireEvent.click(button);
		expect(button).toHaveAttribute('aria-pressed', 'true');
		expect(JSON.parse(localStorage.getItem('kodeStream.canvas-focus') ?? 'null')).toBe(true);
		first.unmount();
		renderBoard(baseProjection());
		expect(screen.getByRole('button', { name: /^Focus/ })).toHaveAttribute('aria-pressed', 'true');
	});

	it('keeps many placements virtualized', () => {
		const count = 100;
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

/** Adds a second plan and a non-live, non-running session launched from it, so Focus mode has both an active pair (item-1/session-1) and an inactive one (item-2/session-2) to distinguish. */
function focusProjection(): CanvasProjection {
	const base = baseProjection();
	const plan = base.nodes[1];
	const session = base.nodes[2];
	const plan2: CanvasNode = { ...plan, id: 'plan:item-2', entityRef: { ...plan.entityRef, itemId: 'item-2' }, position: { x: 900, y: 0 }, plan: { ...plan.plan!, itemId: 'item-2', identifier: 'PM-038' } };
	const session2: CanvasNode = { ...session, id: 'session:session-2', entityRef: { ...session.entityRef, sessionId: 'session-2' }, position: { x: 1200, y: 420 }, session: { record: { ...session.session!.record, id: 'session-2', state: 'exited', live: false } } };
	return {
		...base,
		nodes: [...base.nodes, plan2, session2],
		connections: [...base.connections, { id: 'three', source: 'plan:item-2', target: 'session:session-2', kind: 'session_launched_from', sourceOfTruth: 'derived' }]
	};
}

/** Two active plans: item-1 with one active session, item-2 with two, so the
 * column-and-row Focus layout rule can be asserted against exact positions. */
function focusLayoutProjection(): CanvasProjection {
	const base = baseProjection();
	const plan1 = base.nodes[1];
	const session1 = base.nodes[2];
	const plan2: CanvasNode = { ...plan1, id: 'plan:item-2', entityRef: { ...plan1.entityRef, itemId: 'item-2' }, position: { x: 900, y: 900 }, plan: { ...plan1.plan!, itemId: 'item-2', identifier: 'PM-038' } };
	const session2: CanvasNode = { ...session1, id: 'session:session-2', entityRef: { ...session1.entityRef, sessionId: 'session-2' }, position: { x: 1200, y: 900 }, session: { record: { ...session1.session!.record, id: 'session-2', provider: 'claude' } } };
	const session3: CanvasNode = { ...session1, id: 'session:session-3', entityRef: { ...session1.entityRef, sessionId: 'session-3' }, position: { x: 1600, y: 900 }, session: { record: { ...session1.session!.record, id: 'session-3', provider: 'gemini' } } };
	return {
		...base,
		nodes: [...base.nodes, plan2, session2, session3],
		connections: [
			...base.connections,
			{ id: 'three', source: 'plan:item-2', target: 'session:session-2', kind: 'session_launched_from', sourceOfTruth: 'derived' },
			{ id: 'four', source: 'plan:item-2', target: 'session:session-3', kind: 'session_launched_from', sourceOfTruth: 'derived' }
		]
	};
}

describe('CanvasBoard toolbar', () => {
	it('leaves the view switch to the page and puts Node type before Focus', () => {
		renderBoard(baseProjection());
		expect(screen.queryByRole('group', { name: 'Agent Space view' })).not.toBeInTheDocument();
		const nodeType = screen.getByRole('button', { name: 'Node type' });
		expect(nodeType.compareDocumentPosition(screen.getByRole('button', { name: 'Focus' })) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});
});

describe('CanvasBoard ephemeral sessions', () => {
	it('labels a live unplaced session and offers Pin instead of Remove', () => {
		const base = baseProjection();
		const ephemeral: CanvasNode = { ...base.nodes[2], revision: 0, collapsed: false, ephemeral: true };
		const onPinSession = vi.fn();
		const onRemove = vi.fn();
		renderBoard({ ...base, nodes: [base.nodes[0], base.nodes[1], ephemeral] }, { selectedId: ephemeral.id, onPinSession, onRemove });
		expect(screen.getByText('Live session')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /Remove from Canvas/ })).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: /Pin to Canvas/ }));
		expect(onPinSession).toHaveBeenCalledWith('session-1');
		expect(onRemove).not.toHaveBeenCalled();
	});
});

import { memo, useEffect, useMemo, useState } from 'react';
import { Background, Controls, Handle, Position, ReactFlow, ReactFlowProvider, useNodesState, useReactFlow } from '@xyflow/react';
import type { Edge, Node, NodeProps, OnMoveEnd, OnNodeDrag } from '@xyflow/react';
import { Box, GitBranch, Play, Search, TerminalSquare, Trash2, Workflow } from 'lucide-react';
import type { CanvasNode as DomainNode, CanvasPosition, CanvasProjection, CanvasViewport, GitStatus } from '../../lib/types';
import '@xyflow/react/dist/style.css';

interface CanvasNodeData extends Record<string, unknown> {
	node: DomainNode;
	selected: boolean;
	conflicted: boolean;
	onSelect: (id: string) => void;
	onKeyboardMove: (node: DomainNode, position: CanvasPosition) => void;
	layoutAvailable: boolean;
}

type CanvasFlowNode = Node<CanvasNodeData>;

const nodeTypes = { workspace: memo(WorkspaceCanvasNode), plan: memo(PlanCanvasNode), session: memo(SessionCanvasNode) };

export function CanvasBoard({ projection, conflicts, selectedId, onSelect, onMoveNode, onSaveViewport, onReloadPosition, onReapplyPosition, onPlaceUnplaced, onReset, onRemove }: {
	projection: CanvasProjection;
	conflicts: string[];
	selectedId?: string;
	onSelect: (id?: string) => void;
	onMoveNode: (id: string, position: CanvasPosition) => void;
	onSaveViewport: (viewport: CanvasViewport) => void;
	onReloadPosition: (id: string) => void;
	onReapplyPosition: (id: string) => void;
	onPlaceUnplaced: () => void;
	onReset: () => void;
	onRemove: (node: DomainNode) => void;
}) {
	const layoutCapability = projection.nodes.find((node) => node.workspace)?.workspace?.actions['layout.move'];
	const layoutAvailable = layoutCapability?.state === 'available';
	const modelNodes = useMemo(() => projection.nodes.map((node): CanvasFlowNode => ({
		id: node.id,
		type: node.kind,
		position: node.position,
		draggable: canMove(node, layoutAvailable),
		selectable: true,
		data: { node, selected: node.id === selectedId, conflicted: conflicts.includes(node.id), onSelect: (id) => onSelect(id), onKeyboardMove: (candidate, position) => onMoveNode(candidate.id, position), layoutAvailable }
	})), [conflicts, layoutAvailable, onMoveNode, onSelect, projection.nodes, selectedId]);
	const [nodes, setNodes, onNodesChange] = useNodesState<CanvasFlowNode>(modelNodes);
	useEffect(() => setNodes(modelNodes), [modelNodes, setNodes]);
	const edges = useMemo(() => projection.connections.map((connection): Edge => ({ id: connection.id, source: connection.source, target: connection.target, selectable: false, focusable: false, className: `canvas-edge canvas-edge-${connection.kind}` })), [projection.connections]);
	const onDragStop: OnNodeDrag<CanvasFlowNode> = (_, node) => onMoveNode(node.id, node.position);
	const onMoveEnd: OnMoveEnd = (_, viewport) => onSaveViewport(viewport);
	const selected = projection.nodes.find((node) => node.id === selectedId);

	return <ReactFlowProvider><div className="canvas-board-shell">
		<div className="canvas-toolbar" aria-label="Canvas tools">
			<CanvasSearch nodes={projection.nodes} onSelect={onSelect} />
			<button type="button" onClick={onPlaceUnplaced} disabled={!layoutAvailable || projection.unplaced.length === 0} title={!layoutAvailable ? layoutCapability?.message : undefined}><Box size={14} /> Place new items ({projection.unplaced.length})</button>
			<button type="button" onClick={onReset} disabled={!layoutAvailable || projection.nodes.length === 0} title={!layoutAvailable ? layoutCapability?.message : undefined}><Workflow size={14} /> Reset layout</button>
			{selected && <button type="button" onClick={() => onRemove(selected)} disabled={!layoutAvailable} title={!layoutAvailable ? layoutCapability?.message : undefined}><Trash2 size={14} /> Remove from Canvas</button>}
			{selected && conflicts.includes(selected.id) && <span className="canvas-conflict-actions" role="alert"><span>Position conflict</span><button type="button" onClick={() => onReloadPosition(selected.id)}>Reload position</button><button type="button" onClick={() => onReapplyPosition(selected.id)}>Reapply my move</button></span>}
		</div>
		<div className="sr-only" role="status" aria-live="polite">{selected ? `Selected ${nodeSearchText(selected)}` : 'No Canvas node selected'}</div>
		<div className="canvas-board" data-node-count={nodes.length}>
			<ReactFlow key={projection.layout.id} nodes={nodes} edges={edges} nodeTypes={nodeTypes} onNodesChange={onNodesChange} onNodeDragStop={onDragStop} onNodeClick={(_, node) => onSelect(node.id)} onPaneClick={() => onSelect(undefined)} onMoveEnd={onMoveEnd} defaultViewport={projection.layout.viewport} minZoom={0.1} maxZoom={2} onlyRenderVisibleElements nodesDraggable elementsSelectable edgesFocusable={false} fitView={false}>
				<Background color="var(--line)" gap={24} />
				<Controls position="bottom-left" showInteractive={false} />
			</ReactFlow>
		</div>
	</div></ReactFlowProvider>;
}

function WorkspaceCanvasNode({ data }: NodeProps<CanvasFlowNode>) {
	const node = data.node;
	const workspace = node.workspace;
	return <NodeFrame data={data} eyebrow="Workspace" icon={<Workflow size={15} />}>
		<strong>{workspace?.name || 'Unavailable workspace'}</strong>
		<span><GitBranch size={12} /> {workspace?.branch || node.entityRef.branchKey || 'Unknown branch'}</span>
		<span>{gitSummary(workspace?.git)}</span>
		{workspace?.commit && <span>HEAD {workspace.commit.slice(0, 8)}</span>}
		{workspace?.verification && <><span className={`canvas-verification-result result-${workspace.verification.status} freshness-${workspace.verification.freshness}`}>{workspace.verification.status}</span><span>Freshness: {workspace.verification.freshness || 'inconclusive'}</span></>}
	</NodeFrame>;
}

function PlanCanvasNode({ data }: NodeProps<CanvasFlowNode>) {
	const node = data.node;
	return <NodeFrame data={data} eyebrow="Plan" icon={<Box size={15} />}>
		<strong>{node.plan?.identifier || node.entityRef.identifier || 'Unavailable plan'}</strong>
		<span>{node.plan?.title || stateLabel(node)}</span>
		<span><GitBranch size={12} /> {node.plan?.branch || node.entityRef.branchKey}</span>
	</NodeFrame>;
}

function SessionCanvasNode({ data }: NodeProps<CanvasFlowNode>) {
	const record = data.node.session?.record;
	return <NodeFrame data={data} eyebrow="Session" icon={<TerminalSquare size={15} />}>
		<strong>{record?.provider || 'Unavailable session'}</strong>
		<span className={`canvas-session-state state-${record?.state || 'stale'}`}><Play size={11} /> {record?.state || stateLabel(data.node)}</span>
		<span>{record?.requestedBranch || data.node.entityRef.branchKey}</span>
	</NodeFrame>;
}

function NodeFrame({ data, eyebrow, icon, children }: { data: CanvasNodeData; eyebrow: string; icon: React.ReactNode; children: React.ReactNode }) {
	return <div data-canvas-node-id={data.node.id} className={`canvas-semantic-node kind-${data.node.kind} state-${data.node.state}${data.selected ? ' selected' : ''}${data.conflicted ? ' conflicted' : ''}`} role="button" tabIndex={0} aria-label={`${eyebrow}: ${nodeSearchText(data.node)}`} onClick={(event) => { event.stopPropagation(); data.onSelect(data.node.id); }} onKeyDown={(event) => {
		if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); data.onSelect(data.node.id); return; }
		if (!canMove(data.node, data.layoutAvailable) || !['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return;
		event.preventDefault();
		const step = event.shiftKey ? 1 : 12;
		const position = { ...data.node.position };
		if (event.key === 'ArrowLeft') position.x -= step;
		if (event.key === 'ArrowRight') position.x += step;
		if (event.key === 'ArrowUp') position.y -= step;
		if (event.key === 'ArrowDown') position.y += step;
		data.onKeyboardMove(data.node, position);
	}}>
		<Handle className="canvas-derived-handle" type="target" position={Position.Left} isConnectable={false} />
		<header>{icon}<span>{eyebrow}</span>{data.node.state !== 'resolved' && <em>{data.node.state}</em>}</header>
		<div>{children}</div>
		<Handle className="canvas-derived-handle" type="source" position={Position.Right} isConnectable={false} />
	</div>;
}

function CanvasSearch({ nodes, onSelect }: { nodes: DomainNode[]; onSelect: (id?: string) => void }) {
	const [query, setQuery] = useState('');
	const [matches, setMatches] = useState<DomainNode[]>([]);
	const { fitView, getNode } = useReactFlow();
	const search = (value: string) => {
		setQuery(value);
		const normalized = value.trim().toLowerCase();
		setMatches(normalized ? nodes.filter((node) => nodeSearchText(node).toLowerCase().includes(normalized)).slice(0, 8) : []);
	};
	const focus = (node: DomainNode) => {
		onSelect(node.id);
		setMatches([]);
		const target = getNode(node.id);
		if (target) void fitView({ nodes: [target], padding: 0.8, maxZoom: 1.25, duration: 180 });
		requestAnimationFrame(() => document.querySelector<HTMLElement>(`[data-canvas-node-id="${node.id.replaceAll('"', '\\"')}"]`)?.focus());
	};
	return <div className="canvas-search"><Search size={14} /><input aria-label="Search Canvas nodes" value={query} onChange={(event) => search(event.target.value)} placeholder="Search plans and sessions" />{matches.length > 0 && <div className="canvas-search-results" role="listbox">{matches.map((node) => <button key={node.id} type="button" role="option" onClick={() => focus(node)}>{nodeSearchText(node)}</button>)}</div>}</div>;
}

function canMove(node: DomainNode, workspaceLayoutAvailable = false) {
	const actions = node.workspace?.actions ?? node.plan?.actions;
	return actions?.['layout.move']?.state === 'available' || (!actions && workspaceLayoutAvailable);
}

function nodeSearchText(node: DomainNode) {
	if (node.workspace) return `${node.workspace.name ?? ''} ${node.workspace.branch ?? ''}`.trim();
	if (node.plan) return `${node.plan.identifier ?? ''} ${node.plan.title ?? ''} ${node.plan.branch ?? ''}`.trim();
	if (node.session) return `${node.session.record.provider} ${node.session.record.requestedBranch} ${node.session.record.state}`;
	return `${node.kind} ${node.entityRef.identifier ?? node.entityRef.sessionId ?? ''}`.trim();
}

function stateLabel(node: DomainNode) { return node.state === 'forbidden' ? 'Access restricted' : 'Reference unavailable'; }
function gitSummary(status?: GitStatus) {
	if (!status) return 'Git status unavailable';
	if (status.conflicted) return `${status.changes.length} changes · conflicted`;
	return status.dirty ? `${status.changes.length} changed files` : 'Clean working tree';
}

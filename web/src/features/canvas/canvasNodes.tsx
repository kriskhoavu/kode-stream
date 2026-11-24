import { useEffect, useRef } from 'react';
import { Handle, NodeResizer, Position, useReactFlow } from '@xyflow/react';
import type { Node as XYNode, NodeProps } from '@xyflow/react';
import { Box, GitBranch, Maximize2, Minimize2, Play, TerminalSquare, Workflow, X } from 'lucide-react';
import type { CanvasNode as DomainNode, CanvasPosition, CanvasSection, CanvasSize, GitStatus } from '../../lib/types';
import { SessionTerminal } from '../ai-session/SessionTerminal';
import { canMove, formatFacet, nodeSearchText } from './canvasBoardUtils';

export interface CanvasNodeData extends Record<string, unknown> {
	node: DomainNode;
	selected: boolean;
	sectionSelected?: boolean;
	conflicted: boolean;
	onSelect: (id?: string, additive?: boolean) => void;
	onKeyboardMove: (node: DomainNode, position: CanvasPosition) => void;
	onResizeNode: (id: string, size: CanvasSize) => void;
	onSetCollapsed: (id: string, collapsed: boolean) => void;
	onReload: () => Promise<unknown> | void;
	layoutAvailable: boolean;
	focusOn?: boolean;
	section?: CanvasSection;
	onRemoveSection?: (id: string) => void;
	onMoveSection?: (id: string, position: CanvasPosition, delta: CanvasPosition) => void;
}

export type CanvasFlowNode = XYNode<CanvasNodeData>;

export function CanvasSectionNode({ data }: NodeProps<CanvasFlowNode>) {
	const section = data.section;
	const { screenToFlowPosition } = useReactFlow();
	const moveCleanupRef = useRef<(() => void) | null>(null);
	useEffect(() => () => moveCleanupRef.current?.(), []);
	if (!section) return null;
	const startMove = (event: React.PointerEvent<HTMLElement>) => {
		if (!section) return;
		const movingSection = section;
		if (!data.layoutAvailable || (event.target as HTMLElement).closest('button')) return;
		event.preventDefault();
		event.stopPropagation();
		moveCleanupRef.current?.();
		const start = screenToFlowPosition({ x: event.clientX, y: event.clientY });
		function cleanup() {
			window.removeEventListener('pointerup', finish);
			if (moveCleanupRef.current === cleanup) moveCleanupRef.current = null;
		}
		function finish(upEvent: PointerEvent) {
			cleanup();
			const end = screenToFlowPosition({ x: upEvent.clientX, y: upEvent.clientY });
			const delta = { x: end.x - start.x, y: end.y - start.y };
			if (delta.x || delta.y) data.onMoveSection?.(movingSection.id, { x: movingSection.position.x + delta.x, y: movingSection.position.y + delta.y }, delta);
		}
		window.addEventListener('pointerup', finish, { once: true });
		moveCleanupRef.current = cleanup;
	};
	const sectionKind = section.id === 'workspace-group' ? 'workspace' : section.id.startsWith('service-') ? 'service' : 'custom';
	const moveByKeyboard = (event: React.KeyboardEvent<HTMLButtonElement>) => {
		if (!data.layoutAvailable || !['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return;
		event.preventDefault();
		const step = event.shiftKey ? 1 : 12;
		const position = { ...section.position };
		if (event.key === 'ArrowLeft') position.x -= step;
		if (event.key === 'ArrowRight') position.x += step;
		if (event.key === 'ArrowUp') position.y -= step;
		if (event.key === 'ArrowDown') position.y += step;
		data.onMoveSection?.(section.id, position, { x: position.x - section.position.x, y: position.y - section.position.y });
	};
	return <section className={`canvas-section-boundary ${sectionKind}`} aria-label={`Section: ${section.title}`}><header className="nodrag nopan"><button className="canvas-section-move-handle nodrag nopan" type="button" aria-label={`Move section ${section.title}`} onPointerDown={startMove} onKeyDown={moveByKeyboard}><strong className={`canvas-section-title ${sectionKind}`}>{section.title}</strong><span>{section.nodeIds.length} nodes</span></button><button className="nodrag nopan" type="button" aria-label={`Remove section ${section.title}`} onClick={() => data.onRemoveSection?.(section.id)}><X size={13} /></button></header></section>;
}

export function WorkspaceCanvasNode({ data }: NodeProps<CanvasFlowNode>) {
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

export function PlanCanvasNode({ data }: NodeProps<CanvasFlowNode>) {
	const node = data.node;
	return <NodeFrame data={data} eyebrow="Plan" icon={<Box size={15} />}>
		<strong>{node.plan?.identifier || node.entityRef.identifier || 'Unavailable plan'}</strong>
		<span>{node.plan?.title || stateLabel(node)}</span>
		{node.plan && <span>{node.plan.service || 'Other'} · {formatFacet(node.plan.status)}</span>}
		<span><GitBranch size={12} /> {node.plan?.branch || node.entityRef.branchKey}</span>
	</NodeFrame>;
}

export function SessionCanvasNode({ data }: NodeProps<CanvasFlowNode>) {
	const record = data.node.session?.record;
	const expanded = Boolean(record && !data.node.collapsed);
	// Focus force-expands sessions, so collapse and close must not persist that state.
	const disclosure = record && !data.focusOn ? <button className="icon-button canvas-node-action nodrag nopan" type="button" aria-label={expanded ? 'Collapse session terminal' : 'Expand session terminal'} title={expanded ? 'Collapse terminal' : 'Expand terminal'} onClick={(event) => { event.stopPropagation(); data.onSelect(data.node.id); data.onSetCollapsed(data.node.id, expanded); }}>{expanded ? <Minimize2 size={14} /> : <Maximize2 size={14} />}</button> : undefined;
	return <NodeFrame data={data} eyebrow={data.node.ephemeral ? 'Live session' : 'Session'} icon={<TerminalSquare size={15} />} action={disclosure} expanded={expanded} resizable={expanded && !data.focusOn} addon={record ? <SessionTerminal record={record} visible={expanded} onClose={() => { if (!data.focusOn) data.onSetCollapsed(data.node.id, true); }} onReload={data.onReload} /> : undefined}>
		<strong>{record?.provider || 'Unavailable session'}</strong>
		<span className={`canvas-session-state state-${record?.state || 'stale'}`}><Play size={11} /> {record?.state || stateLabel(data.node)}</span>
		<span>{record?.requestedBranch || data.node.entityRef.branchKey}</span>
	</NodeFrame>;
}

export function NodeFrame({ data, eyebrow, icon, children, addon, action, expanded = false, resizable = false }: { data: CanvasNodeData; eyebrow: string; icon: React.ReactNode; children: React.ReactNode; addon?: React.ReactNode; action?: React.ReactNode; expanded?: boolean; resizable?: boolean }) {
	return <div className={`canvas-semantic-node kind-${data.node.kind} state-${data.node.state}${data.selected ? ' selected' : ''}${data.sectionSelected ? ' section-selected' : ''}${data.conflicted ? ' conflicted' : ''}${expanded ? ' expanded' : ''}`}>
		{resizable && <NodeResizer isVisible minWidth={520} minHeight={500} maxWidth={5000} maxHeight={5000} handleClassName="canvas-session-resize-handle nodrag nopan" lineClassName="canvas-session-resize-line" onResizeEnd={(_, size) => data.onResizeNode(data.node.id, { width: Math.round(size.width), height: Math.round(size.height) })} />}
		<Handle className="canvas-derived-handle" type="target" position={Position.Left} isConnectable={false} />
		<div data-canvas-node-id={data.node.id} className="canvas-node-select-target" role="button" tabIndex={0} aria-label={`${eyebrow}: ${nodeSearchText(data.node)}`} onClick={(event) => { event.stopPropagation(); data.onSelect(data.node.id, event.metaKey || event.ctrlKey); }} onKeyDown={(event) => {
		if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); data.onSelect(data.node.id); return; }
		if (data.focusOn || !canMove(data.node, data.layoutAvailable) || !['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return;
		event.preventDefault();
		const step = event.shiftKey ? 1 : 12;
		const position = { ...data.node.position };
		if (event.key === 'ArrowLeft') position.x -= step;
		if (event.key === 'ArrowRight') position.x += step;
		if (event.key === 'ArrowUp') position.y -= step;
		if (event.key === 'ArrowDown') position.y += step;
		data.onKeyboardMove(data.node, position);
		}}>
		<header>{icon}<span>{eyebrow}</span>{data.node.state !== 'resolved' && <em>{data.node.state}</em>}</header>
		<div className="canvas-node-summary">{children}</div>
		</div>
		{action}
		{addon}
		<Handle className="canvas-derived-handle" type="source" position={Position.Right} isConnectable={false} />
	</div>;
}

export function stateLabel(node: DomainNode) { return node.state === 'forbidden' ? 'Access restricted' : 'Reference unavailable'; }
export function gitSummary(status?: GitStatus) {
	if (!status) return 'Git status unavailable';
	if (status.conflicted) return `${status.changes.length} changes · conflicted`;
	return status.dirty ? `${status.changes.length} changed files` : 'Clean working tree';
}

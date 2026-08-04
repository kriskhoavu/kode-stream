import { memo, useEffect, useMemo, useRef, useState } from 'react';
import { Background, Controls, Handle, NodeResizer, Position, ReactFlow, ReactFlowProvider, useNodesState } from '@xyflow/react';
import type { Edge, Node as XYNode, NodeProps, OnMoveEnd, OnNodeDrag } from '@xyflow/react';
import { Bot, Box, ChevronDown, GitBranch, Maximize2, Minimize2, Play, Search, Square, TerminalSquare, Trash2, Workflow } from 'lucide-react';
import { api } from '../../lib/api';
import type { CanvasNode as DomainNode, CanvasPosition, CanvasProjection, CanvasViewport, EmbeddedAISessionResult, EmbeddedAISessionState, GitStatus, SafeSessionRecord } from '../../lib/types';
import { EmbeddedTerminal } from '../ai-session/EmbeddedTerminal';
import '@xyflow/react/dist/style.css';

interface CanvasNodeData extends Record<string, unknown> {
	node: DomainNode;
	selected: boolean;
	conflicted: boolean;
	onSelect: (id?: string) => void;
	onKeyboardMove: (node: DomainNode, position: CanvasPosition) => void;
	onSetCollapsed: (id: string, collapsed: boolean) => void;
	onReload: () => Promise<unknown> | void;
	layoutAvailable: boolean;
}

type CanvasFlowNode = XYNode<CanvasNodeData>;

const nodeTypes = { workspace: memo(WorkspaceCanvasNode), plan: memo(PlanCanvasNode), session: memo(SessionCanvasNode) };

export function CanvasBoard({ projection, conflicts, selectedId, onSelect, onMoveNode, onSetCollapsed, onArrange, onSaveViewport, onReload, onReloadPosition, onReapplyPosition, onReset, onRemove, onOpenAISession, aiSessionDisabled = true }: {
	projection: CanvasProjection;
	conflicts: string[];
	selectedId?: string;
	onSelect: (id?: string) => void;
	onMoveNode: (id: string, position: CanvasPosition) => void;
	onSetCollapsed: (id: string, collapsed: boolean) => void;
	onArrange: (grouping: 'status' | 'service' | 'service_status') => void;
	onSaveViewport: (viewport: CanvasViewport) => void;
	onReload: () => Promise<unknown> | void;
	onReloadPosition: (id: string) => void;
	onReapplyPosition: (id: string) => void;
	onReset: () => void;
	onRemove: (node: DomainNode) => void;
	onOpenAISession?: () => void;
	aiSessionDisabled?: boolean;
}) {
	const [statusFilters, setStatusFilters] = useState<string[]>([]);
	const [serviceFilters, setServiceFilters] = useState<string[]>([]);
	const [searchQuery, setSearchQuery] = useState('');
	const [openMenu, setOpenMenu] = useState('');
	const layoutCapability = projection.nodes.find((node) => node.workspace)?.workspace?.actions['layout.move'];
	const layoutAvailable = layoutCapability?.state === 'available';
	const statuses = useMemo(() => Array.from(new Set(projection.nodes.flatMap((node) => node.plan ? [node.plan.status] : []))), [projection.nodes]);
	const services = useMemo(() => Array.from(new Set(projection.nodes.flatMap((node) => node.plan?.service ? [node.plan.service] : []))).sort(), [projection.nodes]);
	const normalizedSearchQuery = searchQuery.trim().toLowerCase();
	const visibleDomainNodes = useMemo(() => projection.nodes.filter((node) => (!node.plan || ((statusFilters.length === 0 || statusFilters.includes(node.plan.status)) && (serviceFilters.length === 0 || serviceFilters.includes(node.plan.service ?? '')))) && (!normalizedSearchQuery || nodeSearchText(node).toLowerCase().includes(normalizedSearchQuery))), [normalizedSearchQuery, projection.nodes, serviceFilters, statusFilters]);
	const visibleIDs = useMemo(() => new Set(visibleDomainNodes.map((node) => node.id)), [visibleDomainNodes]);
	const hiddenCount = projection.nodes.length - visibleDomainNodes.length;
	const modelNodes = useMemo(() => visibleDomainNodes.map((node): CanvasFlowNode => ({
		id: node.id,
		type: node.kind,
		position: node.position,
		draggable: canMove(node, layoutAvailable),
		selectable: true,
		data: { node, selected: node.id === selectedId, conflicted: conflicts.includes(node.id), onSelect: (id) => onSelect(id), onKeyboardMove: (candidate, position) => onMoveNode(candidate.id, position), onSetCollapsed, onReload, layoutAvailable }
	})), [conflicts, layoutAvailable, onMoveNode, onReload, onSelect, onSetCollapsed, selectedId, visibleDomainNodes]);
	const [nodes, setNodes, onNodesChange] = useNodesState<CanvasFlowNode>(modelNodes);
	useEffect(() => setNodes(modelNodes), [modelNodes, setNodes]);
	const edges = useMemo(() => projection.connections.filter((connection) => visibleIDs.has(connection.source) && visibleIDs.has(connection.target)).map((connection): Edge => ({ id: connection.id, source: connection.source, target: connection.target, selectable: false, focusable: false, className: `canvas-edge canvas-edge-${connection.kind}` })), [projection.connections, visibleIDs]);
	const onDragStop: OnNodeDrag<CanvasFlowNode> = (_, node) => onMoveNode(node.id, node.position);
	const onMoveEnd: OnMoveEnd = (_, viewport) => onSaveViewport(viewport);
	const selected = projection.nodes.find((node) => node.id === selectedId);

	return <ReactFlowProvider><div className="canvas-board-shell">
		<div className="canvas-toolbar" aria-label="Canvas tools">
			<div className="canvas-board-toolbar">
				<CanvasSearch query={searchQuery} onQueryChange={setSearchQuery} />
				<button className="primary" type="button" disabled={aiSessionDisabled} onClick={onOpenAISession}><Bot size={15} /> AI session</button>
				<button className="secondary" type="button" onClick={onReset} disabled={!layoutAvailable || projection.nodes.length === 0} title={!layoutAvailable ? layoutCapability?.message : undefined}><Workflow size={15} /> Reset layout</button>
				{selected && <button className="secondary danger" type="button" onClick={() => onRemove(selected)} disabled={!layoutAvailable} title={!layoutAvailable ? layoutCapability?.message : undefined}><Trash2 size={15} /> Remove from Canvas</button>}
			</div>
			<div className="canvas-facet-row">
				<div className="facet-bar">
					<CanvasFacetMenu title="Status" options={statuses.map((value) => ({ value, label: formatFacet(value) }))} selected={statusFilters} open={openMenu === 'status'} onOpen={() => setOpenMenu(openMenu === 'status' ? '' : 'status')} onClose={() => setOpenMenu('')} onToggle={(value) => setStatusFilters((current) => toggleValue(current, value))} onClear={() => setStatusFilters([])} />
					<CanvasFacetMenu title="Service" options={services.map((value) => ({ value, label: value }))} selected={serviceFilters} open={openMenu === 'service'} onOpen={() => setOpenMenu(openMenu === 'service' ? '' : 'service')} onClose={() => setOpenMenu('')} onToggle={(value) => setServiceFilters((current) => toggleValue(current, value))} onClear={() => setServiceFilters([])} />
					<CanvasArrangeMenu open={openMenu === 'group'} disabled={!layoutAvailable} onOpen={() => setOpenMenu(openMenu === 'group' ? '' : 'group')} onClose={() => setOpenMenu('')} onArrange={onArrange} />
				</div>
				<span className="filter-summary">{visibleDomainNodes.length} of {projection.nodes.length} nodes{hiddenCount > 0 ? ` · ${hiddenCount} hidden` : ''}</span>
			</div>
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
		{node.plan && <span>{node.plan.service || 'Other'} · {formatFacet(node.plan.status)}</span>}
		<span><GitBranch size={12} /> {node.plan?.branch || node.entityRef.branchKey}</span>
	</NodeFrame>;
}

function SessionCanvasNode({ data }: NodeProps<CanvasFlowNode>) {
	const record = data.node.session?.record;
	const expanded = Boolean(record && !data.node.collapsed);
	const disclosure = record ? <button className="icon-button canvas-node-action nodrag nopan" type="button" aria-label={expanded ? 'Collapse session terminal' : 'Expand session terminal'} title={expanded ? 'Collapse terminal' : 'Expand terminal'} onClick={(event) => { event.stopPropagation(); data.onSelect(data.node.id); data.onSetCollapsed(data.node.id, expanded); }}>{expanded ? <Minimize2 size={14} /> : <Maximize2 size={14} />}</button> : undefined;
	return <NodeFrame data={data} eyebrow="Session" icon={<TerminalSquare size={15} />} action={disclosure} expanded={expanded} resizable={expanded} addon={record ? <CanvasSessionTerminal record={record} visible={expanded} onClose={() => data.onSetCollapsed(data.node.id, true)} onReload={data.onReload} /> : undefined}>
		<strong>{record?.provider || 'Unavailable session'}</strong>
		<span className={`canvas-session-state state-${record?.state || 'stale'}`}><Play size={11} /> {record?.state || stateLabel(data.node)}</span>
		<span>{record?.requestedBranch || data.node.entityRef.branchKey}</span>
	</NodeFrame>;
}

function NodeFrame({ data, eyebrow, icon, children, addon, action, expanded = false, resizable = false }: { data: CanvasNodeData; eyebrow: string; icon: React.ReactNode; children: React.ReactNode; addon?: React.ReactNode; action?: React.ReactNode; expanded?: boolean; resizable?: boolean }) {
	return <div className={`canvas-semantic-node kind-${data.node.kind} state-${data.node.state}${data.selected ? ' selected' : ''}${data.conflicted ? ' conflicted' : ''}${expanded ? ' expanded' : ''}`}>
		{resizable && <NodeResizer isVisible minWidth={520} minHeight={500} handleClassName="canvas-session-resize-handle" lineClassName="canvas-session-resize-line" />}
		<Handle className="canvas-derived-handle" type="target" position={Position.Left} isConnectable={false} />
		<div data-canvas-node-id={data.node.id} className="canvas-node-select-target" role="button" tabIndex={0} aria-label={`${eyebrow}: ${nodeSearchText(data.node)}`} onClick={(event) => { event.stopPropagation(); data.onSelect(data.node.id); }} onKeyDown={(event) => {
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
		<header>{icon}<span>{eyebrow}</span>{data.node.state !== 'resolved' && <em>{data.node.state}</em>}</header>
		<div className="canvas-node-summary">{children}</div>
		</div>
		{action}
		{addon}
		<Handle className="canvas-derived-handle" type="source" position={Position.Right} isConnectable={false} />
	</div>;
}

function CanvasSessionTerminal({ record, visible, onClose, onReload }: { record: SafeSessionRecord; visible: boolean; onClose: () => void; onReload: () => Promise<unknown> | void }) {
	const [result, setResult] = useState<EmbeddedAISessionResult>();
	const [attaching, setAttaching] = useState(false);
	const [error, setError] = useState('');
	const [retry, setRetry] = useState(0);
	const attempted = useRef(-1);
	const active = record.state === 'starting' || record.state === 'running';

	useEffect(() => {
		if (!visible || !record.live || result || attempted.current === retry) return;
		attempted.current = retry;
		setAttaching(true);
		setError('');
		void Promise.all([api.embeddedAISession(record.id), api.embeddedAISessionGrant(record.id)]).then(([session, grant]) => {
			setResult({ session, grant, record });
		}).catch(() => setError('The live process could not be reattached.')).finally(() => setAttaching(false));
	}, [record, result, retry, visible]);

	const cancel = async () => {
		if (!window.confirm('Cancel this terminal process? Its durable session metadata will remain available.')) return;
		setError('');
		try {
			const session = await api.cancelEmbeddedAISession(record.id);
			setResult((current) => current ? { ...current, session } : current);
			await Promise.resolve(onReload());
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'Could not cancel the terminal process.');
		}
	};
	const stateChanged = (state: EmbeddedAISessionState, exitCode?: number) => {
		setResult((current) => current ? { ...current, session: { ...current.session, state, exitCode } } : current);
		if (state !== 'starting' && state !== 'running') void onReload();
	};

	return <section className="canvas-session-terminal nodrag nopan nowheel" hidden={!visible} aria-label={`${record.provider} Canvas terminal`} onClick={(event) => event.stopPropagation()} onPointerDown={(event) => event.stopPropagation()}>
		{result && <EmbeddedTerminal initial={result} visible={visible} mode="side_panel" title={`${record.provider} terminal`} subtitle={record.requestedBranch} cancelOnClose={false} compact onStartMove={() => {}} onToggleDockMode={() => {}} onToggleMinimize={() => {}} onToggleMaximize={() => {}} onClose={onClose} onStateChange={stateChanged} />}
		{attaching && <p role="status">Connecting to terminal…</p>}
		{!record.live && <div className="canvas-session-lifecycle"><strong>Terminal session {record.state}</strong><span>Branch {record.requestedBranch}</span>{record.exitCode !== undefined && <span>Exit code {record.exitCode}</span>}{record.state === 'interrupted' && <p>The application restarted without this process. Reconnect is unavailable; launch a new session.</p>}</div>}
		{error && <div className="canvas-session-terminal-error" role="alert"><span>{error}</span>{record.live && <button type="button" onClick={() => { setError(''); setRetry((value) => value + 1); }}>Retry connection</button>}</div>}
		{active && <div className="canvas-session-terminal-actions"><button className="danger-confirm" type="button" onClick={() => void cancel()}><Square size={13} /> Cancel process</button></div>}
	</section>;
}

function CanvasSearch({ query, onQueryChange }: { query: string; onQueryChange: (value: string) => void }) {
	return <label className="filter-input plan-search canvas-search"><Search size={15} /><input aria-label="Search Canvas nodes" value={query} onChange={(event) => onQueryChange(event.target.value)} placeholder="Search items..." /></label>;
}

function CanvasFacetMenu({ title, options, selected, open, onOpen, onClose, onToggle, onClear }: { title: string; options: Array<{ value: string; label: string }>; selected: string[]; open: boolean; onOpen: () => void; onClose: () => void; onToggle: (value: string) => void; onClear: () => void }) {
	const menuRef = useRef<HTMLElement>(null);
	useEffect(() => {
		if (!open) return;
		const outside = (event: PointerEvent) => { if (!menuRef.current?.contains(event.target as Node)) onClose(); };
		const escape = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
		document.addEventListener('pointerdown', outside);
		document.addEventListener('keydown', escape);
		return () => { document.removeEventListener('pointerdown', outside); document.removeEventListener('keydown', escape); };
	}, [onClose, open]);
	if (options.length === 0) return null;
	return <section className="facet-menu" ref={menuRef}><button type="button" className={selected.length ? 'facet-trigger active' : 'facet-trigger'} aria-expanded={open} onClick={onOpen}><span>{title}</span><span className="facet-trigger-right">{selected.length > 0 && <strong>{selected.length}</strong>}<ChevronDown className={open ? 'facet-chevron open' : 'facet-chevron'} size={15} /></span></button>{open && <div className="facet-popover"><div className="facet-popover-header"><strong>{title}</strong><button type="button" onClick={onClear} disabled={selected.length === 0}>Clear</button></div><div className="facet-option-list">{options.map((option) => <label className="facet-option" key={option.value}><input type="checkbox" checked={selected.includes(option.value)} onChange={() => onToggle(option.value)} /><span>{option.label}</span></label>)}</div></div>}</section>;
}

function CanvasArrangeMenu({ open, disabled, onOpen, onClose, onArrange }: { open: boolean; disabled: boolean; onOpen: () => void; onClose: () => void; onArrange: (grouping: 'status' | 'service' | 'service_status') => void }) {
	const menuRef = useRef<HTMLElement>(null);
	useEffect(() => {
		if (!open) return;
		const outside = (event: PointerEvent) => { if (!menuRef.current?.contains(event.target as Node)) onClose(); };
		const escape = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
		document.addEventListener('pointerdown', outside);
		document.addEventListener('keydown', escape);
		return () => { document.removeEventListener('pointerdown', outside); document.removeEventListener('keydown', escape); };
	}, [onClose, open]);
	const choose = (value: 'status' | 'service' | 'service_status') => { onArrange(value); onClose(); };
	return <section className="facet-menu canvas-arrange-menu" ref={menuRef}><button type="button" className="facet-trigger" aria-expanded={open} disabled={disabled} onClick={onOpen}><span>Group</span><ChevronDown className={open ? 'facet-chevron open' : 'facet-chevron'} size={15} /></button>{open && <div className="facet-popover"><div className="facet-popover-header"><strong>Arrange plans by</strong></div><div className="facet-option-list"><button type="button" onClick={() => choose('status')}>Status</button><button type="button" onClick={() => choose('service')}>Service</button><button type="button" onClick={() => choose('service_status')}>Service + status</button></div></div>}</section>;
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
function formatFacet(value: string) { return value.replaceAll('_', ' ').replace(/^./, (character) => character.toUpperCase()); }
function toggleValue(values: string[], value: string) { return values.includes(value) ? values.filter((candidate) => candidate !== value) : [...values, value]; }
function gitSummary(status?: GitStatus) {
	if (!status) return 'Git status unavailable';
	if (status.conflicted) return `${status.changes.length} changes · conflicted`;
	return status.dirty ? `${status.changes.length} changed files` : 'Clean working tree';
}

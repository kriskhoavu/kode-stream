import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { RefObject } from 'react';
import { Background, Handle, NodeResizer, Position, ReactFlow, ReactFlowProvider, useNodesState, useReactFlow } from '@xyflow/react';
import type { Edge, Node as XYNode, NodeProps, OnMoveEnd, OnNodeDrag } from '@xyflow/react';
import { Bot, Box, ChevronDown, Expand, GitBranch, LayoutGrid, Maximize, Maximize2, Minimize2, Minus, Play, Plus, RotateCcw, Search, Square, TerminalSquare, Trash2, WandSparkles, Workflow, X } from 'lucide-react';
import { api } from '../../shared/api';
import type { CanvasNode as DomainNode, CanvasPosition, CanvasProjection, CanvasSection, CanvasViewport, EmbeddedAISessionResult, EmbeddedAISessionState, GitStatus, SafeSessionRecord } from '../../lib/types';
import { EmbeddedTerminal } from '../ai-session/EmbeddedTerminal';
import { readPreference, writePreference } from '../../shared/preferences/store';
import '@xyflow/react/dist/style.css';

interface CanvasNodeData extends Record<string, unknown> {
	node: DomainNode;
	selected: boolean;
	sectionSelected?: boolean;
	conflicted: boolean;
	onSelect: (id?: string, additive?: boolean) => void;
	onKeyboardMove: (node: DomainNode, position: CanvasPosition) => void;
	onSetCollapsed: (id: string, collapsed: boolean) => void;
	onReload: () => Promise<unknown> | void;
	layoutAvailable: boolean;
	focusOn?: boolean;
	section?: CanvasSection;
	onRemoveSection?: (id: string) => void;
	onMoveSection?: (id: string, position: CanvasPosition, delta: CanvasPosition) => void;
}

const focusPreferenceKey = 'canvas-focus';

/** Session nodes counted as active by Focus mode: launched and still live. */
function isFocusActiveSession(node: DomainNode): boolean {
	const record = node.session?.record;
	return Boolean(record?.live) && (record?.state === 'starting' || record?.state === 'running');
}

/* Virtual focus layout constants. Plans stack in a left column at x = 0; each
 * plan's sessions sit in a row to its right, expanded so the terminal shows.
 * The session pitch is sized off the same node dimensions used elsewhere on
 * the board (245x116 collapsed, 660x500 expanded); row height is derived per
 * row from the tallest node it contains (see focusNodeSize). */
const FOCUS_GUTTER = 40;
const FOCUS_PLAN_WIDTH = 245;
const FOCUS_SESSION_PITCH = 660 + FOCUS_GUTTER;

function focusNodeSize(node: DomainNode, focusOn: boolean): { width: number; height: number } {
	const expanded = Boolean(node.session) && (focusOn || !node.collapsed);
	return expanded ? { width: 660, height: 500 } : { width: 245, height: 116 };
}

type CanvasFlowNode = XYNode<CanvasNodeData>;

const nodeTypes = { workspace: memo(WorkspaceCanvasNode), plan: memo(PlanCanvasNode), session: memo(SessionCanvasNode), section: memo(CanvasSectionNode) };

export function CanvasBoard({ projection, sections = [], conflicts, selectedId, onSelect, onMoveNode, onSetCollapsed, onArrange, serviceGridColumns = 1, onSaveViewport, onReload, onReloadPosition, onReapplyPosition, onReset, onRemove, onCreateSection, onMoveSection, onRemoveSection, onOpenAISession, aiSessionDisabled = true }: {
	projection: CanvasProjection;
	sections?: CanvasSection[];
	conflicts: string[];
	selectedId?: string;
	onSelect: (id?: string) => void;
	onMoveNode: (id: string, position: CanvasPosition) => void;
	onSetCollapsed: (id: string, collapsed: boolean) => void;
	onArrange: (grouping: 'status' | 'service' | 'service_status' | 'service_grid') => void;
	serviceGridColumns?: number;
	onSaveViewport: (viewport: CanvasViewport) => void;
	onReload: () => Promise<unknown> | void;
	onReloadPosition: (id: string) => void;
	onReapplyPosition: (id: string) => void;
	onReset: () => void;
	onRemove: (node: DomainNode) => void;
	onCreateSection?: (nodeIds: string[]) => void;
	onMoveSection?: (id: string, position: CanvasPosition, delta: CanvasPosition) => void;
	onRemoveSection?: (id: string) => void;
	onOpenAISession?: () => void;
	aiSessionDisabled?: boolean;
}) {
	const [statusFilters, setStatusFilters] = useState<string[]>([]);
	const [serviceFilters, setServiceFilters] = useState<string[]>([]);
	const [nodeTypeFilters, setNodeTypeFilters] = useState<string[]>([]);
	const [focusOn, setFocusOn] = useState(() => readPreference<boolean>(focusPreferenceKey, (value) => typeof value === 'boolean' ? value : undefined, false));
	useEffect(() => { writePreference(focusPreferenceKey, focusOn); }, [focusOn]);
	const [sectionSelection, setSectionSelection] = useState<string[]>([]);
	const [searchQuery, setSearchQuery] = useState('');
	const [openMenu, setOpenMenu] = useState('');
	const [isFullscreen, setIsFullscreen] = useState(false);
	const canvasShellRef = useRef<HTMLDivElement>(null);
	const boxSelectingRef = useRef(false);
	const layoutCapability = projection.nodes.find((node) => node.workspace)?.workspace?.actions['layout.move'];
	const layoutAvailable = layoutCapability?.state === 'available';
	const canvasNodes = useMemo(() => projection.nodes.filter((node) => node.kind !== 'workspace'), [projection.nodes]);
	const focusActiveSet = useMemo(() => {
		const sessionIds = new Set<string>();
		const planIds = new Set<string>();
		for (const node of projection.nodes) {
			if (node.kind !== 'session' || !isFocusActiveSession(node)) continue;
			sessionIds.add(node.id);
			const itemId = node.session?.record.planRef?.itemId;
			if (itemId) planIds.add(`plan:${itemId}`);
		}
		for (const connection of projection.connections) {
			if (connection.kind === 'session_launched_from' && sessionIds.has(connection.target)) planIds.add(connection.source);
		}
		return { sessionIds, planIds };
	}, [projection.connections, projection.nodes]);
	const focusLabel = !focusOn ? 'Focus' : focusActiveSet.sessionIds.size > 0 ? `Focus · ${count(focusActiveSet.planIds.size, 'plan')} · ${count(focusActiveSet.sessionIds.size, 'session')}` : 'Focus · nothing running';
	const statuses = useMemo(() => Array.from(new Set(canvasNodes.flatMap((node) => node.plan ? [node.plan.status] : []))), [canvasNodes]);
	const services = useMemo(() => Array.from(new Set(canvasNodes.flatMap((node) => node.plan?.service ? [node.plan.service] : []))).sort(), [canvasNodes]);
	const normalizedSearchQuery = searchQuery.trim().toLowerCase();
	const visibleDomainNodes = useMemo(() => {
		const baseMatches = (node: DomainNode) => {
		const planMatches = !node.plan || ((statusFilters.length === 0 || statusFilters.includes(node.plan.status)) && (serviceFilters.length === 0 || serviceFilters.includes(node.plan.service ?? '')));
		const typeMatches = node.kind === 'workspace' || nodeTypeFilters.length === 0 || nodeTypeFilters.includes(node.kind);
		return planMatches && typeMatches;
		};
		const directMatches = canvasNodes.filter((node) => baseMatches(node) && (!normalizedSearchQuery || nodeSearchText(node).toLowerCase().includes(normalizedSearchQuery)));
		const filteredMatches = !normalizedSearchQuery ? directMatches : (() => {
			const matchingIDs = new Set(directMatches.map((node) => node.id));
			const linkedSessionIDs = new Set(projection.connections.flatMap((connection) => {
				if (matchingIDs.has(connection.source)) return [connection.target];
				if (matchingIDs.has(connection.target)) return [connection.source];
				return [];
			}).filter((id) => canvasNodes.some((node) => node.id === id && node.kind === 'session')));
			return canvasNodes.filter((node) => baseMatches(node) && (matchingIDs.has(node.id) || linkedSessionIDs.has(node.id)));
		})();
		if (!focusOn) return filteredMatches;
		return filteredMatches.filter((node) => focusActiveSet.sessionIds.has(node.id) || focusActiveSet.planIds.has(node.id));
	}, [canvasNodes, focusActiveSet, focusOn, nodeTypeFilters, normalizedSearchQuery, projection.connections, serviceFilters, statusFilters]);
	const visibleIDs = useMemo(() => new Set(visibleDomainNodes.map((node) => node.id)), [visibleDomainNodes]);
	const hiddenCount = canvasNodes.length - visibleDomainNodes.length;
	/** Positions for the active set while Focus is on: plans in a left column,
	 * each plan's sessions in a row to its right. Virtual only — never written
	 * back to the projection. */
	const focusLayout = useMemo(() => {
		if (!focusOn) return undefined;
		const visibleIdSet = new Set(visibleDomainNodes.map((node) => node.id));
		const sessionsByPlan = new Map<string, DomainNode[]>();
		const planlessSessions: DomainNode[] = [];
		for (const node of visibleDomainNodes) {
			if (node.kind !== 'session') continue;
			const connection = projection.connections.find((candidate) => candidate.kind === 'session_launched_from' && candidate.target === node.id && visibleIdSet.has(candidate.source));
			if (connection) {
				const list = sessionsByPlan.get(connection.source) ?? [];
				list.push(node);
				sessionsByPlan.set(connection.source, list);
			} else {
				planlessSessions.push(node);
			}
		}
		const positions = new Map<string, CanvasPosition>();
		let y = 0;
		for (const plan of visibleDomainNodes.filter((node) => node.kind === 'plan')) {
			positions.set(plan.id, { x: 0, y });
			let x = FOCUS_PLAN_WIDTH + FOCUS_GUTTER;
			let rowHeight = focusNodeSize(plan, focusOn).height;
			for (const session of sessionsByPlan.get(plan.id) ?? []) {
				positions.set(session.id, { x, y });
				x += FOCUS_SESSION_PITCH;
				rowHeight = Math.max(rowHeight, focusNodeSize(session, focusOn).height);
			}
			y += rowHeight + FOCUS_GUTTER;
		}
		for (const session of planlessSessions) {
			positions.set(session.id, { x: 0, y });
			y += focusNodeSize(session, focusOn).height + FOCUS_GUTTER;
		}
		return positions;
	}, [focusOn, projection.connections, visibleDomainNodes]);
	const effectivePosition = useCallback((node: DomainNode) => (focusOn ? focusLayout?.get(node.id) : undefined) ?? node.position, [focusLayout, focusOn]);
	const visibleSections = useMemo(() => {
		const filtered = sections.filter((section) => section.id === 'workspace-group' ? visibleDomainNodes.length > 0 : section.nodeIds.some((id) => visibleIDs.has(id)));
		const filtersActive = Boolean(normalizedSearchQuery || statusFilters.length || serviceFilters.length || nodeTypeFilters.length || focusOn);
		const tightened = !filtersActive ? filtered : filtered.map((section) => {
			if (section.id === 'workspace-group') return section;
			const members = visibleDomainNodes.filter((node) => section.nodeIds.includes(node.id));
			if (members.length === 0) return section;
			const left = Math.min(...members.map((node) => effectivePosition(node).x));
			const top = Math.min(...members.map((node) => effectivePosition(node).y));
			const right = Math.max(...members.map((node) => effectivePosition(node).x + focusNodeSize(node, focusOn).width));
			const bottom = Math.max(...members.map((node) => effectivePosition(node).y + focusNodeSize(node, focusOn).height));
			return { ...section, position: { x: left - 28, y: top - 32 }, width: Math.max(301, right - left + 56), height: bottom - top + 64 };
		});
		const enclosed = [
			...tightened.filter((section) => section.id !== 'workspace-group').map((section) => ({ left: section.position.x, top: section.position.y, right: section.position.x + section.width, bottom: section.position.y + section.height })),
			...visibleDomainNodes.map((node) => ({ left: effectivePosition(node).x, top: effectivePosition(node).y, right: effectivePosition(node).x + focusNodeSize(node, focusOn).width, bottom: effectivePosition(node).y + focusNodeSize(node, focusOn).height }))
		];
		if (enclosed.length === 0) return tightened;
		const left = Math.min(...enclosed.map((bounds) => bounds.left));
		const top = Math.min(...enclosed.map((bounds) => bounds.top));
		const right = Math.max(...enclosed.map((bounds) => bounds.right));
		const bottom = Math.max(...enclosed.map((bounds) => bounds.bottom));
		return tightened.map((section) => section.id === 'workspace-group' ? { ...section, position: { x: left - 52, y: top - 82 }, width: right - left + 104, height: bottom - top + 126 } : section);
	}, [effectivePosition, focusOn, nodeTypeFilters.length, normalizedSearchQuery, sections, serviceFilters.length, statusFilters.length, visibleDomainNodes, visibleIDs]);
	const modelNodes = useMemo(() => [...(focusOn ? [] : visibleSections.map((section): CanvasFlowNode => ({
		id: `section:${section.id}`,
		type: 'section',
		position: section.position,
		draggable: false,
		selectable: false,
		style: { width: section.width, height: section.height, zIndex: 0, pointerEvents: 'none' },
		data: { node: {} as DomainNode, section, selected: false, conflicted: false, onSelect: () => {}, onKeyboardMove: () => {}, onSetCollapsed: () => {}, onReload, layoutAvailable, onRemoveSection, onMoveSection }
	}))), ...visibleDomainNodes.map((node): CanvasFlowNode => {
		const renderNode = focusOn && node.kind === 'session' ? { ...node, collapsed: false } : node;
		return {
			id: node.id,
			type: node.kind,
			position: effectivePosition(node),
			draggable: focusOn ? false : canMove(node, layoutAvailable),
			selectable: true,
			data: { node: renderNode, selected: node.id === selectedId, sectionSelected: sectionSelection.includes(node.id), conflicted: conflicts.includes(node.id), onSelect: (id, additive) => additive && id ? toggleSectionSelection(id) : onSelect(id), onKeyboardMove: (candidate, position) => onMoveNode(candidate.id, position), onSetCollapsed, onReload, layoutAvailable, focusOn }
		};
	})], [conflicts, effectivePosition, focusOn, layoutAvailable, onMoveNode, onMoveSection, onReload, onRemoveSection, onSelect, onSetCollapsed, sectionSelection, selectedId, visibleDomainNodes, visibleSections]);
	const [nodes, setNodes, onNodesChange] = useNodesState<CanvasFlowNode>(modelNodes);
	useEffect(() => setNodes(modelNodes), [modelNodes, setNodes]);
	const focusNodeIdsKey = useMemo(() => nodes.map((node) => node.id).join(','), [nodes]);
	const focusSavedViewportRef = useRef<CanvasViewport | undefined>(undefined);
	const focusPreviousOnRef = useRef(focusOn);
	const edges = useMemo(() => projection.connections.filter((connection) => visibleIDs.has(connection.source) && visibleIDs.has(connection.target)).map((connection): Edge => ({ id: connection.id, source: connection.source, target: connection.target, selectable: false, focusable: false, className: `canvas-edge canvas-edge-${connection.kind}` })), [projection.connections, visibleIDs]);
	const onDragStop: OnNodeDrag<CanvasFlowNode> = (_, node) => {
		if (focusOn) return;
		if (node.type === 'section') {
			const id = node.id.replace(/^section:/, '');
			const section = sections.find((candidate) => candidate.id === id);
			onMoveSection?.(id, node.position, { x: node.position.x - (section?.position.x ?? node.position.x), y: node.position.y - (section?.position.y ?? node.position.y) });
			return;
		}
		onMoveNode(node.id, node.position);
	};
	const onMoveEnd: OnMoveEnd = (_, viewport) => { if (!focusOn) onSaveViewport(viewport); };
	const selected = projection.nodes.find((node) => node.id === selectedId);
	useEffect(() => {
		const syncFullscreen = () => setIsFullscreen(document.fullscreenElement === canvasShellRef.current);
		document.addEventListener('fullscreenchange', syncFullscreen);
		return () => document.removeEventListener('fullscreenchange', syncFullscreen);
	}, []);
	const toggleFullscreen = () => {
		if (document.fullscreenElement === canvasShellRef.current) { void document.exitFullscreen?.(); return; }
		void canvasShellRef.current?.requestFullscreen?.();
	};
	const toggleSectionSelection = (id: string) => setSectionSelection((current) => current.includes(id) ? current.filter((nodeID) => nodeID !== id) : [...current, id]);
	const updateSectionSelection = useCallback(({ nodes: selectedNodes }: { nodes: Array<{ id: string; type?: string }> }) => {
		if (!boxSelectingRef.current) return;
		const next = selectedNodes.filter((node) => node.type !== 'section').map((node) => node.id).sort();
		setSectionSelection((current) => current.length === next.length && current.every((id, index) => id === next[index]) ? current : next);
	}, []);

	return <ReactFlowProvider><div className="canvas-board-shell" ref={canvasShellRef}>
		<div className="canvas-toolbar" aria-label="Canvas tools">
			<div className="canvas-board-toolbar">
				<CanvasSearch query={searchQuery} onQueryChange={setSearchQuery} />
				<button className="primary" type="button" disabled={aiSessionDisabled} onClick={onOpenAISession}><Bot size={15} /> AI session</button>
				<div className="canvas-focus-control"><button className={focusOn ? 'secondary active' : 'secondary'} type="button" aria-pressed={focusOn} aria-describedby={focusOn ? 'canvas-focus-hint' : undefined} onClick={() => setFocusOn((value) => !value)}>{focusLabel}</button>{focusOn && focusActiveSet.sessionIds.size === 0 && <button className="link-button" type="button" onClick={() => setFocusOn(false)}>Turn off</button>}{focusOn && <span className="canvas-focus-hint" id="canvas-focus-hint">Layout is automatic while Focus is on</span>}</div>
				{onCreateSection && <button className="secondary" type="button" disabled={sectionSelection.length < 2} title={sectionSelection.length < 2 ? 'Cmd/Ctrl-click or drag-select at least two nodes to create a section.' : 'Create a named section around the selected nodes.'} onClick={() => { onCreateSection(sectionSelection); setSectionSelection([]); }}>Create section{sectionSelection.length ? ` (${sectionSelection.length})` : ''}</button>}
				{selected && <button className="secondary danger" type="button" onClick={() => onRemove(selected)} disabled={!layoutAvailable} title={!layoutAvailable ? layoutCapability?.message : undefined}><Trash2 size={15} /> Remove from Canvas</button>}
			</div>
			<div className="canvas-facet-row">
				<div className="facet-bar">
					<CanvasFacetMenu title="Status" options={statuses.map((value) => ({ value, label: formatFacet(value) }))} selected={statusFilters} open={openMenu === 'status'} onOpen={() => setOpenMenu(openMenu === 'status' ? '' : 'status')} onClose={() => setOpenMenu('')} onToggle={(value) => setStatusFilters((current) => toggleValue(current, value))} onClear={() => setStatusFilters([])} />
					<CanvasFacetMenu title="Service" options={services.map((value) => ({ value, label: value }))} selected={serviceFilters} open={openMenu === 'service'} onOpen={() => setOpenMenu(openMenu === 'service' ? '' : 'service')} onClose={() => setOpenMenu('')} onToggle={(value) => setServiceFilters((current) => toggleValue(current, value))} onClear={() => setServiceFilters([])} />
					<CanvasArrangeMenu open={openMenu === 'group'} disabled={!layoutAvailable} onOpen={() => setOpenMenu(openMenu === 'group' ? '' : 'group')} onClose={() => setOpenMenu('')} onArrange={onArrange} />
					<CanvasFacetMenu title="Node type" options={[{ value: 'plan', label: 'Plans' }, { value: 'session', label: 'Terminal sessions' }]} selected={nodeTypeFilters} open={openMenu === 'nodeType'} onOpen={() => setOpenMenu(openMenu === 'nodeType' ? '' : 'nodeType')} onClose={() => setOpenMenu('')} onToggle={(value) => setNodeTypeFilters((current) => toggleValue(current, value))} onClear={() => setNodeTypeFilters([])} />
				</div>
				<span className="filter-summary">{visibleDomainNodes.length} of {canvasNodes.length} nodes{hiddenCount > 0 ? ` · ${hiddenCount} hidden` : ''}</span>
			</div>
			<CanvasNodeLegend />
			{selected && conflicts.includes(selected.id) && <span className="canvas-conflict-actions" role="alert"><span>Position conflict</span><button type="button" onClick={() => onReloadPosition(selected.id)}>Reload position</button><button type="button" onClick={() => onReapplyPosition(selected.id)}>Reapply my move</button></span>}
		</div>
		<div className="sr-only" role="status" aria-live="polite">{selected ? `Selected ${nodeSearchText(selected)}` : 'No Canvas node selected'}</div>
		<div className="canvas-board" data-node-count={nodes.length}>
			<ReactFlow key={projection.layout.id} nodes={nodes} edges={edges} nodeTypes={nodeTypes} onNodesChange={onNodesChange} onNodeDragStop={onDragStop} onNodeClick={(event, node) => { if (node.type === 'section' || event.metaKey || event.ctrlKey) return; onSelect(node.id); }} onSelectionStart={() => { boxSelectingRef.current = true; }} onSelectionEnd={() => { requestAnimationFrame(() => { boxSelectingRef.current = false; }); }} onSelectionChange={updateSectionSelection} onPaneClick={() => onSelect(undefined)} onMoveEnd={onMoveEnd} defaultViewport={projection.layout.viewport} minZoom={0.1} maxZoom={2} onlyRenderVisibleElements nodesDraggable elementsSelectable selectionOnDrag edgesFocusable={false} fitView={false}>
				<Background color="var(--line)" gap={24} />
				<CanvasSearchViewport query={normalizedSearchQuery} />
				<CanvasFocusViewport focusOn={focusOn} nodeIdsKey={focusNodeIdsKey} savedViewportRef={focusSavedViewportRef} previousFocusOnRef={focusPreviousOnRef} />
				<CanvasGraphControls layoutAvailable={layoutAvailable} layoutMessage={layoutCapability?.message} onArrange={onArrange} serviceGridColumns={serviceGridColumns} onReset={onReset} isFullscreen={isFullscreen} onToggleFullscreen={toggleFullscreen} />
			</ReactFlow>
		</div>
	</div></ReactFlowProvider>;
}

function CanvasSearchViewport({ query }: { query: string }) {
	const { fitView } = useReactFlow();
	const previousQuery = useRef(query);
	useEffect(() => {
		if (previousQuery.current === query) return;
		previousQuery.current = query;
		const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
		const frame = requestAnimationFrame(() => void fitView({ padding: 0.22, maxZoom: 1.15, duration: reduceMotion ? 0 : 180 }));
		return () => cancelAnimationFrame(frame);
	}, [fitView, query]);
	return null;
}

/** Saves the viewport that was current when Focus is switched on and restores
 * it when Focus is switched off; the prior viewport is kept only in a ref
 * hoisted into CanvasBoard, so it survives the `<ReactFlow key={layoutId}>`
 * remount that would otherwise drop a ref living inside it. Focus never
 * writes to the saved layout.
 *
 * Fitting the view is handled separately, keyed on the rendered node list:
 * it re-runs (mount included) whenever that list changes while Focus is on,
 * after a double requestAnimationFrame so the forced-expanded session nodes
 * have been measured before the fit reads their size. */
function CanvasFocusViewport({ focusOn, nodeIdsKey, savedViewportRef, previousFocusOnRef }: { focusOn: boolean; nodeIdsKey: string; savedViewportRef: RefObject<CanvasViewport | undefined>; previousFocusOnRef: RefObject<boolean> }) {
	const { fitView, getViewport, setViewport } = useReactFlow();
	useEffect(() => {
		if (previousFocusOnRef.current === focusOn) return;
		previousFocusOnRef.current = focusOn;
		if (focusOn) {
			savedViewportRef.current = getViewport();
			return;
		}
		if (!savedViewportRef.current) return;
		const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
		void setViewport(savedViewportRef.current, { duration: reduceMotion ? 0 : 200 });
	}, [focusOn, getViewport, previousFocusOnRef, savedViewportRef, setViewport]);
	useEffect(() => {
		if (!focusOn) return;
		const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
		const duration = reduceMotion ? 0 : 200;
		let innerFrame = 0;
		const outerFrame = requestAnimationFrame(() => {
			innerFrame = requestAnimationFrame(() => void fitView({ padding: 0.15, maxZoom: 1.2, duration }));
		});
		return () => { cancelAnimationFrame(outerFrame); cancelAnimationFrame(innerFrame); };
	}, [fitView, focusOn, nodeIdsKey]);
	return null;
}

function CanvasNodeLegend() {
	return <aside className="canvas-node-legend" aria-label="Node type legend"><span className="workspace"><i />Workspace group</span><span className="service"><i />Service group</span><span className="plan"><i />Plan</span><span className="session"><i />Terminal session</span></aside>;
}

function CanvasSectionNode({ data }: NodeProps<CanvasFlowNode>) {
	const section = data.section;
	const { screenToFlowPosition } = useReactFlow();
	if (!section) return null;
	const startMove = (event: React.PointerEvent<HTMLElement>) => {
		if (!data.layoutAvailable || (event.target as HTMLElement).closest('button')) return;
		event.preventDefault();
		event.stopPropagation();
		const start = screenToFlowPosition({ x: event.clientX, y: event.clientY });
		const finish = (upEvent: PointerEvent) => {
			const end = screenToFlowPosition({ x: upEvent.clientX, y: upEvent.clientY });
			const delta = { x: end.x - start.x, y: end.y - start.y };
			if (delta.x || delta.y) data.onMoveSection?.(section.id, { x: section.position.x + delta.x, y: section.position.y + delta.y }, delta);
			window.removeEventListener('pointerup', finish);
		};
		window.addEventListener('pointerup', finish);
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

function CanvasGraphControls({ layoutAvailable, layoutMessage, onArrange, serviceGridColumns, onReset, isFullscreen, onToggleFullscreen }: { layoutAvailable: boolean; layoutMessage?: string; onArrange: (grouping: 'status' | 'service' | 'service_status' | 'service_grid') => void; serviceGridColumns: number; onReset: () => void; isFullscreen: boolean; onToggleFullscreen: () => void }) {
	const { fitView, zoomIn, zoomOut } = useReactFlow();
	const unavailableTitle = !layoutAvailable ? layoutMessage : undefined;
	return <div className="canvas-graph-controls-panel" aria-label="Canvas graph controls">
		<button type="button" className="canvas-graph-control-button" aria-label="Zoom in" title="Zoom in" onClick={() => void zoomIn()}><Plus size={15} /></button>
		<button type="button" className="canvas-graph-control-button" aria-label="Zoom out" title="Zoom out" onClick={() => void zoomOut()}><Minus size={15} /></button>
		<button type="button" className="canvas-graph-control-button" aria-label="Fit Canvas to view" title="Fit Canvas to view" onClick={() => void fitView({ padding: 0.08, maxZoom: 1.35 })}><Maximize size={15} /></button>
		<button type="button" className="canvas-graph-control-button" aria-label={isFullscreen ? 'Exit fullscreen Canvas' : 'Open fullscreen Canvas'} title={isFullscreen ? 'Exit fullscreen' : 'Open fullscreen'} onClick={onToggleFullscreen}>{isFullscreen ? <Minimize2 size={15} /> : <Expand size={15} />}</button>
		<button type="button" className="canvas-graph-control-button" aria-label="Beautify Canvas layout" title="Beautify Canvas layout" disabled={!layoutAvailable} onClick={() => onArrange('status')}><WandSparkles size={15} /></button>
		<button type="button" className="canvas-graph-control-button" aria-label={`Arrange service plans in ${serviceGridColumns === 4 ? 1 : serviceGridColumns + 1} columns`} title={`Cycle service grid: ${serviceGridColumns === 4 ? 1 : serviceGridColumns + 1} columns`} disabled={!layoutAvailable} onClick={() => onArrange('service_grid')}><LayoutGrid size={15} /></button>
		<button type="button" className="canvas-graph-control-button" aria-label="Reset layout" title={unavailableTitle || 'Reset layout'} disabled={!layoutAvailable} onClick={onReset}><RotateCcw size={15} /></button>
	</div>;
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
	// While Focus is on, sessions are force-expanded (see CanvasBoard's renderNode); the
	// disclosure and the terminal's close action would otherwise persist that invented,
	// virtual collapsed state through onSetCollapsed. Hide the disclosure and make close
	// a no-op instead.
	const disclosure = record && !data.focusOn ? <button className="icon-button canvas-node-action nodrag nopan" type="button" aria-label={expanded ? 'Collapse session terminal' : 'Expand session terminal'} title={expanded ? 'Collapse terminal' : 'Expand terminal'} onClick={(event) => { event.stopPropagation(); data.onSelect(data.node.id); data.onSetCollapsed(data.node.id, expanded); }}>{expanded ? <Minimize2 size={14} /> : <Maximize2 size={14} />}</button> : undefined;
	return <NodeFrame data={data} eyebrow="Session" icon={<TerminalSquare size={15} />} action={disclosure} expanded={expanded} resizable={expanded} addon={record ? <CanvasSessionTerminal record={record} visible={expanded} onClose={() => { if (!data.focusOn) data.onSetCollapsed(data.node.id, true); }} onReload={data.onReload} /> : undefined}>
		<strong>{record?.provider || 'Unavailable session'}</strong>
		<span className={`canvas-session-state state-${record?.state || 'stale'}`}><Play size={11} /> {record?.state || stateLabel(data.node)}</span>
		<span>{record?.requestedBranch || data.node.entityRef.branchKey}</span>
	</NodeFrame>;
}

function NodeFrame({ data, eyebrow, icon, children, addon, action, expanded = false, resizable = false }: { data: CanvasNodeData; eyebrow: string; icon: React.ReactNode; children: React.ReactNode; addon?: React.ReactNode; action?: React.ReactNode; expanded?: boolean; resizable?: boolean }) {
	return <div className={`canvas-semantic-node kind-${data.node.kind} state-${data.node.state}${data.selected ? ' selected' : ''}${data.sectionSelected ? ' section-selected' : ''}${data.conflicted ? ' conflicted' : ''}${expanded ? ' expanded' : ''}`}>
		{resizable && <NodeResizer isVisible minWidth={520} minHeight={500} handleClassName="canvas-session-resize-handle" lineClassName="canvas-session-resize-line" />}
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
function count(n: number, noun: string) { return `${n} ${noun}${n === 1 ? '' : 's'}`; }
function formatFacet(value: string) { return value.replaceAll('_', ' ').replace(/^./, (character) => character.toUpperCase()); }
function toggleValue(values: string[], value: string) { return values.includes(value) ? values.filter((candidate) => candidate !== value) : [...values, value]; }
function gitSummary(status?: GitStatus) {
	if (!status) return 'Git status unavailable';
	if (status.conflicted) return `${status.changes.length} changes · conflicted`;
	return status.dirty ? `${status.changes.length} changed files` : 'Clean working tree';
}

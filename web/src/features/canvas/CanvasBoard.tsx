import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Background, ReactFlow, ReactFlowProvider, useNodesState } from '@xyflow/react';
import type { Edge, OnMoveEnd, OnNodeDrag } from '@xyflow/react';
import { Bot, Eye, EyeOff, Pin, Trash2 } from 'lucide-react';
import type { CanvasNode as DomainNode, CanvasPosition, CanvasProjection, CanvasSection, CanvasSize, CanvasViewport } from '../../lib/types';
import { readPreference, writePreference } from '../../shared/preferences/store';
import '@xyflow/react/dist/style.css';
import { type CanvasFlowNode, CanvasSectionNode, WorkspaceCanvasNode, PlanCanvasNode, SessionCanvasNode } from './canvasNodes';
import { CanvasSearch, CanvasFacetMenu, CanvasArrangeMenu, CanvasGraphControls, CanvasNodeLegend } from './CanvasControls';
import { CanvasSearchViewport, CanvasFocusViewport } from './canvasViewports';
import { focusPreferenceKey, isFocusActiveSession, FOCUS_GUTTER, FOCUS_PLAN_WIDTH, focusNodeSize, canMove, nodeSearchText, formatFacet, toggleValue } from './canvasBoardUtils';

const nodeTypes = { workspace: memo(WorkspaceCanvasNode), plan: memo(PlanCanvasNode), session: memo(SessionCanvasNode), section: memo(CanvasSectionNode) };

export function CanvasBoard({ projection, sections = [], conflicts, selectedId, onSelect, onMoveNode, onResizeNode = () => {}, onSetCollapsed, onArrange, serviceGridColumns = 1, onSaveViewport, onReload, onReloadPosition, onReapplyPosition, onReset, onRemove, onCreateSection, onMoveSection, onRemoveSection, onPinSession, onOpenAISession, aiSessionDisabled = true }: {
	projection: CanvasProjection;
	sections?: CanvasSection[];
	conflicts: string[];
	selectedId?: string;
	onSelect: (id?: string) => void;
	onMoveNode: (id: string, position: CanvasPosition) => void;
	onResizeNode?: (id: string, size: CanvasSize) => void;
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
	onPinSession?: (sessionId: string) => void;
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
	// Virtual positions only: they are never written back to the projection.
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
				const size = focusNodeSize(session, focusOn);
				x += size.width + FOCUS_GUTTER;
				rowHeight = Math.max(rowHeight, size.height);
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
		data: { node: {} as DomainNode, section, selected: false, conflicted: false, onSelect: () => {}, onKeyboardMove: () => {}, onResizeNode: () => {}, onSetCollapsed: () => {}, onReload, layoutAvailable, onRemoveSection, onMoveSection }
	}))), ...visibleDomainNodes.map((node): CanvasFlowNode => {
		const renderNode = focusOn && node.kind === 'session' ? { ...node, collapsed: false } : node;
		const expandedSession = renderNode.kind === 'session' && !renderNode.collapsed;
		const size = expandedSession ? focusNodeSize(renderNode, focusOn) : undefined;
		return {
			id: node.id,
			type: node.kind,
			position: effectivePosition(node),
			draggable: focusOn ? false : canMove(node, layoutAvailable),
			selectable: true,
			style: size,
			data: { node: renderNode, selected: node.id === selectedId, sectionSelected: sectionSelection.includes(node.id), conflicted: conflicts.includes(node.id), onSelect: (id, additive) => additive && id ? toggleSectionSelection(id) : onSelect(id), onKeyboardMove: (candidate, position) => onMoveNode(candidate.id, position), onResizeNode, onSetCollapsed, onReload, layoutAvailable, focusOn }
		};
	})], [conflicts, effectivePosition, focusOn, layoutAvailable, onMoveNode, onMoveSection, onReload, onRemoveSection, onResizeNode, onSelect, onSetCollapsed, sectionSelection, selectedId, visibleDomainNodes, visibleSections]);
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
				{(onCreateSection || selected) && <div className={`canvas-board-actions${selected ? ' with-selection' : ''}`} role="group" aria-label="Canvas actions">
					{onCreateSection && <button className="secondary" type="button" disabled={sectionSelection.length < 2} title={sectionSelection.length < 2 ? 'Cmd/Ctrl-click or drag-select at least two nodes to create a section.' : 'Create a named section around the selected nodes.'} onClick={() => { onCreateSection(sectionSelection); setSectionSelection([]); }}>Create section{sectionSelection.length ? ` (${sectionSelection.length})` : ''}</button>}
					{selected?.ephemeral && selected.entityRef.sessionId && <button className="secondary" type="button" onClick={() => onPinSession?.(selected.entityRef.sessionId!)} disabled={!layoutAvailable || !onPinSession} title={!layoutAvailable ? layoutCapability?.message : 'Keep this session on the Canvas after it stops'}><Pin size={15} /> Pin to Canvas</button>}
					{selected && !selected.ephemeral && <button className="secondary danger" type="button" onClick={() => onRemove(selected)} disabled={!layoutAvailable} title={!layoutAvailable ? layoutCapability?.message : undefined}><Trash2 size={15} /> Remove from Canvas</button>}
				</div>}
			</div>
			<div className="canvas-facet-row">
				<div className="facet-bar">
					<CanvasFacetMenu title="Status" options={statuses.map((value) => ({ value, label: formatFacet(value) }))} selected={statusFilters} open={openMenu === 'status'} onOpen={() => setOpenMenu(openMenu === 'status' ? '' : 'status')} onClose={() => setOpenMenu('')} onToggle={(value) => setStatusFilters((current) => toggleValue(current, value))} onClear={() => setStatusFilters([])} />
					<CanvasFacetMenu title="Service" options={services.map((value) => ({ value, label: value }))} selected={serviceFilters} open={openMenu === 'service'} onOpen={() => setOpenMenu(openMenu === 'service' ? '' : 'service')} onClose={() => setOpenMenu('')} onToggle={(value) => setServiceFilters((current) => toggleValue(current, value))} onClear={() => setServiceFilters([])} />
					<CanvasArrangeMenu open={openMenu === 'group'} disabled={!layoutAvailable} onOpen={() => setOpenMenu(openMenu === 'group' ? '' : 'group')} onClose={() => setOpenMenu('')} onArrange={onArrange} />
					<CanvasFacetMenu title="Node type" options={[{ value: 'plan', label: 'Plans' }, { value: 'session', label: 'Terminal sessions' }]} selected={nodeTypeFilters} open={openMenu === 'nodeType'} onOpen={() => setOpenMenu(openMenu === 'nodeType' ? '' : 'nodeType')} onClose={() => setOpenMenu('')} onToggle={(value) => setNodeTypeFilters((current) => toggleValue(current, value))} onClear={() => setNodeTypeFilters([])} />
					<button className={focusOn ? 'secondary canvas-focus-toggle active' : 'secondary canvas-focus-toggle'} type="button" aria-pressed={focusOn} onClick={() => setFocusOn((value) => !value)}>{focusOn ? <Eye size={15} /> : <EyeOff size={15} />} Focus</button>
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

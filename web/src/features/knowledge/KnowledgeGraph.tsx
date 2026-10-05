import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { BookOpen, CircleDot, Expand, GitBranch, LayoutGrid, Maximize, Minimize2, Minus, Network, Plus, RotateCcw, WandSparkles, X } from 'lucide-react';
import { Background, Handle, Position, ReactFlow, useReactFlow } from '@xyflow/react';
import type { Node, NodeProps, OnNodeDrag } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import type { KnowledgeGraph as GraphData, KnowledgeGraphNode as GraphNodeData, KnowledgePage, KnowledgePageDetail } from '../../lib/types';
import { adaptKnowledgeGraph, nodeBucket, type KnowledgeHierarchyRole } from './graphModel';
import { KnowledgeReader } from './KnowledgeReader';

const nodeTypes = { knowledge: KnowledgeNode };

interface KnowledgeNodeData {
	label: string;
	node: GraphNodeData;
	hierarchyRole: KnowledgeHierarchyRole;
	isDomain: boolean;
	groupLabel?: string;
	otherBucket?: boolean;
	onSelect: (slug: string) => void;
	onOpenPanel: (slug: string) => void;
}

const markedGraphTiers: Record<string, string> = { concepts: 'tier-concepts', reference: 'tier-reference' };

function graphTierClass(tier: string): string {
	const marker = markedGraphTiers[tier.toLowerCase()];
	return marker ? `knowledge-flow-node-tier ${marker}` : 'knowledge-flow-node-tier';
}

function KnowledgeNode({ data }: NodeProps) {
	const nodeData = data as unknown as KnowledgeNodeData;
	const node = nodeData.node;
	const roleLabel = nodeData.hierarchyRole === 'domain' ? 'Domain' : nodeData.hierarchyRole === 'root' ? 'Root' : nodeData.hierarchyRole === 'parent' ? 'Parent' : nodeData.hierarchyRole === 'leaf' ? 'Leaf' : 'Related';
	const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
		if (nodeData.isDomain || (event.key !== 'Enter' && event.key !== ' ')) return;
		event.preventDefault();
		nodeData.onSelect(node.id);
	};
	return <div className="knowledge-flow-node" tabIndex={0}>
		<Handle type="target" position={Position.Top} />
		<div className="knowledge-flow-node-card" role={nodeData.isDomain ? undefined : 'button'} tabIndex={nodeData.isDomain ? -1 : 0} onClick={(event) => { event.stopPropagation(); if (!nodeData.isDomain) nodeData.onSelect(node.id); }} onDoubleClick={(event) => { event.stopPropagation(); if (!nodeData.isDomain) nodeData.onOpenPanel(node.id); }} onKeyDown={onKeyDown}>
			<div className="knowledge-flow-node-heading"><strong>{node.title}</strong><span className="knowledge-flow-node-role">{roleLabel}</span></div>
			<span>{node.pageType || 'PAGE'}{node.tier ? <span className={graphTierClass(node.tier)}>{node.tier}</span> : null}</span>
			{nodeData.groupLabel && <span className={nodeData.otherBucket ? 'knowledge-flow-node-group other-bucket' : 'knowledge-flow-node-group'} title={nodeData.otherBucket ? 'In another bucket than the focused page' : undefined}>{nodeData.groupLabel}</span>}
		</div>
		<Handle type="source" position={Position.Bottom} />
	</div>;
}

function KnowledgeGraphControls({ canResetLayout, onResetLayout, onBeautify, onAutoArrange, isFullscreen, onToggleFullscreen }: { canResetLayout: boolean; onResetLayout: () => void; onBeautify: () => void; onAutoArrange: () => void; isFullscreen: boolean; onToggleFullscreen: () => void }) {
	const { fitView, zoomIn, zoomOut } = useReactFlow();
	return <div className="knowledge-graph-controls-panel">
		<button type="button" className="knowledge-graph-control-button" aria-label="Zoom in" title="Zoom in" onClick={() => void zoomIn()}>
			<Plus size={15} />
		</button>
		<button type="button" className="knowledge-graph-control-button" aria-label="Zoom out" title="Zoom out" onClick={() => void zoomOut()}>
			<Minus size={15} />
		</button>
		<button type="button" className="knowledge-graph-control-button" aria-label="Fit graph to view" title="Fit graph to view" onClick={() => void fitView({ padding: 0.08, maxZoom: 1.35 })}>
			<Maximize size={15} />
		</button>
		<button type="button" className="knowledge-graph-control-button" aria-label={isFullscreen ? 'Exit fullscreen graph' : 'Open fullscreen graph'} title={isFullscreen ? 'Exit fullscreen' : 'Open fullscreen'} onClick={onToggleFullscreen}>
			{isFullscreen ? <Minimize2 size={15} /> : <Expand size={15} />}
		</button>
		<button type="button" className="knowledge-graph-control-button" aria-label="Beautify graph layout" title="Beautify graph layout" onClick={onBeautify}>
			<WandSparkles size={15} />
		</button>
		<button type="button" className="knowledge-graph-control-button" aria-label="Auto arrange graph" title="Auto arrange graph" onClick={onAutoArrange}>
			<LayoutGrid size={15} />
		</button>
		<button type="button" className="knowledge-graph-control-button" aria-label="Reset graph layout" title="Reset graph layout" disabled={!canResetLayout} onClick={onResetLayout}>
			<RotateCcw size={15} />
		</button>
	</div>;
}

function FocusedRelationshipViewport({ active, nodeCount, revision }: { active: boolean; nodeCount: number; revision: number }) {
	const { fitView } = useReactFlow();
	useEffect(() => {
		if (active || revision > 0) void fitView({ padding: 0.28, maxZoom: 1.25, duration: 240 });
	}, [active, fitView, nodeCount, revision]);
	return null;
}

function autoArrangeNodes(nodes: Node[], edges: Array<{ source: string; target: string }>): Record<string, { x: number; y: number }> {
	const ids = new Set(nodes.map((node) => node.id));
	const incoming = new Map(nodes.map((node) => [node.id, 0]));
	const outgoing = new Map(nodes.map((node) => [node.id, [] as string[]]));
	for (const edge of edges) {
		if (!ids.has(edge.source) || !ids.has(edge.target) || edge.source === edge.target) continue;
		incoming.set(edge.target, (incoming.get(edge.target) ?? 0) + 1);
		outgoing.get(edge.source)?.push(edge.target);
	}
	const title = (id: string) => String(nodes.find((node) => node.id === id)?.data.label ?? id);
	const queue = [...incoming].filter(([, count]) => count === 0).map(([id]) => id).sort((left, right) => title(left).localeCompare(title(right)));
	const layer = new Map<string, number>(queue.map((id) => [id, 0]));
	while (queue.length) {
		const source = queue.shift()!;
		for (const target of outgoing.get(source) ?? []) {
			layer.set(target, Math.max(layer.get(target) ?? 0, (layer.get(source) ?? 0) + 1));
			const remaining = (incoming.get(target) ?? 1) - 1;
			incoming.set(target, remaining);
			if (remaining === 0) queue.push(target);
		}
		queue.sort((left, right) => title(left).localeCompare(title(right)));
	}
	for (const node of nodes) if (!layer.has(node.id)) layer.set(node.id, 0);
	const byLayer = new Map<number, Node[]>();
	for (const node of nodes) {
		const current = byLayer.get(layer.get(node.id) ?? 0) ?? [];
		current.push(node);
		byLayer.set(layer.get(node.id) ?? 0, current);
	}
	const positions: Record<string, { x: number; y: number }> = {};
	let y = 0;
	for (const level of [...byLayer.keys()].sort((left, right) => left - right)) {
		const entries = byLayer.get(level)!.sort((left, right) => title(left.id).localeCompare(title(right.id)));
		const columns = Math.min(4, Math.max(1, Math.ceil(Math.sqrt(entries.length))));
		for (let index = 0; index < entries.length; index++) positions[entries[index].id] = { x: (index % columns) * 300, y: y + Math.floor(index / columns) * 160 };
		y += Math.ceil(entries.length / columns) * 160 + 120;
	}
	return positions;
}

export function KnowledgeGraph({ graph, pages, selectedSlug, selectedDetail, onSelect, onOpenDetails }: { graph: GraphData; pages: KnowledgePage[]; selectedSlug?: string; selectedDetail?: KnowledgePageDetail | null; onSelect: (slug: string) => void; onOpenDetails: (slug: string) => void }) {
	const [query, setQuery] = useState('');
	const [bucket, setBucket] = useState('');
	const [tier, setTier] = useState('');
	const [focusRelationships, setFocusRelationships] = useState(false);
	const [showConnections, setShowConnections] = useState(false);
	const [isPanelOpen, setIsPanelOpen] = useState(false);
	const [isSelectionDismissed, setIsSelectionDismissed] = useState(false);
	const [isFullscreen, setIsFullscreen] = useState(false);
	const [layoutRevision, setLayoutRevision] = useState(0);
	const [overviewPositions, setOverviewPositions] = useState<Record<string, { x: number; y: number }>>({});
	// Positions dragged in focus mode belong to that focus only: an overview
	// position would drop a neighbour back into its far-off section.
	const [focusPositions, setFocusPositions] = useState<{ slug?: string; positions: Record<string, { x: number; y: number }> }>({ positions: {} });
	const focusSlug = focusRelationships && selectedSlug ? selectedSlug : undefined;
	const nodePositions = focusSlug ? (focusPositions.slug === focusSlug ? focusPositions.positions : {}) : overviewPositions;
	const setNodePositions = (update: Record<string, { x: number; y: number }> | ((current: Record<string, { x: number; y: number }>) => Record<string, { x: number; y: number }>)) => {
		if (focusSlug) setFocusPositions((current) => ({ slug: focusSlug, positions: typeof update === 'function' ? update(current.slug === focusSlug ? current.positions : {}) : update }));
		else setOverviewPositions(update);
	};
	const graphViewRef = useRef<HTMLDivElement>(null);
	const hasCustomLayout = Object.keys(nodePositions).length > 0;
	useEffect(() => {
		if (!isPanelOpen) return;
		const onKeyDown = (event: globalThis.KeyboardEvent) => {
			if (event.key === 'Escape') setIsPanelOpen(false);
		};
		window.addEventListener('keydown', onKeyDown);
		return () => window.removeEventListener('keydown', onKeyDown);
	}, [isPanelOpen]);
	useEffect(() => {
		const onFullscreenChange = () => setIsFullscreen(document.fullscreenElement === graphViewRef.current);
		document.addEventListener('fullscreenchange', onFullscreenChange);
		return () => document.removeEventListener('fullscreenchange', onFullscreenChange);
	}, []);
	// A click shows a page's links; a second click on the same page hides them
	// again. A double click opens the review panel, and leaves the links shown
	// even though its two clicks toggled them off and on.
	const handleSelect = (slug: string) => {
		setIsPanelOpen(false);
		if (slug === selectedSlug && showConnections && !isSelectionDismissed) {
			setShowConnections(false);
			setFocusRelationships(false);
			setIsSelectionDismissed(true);
			return;
		}
		setIsSelectionDismissed(false);
		setShowConnections(true);
		onSelect(slug);
	};
	const openPanel = (slug: string) => {
		setIsSelectionDismissed(false);
		setShowConnections(true);
		setIsPanelOpen(true);
		onSelect(slug);
	};
	const toggleFullscreen = () => {
		if (document.fullscreenElement) { void document.exitFullscreen(); return; }
		void graphViewRef.current?.requestFullscreen();
	};
	const beautifyLayout = () => {
		setNodePositions({});
		setLayoutRevision((revision) => revision + 1);
	};
	const autoArrangeLayout = () => {
		// The focus layout is already arranged around the page; restore it.
		setNodePositions(focusSlug ? {} : autoArrangeNodes(model.nodes, model.edges));
		setLayoutRevision((revision) => revision + 1);
	};
	const filtered = useMemo(() => {
		if (focusRelationships && selectedSlug) {
			const related = new Set([selectedSlug]);
			for (const edge of graph.edges) {
				if (edge.source === selectedSlug) related.add(edge.target);
				if (edge.target === selectedSlug) related.add(edge.source);
			}
			return { ...graph, nodes: graph.nodes.filter((node) => related.has(node.id)), edges: graph.edges.filter((edge) => edge.source === selectedSlug || edge.target === selectedSlug) };
		}
		const needle = query.trim().toLowerCase();
		const nodes = graph.nodes.filter((node) => (!needle || node.title.toLowerCase().includes(needle) || node.id.toLowerCase().includes(needle)) && (!bucket || nodeBucket(node) === bucket) && (!tier || (node.tier ?? '') === tier));
		const allowed = new Set(nodes.map((node) => node.id));
		return { ...graph, nodes, edges: graph.edges.filter((edge) => allowed.has(edge.source) && allowed.has(edge.target)) };
	}, [bucket, focusRelationships, graph, query, selectedSlug, tier]);
	const model = useMemo(() => {
		const adapted = adaptKnowledgeGraph(filtered, isSelectionDismissed ? undefined : selectedSlug, focusSlug);
		return {
			...adapted,
			nodes: adapted.nodes.map((node) => ({
				...node,
				position: nodePositions[node.id] ?? node.position,
				data: {
					...(node.data as unknown as KnowledgeNodeData),
					onSelect: handleSelect,
					onOpenPanel: openPanel
				}
			}))
		};
	}, [filtered, focusSlug, isSelectionDismissed, nodePositions, selectedSlug, showConnections]);
	const handleNodeDragStop: OnNodeDrag<Node> = (_, node) => {
		setNodePositions((current) => {
			const currentPosition = current[node.id];
			if (currentPosition?.x === node.position.x && currentPosition.y === node.position.y) return current;
			return { ...current, [node.id]: node.position };
		});
	};
	const buckets = Array.from(new Set(graph.nodes.map(nodeBucket).filter(Boolean))).sort();
	const tiers = Array.from(new Set(graph.nodes.map((node) => node.tier).filter(Boolean))).sort();
	const selectedPage = pages.find((page) => page.slug === selectedSlug);
	const selectedGraphNode = graph.nodes.find((node) => node.id === selectedSlug);
	const visibleEdges = useMemo(() => {
		const domainLinks = model.edges.filter((edge) => (edge.data as { isDomainLink?: boolean } | undefined)?.isDomainLink || edge.source.startsWith('__knowledge_domain__:'));
		if (!showConnections || !selectedSlug) return domainLinks;
		return model.edges.filter((edge) => edge.source.startsWith('__knowledge_domain__:') || edge.source === selectedSlug || edge.target === selectedSlug);
	}, [model.edges, selectedSlug, showConnections]);
	return <div className="knowledge-graph-view" ref={graphViewRef}>
		<div className="knowledge-graph-filters"><input aria-label="Search graph pages" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search graph" disabled={focusRelationships} /><select aria-label="Filter graph bucket" value={bucket} onChange={(event) => setBucket(event.target.value)} disabled={focusRelationships}><option value="">All buckets</option>{buckets.map((value) => <option key={value}>{value}</option>)}</select><select aria-label="Filter graph tier" value={tier} onChange={(event) => setTier(event.target.value)} disabled={focusRelationships}><option value="">All tiers</option>{tiers.map((value) => <option key={value}>{value}</option>)}</select><button className={focusRelationships ? 'active' : ''} type="button" aria-pressed={focusRelationships} disabled={!selectedSlug} onClick={() => setFocusRelationships((current) => { const next = !current; if (next) { setShowConnections(true); setIsSelectionDismissed(false); } return next; })}>{focusRelationships ? 'Show all components' : 'Focus relationships'}</button></div>
		<div className="knowledge-graph-legend" aria-label="Graph hierarchy legend"><span className="knowledge-legend-domain"><GitBranch size={14} /> Domain parent</span><span className="knowledge-legend-root"><CircleDot size={14} /> Root</span><span className="knowledge-legend-leaf">Leaf</span><span className="knowledge-legend-line"><i /> Domain grouping</span><span className="knowledge-legend-selected"><i /> Selected relationships</span></div>
		{graph.truncated && <p className="knowledge-graph-notice" role="status">Showing {graph.nodes.length} of {graph.totalNodes} pages and {graph.edges.length} of {graph.totalEdges} relationships. Filter by bucket or tier to narrow the graph.</p>}
		<div className="knowledge-graph-layout">
			<div className="knowledge-graph-stage">
				<div className="knowledge-graph-canvas"><ReactFlow nodes={model.nodes} edges={visibleEdges} nodeTypes={nodeTypes} fitView fitViewOptions={{ padding: 0.08, maxZoom: 1.35 }} minZoom={0.2} maxZoom={2} onNodeClick={(_, node) => { if (!(node.data as KnowledgeNodeData).isDomain) handleSelect(node.id); }} onNodeDoubleClick={(_, node) => { if (!(node.data as KnowledgeNodeData).isDomain) openPanel(node.id); }} onNodeDragStop={handleNodeDragStop} nodesDraggable onlyRenderVisibleElements edgesFocusable={false}><Background color="var(--line)" gap={20} /><FocusedRelationshipViewport active={focusRelationships} nodeCount={model.nodes.length} revision={layoutRevision} /><KnowledgeGraphControls canResetLayout={hasCustomLayout} onResetLayout={() => setNodePositions({})} onBeautify={beautifyLayout} onAutoArrange={autoArrangeLayout} isFullscreen={isFullscreen} onToggleFullscreen={toggleFullscreen} /></ReactFlow></div>
				{isPanelOpen && selectedGraphNode && <aside className="knowledge-graph-panel" aria-label="Knowledge graph review panel">
					<div className="knowledge-graph-panel-header">
						<p className="knowledge-graph-panel-eyebrow"><Network size={14} /> {selectedGraphNode.domain || 'root'} · {selectedGraphNode.pageType || 'PAGE'}</p>
						<div className="knowledge-graph-panel-header-actions">
							<button type="button" className="secondary knowledge-graph-open-details" onClick={() => onOpenDetails(selectedGraphNode.id)}><BookOpen size={15} /> Open details page</button>
							<button type="button" className="knowledge-graph-panel-close" aria-label="Close review panel" onClick={() => setIsPanelOpen(false)}><X size={16} /></button>
						</div>
					</div>
					{selectedDetail && selectedDetail.slug === selectedGraphNode.id
						? <KnowledgeReader detail={selectedDetail} onNavigate={openPanel} />
						: <div className="knowledge-graph-panel-loading"><h2>{selectedGraphNode.title}</h2><p>{selectedPage?.summary || 'Loading page details…'}</p></div>}
				</aside>}
			</div>
		</div>
	</div>;
}

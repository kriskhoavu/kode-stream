import { useEffect, useMemo, useState, type KeyboardEvent, type MouseEvent } from 'react';
import { BookOpen, Maximize, Minus, Network, Plus, X } from 'lucide-react';
import { Background, Handle, Position, ReactFlow, useReactFlow } from '@xyflow/react';
import type { NodeProps } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import type { KnowledgeGraph as GraphData, KnowledgeGraphNode as GraphNodeData, KnowledgePage, KnowledgePageDetail } from '../../lib/types';
import { adaptKnowledgeGraph } from './graphModel';
import { KnowledgeReader } from './KnowledgeReader';

const nodeTypes = { knowledge: KnowledgeNode };

interface KnowledgeNodeData {
	label: string;
	node: GraphNodeData;
	onSelect: (slug: string) => void;
	onOpenDetails: (slug: string) => void;
}

function KnowledgeNode({ data }: NodeProps) {
	const nodeData = data as unknown as KnowledgeNodeData;
	const node = nodeData.node;
	const stop = (event: MouseEvent<HTMLElement>) => event.stopPropagation();
	const openDetails = (event: MouseEvent<HTMLButtonElement>) => {
		event.stopPropagation();
		nodeData.onOpenDetails(node.id);
	};
	const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		nodeData.onSelect(node.id);
	};
	return <div className="knowledge-flow-node" tabIndex={0}>
		<Handle type="target" position={Position.Left} />
		<div className="knowledge-flow-node-card" role="button" tabIndex={0} onClick={() => nodeData.onSelect(node.id)} onKeyDown={onKeyDown}>
			<strong>{node.title}</strong>
			<span>{node.pageType || 'PAGE'} · {node.domain}</span>
		</div>
		<div className="knowledge-node-tooltip" role="tooltip">
			<strong>{node.title}</strong>
			<dl><dt>Path</dt><dd>{node.path}</dd><dt>Domain</dt><dd>{node.domain}</dd><dt>Type</dt><dd>{node.pageType || 'Page'}</dd><dt>Links</dt><dd>{node.inbound} in · {node.outbound} out</dd>{node.roles.length > 0 && <><dt>Roles</dt><dd>{node.roles.join(', ')}</dd></>}{node.topics.length > 0 && <><dt>Topics</dt><dd>{node.topics.join(', ')}</dd></>}</dl>
			<div className="knowledge-node-tooltip-actions nodrag nopan" onMouseDown={stop}>
				<button type="button" className="knowledge-node-tooltip-link" onClick={openDetails}>Open details</button>
			</div>
		</div>
		<Handle type="source" position={Position.Right} />
	</div>;
}

function KnowledgeGraphControls() {
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
	</div>;
}

export function KnowledgeGraph({ graph, pages, selectedSlug, selectedDetail, onSelect, onOpenDetails }: { graph: GraphData; pages: KnowledgePage[]; selectedSlug?: string; selectedDetail?: KnowledgePageDetail | null; onSelect: (slug: string) => void; onOpenDetails: (slug: string) => void }) {
	const [query, setQuery] = useState('');
	const [domain, setDomain] = useState('');
	const [pageType, setPageType] = useState('');
	const [focusRelationships, setFocusRelationships] = useState(false);
	const [isPanelOpen, setIsPanelOpen] = useState(false);
	useEffect(() => {
		if (!isPanelOpen) return;
		const onKeyDown = (event: globalThis.KeyboardEvent) => {
			if (event.key === 'Escape') setIsPanelOpen(false);
		};
		window.addEventListener('keydown', onKeyDown);
		return () => window.removeEventListener('keydown', onKeyDown);
	}, [isPanelOpen]);
	const handleSelect = (slug: string) => {
		setIsPanelOpen(true);
		onSelect(slug);
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
		const nodes = graph.nodes.filter((node) => (!needle || node.title.toLowerCase().includes(needle) || node.id.toLowerCase().includes(needle)) && (!domain || node.domain === domain) && (!pageType || node.pageType === pageType));
		const allowed = new Set(nodes.map((node) => node.id));
		return { ...graph, nodes, edges: graph.edges.filter((edge) => allowed.has(edge.source) && allowed.has(edge.target)) };
	}, [domain, focusRelationships, graph, pageType, query, selectedSlug]);
	const model = useMemo(() => {
		const adapted = adaptKnowledgeGraph(filtered, selectedSlug);
		return {
			...adapted,
			nodes: adapted.nodes.map((node) => ({
				...node,
				data: {
					...(node.data as unknown as KnowledgeNodeData),
					onSelect: handleSelect,
					onOpenDetails
				}
			}))
		};
	}, [filtered, onOpenDetails, selectedSlug]);
	const domains = Array.from(new Set(graph.nodes.map((node) => node.domain))).sort();
	const pageTypes = Array.from(new Set(graph.nodes.map((node) => node.pageType).filter(Boolean))).sort();
	const selectedPage = pages.find((page) => page.slug === selectedSlug);
	const selectedGraphNode = graph.nodes.find((node) => node.id === selectedSlug);
	return <div className="knowledge-graph-view">
		<div className="knowledge-graph-filters"><input aria-label="Search graph pages" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search graph" disabled={focusRelationships} /><select aria-label="Filter graph domain" value={domain} onChange={(event) => setDomain(event.target.value)} disabled={focusRelationships}><option value="">All domains</option>{domains.map((value) => <option key={value}>{value}</option>)}</select><select aria-label="Filter graph page type" value={pageType} onChange={(event) => setPageType(event.target.value)} disabled={focusRelationships}><option value="">All page types</option>{pageTypes.map((value) => <option key={value}>{value}</option>)}</select><button className={focusRelationships ? 'active' : ''} type="button" aria-pressed={focusRelationships} disabled={!selectedSlug} onClick={() => setFocusRelationships((current) => !current)}>{focusRelationships ? 'Show all components' : 'Focus relationships'}</button></div>
		{graph.truncated && <p className="knowledge-graph-notice" role="status">Showing {graph.nodes.length} of {graph.totalNodes} pages and {graph.edges.length} of {graph.totalEdges} relationships. Filter by domain to narrow the graph.</p>}
		<div className="knowledge-graph-layout">
			<div className="knowledge-graph-stage">
				<div className="knowledge-graph-canvas"><ReactFlow nodes={model.nodes} edges={model.edges} nodeTypes={nodeTypes} fitView fitViewOptions={{ padding: 0.08, maxZoom: 1.35 }} minZoom={0.2} maxZoom={2} onNodeClick={(_, node) => handleSelect(node.id)} nodesDraggable><Background color="var(--line)" gap={20} /><KnowledgeGraphControls /></ReactFlow></div>
				{isPanelOpen && selectedGraphNode && <aside className="knowledge-graph-panel" aria-label="Knowledge graph review panel">
					<div className="knowledge-graph-panel-header">
						<p className="knowledge-graph-panel-eyebrow"><Network size={14} /> {selectedGraphNode.domain || 'root'} · {selectedGraphNode.pageType || 'PAGE'}</p>
						<div className="knowledge-graph-panel-header-actions">
							<button type="button" className="secondary" onClick={() => onOpenDetails(selectedGraphNode.id)}><BookOpen size={15} /> Open details page</button>
							<button type="button" className="knowledge-graph-panel-close" aria-label="Close review panel" onClick={() => setIsPanelOpen(false)}><X size={16} /></button>
						</div>
					</div>
					{selectedDetail && selectedDetail.slug === selectedGraphNode.id
						? <KnowledgeReader detail={selectedDetail} onNavigate={handleSelect} />
						: <div className="knowledge-graph-panel-loading"><h2>{selectedGraphNode.title}</h2><p>{selectedPage?.summary || 'Loading page details…'}</p></div>}
				</aside>}
			</div>
		</div>
	</div>;
}

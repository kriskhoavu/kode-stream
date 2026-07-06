import { useMemo, useState } from 'react';
import { Background, Controls, MiniMap, ReactFlow } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import type { KnowledgeGraph as GraphData } from '../../lib/types';
import { adaptKnowledgeGraph } from './graphModel';

export function KnowledgeGraph({ graph, selectedSlug, onSelect }: { graph: GraphData; selectedSlug?: string; onSelect: (slug: string) => void }) {
	const [query, setQuery] = useState('');
	const [domain, setDomain] = useState('');
	const [pageType, setPageType] = useState('');
	const filtered = useMemo(() => {
		const needle = query.trim().toLowerCase();
		const nodes = graph.nodes.filter((node) => (!needle || node.title.toLowerCase().includes(needle) || node.id.toLowerCase().includes(needle)) && (!domain || node.domain === domain) && (!pageType || node.pageType === pageType));
		const allowed = new Set(nodes.map((node) => node.id));
		return { ...graph, nodes, edges: graph.edges.filter((edge) => allowed.has(edge.source) && allowed.has(edge.target)) };
	}, [domain, graph, pageType, query]);
	const model = useMemo(() => adaptKnowledgeGraph(filtered, selectedSlug), [filtered, selectedSlug]);
	const domains = Array.from(new Set(graph.nodes.map((node) => node.domain))).sort();
	const pageTypes = Array.from(new Set(graph.nodes.map((node) => node.pageType).filter(Boolean))).sort();
	return <div className="knowledge-graph-view">
		<div className="knowledge-graph-filters"><input aria-label="Search graph pages" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search graph" /><select aria-label="Filter graph domain" value={domain} onChange={(event) => setDomain(event.target.value)}><option value="">All domains</option>{domains.map((value) => <option key={value}>{value}</option>)}</select><select aria-label="Filter graph page type" value={pageType} onChange={(event) => setPageType(event.target.value)}><option value="">All page types</option>{pageTypes.map((value) => <option key={value}>{value}</option>)}</select></div>
		{graph.truncated && <p className="knowledge-graph-notice" role="status">Showing {graph.nodes.length} of {graph.totalNodes} pages and {graph.edges.length} of {graph.totalEdges} relationships. Filter by domain to narrow the graph.</p>}
		<div className="knowledge-graph-canvas"><ReactFlow nodes={model.nodes} edges={model.edges} fitView minZoom={0.2} maxZoom={2} onNodeClick={(_, node) => onSelect(node.id)} nodesDraggable={false}><Background /><MiniMap pannable zoomable /><Controls showInteractive={false} /></ReactFlow></div>
		<section className="knowledge-relationship-list" aria-label="Knowledge relationships"><h2>Relationships</h2><ul>{filtered.edges.map((edge) => <li key={`${edge.source}-${edge.target}`}><button onClick={() => onSelect(edge.source)}>{edge.source}</button><span> links to </span><button onClick={() => onSelect(edge.target)}>{edge.target}</button></li>)}</ul>{!filtered.edges.length && <p>No relationships match the filters.</p>}</section>
	</div>;
}

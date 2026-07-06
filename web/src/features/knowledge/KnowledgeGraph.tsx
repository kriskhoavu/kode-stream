import { useMemo, useState } from 'react';
import { Background, Controls, Handle, MiniMap, Position, ReactFlow } from '@xyflow/react';
import type { NodeProps } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import type { KnowledgeGraph as GraphData, KnowledgeGraphNode as GraphNodeData } from '../../lib/types';
import { adaptKnowledgeGraph } from './graphModel';

const nodeTypes = { knowledge: KnowledgeNode };

function KnowledgeNode({ data }: NodeProps) {
	const node = data.node as GraphNodeData;
	return <div className="knowledge-flow-node" tabIndex={0}>
		<Handle type="target" position={Position.Left} />
		<strong>{node.title}</strong>
		<span>{node.pageType || 'PAGE'} · {node.domain}</span>
		<div className="knowledge-node-tooltip" role="tooltip">
			<strong>{node.title}</strong>
			<dl><dt>Path</dt><dd>{node.path}</dd><dt>Domain</dt><dd>{node.domain}</dd><dt>Type</dt><dd>{node.pageType || 'Page'}</dd><dt>Links</dt><dd>{node.inbound} in · {node.outbound} out</dd>{node.roles.length > 0 && <><dt>Roles</dt><dd>{node.roles.join(', ')}</dd></>}{node.topics.length > 0 && <><dt>Topics</dt><dd>{node.topics.join(', ')}</dd></>}</dl>
			<small>Click to open this page</small>
		</div>
		<Handle type="source" position={Position.Right} />
	</div>;
}

export function KnowledgeGraph({ graph, selectedSlug, onSelect }: { graph: GraphData; selectedSlug?: string; onSelect: (slug: string) => void }) {
	const [query, setQuery] = useState('');
	const [domain, setDomain] = useState('');
	const [pageType, setPageType] = useState('');
	const [focusRelationships, setFocusRelationships] = useState(false);
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
	const model = useMemo(() => adaptKnowledgeGraph(filtered, selectedSlug), [filtered, selectedSlug]);
	const domains = Array.from(new Set(graph.nodes.map((node) => node.domain))).sort();
	const pageTypes = Array.from(new Set(graph.nodes.map((node) => node.pageType).filter(Boolean))).sort();
	return <div className="knowledge-graph-view">
		<div className="knowledge-graph-filters"><input aria-label="Search graph pages" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search graph" disabled={focusRelationships} /><select aria-label="Filter graph domain" value={domain} onChange={(event) => setDomain(event.target.value)} disabled={focusRelationships}><option value="">All domains</option>{domains.map((value) => <option key={value}>{value}</option>)}</select><select aria-label="Filter graph page type" value={pageType} onChange={(event) => setPageType(event.target.value)} disabled={focusRelationships}><option value="">All page types</option>{pageTypes.map((value) => <option key={value}>{value}</option>)}</select><button className={focusRelationships ? 'active' : ''} type="button" aria-pressed={focusRelationships} disabled={!selectedSlug} onClick={() => setFocusRelationships((current) => !current)}>{focusRelationships ? 'Show all components' : 'Focus relationships'}</button></div>
		{graph.truncated && <p className="knowledge-graph-notice" role="status">Showing {graph.nodes.length} of {graph.totalNodes} pages and {graph.edges.length} of {graph.totalEdges} relationships. Filter by domain to narrow the graph.</p>}
		<div className="knowledge-graph-canvas"><ReactFlow nodes={model.nodes} edges={model.edges} nodeTypes={nodeTypes} fitView fitViewOptions={{ padding: 0.08, maxZoom: 1.35 }} minZoom={0.2} maxZoom={2} onNodeClick={(_, node) => onSelect(node.id)} nodesDraggable={false}><Background color="var(--line)" gap={20} /><MiniMap pannable zoomable className="knowledge-graph-minimap" /><Controls className="knowledge-graph-controls" showInteractive={false} /></ReactFlow></div>
	</div>;
}

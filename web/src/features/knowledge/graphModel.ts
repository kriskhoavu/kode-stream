import type { Edge, Node } from '@xyflow/react';
import type { KnowledgeGraph } from '../../lib/types';

export function adaptKnowledgeGraph(graph: KnowledgeGraph, selectedSlug?: string): { nodes: Node[]; edges: Edge[]; neighbors: Set<string> } {
	const neighbors = new Set<string>();
	for (const edge of graph.edges) {
		if (edge.source === selectedSlug) neighbors.add(edge.target);
		if (edge.target === selectedSlug) neighbors.add(edge.source);
	}
	const domainRows = new Map<string, number>();
	const domains = Array.from(new Set(graph.nodes.map((candidate) => candidate.domain))).sort();
	const nodes = graph.nodes.map((node) => {
		const row = domainRows.get(node.domain) ?? 0;
		domainRows.set(node.domain, row + 1);
		const selected = node.id === selectedSlug, neighbor = neighbors.has(node.id);
		return { id: node.id, position: { x: domains.indexOf(node.domain) * 260, y: row * 110 }, data: { label: node.title }, ariaLabel: `${node.title}, ${node.domain}, ${node.inbound} incoming and ${node.outbound} outgoing links`, className: selected ? 'selected' : neighbor ? 'neighbor' : undefined, style: { width: Math.min(220, 150 + node.inbound * 6) } } satisfies Node;
	});
	const edges = graph.edges.map((edge) => ({ id: `${edge.source}->${edge.target}`, source: edge.source, target: edge.target, animated: edge.source === selectedSlug || edge.target === selectedSlug, className: edge.source === selectedSlug || edge.target === selectedSlug ? 'neighbor' : undefined } satisfies Edge));
	return { nodes, edges, neighbors };
}

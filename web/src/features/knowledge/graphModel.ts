import type { Edge, MarkerType, Node } from '@xyflow/react';
import type { KnowledgeGraph } from '../../lib/types';

export function adaptKnowledgeGraph(graph: KnowledgeGraph, selectedSlug?: string): { nodes: Node[]; edges: Edge[]; neighbors: Set<string> } {
	const neighbors = new Set<string>();
	for (const edge of graph.edges) {
		if (edge.source === selectedSlug) neighbors.add(edge.target);
		if (edge.target === selectedSlug) neighbors.add(edge.source);
	}
	const orderedIds = relationshipOrder(graph);
	const order = new Map(orderedIds.map((id, index) => [id, index]));
	// The graph viewport is roughly twice as wide as it is tall. Eight columns for
	// a 50-node graph uses that space more efficiently than the previous wide grid.
	const columns = Math.max(2, Math.ceil(Math.sqrt(graph.nodes.length)));
	const nodes = graph.nodes.map((node) => {
		const index = order.get(node.id) ?? 0;
		const selected = node.id === selectedSlug, neighbor = neighbors.has(node.id);
		return { id: node.id, type: 'knowledge', position: { x: (index % columns) * 280, y: Math.floor(index / columns) * 150 }, data: { label: node.title, node }, ariaLabel: `${node.title}, ${node.domain}, ${node.inbound} incoming and ${node.outbound} outgoing links`, className: selected ? 'selected' : neighbor ? 'neighbor' : undefined } satisfies Node;
	});
	const edges = graph.edges.map((edge) => ({ id: `${edge.source}->${edge.target}`, source: edge.source, target: edge.target, animated: edge.source === selectedSlug || edge.target === selectedSlug, className: edge.source === selectedSlug || edge.target === selectedSlug ? 'neighbor' : undefined, markerEnd: { type: 'arrowclosed' as MarkerType } } satisfies Edge));
	return { nodes, edges, neighbors };
}

function relationshipOrder(graph: KnowledgeGraph): string[] {
	const nodeIds = new Set(graph.nodes.map((node) => node.id));
	const incoming = new Map(graph.nodes.map((node) => [node.id, 0]));
	const outgoing = new Map(graph.nodes.map((node) => [node.id, [] as string[]]));
	for (const edge of graph.edges) {
		if (!nodeIds.has(edge.source) || !nodeIds.has(edge.target)) continue;
		incoming.set(edge.target, (incoming.get(edge.target) ?? 0) + 1);
		outgoing.get(edge.source)?.push(edge.target);
	}
	const byTitle = (left: string, right: string) => {
		const leftTitle = graph.nodes.find((node) => node.id === left)?.title ?? left;
		const rightTitle = graph.nodes.find((node) => node.id === right)?.title ?? right;
		return leftTitle.localeCompare(rightTitle);
	};
	const queue = [...incoming].filter(([, count]) => count === 0).map(([id]) => id).sort(byTitle);
	const result: string[] = [];
	while (queue.length) {
		const id = queue.shift()!;
		result.push(id);
		for (const target of (outgoing.get(id) ?? []).sort(byTitle)) {
			const count = (incoming.get(target) ?? 1) - 1;
			incoming.set(target, count);
			if (count === 0) queue.push(target);
		}
		queue.sort(byTitle);
	}
	return result.concat(graph.nodes.map((node) => node.id).filter((id) => !result.includes(id)).sort(byTitle));
}

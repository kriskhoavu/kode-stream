import type { Edge, MarkerType, Node } from '@xyflow/react';
import type { KnowledgeGraph, KnowledgeGraphNode } from '../../lib/types';

export type KnowledgeHierarchyRole = 'domain' | 'root' | 'parent' | 'leaf' | 'related';

const domainNodePrefix = '__knowledge_domain__:';

interface Hierarchy {
	roleByID: Map<string, KnowledgeHierarchyRole>;
	primaryEdges: Set<string>;
	layers: Map<string, number>;
}

export function adaptKnowledgeGraph(graph: KnowledgeGraph, selectedSlug?: string): { nodes: Node[]; edges: Edge[]; neighbors: Set<string> } {
	const neighbors = new Set<string>();
	for (const edge of graph.edges) {
		if (edge.source === selectedSlug) neighbors.add(edge.target);
		if (edge.target === selectedSlug) neighbors.add(edge.source);
	}
	const displayGraph = withDomainParents(graph);
	const hierarchy = deriveHierarchy(displayGraph);
	const positions = domainSectionPositions(displayGraph.nodes);
	const nodes = displayGraph.nodes.map((node) => {
		const selected = node.id === selectedSlug;
		const neighbor = neighbors.has(node.id);
		const role = hierarchy.roleByID.get(node.id) ?? 'related';
		return {
			id: node.id,
			type: 'knowledge',
			position: positions.get(node.id) ?? { x: 0, y: 0 },
			data: { label: node.title, node, hierarchyRole: role, isDomain: isDomainNode(node.id) },
			ariaLabel: `${node.title}, ${role} node, ${node.domain}, ${node.inbound} incoming and ${node.outbound} outgoing links`,
			className: [selected ? 'selected' : '', neighbor ? 'neighbor' : '', `knowledge-${role}`].filter(Boolean).join(' ')
		} satisfies Node;
	});
	const edges = displayGraph.edges.map((edge) => {
		const primary = hierarchy.primaryEdges.has(edgeKey(edge));
		const selectedRelationship = edge.source === selectedSlug || edge.target === selectedSlug;
		const domainRelationship = isDomainNode(edge.source);
		return {
			id: edgeKey(edge), source: edge.source, target: edge.target,
			animated: selectedRelationship,
			className: [selectedRelationship ? 'selected-link' : domainRelationship ? 'domain-link' : primary ? 'hierarchy' : 'secondary'].filter(Boolean).join(' '),
			markerEnd: selectedRelationship ? { type: 'arrowclosed' as MarkerType, color: 'var(--button-accent)' } : undefined
		} satisfies Edge;
	});
	return { nodes, edges, neighbors };
}

function withDomainParents(graph: KnowledgeGraph): KnowledgeGraph {
	const domains = new Map<string, KnowledgeGraphNode>();
	for (const node of graph.nodes) {
		const domain = node.domain || 'other';
		for (const ancestor of domainAncestors(domain)) {
			if (!domains.has(ancestor)) domains.set(ancestor, { id: domainNodeID(ancestor), title: formatDomain(ancestor), domain: ancestor, pageType: 'DOMAIN', roles: [], topics: [], path: ancestor, inbound: 0, outbound: 0 });
		}
	}
	const visiblePages = graph.nodes.filter((node) => !isContainerOverview(node, domains));
	const pageEdges = visiblePages.map((node) => ({ source: domainNodeID(node.domain || 'other'), target: node.id }));
	const hierarchyEdges = [...domains.keys()].flatMap((domain) => {
		const parent = parentDomain(domain);
		return parent && domains.has(parent) ? [{ source: domainNodeID(parent), target: domainNodeID(domain) }] : [];
	});
	const visibleIDs = new Set(visiblePages.map((node) => node.id));
	return { ...graph, nodes: [...domains.values(), ...visiblePages], edges: [...hierarchyEdges, ...pageEdges, ...graph.edges.filter((edge) => visibleIDs.has(edge.source) && visibleIDs.has(edge.target))] };
}

function deriveHierarchy(graph: KnowledgeGraph): Hierarchy {
	const nodeByID = new Map(graph.nodes.map((node) => [node.id, node]));
	const inbound = new Map(graph.nodes.map((node) => [node.id, [] as string[]]));
	const outgoing = new Map(graph.nodes.map((node) => [node.id, [] as string[]]));
	for (const edge of graph.edges) {
		if (!nodeByID.has(edge.source) || !nodeByID.has(edge.target) || edge.source === edge.target) continue;
		inbound.get(edge.target)?.push(edge.source);
		outgoing.get(edge.source)?.push(edge.target);
	}
	const compare = byTitle(nodeByID);
	const roots = graph.nodes.filter((node) => isDomainNode(node.id) || (inbound.get(node.id)?.length ?? 0) === 0).map((node) => node.id).sort(compare);
	const distance = distancesFromRoots(roots, outgoing);
	const parentByChild = new Map<string, string>();
	for (const node of graph.nodes) {
		if (roots.includes(node.id)) continue;
		const candidates = (inbound.get(node.id) ?? []).filter((source) => (distance.get(source) ?? Infinity) < (distance.get(node.id) ?? Infinity));
		if (candidates.length) parentByChild.set(node.id, candidates.sort((left, right) => (distance.get(left) ?? 0) - (distance.get(right) ?? 0) || compare(left, right))[0]);
	}
	const primaryEdges = new Set([...parentByChild].map(([child, parent]) => `${parent}->${child}`));
	const layers = new Map<string, number>();
	const children = new Map(graph.nodes.map((node) => [node.id, [] as string[]]));
	for (const [child, parent] of parentByChild) children.get(parent)?.push(child);
	for (const list of children.values()) list.sort(compare);
	const visit = (id: string, layer: number) => {
		if (layers.has(id) && (layers.get(id) ?? 0) <= layer) return;
		layers.set(id, layer);
		for (const child of children.get(id) ?? []) visit(child, layer + 1);
	};
	for (const root of roots) visit(root, 0);
	for (const node of graph.nodes.sort((left, right) => compare(left.id, right.id))) if (!layers.has(node.id)) visit(node.id, 0);
	const roleByID = new Map<string, KnowledgeHierarchyRole>();
	for (const node of graph.nodes) {
		if (isDomainNode(node.id)) roleByID.set(node.id, node.domain.includes('/') ? 'root' : 'domain');
		else if ((layers.get(node.id) ?? 0) === 0) roleByID.set(node.id, 'root');
		else if ((children.get(node.id)?.length ?? 0) > 0) roleByID.set(node.id, 'parent');
		else if ((inbound.get(node.id)?.length ?? 0) > 0) roleByID.set(node.id, 'leaf');
		else roleByID.set(node.id, 'related');
	}
	return { roleByID, primaryEdges, layers };
}

function distancesFromRoots(roots: string[], outgoing: Map<string, string[]>): Map<string, number> {
	const distance = new Map<string, number>(roots.map((id) => [id, 0]));
	const queue = [...roots];
	while (queue.length) {
		const source = queue.shift()!;
		const nextDistance = (distance.get(source) ?? 0) + 1;
		for (const target of outgoing.get(source) ?? []) {
			if ((distance.get(target) ?? Infinity) <= nextDistance) continue;
			distance.set(target, nextDistance);
			queue.push(target);
		}
	}
	return distance;
}

function domainSectionPositions(nodes: KnowledgeGraphNode[]): Map<string, { x: number; y: number }> {
	const pagesByDomain = new Map<string, KnowledgeGraphNode[]>();
	for (const node of nodes) {
		if (isDomainNode(node.id)) continue;
		const domain = node.domain || 'other';
		pagesByDomain.set(domain, [...(pagesByDomain.get(domain) ?? []), node]);
	}
	const domainNodes = nodes.filter((node) => isDomainNode(node.id));
	const topDomains = domainNodes.filter((node) => !parentDomain(node.domain)).sort((left, right) => left.title.localeCompare(right.title));
	const positions = new Map<string, { x: number; y: number }>();
	const sectionWidth = 1_320;
	const nodeSpacingX = 300;
	const nodeSpacingY = 160;
	const leavesPerRow = 4;
	let nextSectionX = 0;
	for (const domainNode of topDomains) {
		const children = domainNodes.filter((node) => parentDomain(node.domain) === domainNode.domain).sort((left, right) => left.title.localeCompare(right.title));
		const sectionCount = Math.max(1, children.length);
		const sectionX = nextSectionX;
		positions.set(domainNode.id, { x: sectionX + ((sectionCount - 1) * sectionWidth) / 2 + 150, y: 0 });
		if (children.length) {
			for (let childIndex = 0; childIndex < children.length; childIndex++) {
				const child = children[childIndex];
				const childX = sectionX + childIndex * sectionWidth;
				const pages = (pagesByDomain.get(child.domain) ?? []).sort((left, right) => left.title.localeCompare(right.title) || left.id.localeCompare(right.id));
				positions.set(child.id, { x: childX + 450, y: 180 });
				for (let index = 0; index < pages.length; index++) positions.set(pages[index].id, { x: childX + (index % leavesPerRow) * nodeSpacingX, y: 360 + Math.floor(index / leavesPerRow) * nodeSpacingY });
			}
		} else {
			const pages = (pagesByDomain.get(domainNode.domain) ?? []).sort((left, right) => left.title.localeCompare(right.title) || left.id.localeCompare(right.id));
			for (let index = 0; index < pages.length; index++) positions.set(pages[index].id, { x: sectionX + (index % leavesPerRow) * nodeSpacingX, y: 180 + Math.floor(index / leavesPerRow) * nodeSpacingY });
		}
		nextSectionX += sectionCount * sectionWidth + 180;
	}
	return positions;
}

function edgeKey(edge: { source: string; target: string }): string { return `${edge.source}->${edge.target}`; }

function domainNodeID(domain: string): string { return `${domainNodePrefix}${domain}`; }
function isDomainNode(id: string): boolean { return id.startsWith(domainNodePrefix); }
function formatDomain(domain: string): string { return domain.split(/[\/_-]/).filter(Boolean).map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ') || 'Other'; }
function domainAncestors(domain: string): string[] {
	const parts = domain.split('/').filter(Boolean);
	return parts.map((_, index) => parts.slice(0, index + 1).join('/'));
}
function parentDomain(domain: string): string | undefined {
	const slash = domain.lastIndexOf('/');
	return slash > 0 ? domain.slice(0, slash) : undefined;
}
function isContainerOverview(node: KnowledgeGraphNode, domains: Map<string, KnowledgeGraphNode>): boolean {
	if (node.domain.includes('/')) return false;
	const path = node.path.replace(/\\/g, '/').toLowerCase();
	return path === `${node.domain.toLowerCase()}/readme.md` && [...domains.keys()].some((domain) => domain.startsWith(`${node.domain}/`));
}

function byTitle(nodes: Map<string, KnowledgeGraphNode>): (left: string, right: string) => number {
	return (left, right) => (nodes.get(left)?.title ?? left).localeCompare(nodes.get(right)?.title ?? right) || left.localeCompare(right);
}

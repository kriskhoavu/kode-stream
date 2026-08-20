import type { Edge, MarkerType, Node } from '@xyflow/react';
import type { KnowledgeGraph, KnowledgeGraphNode } from '../../lib/types';

export type KnowledgeHierarchyRole = 'domain' | 'root' | 'parent' | 'leaf' | 'related';

const domainNodePrefix = '__knowledge_domain__:';
const rootGroupKey = 'root';

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

interface Grouping {
	bucket: string;
	area: string;
}

interface Section {
	bucketID: string;
	columns: { areaID?: string; pages: KnowledgeGraphNode[] }[];
}

// Grouping comes from the taxonomy when the index carries it. An index written
// before the taxonomy existed has only `domain`, so bucket and area are derived
// from its first segment and remainder. Either way grouping is two levels deep,
// which is what the positioner can lay out.
function groupingOf(node: KnowledgeGraphNode): Grouping {
	if (node.bucket !== undefined || node.area !== undefined || node.tier !== undefined) {
		return { bucket: node.bucket ?? '', area: node.area ?? '' };
	}
	const parts = (node.domain === rootGroupKey ? '' : node.domain).split('/').filter(Boolean);
	return { bucket: parts[0] ?? '', area: parts.slice(1).join('/') };
}

function bucketKey(grouping: Grouping): string {
	return grouping.bucket || rootGroupKey;
}

function areaKey(grouping: Grouping): string | undefined {
	return grouping.area ? `${bucketKey(grouping)}/${grouping.area}` : undefined;
}

function withDomainParents(graph: KnowledgeGraph): KnowledgeGraph {
	const groups = new Map<string, KnowledgeGraphNode>();
	const bucketsWithAreas = new Set<string>();
	for (const node of graph.nodes) {
		const grouping = groupingOf(node);
		if (areaKey(grouping)) bucketsWithAreas.add(bucketKey(grouping));
	}
	const visiblePages = graph.nodes.filter((node) => !isContainerOverview(node, bucketsWithAreas));
	const groupNode = (key: string, label: string): KnowledgeGraphNode => ({
		id: domainNodeID(key), title: label, domain: key, pageType: 'DOMAIN',
		roles: [], topics: [], path: key, inbound: 0, outbound: 0
	});
	const pageEdges: { source: string; target: string }[] = [];
	for (const node of visiblePages) {
		const grouping = groupingOf(node);
		const bucket = bucketKey(grouping);
		if (!groups.has(bucket)) groups.set(bucket, groupNode(bucket, formatDomain(grouping.bucket || rootGroupKey)));
		const area = areaKey(grouping);
		if (area && !groups.has(area)) groups.set(area, groupNode(area, formatArea(grouping.area)));
		pageEdges.push({ source: domainNodeID(area ?? bucket), target: node.id });
	}
	const hierarchyEdges = [...groups.keys()].filter((key) => key.includes('/')).map((key) => ({
		source: domainNodeID(key.slice(0, key.indexOf('/'))), target: domainNodeID(key)
	}));
	const visibleIDs = new Set(visiblePages.map((node) => node.id));
	return {
		...graph,
		nodes: [...groups.values(), ...visiblePages],
		edges: [...hierarchyEdges, ...pageEdges, ...graph.edges.filter((edge) => visibleIDs.has(edge.source) && visibleIDs.has(edge.target))]
	};
}

// Sections are built from the original graph so grouping and positioning agree.
function sectionsOf(nodes: KnowledgeGraphNode[]): Section[] {
	const order: string[] = [];
	const byBucket = new Map<string, Map<string | undefined, KnowledgeGraphNode[]>>();
	for (const node of nodes) {
		if (isDomainNode(node.id)) continue;
		const grouping = groupingOf(node);
		const bucket = bucketKey(grouping);
		if (!byBucket.has(bucket)) {
			byBucket.set(bucket, new Map());
			order.push(bucket);
		}
		const columns = byBucket.get(bucket)!;
		const area = areaKey(grouping);
		columns.set(area, [...(columns.get(area) ?? []), node]);
	}
	order.sort((left, right) => left.localeCompare(right));
	return order.map((bucket) => {
		const columns = byBucket.get(bucket)!;
		const areaColumns = [...columns.entries()]
			.filter(([area]) => area !== undefined)
			.sort(([left], [right]) => left!.localeCompare(right!));
		const direct = columns.get(undefined);
		return {
			bucketID: domainNodeID(bucket),
			columns: [
				...(direct ? [{ areaID: undefined, pages: sortPages(direct) }] : []),
				...areaColumns.map(([area, pages]) => ({ areaID: domainNodeID(area!), pages: sortPages(pages) }))
			]
		};
	});
}

function sortPages(pages: KnowledgeGraphNode[]): KnowledgeGraphNode[] {
	return [...pages].sort((left, right) => left.title.localeCompare(right.title) || left.id.localeCompare(right.id));
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
	const positions = new Map<string, { x: number; y: number }>();
	const sectionWidth = 1_320;
	const nodeSpacingX = 300;
	const nodeSpacingY = 160;
	const leavesPerRow = 4;
	let nextSectionX = 0;
	for (const section of sectionsOf(nodes)) {
		const columnCount = Math.max(1, section.columns.length);
		const sectionX = nextSectionX;
		positions.set(section.bucketID, { x: sectionX + ((columnCount - 1) * sectionWidth) / 2 + 150, y: 0 });
		for (let columnIndex = 0; columnIndex < section.columns.length; columnIndex++) {
			const column = section.columns[columnIndex];
			const columnX = sectionX + columnIndex * sectionWidth;
			const pageY = column.areaID ? 360 : 180;
			if (column.areaID) positions.set(column.areaID, { x: columnX + 450, y: 180 });
			for (let index = 0; index < column.pages.length; index++) {
				positions.set(column.pages[index].id, {
					x: columnX + (index % leavesPerRow) * nodeSpacingX,
					y: pageY + Math.floor(index / leavesPerRow) * nodeSpacingY
				});
			}
		}
		nextSectionX += columnCount * sectionWidth + 180;
	}
	return positions;
}

function edgeKey(edge: { source: string; target: string }): string { return `${edge.source}->${edge.target}`; }

function domainNodeID(domain: string): string { return `${domainNodePrefix}${domain}`; }
function isDomainNode(id: string): boolean { return id.startsWith(domainNodePrefix); }
function formatDomain(domain: string): string { return domain.split(/[\/_-]/).filter(Boolean).map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ') || 'Other'; }
function formatArea(area: string): string {
	return area.split('/').filter(Boolean).map(formatDomain).join(' / ') || 'Other';
}

// A bucket's own landing page is redundant once the bucket is drawn as a group
// node, but only when that bucket actually has areas beneath it.
function isContainerOverview(node: KnowledgeGraphNode, bucketsWithAreas: Set<string>): boolean {
	const grouping = groupingOf(node);
	if (grouping.area || !grouping.bucket) return false;
	const path = node.path.replace(/\\/g, '/').toLowerCase();
	const bucket = grouping.bucket.toLowerCase();
	const landing = path === `${bucket}/readme.md` || path === `${bucket}/index.md`;
	return landing && bucketsWithAreas.has(grouping.bucket);
}

function byTitle(nodes: Map<string, KnowledgeGraphNode>): (left: string, right: string) => number {
	return (left, right) => (nodes.get(left)?.title ?? left).localeCompare(nodes.get(right)?.title ?? right) || left.localeCompare(right);
}

// nodeBucket exposes the grouping bucket for filters, so the graph filters and
// the graph layout agree on what a bucket is, including for an index written
// before the taxonomy existed.
export function nodeBucket(node: KnowledgeGraphNode): string {
	return groupingOf(node).bucket;
}

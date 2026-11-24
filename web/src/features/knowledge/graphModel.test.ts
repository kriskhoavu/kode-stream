import { describe, expect, it } from 'vitest';
import type { KnowledgeGraph, KnowledgeGraphNode } from '../../lib/types';
import { adaptKnowledgeGraph } from './graphModel';

function graphOf(nodes: Partial<KnowledgeGraphNode>[]): KnowledgeGraph {
	return {
		nodes: nodes.map((node, index) => ({
			id: node.id ?? `n${index}`, title: node.title ?? `Node ${index}`, domain: node.domain ?? '',
			bucket: node.bucket, area: node.area, tier: node.tier,
			pageType: node.pageType ?? 'CONCEPT', roles: [], topics: [],
			path: node.path ?? `${node.domain ?? ''}/${node.id ?? `n${index}`}.md`, inbound: 0, outbound: 0
		})),
		edges: [], totalNodes: nodes.length, totalEdges: 0, truncated: false
	};
}

const atOrigin = (nodes: { position: { x: number; y: number } }[]) =>
	nodes.filter((node) => node.position.x === 0 && node.position.y === 0);

describe('graph grouping by taxonomy', () => {
	it('positions every node in a four-level wiki', () => {
		const model = adaptKnowledgeGraph(graphOf([
			{ id: 'index', title: 'Index', domain: 'root', bucket: '', area: '', tier: '' },
			{ id: 'offer-approval', domain: 'domains/offer/concepts', bucket: 'domains', area: 'offer', tier: 'concepts' },
			{ id: 'offer-perms', domain: 'domains/offer/reference', bucket: 'domains', area: 'offer', tier: 'reference' },
			{ id: 'article-spec', domain: 'domains/master-data/article/reference', bucket: 'domains', area: 'master-data/article', tier: 'reference' },
			{ id: 'article-import', domain: 'domains/master-data/article/concepts', bucket: 'domains', area: 'master-data/article', tier: 'concepts' },
			{ id: 'rollback', domain: 'platform/concepts', bucket: 'platform', area: '', tier: 'concepts' }
		]));

		expect(atOrigin(model.nodes)).toHaveLength(0);
	});

	it('never creates a group node for a tier', () => {
		const model = adaptKnowledgeGraph(graphOf([
			{ id: 'offer-approval', domain: 'domains/offer/concepts', bucket: 'domains', area: 'offer', tier: 'concepts' },
			{ id: 'offer-perms', domain: 'domains/offer/reference', bucket: 'domains', area: 'offer', tier: 'reference' }
		]));

		const groupIDs = model.nodes.filter((node) => node.data.isDomain).map((node) => node.id);
		expect(groupIDs.some((id) => id.includes('concepts') || id.includes('reference'))).toBe(false);
		expect(groupIDs).toHaveLength(2);
	});

	it('carries tier onto the page node so it can be shown as a badge', () => {
		const model = adaptKnowledgeGraph(graphOf([
			{ id: 'offer-approval', domain: 'domains/offer/concepts', bucket: 'domains', area: 'offer', tier: 'concepts' }
		]));

		const page = model.nodes.find((node) => node.id === 'offer-approval');
		expect((page?.data.node as KnowledgeGraphNode).tier).toBe('concepts');
	});

	it('renders a nested area as a single group', () => {
		const model = adaptKnowledgeGraph(graphOf([
			{ id: 'article-spec', domain: 'domains/master-data/article/reference', bucket: 'domains', area: 'master-data/article', tier: 'reference' }
		]));

		const groups = model.nodes.filter((node) => node.data.isDomain);
		expect(groups).toHaveLength(2);
		expect(groups.map((group) => group.data.label)).toEqual(expect.arrayContaining(['Domains', 'Master Data / Article']));
	});

	it('attaches pages directly to a bucket that has no area', () => {
		const model = adaptKnowledgeGraph(graphOf([
			{ id: 'rollback', domain: 'platform/concepts', bucket: 'platform', area: '', tier: 'concepts' }
		]));

		const groups = model.nodes.filter((node) => node.data.isDomain);
		expect(groups).toHaveLength(1);
		expect(atOrigin(model.nodes)).toHaveLength(0);
		expect(model.edges).toEqual(expect.arrayContaining([
			expect.objectContaining({ source: groups[0].id, target: 'rollback' })
		]));
	});

	it('groups pages with no bucket under a single section', () => {
		const model = adaptKnowledgeGraph(graphOf([
			{ id: 'index', title: 'Index', domain: 'root', bucket: '', area: '', tier: '' },
			{ id: 'conventions', title: 'Conventions', domain: 'root', bucket: '', area: '', tier: '' }
		]));

		expect(model.nodes.filter((node) => node.data.isDomain)).toHaveLength(1);
		expect(atOrigin(model.nodes)).toHaveLength(0);
	});

	it('derives grouping from domain when the taxonomy is absent', () => {
		const model = adaptKnowledgeGraph(graphOf([
			{ id: 'offer-approval', domain: 'offer' },
			{ id: 'article-spec', domain: 'master-data/article' },
			{ id: 'deep', domain: 'domains/offer/concepts' }
		]));

		expect(atOrigin(model.nodes)).toHaveLength(0);
		const labels = model.nodes.filter((node) => node.data.isDomain).map((node) => node.data.label);
		expect(labels).toEqual(expect.arrayContaining(['Offer', 'Master Data', 'Article', 'Domains']));
	});
});

describe('focus layout', () => {
	const focusGraph = (): KnowledgeGraph => {
		const graph = graphOf([
			{ id: 'page', title: 'Export Format', domain: 'domains/orders/reference', bucket: 'domains', area: 'orders', tier: 'reference' },
			{ id: 'in-a', title: 'Close an Order', domain: 'domains/orders/concepts', bucket: 'domains', area: 'orders', tier: 'concepts' },
			{ id: 'in-b', title: 'Base Setup', domain: 'e2e-testing', bucket: 'e2e-testing', area: '', tier: '' },
			{ id: 'out-a', title: 'Returns', domain: 'domains/orders/concepts', bucket: 'domains', area: 'orders', tier: 'concepts' },
			{ id: 'both', title: 'Billing Contract', domain: 'domains/billing/reference', bucket: 'domains', area: 'billing', tier: 'reference' }
		]);
		graph.edges = [
			{ source: 'in-a', target: 'page' }, { source: 'in-b', target: 'page' },
			{ source: 'page', target: 'out-a' }, { source: 'page', target: 'both' }, { source: 'both', target: 'page' }
		] as KnowledgeGraph['edges'];
		return graph;
	};
	const positionOf = (model: ReturnType<typeof adaptKnowledgeGraph>, id: string) => model.nodes.find((node) => node.id === id)!.position;

	it('places linking pages above the page and linked pages below, without group nodes', () => {
		const model = adaptKnowledgeGraph(focusGraph(), 'page', 'page');

		expect(model.nodes.map((node) => node.id).sort()).toEqual(['both', 'in-a', 'in-b', 'out-a', 'page']);
		expect(positionOf(model, 'page')).toEqual({ x: 0, y: 0 });
		expect(positionOf(model, 'in-a').y).toBeLessThan(0);
		expect(positionOf(model, 'in-b').y).toBeLessThan(0);
		expect(positionOf(model, 'out-a').y).toBeGreaterThan(0);
		expect(positionOf(model, 'both').y).toBeGreaterThan(0);
	});

	it('keeps each side within a few hundred pixels of the page and names each node’s group', () => {
		const model = adaptKnowledgeGraph(focusGraph(), 'page', 'page');

		for (const node of model.nodes) expect(Math.abs(node.position.x)).toBeLessThanOrEqual(600);
		const groups = Object.fromEntries(model.nodes.map((node) => [node.id, (node.data as { groupLabel?: string }).groupLabel]));
		expect(groups).toMatchObject({ 'in-a': 'Domains › Orders', 'in-b': 'E2e Testing', both: 'Domains › Billing' });
		const otherBucket = Object.fromEntries(model.nodes.map((node) => [node.id, (node.data as { otherBucket?: boolean }).otherBucket]));
		expect(otherBucket).toMatchObject({ page: false, 'in-a': false, 'in-b': true, both: false });
	});

	it('wraps a crowded side into rows of five', () => {
		const graph = graphOf([{ id: 'page', title: 'Hub', domain: 'root' }, ...Array.from({ length: 12 }, (_, index) => ({ id: `out-${index}`, title: `Out ${index}`, domain: 'root' }))]);
		graph.edges = Array.from({ length: 12 }, (_, index) => ({ source: 'page', target: `out-${index}` })) as KnowledgeGraph['edges'];
		const model = adaptKnowledgeGraph(graph, 'page', 'page');

		const rows = new Set(model.nodes.filter((node) => node.id !== 'page').map((node) => node.position.y));
		expect(rows.size).toBe(3);
	});
});

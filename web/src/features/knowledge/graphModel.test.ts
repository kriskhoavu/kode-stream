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
	// The reported defect: a wiki more than two directories deep left most
	// nodes without a position, stacking them at the origin.
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

	// A tier is a partition of an area's pages, not a container, so it must not
	// become a grouping node.
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

	// A nested area stays one row; wiki depth must not add view depth.
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

	// An index written before the taxonomy existed carries only domain. Grouping
	// must derive bucket and area from it rather than collapsing everything.
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

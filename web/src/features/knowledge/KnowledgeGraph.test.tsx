import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { KnowledgeGraph as GraphData, KnowledgePage, KnowledgePageDetail } from '../../lib/types';
import { adaptKnowledgeGraph } from './graphModel';

const reactFlowSpy = vi.fn();

vi.mock('@xyflow/react', () => ({
		ReactFlow: ({ nodes, edges, nodeTypes, onNodeClick, onNodeDragStop, nodesDraggable, children }: { nodes: Array<{ id: string; type?: string; position: { x: number; y: number }; data: { label: string } }>; edges: Array<{ id: string }>; nodeTypes?: Record<string, React.ComponentType<{ data: { label: string } }>>; onNodeClick: (event: unknown, node: { id: string }) => void; onNodeDragStop?: (event: unknown, node: { id: string; position: { x: number; y: number } }) => void; nodesDraggable?: boolean; children: React.ReactNode }) => {
			reactFlowSpy({ nodes, edges, nodesDraggable });
		return <div data-testid="flow" data-draggable={nodesDraggable ? 'true' : 'false'}>
			{nodes.map((node) => {
				const NodeComponent = node.type ? nodeTypes?.[node.type] : undefined;
				return <div key={node.id}>
					<button onClick={() => onNodeClick({}, node)}>{node.data.label}</button>
					<button type="button" onClick={() => onNodeDragStop?.({}, { ...node, position: { x: 444, y: 222 } })}>Drag {node.data.label}</button>
					{NodeComponent ? <NodeComponent data={node.data} /> : null}
				</div>;
			})}
			{children}
		</div>;
	},
	Background: () => null,
	Handle: () => null,
	Panel: ({ children }: { children: React.ReactNode }) => <div data-testid="controls">{children}</div>,
	Position: { Left: 'left', Right: 'right' },
	useReactFlow: () => ({ zoomIn: vi.fn(), zoomOut: vi.fn(), fitView: vi.fn() })
}));

const graph: GraphData = {
	nodes: [
		{ id: 'a', title: 'Alpha', domain: 'offer', pageType: 'CONCEPT', roles: [], topics: [], path: 'a.md', inbound: 1, outbound: 1 },
		{ id: 'b', title: 'Beta', domain: 'article', pageType: 'HOW_TO', roles: [], topics: [], path: 'b.md', inbound: 1, outbound: 1 }
	],
	edges: [{ source: 'a', target: 'b' }], totalNodes: 5, totalEdges: 8, truncated: true
};

const pages: KnowledgePage[] = [
	{ slug: 'a', title: 'Alpha', domain: 'offer', pageType: 'CONCEPT', roles: ['BA'], topics: ['workflow'], path: 'a.md', summary: 'Alpha summary', sourceRefs: [], links: [], backlinks: [] },
	{ slug: 'b', title: 'Beta', domain: 'article', pageType: 'HOW_TO', roles: [], topics: [], path: 'b.md', summary: 'Beta summary', sourceRefs: [], links: [], backlinks: [] }
];

const detail: KnowledgePageDetail = {
	...pages[0],
	warnings: [],
	content: { id: 'a', path: 'a.md', content: '---\nslug: a\ntitle: Alpha\n---\n# Alpha\nMain roadmap preview content with enough text to validate the simplified graph panel preview.', language: 'markdown', hash: 'hash', kind: 'markdown', sizeBytes: 120, editable: false }
};

describe('Knowledge graph', () => {
	it('adapts selected nodes, neighbors, and deterministic directed edges', () => {
		const model = adaptKnowledgeGraph(graph, 'a');
		const offerDomain = model.nodes.find((node) => node.id === '__knowledge_domain__:offer');
		const articleDomain = model.nodes.find((node) => node.id === '__knowledge_domain__:article');
		expect(offerDomain?.className).toContain('knowledge-domain');
		expect(model.edges).toEqual(expect.arrayContaining([expect.objectContaining({ source: '__knowledge_domain__:offer', target: 'a' })]));
		expect(offerDomain?.position.y).toBeLessThan(model.nodes.find((node) => node.id === 'a')!.position.y);
		expect(articleDomain?.position.x).not.toEqual(offerDomain?.position.x);
		expect(articleDomain?.position.y).toEqual(offerDomain?.position.y);
		expect(model.nodes.find((node) => node.id === 'a')?.className).toContain('selected');
		expect(model.nodes.find((node) => node.id === 'a')?.className).toContain('knowledge-leaf');
		expect(model.nodes.find((node) => node.id === 'b')?.className).toContain('neighbor');
		expect(model.nodes.find((node) => node.id === 'b')?.className).toContain('knowledge-leaf');
		expect(model.edges.find((edge) => edge.id === 'a->b')).toEqual(expect.objectContaining({ id: 'a->b', source: 'a', target: 'b', animated: true, className: 'selected-link' }));
	});

	it('connects nested domains to their directory parent', () => {
		const nestedGraph: GraphData = {
			...graph,
			nodes: [
				{ ...graph.nodes[0], id: 'master-data-overview', title: 'Master Data', domain: 'master-data', path: 'master-data/README.md' },
				...graph.nodes.map((node) => ({ ...node, domain: node.id === 'a' ? 'master-data/article' : 'master-data/customer' }))
			]
		};
		const model = adaptKnowledgeGraph(nestedGraph);
		expect(model.edges).toEqual(expect.arrayContaining([
			expect.objectContaining({ source: '__knowledge_domain__:master-data', target: '__knowledge_domain__:master-data/article' }),
			expect.objectContaining({ source: '__knowledge_domain__:master-data', target: '__knowledge_domain__:master-data/customer' })
		]));
		expect(model.nodes.find((node) => node.id === 'master-data-overview')).toBeUndefined();
		expect(model.nodes.find((node) => node.id === '__knowledge_domain__:master-data/article')?.className).toContain('knowledge-root');
		expect(model.nodes.find((node) => node.id === '__knowledge_domain__:master-data')?.position.y).toBeLessThan(model.nodes.find((node) => node.id === '__knowledge_domain__:master-data/article')!.position.y);
	});

	it('filters graph nodes and selects them', async () => {
		const { KnowledgeGraph } = await import('./KnowledgeGraph');
		const onSelect = vi.fn();
		render(<KnowledgeGraph graph={graph} pages={pages} onSelect={onSelect} onOpenDetails={vi.fn()} />);
		expect(screen.getByRole('status')).toHaveTextContent('Showing 2 of 5 pages');
		fireEvent.click(within(screen.getByTestId('flow')).getByRole('button', { name: 'Alpha' })); expect(onSelect).toHaveBeenCalledWith('a');
		fireEvent.change(screen.getByRole('combobox', { name: 'Filter graph bucket' }), { target: { value: 'offer' } });
		expect(within(screen.getByTestId('flow')).queryByRole('button', { name: 'Beta' })).not.toBeInTheDocument();
		expect(screen.queryByRole('region', { name: 'Knowledge relationships' })).not.toBeInTheDocument();
	});

	it('filters by bucket and tier instead of a flat domain path', async () => {
		const { KnowledgeGraph } = await import('./KnowledgeGraph');
		const taxonomyGraph: GraphData = {
			...graph,
			nodes: [
				{ id: 'approval', title: 'Approval', domain: 'domains/offer/concepts', bucket: 'domains', area: 'offer', tier: 'concepts', pageType: 'HOW_TO', roles: [], topics: [], path: 'domains/offer/concepts/approval.md', inbound: 0, outbound: 0 },
				{ id: 'permissions', title: 'Permissions', domain: 'domains/offer/reference', bucket: 'domains', area: 'offer', tier: 'reference', pageType: 'REFERENCE', roles: [], topics: [], path: 'domains/offer/reference/permissions.md', inbound: 0, outbound: 0 },
				{ id: 'rollback', title: 'Rollback', domain: 'platform/concepts', bucket: 'platform', area: '', tier: 'concepts', pageType: 'CONCEPT', roles: [], topics: [], path: 'platform/concepts/rollback.md', inbound: 0, outbound: 0 }
			],
			edges: []
		};
		render(<KnowledgeGraph graph={taxonomyGraph} pages={[]} onSelect={vi.fn()} onOpenDetails={vi.fn()} />);

		const buckets = screen.getByRole('combobox', { name: 'Filter graph bucket' });
		expect(within(buckets).getByRole('option', { name: 'domains' })).toBeInTheDocument();
		expect(within(buckets).getByRole('option', { name: 'platform' })).toBeInTheDocument();

		fireEvent.change(buckets, { target: { value: 'domains' } });
		expect(within(screen.getByTestId('flow')).queryByRole('button', { name: 'Rollback' })).not.toBeInTheDocument();
		expect(within(screen.getByTestId('flow')).getByRole('button', { name: 'Approval' })).toBeInTheDocument();

		fireEvent.change(screen.getByRole('combobox', { name: 'Filter graph tier' }), { target: { value: 'reference' } });
		expect(within(screen.getByTestId('flow')).getByRole('button', { name: 'Permissions' })).toBeInTheDocument();
		expect(within(screen.getByTestId('flow')).queryByRole('button', { name: 'Approval' })).not.toBeInTheDocument();
	});

	it('focuses the graph on the selected node and its direct relationships', async () => {
		const { KnowledgeGraph } = await import('./KnowledgeGraph');
		const graphWithUnrelatedNode: GraphData = {
			...graph,
			nodes: [...graph.nodes, { id: 'c', title: 'Gamma', domain: 'other', pageType: 'REFERENCE', roles: [], topics: [], path: 'c.md', inbound: 0, outbound: 0 }]
		};
		render(<KnowledgeGraph graph={graphWithUnrelatedNode} pages={pages} selectedSlug="a" onSelect={vi.fn()} onOpenDetails={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Focus relationships' }));
		expect(within(screen.getByTestId('flow')).getByRole('button', { name: 'Alpha' })).toBeInTheDocument();
		expect(within(screen.getByTestId('flow')).getByRole('button', { name: 'Beta' })).toBeInTheDocument();
		expect(within(screen.getByTestId('flow')).queryByRole('button', { name: 'Gamma' })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Show all components' })).toHaveAttribute('aria-pressed', 'true');
		expect(reactFlowSpy.mock.calls.at(-1)?.[0].edges).toHaveLength(3);
		fireEvent.click(screen.getByRole('button', { name: 'Hide selected links' }));
		expect(screen.getByRole('button', { name: 'Focus relationships' })).toHaveAttribute('aria-pressed', 'false');
		expect(within(screen.getByTestId('flow')).getByRole('button', { name: 'Gamma' })).toBeInTheDocument();
		expect(reactFlowSpy.mock.calls.at(-1)?.[0].nodes.find((node: { id: string }) => node.id === 'a')?.className).not.toContain('selected');
	});

	it('shows gray domain grouping links in the overview and adds selected links on request', async () => {
		const { KnowledgeGraph } = await import('./KnowledgeGraph');
		render(<KnowledgeGraph graph={graph} pages={pages} selectedSlug="a" onSelect={vi.fn()} onOpenDetails={vi.fn()} />);
		expect(reactFlowSpy.mock.calls.at(-1)?.[0].edges).toHaveLength(2);
		fireEvent.click(screen.getByRole('button', { name: 'Show selected links' }));
		expect(reactFlowSpy.mock.calls.at(-1)?.[0].edges).toHaveLength(3);
	});

	it('shows a review panel, enables dragging, and opens details from the tooltip action', async () => {
		const { KnowledgeGraph } = await import('./KnowledgeGraph');
		const onSelect = vi.fn();
		const onOpenDetails = vi.fn();
		render(<KnowledgeGraph graph={graph} pages={pages} selectedSlug="a" selectedDetail={detail} onSelect={onSelect} onOpenDetails={onOpenDetails} />);

		expect(reactFlowSpy).toHaveBeenCalled();
		expect(screen.getByTestId('flow')).toHaveAttribute('data-draggable', 'true');
		expect(screen.getByRole('button', { name: 'Beautify graph layout' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Auto arrange graph' })).toBeInTheDocument();
		expect(screen.queryByTestId('minimap')).not.toBeInTheDocument();
		expect(screen.queryByLabelText('Knowledge graph review panel')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Review in panel' })).not.toBeInTheDocument();

		fireEvent.click(within(screen.getByTestId('flow')).getByRole('button', { name: 'Alpha' }));
		expect(screen.queryByLabelText('Knowledge graph review panel')).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Close node tooltip' })).toBeInTheDocument();
		fireEvent.click(within(screen.getByTestId('flow')).getByRole('button', { name: 'Alpha' }));
		const panel = screen.getByLabelText('Knowledge graph review panel');
		expect(screen.queryByRole('button', { name: 'Close node tooltip' })).not.toBeInTheDocument();
		await waitFor(() => expect(panel).toHaveTextContent('Main roadmap preview content'), { timeout: 5_000 });
		expect(panel).toHaveTextContent('Outgoing links');
		expect(panel).not.toHaveTextContent('Relationships');
		fireEvent.click(within(panel).getByRole('button', { name: 'Open details page' }));
		expect(onOpenDetails).toHaveBeenCalledWith('a');
		expect(onSelect).toHaveBeenCalledWith('a');
		fireEvent.click(within(screen.getByTestId('flow')).getByRole('button', { name: 'Alpha' }));
		expect(screen.queryByLabelText('Knowledge graph review panel')).not.toBeInTheDocument();
	});

	it('keeps manually moved node positions until the graph layout is reset', async () => {
		const { KnowledgeGraph } = await import('./KnowledgeGraph');
		render(<KnowledgeGraph graph={graph} pages={pages} onSelect={vi.fn()} onOpenDetails={vi.fn()} />);

		expect(screen.getByRole('button', { name: 'Reset graph layout' })).toBeDisabled();
		const initialNodes = reactFlowSpy.mock.calls.at(-1)?.[0].nodes;
		const initialPosition = initialNodes.find((node: { id: string }) => node.id === 'a')?.position;
		expect(initialPosition).toEqual(expect.objectContaining({ x: expect.any(Number), y: expect.any(Number) }));

		fireEvent.click(screen.getByRole('button', { name: 'Drag Alpha' }));
		expect(screen.getByRole('button', { name: 'Reset graph layout' })).toBeEnabled();
		const movedNodes = reactFlowSpy.mock.calls.at(-1)?.[0].nodes;
		expect(movedNodes.find((node: { id: string }) => node.id === 'a')?.position).toEqual({ x: 444, y: 222 });
		fireEvent.click(screen.getByRole('button', { name: 'Auto arrange graph' }));
		const arrangedNodes = reactFlowSpy.mock.calls.at(-1)?.[0].nodes;
		expect(arrangedNodes.find((node: { id: string }) => node.id === 'a')?.position).not.toEqual({ x: 444, y: 222 });

		fireEvent.click(screen.getByRole('button', { name: 'Reset graph layout' }));
		expect(screen.getByRole('button', { name: 'Reset graph layout' })).toBeDisabled();
		const resetNodes = reactFlowSpy.mock.calls.at(-1)?.[0].nodes;
		expect(resetNodes.find((node: { id: string }) => node.id === 'a')?.position).toEqual(initialPosition);
	});

	it('allows closing the review panel and reopening it from the same selected node', async () => {
		const { KnowledgeGraph } = await import('./KnowledgeGraph');
		const onSelect = vi.fn();
		render(<KnowledgeGraph graph={graph} pages={pages} selectedSlug="a" selectedDetail={detail} onSelect={onSelect} onOpenDetails={vi.fn()} />);

		fireEvent.click(within(screen.getByTestId('flow')).getByRole('button', { name: 'Alpha' }));
		fireEvent.click(within(screen.getByTestId('flow')).getByRole('button', { name: 'Alpha' }));
		expect(screen.getByLabelText('Knowledge graph review panel')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Close review panel' }));
		expect(screen.queryByLabelText('Knowledge graph review panel')).not.toBeInTheDocument();

		fireEvent.click(within(screen.getByTestId('flow')).getByRole('button', { name: 'Alpha' }));
		expect(onSelect).toHaveBeenCalledWith('a');
		expect(screen.getByLabelText('Knowledge graph review panel')).toBeInTheDocument();
	});

	it('closes the review panel when Escape is pressed', async () => {
		const { KnowledgeGraph } = await import('./KnowledgeGraph');
		render(<KnowledgeGraph graph={graph} pages={pages} selectedSlug="a" selectedDetail={detail} onSelect={vi.fn()} onOpenDetails={vi.fn()} />);

		fireEvent.click(within(screen.getByTestId('flow')).getByRole('button', { name: 'Alpha' }));
		fireEvent.click(within(screen.getByTestId('flow')).getByRole('button', { name: 'Alpha' }));
		expect(screen.getByLabelText('Knowledge graph review panel')).toBeInTheDocument();
		fireEvent.keyDown(window, { key: 'Escape' });
		expect(screen.queryByLabelText('Knowledge graph review panel')).not.toBeInTheDocument();
	});
});

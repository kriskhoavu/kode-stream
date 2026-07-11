import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { KnowledgeGraph as GraphData, KnowledgePage, KnowledgePageDetail } from '../../lib/types';
import { adaptKnowledgeGraph } from './graphModel';

const reactFlowSpy = vi.fn();

vi.mock('@xyflow/react', () => ({
	ReactFlow: ({ nodes, nodeTypes, onNodeClick, onNodeDragStop, nodesDraggable, children }: { nodes: Array<{ id: string; type?: string; position: { x: number; y: number }; data: { label: string } }>; nodeTypes?: Record<string, React.ComponentType<{ data: { label: string } }>>; onNodeClick: (event: unknown, node: { id: string }) => void; onNodeDragStop?: (event: unknown, node: { id: string; position: { x: number; y: number } }) => void; nodesDraggable?: boolean; children: React.ReactNode }) => {
		reactFlowSpy({ nodes, nodesDraggable });
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
		expect(model.nodes.find((node) => node.id === 'a')?.className).toBe('selected');
		expect(model.nodes.find((node) => node.id === 'b')?.className).toBe('neighbor');
		expect(model.edges[0]).toEqual(expect.objectContaining({ id: 'a->b', source: 'a', target: 'b', animated: true }));
	});

	it('filters graph nodes and selects them', async () => {
		const { KnowledgeGraph } = await import('./KnowledgeGraph');
		const onSelect = vi.fn();
		render(<KnowledgeGraph graph={graph} pages={pages} onSelect={onSelect} onOpenDetails={vi.fn()} />);
		expect(screen.getByRole('status')).toHaveTextContent('Showing 2 of 5 pages');
		fireEvent.click(within(screen.getByTestId('flow')).getByRole('button', { name: 'Alpha' })); expect(onSelect).toHaveBeenCalledWith('a');
		fireEvent.change(screen.getByRole('combobox', { name: 'Filter graph domain' }), { target: { value: 'offer' } });
		expect(within(screen.getByTestId('flow')).queryByRole('button', { name: 'Beta' })).not.toBeInTheDocument();
		expect(screen.queryByRole('region', { name: 'Knowledge relationships' })).not.toBeInTheDocument();
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
	});

	it('shows a review panel, enables dragging, and opens details from the tooltip action', async () => {
		const { KnowledgeGraph } = await import('./KnowledgeGraph');
		const onSelect = vi.fn();
		const onOpenDetails = vi.fn();
		render(<KnowledgeGraph graph={graph} pages={pages} selectedSlug="a" selectedDetail={detail} onSelect={onSelect} onOpenDetails={onOpenDetails} />);

		expect(reactFlowSpy).toHaveBeenCalled();
		expect(screen.getByTestId('flow')).toHaveAttribute('data-draggable', 'true');
		expect(screen.queryByTestId('minimap')).not.toBeInTheDocument();
		expect(screen.queryByLabelText('Knowledge graph review panel')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Review in panel' })).not.toBeInTheDocument();

		fireEvent.click(within(screen.getByTestId('flow')).getByRole('button', { name: 'Alpha' }));
		const panel = screen.getByLabelText('Knowledge graph review panel');
		await waitFor(() => expect(panel).toHaveTextContent('Main roadmap preview content'), { timeout: 5_000 });
		expect(panel).toHaveTextContent('Outgoing links');
		expect(panel).not.toHaveTextContent('Relationships');
		fireEvent.click(within(panel).getByRole('button', { name: 'Open details page' }));
		expect(onOpenDetails).toHaveBeenCalledWith('a');
		expect(onSelect).toHaveBeenCalledWith('a');
	});

	it('keeps manually moved node positions until the graph layout is reset', async () => {
		const { KnowledgeGraph } = await import('./KnowledgeGraph');
		render(<KnowledgeGraph graph={graph} pages={pages} onSelect={vi.fn()} onOpenDetails={vi.fn()} />);

		expect(screen.getByRole('button', { name: 'Reset graph layout' })).toBeDisabled();
		const initialNodes = reactFlowSpy.mock.calls.at(-1)?.[0].nodes;
		expect(initialNodes.find((node: { id: string }) => node.id === 'a')?.position).toEqual({ x: 0, y: 0 });

		fireEvent.click(screen.getByRole('button', { name: 'Drag Alpha' }));
		expect(screen.getByRole('button', { name: 'Reset graph layout' })).toBeEnabled();
		const movedNodes = reactFlowSpy.mock.calls.at(-1)?.[0].nodes;
		expect(movedNodes.find((node: { id: string }) => node.id === 'a')?.position).toEqual({ x: 444, y: 222 });

		fireEvent.click(screen.getByRole('button', { name: 'Reset graph layout' }));
		expect(screen.getByRole('button', { name: 'Reset graph layout' })).toBeDisabled();
		const resetNodes = reactFlowSpy.mock.calls.at(-1)?.[0].nodes;
		expect(resetNodes.find((node: { id: string }) => node.id === 'a')?.position).toEqual({ x: 0, y: 0 });
	});

	it('allows closing the review panel and reopening it from the same selected node', async () => {
		const { KnowledgeGraph } = await import('./KnowledgeGraph');
		const onSelect = vi.fn();
		render(<KnowledgeGraph graph={graph} pages={pages} selectedSlug="a" selectedDetail={detail} onSelect={onSelect} onOpenDetails={vi.fn()} />);

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
		expect(screen.getByLabelText('Knowledge graph review panel')).toBeInTheDocument();
		fireEvent.keyDown(window, { key: 'Escape' });
		expect(screen.queryByLabelText('Knowledge graph review panel')).not.toBeInTheDocument();
	});
});

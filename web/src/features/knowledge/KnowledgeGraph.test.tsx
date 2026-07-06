import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { KnowledgeGraph as GraphData } from '../../lib/types';
import { adaptKnowledgeGraph } from './graphModel';

vi.mock('@xyflow/react', () => ({
	ReactFlow: ({ nodes, onNodeClick, children }: { nodes: Array<{ id: string; data: { label: string } }>; onNodeClick: (event: unknown, node: { id: string }) => void; children: React.ReactNode }) => <div data-testid="flow">{nodes.map((node) => <button key={node.id} onClick={() => onNodeClick({}, node)}>{node.data.label}</button>)}{children}</div>,
	Background: () => null, Controls: () => null, MiniMap: () => null
}));

const graph: GraphData = {
	nodes: [
		{ id: 'a', title: 'Alpha', domain: 'offer', pageType: 'CONCEPT', roles: [], topics: [], path: 'a.md', inbound: 1, outbound: 1 },
		{ id: 'b', title: 'Beta', domain: 'article', pageType: 'HOW_TO', roles: [], topics: [], path: 'b.md', inbound: 1, outbound: 1 }
	],
	edges: [{ source: 'a', target: 'b' }], totalNodes: 5, totalEdges: 8, truncated: true
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
		render(<KnowledgeGraph graph={graph} onSelect={onSelect} />);
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
		render(<KnowledgeGraph graph={graphWithUnrelatedNode} selectedSlug="a" onSelect={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'Focus relationships' }));
		expect(within(screen.getByTestId('flow')).getByRole('button', { name: 'Alpha' })).toBeInTheDocument();
		expect(within(screen.getByTestId('flow')).getByRole('button', { name: 'Beta' })).toBeInTheDocument();
		expect(within(screen.getByTestId('flow')).queryByRole('button', { name: 'Gamma' })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Show all components' })).toHaveAttribute('aria-pressed', 'true');
	});
});

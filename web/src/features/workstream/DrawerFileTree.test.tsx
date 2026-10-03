import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { FileNode } from '../../lib/types';
import { DrawerFileTree } from './DrawerFileTree';

const nodes: FileNode[] = [
	{ id: 'f-plan', name: 'plan.yaml', path: 'plan.yaml', type: 'file' },
	{ id: 'd-scenario', name: 'scenario', path: 'scenario', type: 'directory', children: [
		{ id: 'f-s1', name: 'scenario-01.md', path: 'scenario/scenario-01.md', type: 'file' }
	] },
	{ id: 'd-design', name: 'design', path: 'design', type: 'directory', children: [
		{ id: 'd-assets', name: 'assets', path: 'design/assets', type: 'directory', children: [
			{ id: 'f-png', name: 'price.png', path: 'design/assets/price.png', type: 'file' }
		] }
	] },
	{ id: 'f-readme', name: 'README.md', path: 'README.md', type: 'file' }
];

describe('DrawerFileTree', () => {
	it('lists folders before files, expands top-level folders and the open file’s folders, and opens a file by id', () => {
		const onOpen = vi.fn();
		render(<DrawerFileTree nodes={nodes} activeId="f-png" onOpen={onOpen} />);
		const tree = screen.getByRole('tree', { name: 'Item files' });
		const top = within(tree).getAllByRole('treeitem').filter((item) => item.parentElement === tree).map((item) => item.getAttribute('aria-label'));
		expect(top).toEqual(['design', 'scenario', 'plan.yaml', 'README.md']);
		expect(screen.getByRole('treeitem', { name: 'assets' })).toHaveAttribute('aria-expanded', 'true');
		expect(screen.getByRole('treeitem', { name: 'price.png' })).toHaveAttribute('aria-selected', 'true');
		fireEvent.click(screen.getByRole('button', { name: 'scenario-01.md' }));
		expect(onOpen).toHaveBeenCalledWith('f-s1');
	});

	it('collapses and expands a folder', () => {
		render(<DrawerFileTree nodes={nodes} onOpen={vi.fn()} />);
		fireEvent.click(screen.getByRole('button', { name: 'scenario' }));
		expect(screen.queryByRole('button', { name: 'scenario-01.md' })).not.toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'scenario' }));
		expect(screen.getByRole('button', { name: 'scenario-01.md' })).toBeInTheDocument();
	});
});

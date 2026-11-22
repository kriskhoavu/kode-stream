import type { FileNode } from '../../lib/types';

/** The first file in a tree, depth first. */
export function firstFile(nodes: FileNode[]): FileNode | null {
	for (const node of nodes) {
		if (node.type === 'file') return node;
		const child = firstFile(node.children ?? []);
		if (child) return child;
	}
	return null;
}

/** A README.md at the top of the tree, or, unless `rootOnly`, anywhere in it. */
export function findReadme(nodes: FileNode[], rootOnly: boolean): FileNode | null {
	for (const node of nodes) {
		if (node.type === 'file' && node.name.toLowerCase() === 'readme.md') return node;
		if (!rootOnly && node.type === 'directory') {
			const child = findReadme(node.children ?? [], false);
			if (child) return child;
		}
	}
	return null;
}

/** The file an item opens on: its top README, any README, else its first file. */
export function preferredFile(nodes: FileNode[]): FileNode | null {
	return findReadme(nodes, true) ?? findReadme(nodes, false) ?? firstFile(nodes);
}

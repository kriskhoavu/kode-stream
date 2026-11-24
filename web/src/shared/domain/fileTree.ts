import type { FileNode } from '../../lib/types';

export function firstFile(nodes: FileNode[]): FileNode | null {
	for (const node of nodes) {
		if (node.type === 'file') return node;
		const child = firstFile(node.children ?? []);
		if (child) return child;
	}
	return null;
}

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

export function preferredFile(nodes: FileNode[]): FileNode | null {
	return findReadme(nodes, true) ?? findReadme(nodes, false) ?? firstFile(nodes);
}

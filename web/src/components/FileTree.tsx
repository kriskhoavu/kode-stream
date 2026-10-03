import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { ChevronRight, File, FileText, Folder, FolderOpen } from 'lucide-react';
import type { FileNode } from '../lib/types';

/** The ids of the folders that lead to fileId, outermost first; empty when it is not in the tree. */
function foldersLeadingTo(nodes: FileNode[], fileId: string, trail: string[] = []): string[] | undefined {
	for (const node of nodes) {
		if (node.type === 'file' && node.id === fileId) return trail;
		if (node.type === 'directory') {
			const found = foldersLeadingTo(node.children ?? [], fileId, [...trail, node.id]);
			if (found) return found;
		}
	}
	return undefined;
}

function sortedNodes(nodes: FileNode[]): FileNode[] {
	return [...nodes].sort((a, b) => a.type === b.type ? a.name.localeCompare(b.name) : a.type === 'directory' ? -1 : 1);
}

/**
 * An item's files as a tree, shown inside the document dropdown. Top-level
 * folders and the folders leading to the open document start expanded;
 * opening a file calls onOpen with its id.
 */
export function FileTree({ nodes, activeId, onOpen }: { nodes: FileNode[]; activeId?: string; onOpen: (fileId: string) => void }) {
	const topFolders = useMemo(() => nodes.filter((node) => node.type === 'directory').map((node) => node.id), [nodes]);
	const [expanded, setExpanded] = useState<Set<string>>(() => new Set(topFolders));
	useEffect(() => {
		setExpanded((current) => {
			const next = new Set(current);
			for (const id of topFolders) next.add(id);
			for (const id of (activeId ? foldersLeadingTo(nodes, activeId) : undefined) ?? []) next.add(id);
			return next.size === current.size ? current : next;
		});
	}, [activeId, nodes, topFolders]);
	const toggle = (id: string) => setExpanded((current) => {
		const next = new Set(current);
		if (next.has(id)) next.delete(id); else next.add(id);
		return next;
	});
	if (nodes.length === 0) return <p className="file-tree-empty">No files.</p>;
	return <ul className="file-tree" role="tree" aria-label="Item files">
		{sortedNodes(nodes).map((node) => <TreeRow key={node.id} node={node} depth={0} expanded={expanded} activeId={activeId} onToggle={toggle} onOpen={onOpen} />)}
	</ul>;
}

function TreeRow({ node, depth, expanded, activeId, onToggle, onOpen }: { node: FileNode; depth: number; expanded: Set<string>; activeId?: string; onToggle: (id: string) => void; onOpen: (fileId: string) => void }) {
	const style = { '--tree-depth': depth } as CSSProperties;
	if (node.type === 'directory') {
		const open = expanded.has(node.id);
		return <li role="treeitem" aria-expanded={open} aria-label={node.name}>
			<button type="button" className="file-tree-row folder" style={style} onClick={() => onToggle(node.id)}>
				<ChevronRight size={13} className={open ? 'file-tree-chevron open' : 'file-tree-chevron'} />
				{open ? <FolderOpen size={14} /> : <Folder size={14} />}
				<span>{node.name}</span>
			</button>
			{open && <ul role="group">{sortedNodes(node.children ?? []).map((child) => <TreeRow key={child.id} node={child} depth={depth + 1} expanded={expanded} activeId={activeId} onToggle={onToggle} onOpen={onOpen} />)}</ul>}
		</li>;
	}
	const active = node.id === activeId;
	const Icon = /\.(md|mdx|markdown|txt)$/i.test(node.name) ? FileText : File;
	return <li role="treeitem" aria-selected={active} aria-label={node.name}>
		<button type="button" className={active ? 'file-tree-row active' : 'file-tree-row'} style={style} title={node.path} aria-current={active ? 'true' : undefined} onClick={() => onOpen(node.id)}>
			<span className="file-tree-chevron-space" aria-hidden="true" />
			<Icon size={14} />
			<span>{node.name}</span>
		</button>
	</li>;
}

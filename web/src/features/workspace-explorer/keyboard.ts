import type { VisibleExplorerRow } from './types';
import { explorerNodeId } from './tree';

export interface TreeKeyboardResult {
  activeIndex: number;
  toggleNodeId?: string;
  select?: VisibleExplorerRow;
}

export function treeKeyboardAction(key: string, rows: VisibleExplorerRow[], activeIndex: number, expandedNodeIds: Set<string>): TreeKeyboardResult {
  if (rows.length === 0) return { activeIndex: -1 };
  const index = Math.min(Math.max(activeIndex, 0), rows.length - 1);
  const row = rows[index];
  const nodeId = explorerNodeId(row.workspaceId, row.node.path);
  if (key === 'ArrowDown') return { activeIndex: Math.min(index + 1, rows.length - 1) };
  if (key === 'ArrowUp') return { activeIndex: Math.max(index - 1, 0) };
  if (key === 'Home') return { activeIndex: 0 };
  if (key === 'End') return { activeIndex: rows.length - 1 };
  if (key === 'Enter' || key === ' ') return { activeIndex: index, select: row };
  if (key === 'ArrowRight' && (row.node.type === 'workspace' || row.node.type === 'directory') && !expandedNodeIds.has(nodeId)) {
    return { activeIndex: index, toggleNodeId: nodeId };
  }
  if (key === 'ArrowLeft' && expandedNodeIds.has(nodeId)) return { activeIndex: index, toggleNodeId: nodeId };
  if (key === 'ArrowLeft' && row.parentId) {
    const parentIndex = rows.findIndex((candidate) => explorerNodeId(candidate.workspaceId, candidate.node.path) === row.parentId);
    return { activeIndex: parentIndex >= 0 ? parentIndex : index };
  }
  return { activeIndex: index };
}

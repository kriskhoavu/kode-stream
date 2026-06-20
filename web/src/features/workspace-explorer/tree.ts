import type { ItemSummary, WorkspaceConfig, WorkspaceTreeEntry } from '../../lib/types';
import type { DirectoryCacheEntry, ExplorerItemDecoration, VisibleExplorerRow, WorkspaceRootNode } from './types';

export function explorerNodeId(workspaceId: string, path: string): string {
  return `${workspaceId}:${normalizeExplorerPath(path)}`;
}

export function directoryCacheKey(workspaceId: string, path: string, includeIgnored: boolean): string {
  return `${explorerNodeId(workspaceId, path)}:${includeIgnored ? 'ignored' : 'visible'}`;
}

export function buildItemDecorations(items: ItemSummary[]): Map<string, ExplorerItemDecoration> {
  const result = new Map<string, ExplorerItemDecoration>();
  for (const item of items) {
    if (!item.workspaceId || !item.itemPath) continue;
    result.set(explorerNodeId(item.workspaceId, item.itemPath), {
      itemId: item.id,
      identifier: item.identifier,
      title: item.title,
      status: item.status,
      owner: item.owner,
      branch: item.branch,
      warnings: 0
    });
  }
  return result;
}

export function flattenVisibleTree({
  workspaces,
  expandedNodeIds,
  cache,
  includeIgnored,
  decorations,
  filter = ''
}: {
  workspaces: WorkspaceConfig[];
  expandedNodeIds: Set<string>;
  cache: Map<string, DirectoryCacheEntry>;
  includeIgnored: boolean;
  decorations: Map<string, ExplorerItemDecoration>;
  filter?: string;
}): VisibleExplorerRow[] {
  const rows: VisibleExplorerRow[] = [];
  const query = filter.trim().toLocaleLowerCase();
  const appendChildren = (workspaceId: string, parentPath: string, parentId: string, level: number) => {
    const children = cache.get(directoryCacheKey(workspaceId, parentPath, includeIgnored))?.entries ?? [];
    children.forEach((entry, index) => {
      const id = explorerNodeId(workspaceId, entry.path);
      const row: VisibleExplorerRow = {
        node: entry,
        level,
        parentId,
        positionInSet: index + 1,
        setSize: children.length,
        workspaceId,
        item: decorations.get(id)
      };
      if (!query || matchesRow(row, query)) rows.push(row);
      if (entry.type === 'directory' && expandedNodeIds.has(id)) appendChildren(workspaceId, entry.path, id, level + 1);
    });
  };
  workspaces.forEach((workspace, index) => {
    const id = explorerNodeId(workspace.id, '');
    const root: WorkspaceRootNode = { id, name: workspace.name, path: '', type: 'workspace', workspace, hasChildren: true };
    const row: VisibleExplorerRow = { node: root, level: 0, positionInSet: index + 1, setSize: workspaces.length, workspaceId: workspace.id };
    if (!query || matchesRow(row, query)) rows.push(row);
    if (expandedNodeIds.has(id)) appendChildren(workspace.id, '', id, 1);
  });
  return rows;
}

function matchesRow(row: VisibleExplorerRow, query: string): boolean {
  return [row.node.name, row.node.path, row.item?.identifier, row.item?.title].some((value) => value?.toLocaleLowerCase().includes(query));
}

export function normalizeExplorerPath(path: string): string {
  return path.replaceAll('\\', '/').replace(/^\/+|\/+$/g, '');
}

export function isDirectoryEntry(node: VisibleExplorerRow['node']): node is WorkspaceTreeEntry {
  return node.type === 'directory';
}

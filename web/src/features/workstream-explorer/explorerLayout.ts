import { normalizeExplorerPath } from './tree';
import type { VisibleExplorerRow } from './types';

export function boundedNumber(value: string | null, fallback: number, bound: (value: number) => number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? bound(parsed) : fallback;
}

export function boundedLeftPanelWidth(value: number): number {
  return Math.min(640, Math.max(300, value));
}

export function boundedRightPanelWidth(value: number): number {
  return Math.min(520, Math.max(220, value));
}

export function tabKey(workspaceId: string, path: string): string {
  return `${workspaceId}:${path}`;
}

export function restrictRowsToRoot(rows: VisibleExplorerRow[], workspaceId: string | undefined, rootPath: string): VisibleExplorerRow[] {
  if (!workspaceId || !rootPath) return rows;
  const rootRow = rows.find((row) => row.workspaceId === workspaceId && normalizeExplorerPath(row.node.path) === rootPath);
  if (!rootRow) return [];
  const baseLevel = rootRow.level;
  return rows
    .filter((row) => row.workspaceId === workspaceId && (normalizeExplorerPath(row.node.path) === rootPath || normalizeExplorerPath(row.node.path).startsWith(`${rootPath}/`)))
    .map((row) => ({
      ...row,
      level: Math.max(0, row.level - baseLevel),
      parentId: normalizeExplorerPath(row.node.path) === rootPath ? undefined : row.parentId
    }));
}

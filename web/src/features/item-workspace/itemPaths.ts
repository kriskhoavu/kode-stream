import { firstFile } from '../../shared/domain/fileTree';
import type { FileContent, FileNode, GitChange, GitStatus, ItemDetail } from '../../lib/types';
import type { TreeFileState } from '../file-tree/FileStateIcon';

export function fileDirectoryPaths(nodes: FileNode[]): Set<string> {
  const paths = new Set<string>();
  const visit = (entries: FileNode[]) => entries.forEach((node) => {
    if (node.type !== 'directory') return;
    paths.add(node.path);
    visit(node.children ?? []);
  });
  visit(nodes);
  return paths;
}

export function hasFile(nodes: FileNode[]): boolean {
  return firstFile(nodes) !== null;
}

export function currentGitPath(plan: ItemDetail | null, file: FileContent | null): string {
  if (!plan?.itemPath || !file?.path) return '';
  return `${plan.itemPath.replace(/\/$/, '')}/${file.path.replace(/^\//, '')}`;
}

export function workspacePathForSelection(plan: ItemDetail | null, file: FileContent | null): string | undefined {
  const itemPath = normalizePath(plan?.itemPath ?? '');
  const filePath = normalizePath(file?.path ?? '');
  if (itemPath && filePath) return `${itemPath}/${filePath}`;
  return itemPath || undefined;
}

export function buildFileStateMap(plan: ItemDetail | null, gitStatus: GitStatus | null, file: FileContent | null, dirtyFile: boolean): Map<string, TreeFileState> {
  const stateByPath = new Map<string, TreeFileState>();
  const itemPath = normalizePath(plan?.itemPath ?? '');
  for (const change of gitStatus?.changes ?? []) {
    const localPath = localItemPath(itemPath, change);
    if (localPath) stateByPath.set(localPath, change.status);
  }
  if (dirtyFile && file?.path) {
    stateByPath.set(normalizePath(file.path), 'unsaved');
  }
  return stateByPath;
}

export function localItemPath(itemPath: string, change: GitChange): string {
  const path = normalizePath(change.path);
  const oldPath = normalizePath(change.oldPath ?? '');
  return stripItemPath(path, itemPath) || stripItemPath(oldPath, itemPath);
}

export function stripItemPath(path: string, itemPath: string): string {
  if (!path) return '';
  if (!itemPath) return path;
  if (path === itemPath) return '';
  return path.startsWith(`${itemPath}/`) ? path.slice(itemPath.length + 1) : '';
}

export function normalizePath(path: string): string {
  return path.replace(/^\/+/, '').replace(/\/+$/, '');
}

export function normalizeLocalPath(path: string): string {
  return path.trim().replace(/\/+$/, '');
}

export function parentPath(path: string): string {
  const parts = normalizePath(path).split('/').filter(Boolean);
  parts.pop();
  return parts.join('/');
}

export function isAutomationSpecPath(path: string): boolean {
  return /\.(?:cy|spec|test)\.(?:ts|tsx|js|jsx)$/i.test(normalizePath(path));
}

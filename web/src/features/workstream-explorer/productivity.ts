import type { WorkspacePathGitState } from '../../lib/types';
import { explorerNodeId, normalizeExplorerPath } from './tree';

const gitStatePriority: Record<WorkspacePathGitState['status'], number> = {
  conflicted: 7,
  deleted: 6,
  renamed: 5,
  added: 4,
  untracked: 3,
  modified: 2,
  copied: 1
};

export function buildWorkspaceGitStateMap(workspaceId: string, states: WorkspacePathGitState[]): Map<string, WorkspacePathGitState> {
  const result = new Map<string, WorkspacePathGitState>();
  const add = (path: string, state: WorkspacePathGitState) => {
    path = normalizeExplorerPath(path);
    while (path) {
      const key = explorerNodeId(workspaceId, path);
      const existing = result.get(key);
      if (!existing || gitStatePriority[state.status] > gitStatePriority[existing.status]) {
        result.set(key, { ...state, path });
      }
      const separator = path.lastIndexOf('/');
      path = separator >= 0 ? path.slice(0, separator) : '';
    }
  };
  for (const state of states) {
    add(state.path, state);
    if (state.oldPath) add(state.oldPath, state);
  }
  return result;
}

export function ancestorDirectoryPaths(path: string, targetType: 'file' | 'directory' = 'file'): string[] {
  const segments = normalizeExplorerPath(path).split('/').filter(Boolean);
  const count = targetType === 'directory' ? segments.length : Math.max(segments.length - 1, 0);
  return ['', ...segments.slice(0, count).map((_, index) => segments.slice(0, index + 1).join('/'))];
}

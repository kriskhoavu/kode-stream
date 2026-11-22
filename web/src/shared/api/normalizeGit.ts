import type { GitActivityEntry, GitOperationResult, GitStatus } from '../../lib/types';

export function normalizeGitStatus(status: GitStatus): GitStatus {
  return {
    ...status,
    ahead: status.ahead ?? 0,
    behind: status.behind ?? 0,
    dirty: Boolean(status.dirty),
    conflicted: Boolean(status.conflicted),
    changes: Array.isArray(status.changes) ? status.changes : []
  };
}

export function normalizeGitActivityEntry(entry: GitActivityEntry): GitActivityEntry {
  return {
    ...entry,
    committedAt: entry.committedAt ?? '',
    author: entry.author ?? '',
    message: entry.message ?? '',
    paths: Array.isArray(entry.paths) ? entry.paths : []
  };
}

export function normalizeGitResult(result: GitOperationResult): GitOperationResult {
  return {
    ...result,
    ok: Boolean(result.ok),
    status: normalizeGitStatus(result.status ?? { workspaceId: '', branch: '', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] })
  };
}

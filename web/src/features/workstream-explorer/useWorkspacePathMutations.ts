import { useState } from 'react';
import { api } from '../../lib/api';
import type { WorkspaceDirectoryCreateInput, WorkspaceFileCreateInput, WorkspacePathMutationResult, WorkspacePathRenameInput } from '../../lib/types';

export function useWorkspacePathMutations(onSuccess: (result: WorkspacePathMutationResult) => void | Promise<void>) {
  const [busy, setBusy] = useState<'file' | 'directory' | 'rename' | ''>('');
  const [error, setError] = useState('');

  const run = async (kind: Exclude<typeof busy, ''>, operation: () => Promise<WorkspacePathMutationResult>) => {
    setBusy(kind);
    setError('');
    try {
      const result = await operation();
      await onSuccess(result);
      return result;
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'Workspace path operation failed');
      return null;
    } finally {
      setBusy('');
    }
  };

  return {
    busy,
    error,
    clearError: () => setError(''),
    createFile: (workspaceId: string, input: WorkspaceFileCreateInput) => run('file', () => api.createWorkspaceFile(workspaceId, input)),
    createDirectory: (workspaceId: string, input: WorkspaceDirectoryCreateInput) => run('directory', () => api.createWorkspaceDirectory(workspaceId, input)),
    rename: (workspaceId: string, input: WorkspacePathRenameInput) => run('rename', () => api.renameWorkspacePath(workspaceId, input))
  };
}

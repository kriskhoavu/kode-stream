import { useCallback, useEffect, useState } from 'react';
import { ApiError, api } from '../../lib/api';
import type { WorkspaceBranches, WorkspaceConfig } from '../../lib/types';

export interface WorkspaceBranchState extends WorkspaceBranches {
  loading: boolean;
  switching: boolean;
  error: string;
  recoveryHint: string;
}

export function useWorkspaceBranches(workspaces: WorkspaceConfig[], onSwitched?: (workspaceId: string, branch: string) => void | Promise<void>) {
  const [states, setStates] = useState<Record<string, WorkspaceBranchState>>({});
  const workspaceKey = workspaces.map((workspace) => workspace.id).join('\u0000');

  const load = useCallback(async (workspace: WorkspaceConfig) => {
    setStates((current) => ({ ...current, [workspace.id]: { ...fallbackState(workspace), ...current[workspace.id], loading: true, error: '', recoveryHint: '' } }));
    try {
      const response = await api.workspaceBranches(workspace.id);
      setStates((current) => ({ ...current, [workspace.id]: { ...response, loading: false, switching: current[workspace.id]?.switching ?? false, error: '', recoveryHint: '' } }));
    } catch (caught) {
      setStates((current) => ({
        ...current,
        [workspace.id]: {
          ...fallbackState(workspace),
          ...current[workspace.id],
          loading: false,
          switching: false,
          error: caught instanceof Error ? caught.message : 'Branches failed to load',
          recoveryHint: caught instanceof ApiError ? caught.recoveryHint ?? '' : ''
        }
      }));
    }
  }, []);

  useEffect(() => {
    const ids = new Set(workspaces.map((workspace) => workspace.id));
    setStates((current) => Object.fromEntries(Object.entries(current).filter(([workspaceId]) => ids.has(workspaceId))));
    workspaces.forEach((workspace) => void load(workspace));
  }, [load, workspaceKey]);

  const switchBranch = useCallback(async (workspace: WorkspaceConfig, branch: string) => {
    const current = states[workspace.id];
    if (!branch || branch === current?.current || current?.switching) return branch === current?.current;
    setStates((value) => ({ ...value, [workspace.id]: { ...(value[workspace.id] ?? fallbackState(workspace)), switching: true, error: '', recoveryHint: '' } }));
    try {
      const result = await switchBranchWithConfirmation(workspace.id, branch);
      if (!result.ok) {
        throw new ApiError(result.message ?? 'Branch switch failed', result.recoveryHint);
      }
      await onSwitched?.(workspace.id, result.status.branch || branch);
      setStates((value) => ({
        ...value,
        [workspace.id]: {
          ...(value[workspace.id] ?? fallbackState(workspace)),
          current: result.status.branch || branch,
          switching: false,
          error: '',
          recoveryHint: ''
        }
      }));
      await load(workspace);
      return true;
    } catch (caught) {
      setStates((value) => ({
        ...value,
        [workspace.id]: {
          ...(value[workspace.id] ?? fallbackState(workspace)),
          switching: false,
          error: caught instanceof Error ? caught.message : 'Branch switch failed',
          recoveryHint: caught instanceof ApiError ? caught.recoveryHint ?? '' : ''
        }
      }));
      return false;
    }
  }, [load, onSwitched, states]);

  return { states, load, switchBranch };
}

function fallbackState(workspace: WorkspaceConfig): WorkspaceBranchState {
  return {
    workspaceId: workspace.id,
    current: workspace.baselineBranch,
    branches: workspace.baselineBranch ? [workspace.baselineBranch] : [],
    loading: false,
    switching: false,
    error: '',
    recoveryHint: ''
  };
}

async function switchBranchWithConfirmation(workspaceId: string, branch: string) {
  try {
    return await api.switchBranch(workspaceId, { name: branch, confirm: false });
  } catch (caught) {
    if (!(caught instanceof ApiError) || !requiresDirtyTreeConfirmation(caught)) throw caught;
    const confirmed = window.confirm(`${caught.message}\n\n${caught.recoveryHint || 'Review local changes before switching branches.'}`);
    if (!confirmed) throw caught;
    return api.switchBranch(workspaceId, { name: branch, confirm: true });
  }
}

function requiresDirtyTreeConfirmation(error: ApiError): boolean {
  const text = `${error.message} ${error.recoveryHint ?? ''}`.toLocaleLowerCase();
  return text.includes('confirm to switch') || (text.includes('local changes') && text.includes('confirm'));
}

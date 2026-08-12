import { useCallback, useEffect, useState } from 'react';
import { ApiError, api } from '../../shared/api';
import type { WorkspaceBranches, WorkspaceConfig } from '../../lib/types';

export interface WorkspaceBranchState extends WorkspaceBranches {
  loading: boolean;
  switching: boolean;
  error: string;
  recoveryHint: string;
}
export interface BranchSwitchDecision { workspace: WorkspaceConfig; target: string; canCarry: boolean; stashMessage: string }

export function useWorkspaceBranches(workspaces: WorkspaceConfig[], onSwitched?: (workspaceId: string, branch: string) => void | Promise<void>) {
  const [states, setStates] = useState<Record<string, WorkspaceBranchState>>({});
	const [decision, setDecision] = useState<BranchSwitchDecision | null>(null);
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

  useEffect(() => {
    const refreshBranches = () => workspaces.forEach((workspace) => void load(workspace));
    const refreshVisibleBranches = () => {
      if (document.visibilityState === 'visible') refreshBranches();
    };
    window.addEventListener('focus', refreshBranches);
    document.addEventListener('visibilitychange', refreshVisibleBranches);
    return () => {
      window.removeEventListener('focus', refreshBranches);
      document.removeEventListener('visibilitychange', refreshVisibleBranches);
    };
  }, [load, workspaceKey]);

  const switchBranch = useCallback(async (workspace: WorkspaceConfig, branch: string) => {
    const current = states[workspace.id];
    if (!branch || branch === current?.current || current?.switching) return branch === current?.current;
    setStates((value) => ({ ...value, [workspace.id]: { ...(value[workspace.id] ?? fallbackState(workspace)), switching: true, error: '', recoveryHint: '' } }));
    try {
		const result = await api.switchBranch(workspace.id, { name: branch });
      if (!result.ok) {
        throw new ApiError(result.message ?? 'Branch switch failed', result.recoveryHint);
      }
      await onSwitched?.(workspace.id, result.status.branch || branch);
		const refreshHint = result.refreshRequired
			? (result.refreshError ? `Git completed, but refresh is required: ${result.refreshError}` : 'Git completed. Reload the workspace to refresh it.')
			: '';
      setStates((value) => ({
        ...value,
        [workspace.id]: {
          ...(value[workspace.id] ?? fallbackState(workspace)),
          current: result.status.branch || branch,
          switching: false,
          error: '',
	          recoveryHint: refreshHint
        }
      }));
      await load(workspace);
      return true;
		} catch (caught) {
			if (caught instanceof ApiError && caught.code === 'branch_switch_decision_required') {
				setDecision({ workspace, target: branch, canCarry: caught.details?.canCarryChanges === 'true', stashMessage: `Kode Stream: stash ${caught.details?.sourceBranch || current?.current || workspace.baselineBranch} before switching to ${branch}` });
				setStates((value) => ({ ...value, [workspace.id]: { ...(value[workspace.id] ?? fallbackState(workspace)), switching: false, error: '', recoveryHint: '' } }));
				return false;
			}
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

	const resolveDecision = useCallback(async (strategy: 'carry' | 'stash', stashMessage: string) => {
		if (!decision) return false;
		const { workspace, target } = decision;
		setDecision(null);
		setStates((value) => ({ ...value, [workspace.id]: { ...(value[workspace.id] ?? fallbackState(workspace)), switching: true, error: '', recoveryHint: '' } }));
		try {
			const result = await api.switchBranch(workspace.id, { name: target, strategy, stashMessage: strategy === 'stash' ? stashMessage : undefined });
			await onSwitched?.(workspace.id, result.status.branch || target);
			setStates((value) => ({ ...value, [workspace.id]: { ...(value[workspace.id] ?? fallbackState(workspace)), current: result.status.branch || target, switching: false, error: '', recoveryHint: '' } }));
			await load(workspace);
			return true;
		} catch (caught) { setStates((value) => ({ ...value, [workspace.id]: { ...(value[workspace.id] ?? fallbackState(workspace)), switching: false, error: caught instanceof Error ? caught.message : 'Branch switch failed', recoveryHint: caught instanceof ApiError ? caught.recoveryHint ?? '' : '' } })); return false; }
	}, [decision, load, onSwitched]);

	return { states, load, switchBranch, decision, resolveDecision, cancelDecision: () => setDecision(null) };
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

import type { WorkspaceConfig } from '../../lib/types';
import type { WorkspaceBranchState } from './useWorkspaceBranches';
import './workspace-branch-selector.css';

export function WorkspaceBranchSelector({ workspace, state, onChange }: {
  workspace: WorkspaceConfig;
  state?: WorkspaceBranchState;
  onChange: (branch: string) => void;
}) {
  const current = state?.current || workspace.baselineBranch;
  const branches = state?.branches.length ? state.branches : current ? [current] : [];
  const errorDetail = [state?.error, state?.recoveryHint].filter(Boolean).join(' ');
  return <span className="workspace-branch-control" onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}>
    <select
      aria-label={`Branch for ${workspace.name}`}
      value={current}
      disabled={state?.loading || state?.switching}
      title={errorDetail || `Current branch: ${current}`}
      onChange={(event) => onChange(event.target.value)}
    >
      {branches.map((branch) => <option key={branch} value={branch}>{branch}</option>)}
    </select>
    {state?.switching && <small role="status">Switching…</small>}
    {state?.error && <small className="workspace-branch-error" role="alert" title={errorDetail}>!</small>}
  </span>;
}

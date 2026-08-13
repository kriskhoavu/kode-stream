import { useState, type CSSProperties } from 'react';
import type { WorkspaceConfig } from '../../lib/types';
import type { WorkspaceBranchState } from './useWorkspaceBranches';
import { useModalDialog } from '../../components/overlay';
import './workspace-branch-selector.css';

export function WorkspaceBranchSelector({ workspace, state, onChange, decision, onDecision, onCancelDecision }: {
  workspace: WorkspaceConfig;
  state?: WorkspaceBranchState;
  onChange: (branch: string) => void;
  decision?: { workspace: WorkspaceConfig; target: string; canCarry: boolean; stashMessage: string } | null;
  onDecision?: (strategy: 'carry' | 'stash', stashMessage: string) => void;
  onCancelDecision?: () => void;
}) {
	const [stashMessage, setStashMessage] = useState('');
	const activeDecision = decision?.workspace.id === workspace.id ? decision : null;
	const dialogOpen = activeDecision !== null;
	const dialog = useModalDialog(() => onCancelDecision?.(), true, dialogOpen);
  const current = state?.current || workspace.baselineBranch;
  const branches = state?.branches.length ? state.branches : current ? [current] : [];
  const errorDetail = [state?.error, state?.recoveryHint].filter(Boolean).join(' ');
  const style = { '--branch-name-length': current.length } as CSSProperties & Record<'--branch-name-length', number>;
  return <><span className="workspace-branch-control" style={style} onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}>
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
  </span>{activeDecision && <div className="confirm-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onCancelDecision?.(); }}><section ref={dialog.ref} className="confirm-dialog branch-switch-dialog" role="dialog" aria-modal="true" aria-labelledby={dialog.titleId}><header><h2 id={dialog.titleId}>Protect local changes</h2></header><p>Choose how to protect changes before switching to {activeDecision.target}.</p><label>Stash message<input data-autofocus value={stashMessage || activeDecision.stashMessage} onChange={(event) => setStashMessage(event.target.value)} /></label><footer><button type="button" onClick={onCancelDecision}>Cancel</button>{activeDecision.canCarry && <button type="button" onClick={() => onDecision?.('carry', '')}>Move changes and switch</button>}<button type="button" onClick={() => onDecision?.('stash', stashMessage || activeDecision.stashMessage)}>Stash and switch</button></footer></section></div>}</>;
}

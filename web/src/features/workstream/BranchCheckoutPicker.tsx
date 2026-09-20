import { useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, KeyboardEvent as ReactKeyboardEvent } from 'react';
import { ChevronDown, GitBranch, GitCommitHorizontal, Search, ArchiveRestore } from 'lucide-react';
import { ApiError, api } from '../../shared/api';
import type { GitStashEntry, GitStatus } from '../../lib/types';
import { useModalDialog } from '../../components/overlay';

export function BranchCheckoutPicker({ workspaceId, currentCheckoutBranch, branches, gitStatus, disabled = false, ariaLabel, listboxLabel, onSwitched }: {
  workspaceId: string; currentCheckoutBranch: string; branches: string[]; gitStatus?: GitStatus | null; disabled?: boolean; ariaLabel: string; listboxLabel: string; onSwitched: () => void | Promise<void>;
}) {
  const [open, setOpen] = useState(false); const [search, setSearch] = useState(''); const [activeIndex, setActiveIndex] = useState(0);
  const [busy, setBusy] = useState(false); const [decision, setDecision] = useState<{ target: string; canCarry: boolean } | null>(null);
  const [stashMessage, setStashMessage] = useState(''); const [error, setError] = useState('');
  const [stashesOpen, setStashesOpen] = useState(false); const [stashes, setStashes] = useState<GitStashEntry[]>([]); const [stashLoading, setStashLoading] = useState(false);
	const [resolvedGitStatus, setResolvedGitStatus] = useState<GitStatus | null>(gitStatus ?? null);
	const decisionDialog = useModalDialog(() => setDecision(null), !busy, Boolean(decision));
	const stashesDialog = useModalDialog(() => setStashesOpen(false), !stashLoading, stashesOpen);
  const pickerRef = useRef<HTMLDivElement | null>(null); const optionRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const generationRef = useRef(0); const stashesAbortRef = useRef<AbortController | null>(null);
  const gitState = checkoutGitState(resolvedGitStatus);
  const options = useMemo(() => { const query = search.trim().toLowerCase(); const all = order(unique([...branches, currentCheckoutBranch]), currentCheckoutBranch); const pinned = all.filter((branch) => primary(branch) || branch === currentCheckoutBranch); return order(unique([...pinned, ...all.filter((branch) => !query || branch.toLowerCase().includes(query))]), currentCheckoutBranch); }, [branches, currentCheckoutBranch, search]);
  useEffect(() => { if (!open) return; const outside = (event: PointerEvent) => { if (!pickerRef.current?.contains(event.target as Node | null)) { setOpen(false); setSearch(''); } }; const escape = (event: KeyboardEvent) => { if (event.key === 'Escape') { setOpen(false); setSearch(''); } }; document.addEventListener('pointerdown', outside); document.addEventListener('keydown', escape); return () => { document.removeEventListener('pointerdown', outside); document.removeEventListener('keydown', escape); }; }, [open]);
  useEffect(() => { if (open) setActiveIndex(Math.max(0, options.indexOf(currentCheckoutBranch))); }, [currentCheckoutBranch, open, options]);
  useEffect(() => { if (open) optionRefs.current[activeIndex]?.scrollIntoView?.({ block: 'nearest' }); }, [activeIndex, open]);
  useEffect(() => {
    const generation = ++generationRef.current;
    return () => { if (generationRef.current === generation) generationRef.current += 1; stashesAbortRef.current?.abort(); };
  }, [workspaceId]);
  useEffect(() => {
    if (gitStatus !== undefined) { setResolvedGitStatus(gitStatus); return; }
    const controller = new AbortController();
    setResolvedGitStatus(null);
    void (async () => {
      try {
        const status = await api.gitStatus(workspaceId, controller.signal);
        if (!controller.signal.aborted) setResolvedGitStatus(status);
      } catch {
        if (!controller.signal.aborted) setResolvedGitStatus(null);
      }
    })();
    return () => controller.abort();
  }, [currentCheckoutBranch, gitStatus, workspaceId]);
  const keys = (event: ReactKeyboardEvent) => { if (event.key === 'ArrowDown') { event.preventDefault(); setActiveIndex((value) => options.length ? (value + 1) % options.length : 0); } else if (event.key === 'ArrowUp') { event.preventDefault(); setActiveIndex((value) => options.length ? (value - 1 + options.length) % options.length : 0); } else if (event.key === 'Home') { event.preventDefault(); setActiveIndex(0); } else if (event.key === 'End') { event.preventDefault(); setActiveIndex(Math.max(0, options.length - 1)); } else if (event.key === 'Enter' && options[activeIndex]) { event.preventDefault(); void select(options[activeIndex]); } };
  const execute = async (target: string, strategy?: 'carry' | 'stash') => { const generation = generationRef.current; const isCurrent = () => generationRef.current === generation; setBusy(true); setError(''); try { await api.switchBranch(workspaceId, { name: target, strategy, stashMessage: strategy === 'stash' ? stashMessage.trim() : undefined }); if (!isCurrent()) return; setDecision(null); setOpen(false); setSearch(''); await onSwitched(); } catch (caught) { if (!isCurrent()) return; if (caught instanceof ApiError && caught.code === 'branch_switch_decision_required' && !strategy) { const canCarry = caught.details?.canCarryChanges === 'true'; setDecision({ target, canCarry }); setStashMessage(`Kode Stream: stash ${caught.details?.sourceBranch || currentCheckoutBranch} before switching to ${target}`); return; } setError(caught instanceof Error ? caught.message : 'Branch switch failed'); } finally { if (isCurrent()) setBusy(false); } };
  const select = async (target: string) => { if (target !== currentCheckoutBranch && !busy) await execute(target); };
	const openStashes = async () => { stashesAbortRef.current?.abort(); const controller = new AbortController(); stashesAbortRef.current = controller; const generation = generationRef.current; setStashesOpen(true); setStashLoading(true); setError(''); try { const next = await api.stashes(workspaceId, controller.signal); if (!controller.signal.aborted && generationRef.current === generation) setStashes(next); } catch (caught) { if (!controller.signal.aborted && generationRef.current === generation) setError(caught instanceof Error ? caught.message : 'Stashes failed to load'); } finally { if (!controller.signal.aborted && generationRef.current === generation) setStashLoading(false); if (stashesAbortRef.current === controller) stashesAbortRef.current = null; } };
	const applyStash = async (stash: GitStashEntry) => { if (!window.confirm(`Apply ${stash.ref}? The stash will be kept so it can be recovered if Git reports conflicts.`)) return; const generation = generationRef.current; const isCurrent = () => generationRef.current === generation; setStashLoading(true); setError(''); try { const result = await api.applyStash(workspaceId, stash.ref); if (!isCurrent()) return; if (!result.ok) throw new Error(result.message || 'Stash failed to apply'); setStashesOpen(false); await onSwitched(); } catch (caught) { if (isCurrent()) setError(caught instanceof Error ? caught.message : 'Stash failed to apply'); } finally { if (isCurrent()) setStashLoading(false); } };
  return <>
    <div className="branch-checkout-control">
    <div className="branch-context-chip" aria-label={`Checkout: ${currentCheckoutBranch}`} style={{ '--branch-name-length': currentCheckoutBranch.length } as CSSProperties & Record<'--branch-name-length', number>}>
      <span className={`branch-git-status-icon is-${gitState.state}`} role="img" aria-label={gitState.label} title={gitState.label}><GitBranch size={14} aria-hidden="true" /></span><span>Checkout</span><div className="branch-selector-wrap" ref={pickerRef}><button type="button" className="branch-picker-trigger" aria-label={ariaLabel} aria-haspopup="listbox" aria-expanded={open} disabled={disabled || busy} onKeyDown={(event) => { if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); setOpen(true); setSearch(''); } }} onClick={() => { setOpen((value) => !value); setSearch(''); }}><span>{currentCheckoutBranch}</span><ChevronDown size={14} /></button>
      {open && <div className="branch-picker-menu"><label className="branch-picker-search"><Search size={14} /><input value={search} onChange={(event) => setSearch(event.target.value)} onKeyDown={keys} placeholder="Search branches..." aria-label="Search branches" autoFocus /></label><div className="branch-picker-options" role="listbox" aria-label={listboxLabel} onKeyDown={keys}>{options.map((branch, index) => <button type="button" role="option" aria-selected={branch === currentCheckoutBranch} data-active={index === activeIndex ? 'true' : undefined} ref={(element) => { optionRefs.current[index] = element; }} key={branch} title={branch === currentCheckoutBranch ? 'Current checkout branch' : undefined} onMouseEnter={() => setActiveIndex(index)} onClick={() => void select(branch)}><span className="branch-option-checkout-slot">{branch === currentCheckoutBranch && <GitCommitHorizontal className="branch-option-checkout" size={14} />}</span><span className="branch-option-label">{branch}</span></button>)}{options.length === 0 && <span className="branch-picker-empty">No branches found</span>}</div></div>}</div>
    </div><button className="branch-stashes-button" type="button" aria-label="View stashes" title="View and apply stashes" disabled={busy} onClick={() => void openStashes()}><ArchiveRestore size={15} /></button>{error && <span className="branch-switch-error" role="alert">{error}</span>}</div>
    {decision && <div className="confirm-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) setDecision(null); }}><section ref={decisionDialog.ref} className="confirm-dialog branch-switch-dialog" role="dialog" aria-modal="true" aria-labelledby={decisionDialog.titleId}><header><h2 id={decisionDialog.titleId}>Protect local changes</h2><button data-autofocus className="icon-button" type="button" aria-label="Close dialog" disabled={busy} onClick={() => setDecision(null)}>×</button></header><p>Switching from {currentCheckoutBranch} to {decision.target} has local changes. Choose how to protect them.</p><label className="branch-stash-message"><span>Stash message</span><input value={stashMessage} disabled={busy} onChange={(event) => setStashMessage(event.target.value)} /></label><footer><button className="ghost" type="button" disabled={busy} onClick={() => setDecision(null)}>Cancel</button>{decision.canCarry && <button className="secondary" type="button" disabled={busy} onClick={() => void execute(decision.target, 'carry')}>Move changes and switch</button>}<button className="primary" type="button" disabled={busy || !stashMessage.trim()} onClick={() => void execute(decision.target, 'stash')}>{busy ? 'Switching…' : 'Stash and switch'}</button></footer></section></div>}
    {stashesOpen && <div className="confirm-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !stashLoading) setStashesOpen(false); }}><section ref={stashesDialog.ref} className="confirm-dialog stash-dialog" role="dialog" aria-modal="true" aria-labelledby={stashesDialog.titleId}><header><h2 id={stashesDialog.titleId}>Stashes</h2><button data-autofocus className="icon-button" type="button" aria-label="Close dialog" disabled={stashLoading} onClick={() => setStashesOpen(false)}>×</button></header>{stashLoading && <p>Loading stashes…</p>}{!stashLoading && stashes.length === 0 && <p>No stashes for this workspace.</p>}{!stashLoading && stashes.map((stash) => <div className="stash-entry" key={stash.ref}><div><code>{stash.ref}</code><span>{stash.message}</span></div><button className="secondary" type="button" onClick={() => void applyStash(stash)}>Apply</button></div>)}<footer><button className="ghost" type="button" disabled={stashLoading} onClick={() => setStashesOpen(false)}>Close</button></footer></section></div>}
  </>;
}
function primary(branch: string) { const name = branch.toLowerCase(); return name === 'main' || name === 'master'; }
function order(values: string[], current: string) { return [...values].sort((a, b) => { const rank = (branch: string) => branch.toLowerCase() === 'main' ? 0 : branch.toLowerCase() === 'master' ? 1 : branch === current ? 2 : 3; return rank(a) - rank(b) || a.localeCompare(b); }); }
function unique(values: string[]) { return Array.from(new Set(values.filter(Boolean))).sort((a, b) => a.localeCompare(b)); }

function checkoutGitState(git?: GitStatus | null): { state: 'synced' | 'not-synced' | 'unknown'; label: string } {
  if (!git?.upstream) return { state: 'unknown', label: 'Git sync status unavailable' };
  const drift = [git.ahead > 0 ? `${git.ahead} ahead` : '', git.behind > 0 ? `${git.behind} behind` : ''].filter(Boolean).join(', ');
  if (drift) return { state: 'not-synced', label: `Git not in sync with ${git.upstream}: ${drift}` };
  if (!hasFetched(git.fetchedAt)) return { state: 'unknown', label: `Git sync with ${git.upstream} has not been checked` };
  return { state: 'synced', label: `Git in sync with ${git.upstream}` };
}

function hasFetched(fetchedAt?: string): boolean {
  if (!fetchedAt) return false;
  const stamp = new Date(fetchedAt).getTime();
  return Number.isFinite(stamp) && stamp >= Date.UTC(2000, 0, 1);
}

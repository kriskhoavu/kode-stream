import type { ExplorerUnifiedResult, PathDialog } from './explorerPageTypes';
import { useMemo, useState } from 'react';
import type { CSSProperties, RefObject } from 'react';
import { ChevronDown, ChevronRight, File, Folder, FolderGit2, RotateCcw, X } from 'lucide-react';
import { useModalDialog } from '../../components/overlay';
import { FileStateIcon } from '../file-tree/FileStateIcon';
import type { VisibleExplorerRow } from './types';
import { WorkspaceBranchSelector } from './WorkspaceBranchSelector';
import type { WorkspaceBranchState } from './useWorkspaceBranches';
import type { WorkspaceConfig, WorkspacePathGitState } from '../../lib/types';
import { parseGitDiff } from '../../shared/domain/diff';
import { ContentSearchResultRow, PathSearchResultRow } from '../content-search/ContentSearch';
import { ModalLayer } from '../../components/ModalLayer';

export function ExplorerTreeRow({ row, gitState, branchState, decision, onDecision, onCancelDecision, showBranchSelector = true, active, selected, expanded, onFocus, onSelect, onToggle, onBranchChange }: { row: VisibleExplorerRow; gitState?: WorkspacePathGitState; branchState?: WorkspaceBranchState; decision?: { workspace: WorkspaceConfig; target: string; canCarry: boolean; stashMessage: string } | null; onDecision: (strategy: 'carry' | 'stash', message: string) => void; onCancelDecision: () => void; showBranchSelector?: boolean; active: boolean; selected: boolean; expanded: boolean; onFocus: () => void; onSelect: () => void; onToggle: () => void; onBranchChange: (workspace: WorkspaceConfig, branch: string) => void }) {
  const expandable = row.node.type === 'workspace' || row.node.type === 'directory';
  const workspace = row.node.type === 'workspace' ? row.node.workspace : undefined;
  return <div className={`explorer-tree-row${selected ? ' selected' : ''}${active ? ' active' : ''}`} role="treeitem" aria-level={row.level + 1} aria-expanded={expandable ? expanded : undefined} aria-selected={selected} style={{ '--explorer-depth': row.level } as CSSProperties} onMouseEnter={onFocus}>
    <button className="explorer-row-toggle" type="button" tabIndex={-1} onClick={onToggle} disabled={!expandable} aria-label={expandable ? `${expanded ? 'Collapse' : 'Expand'} ${row.node.name}` : undefined}>{expandable ? (expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />) : null}</button>
    <button className="explorer-row-main" type="button" tabIndex={active ? 0 : -1} onFocus={onFocus} onClick={onSelect} onDoubleClick={expandable ? onToggle : undefined}>
      {row.node.type === 'workspace' ? <FolderGit2 className="explorer-node-icon" size={16} /> : row.node.type === 'directory' ? <Folder className="explorer-node-icon" size={16} /> : <File className="explorer-node-icon" size={16} />}
      <span className={`explorer-row-label ${row.node.type}`}>{row.node.name}</span>
      {row.node.type === 'file' && gitState && <FileStateIcon state={gitState.conflict ? 'conflicted' : gitState.status} />}
    </button>
    {workspace && showBranchSelector && <WorkspaceBranchSelector workspace={workspace} state={branchState} onChange={(branch) => onBranchChange(workspace, branch)} decision={decision} onDecision={onDecision} onCancelDecision={onCancelDecision} />}
  </div>;
}

export function ExplorerUnifiedSearchResults({ query, results, loading, error, activeIndex, onActiveIndex, onOpen }: { query: string; results: ExplorerUnifiedResult[]; loading: boolean; error: string; activeIndex: number; onActiveIndex: (index: number) => void; onOpen: (result: ExplorerUnifiedResult) => void }) {
	const onKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (!results.length) return;
    if (event.key === 'ArrowDown') { event.preventDefault(); onActiveIndex(Math.min(activeIndex + 1, results.length - 1)); }
    if (event.key === 'ArrowUp') { event.preventDefault(); onActiveIndex(Math.max(activeIndex - 1, 0)); }
    if (event.key === 'Enter') { event.preventDefault(); onOpen(results[activeIndex] ?? results[0]); }
  };
	return <div className="content-search-results explorer-unified-results" role="listbox" aria-label="File search results" tabIndex={0} onKeyDown={onKeyDown}>
		<div className="content-search-live" aria-live="polite">{loading ? 'Searching files…' : `${results.length} matches`}</div>
		{error && <p className="content-search-message error">{error}</p>}
		{!loading && !error && results.length === 0 && <p className="content-search-message">No matching files or text.</p>}
		{results.map((entry, index) => entry.kind === 'path' ? <PathSearchResultRow key={`path:${entry.result.id}`} result={entry.result} query={query} active={index === activeIndex} onActive={() => onActiveIndex(index)} onOpen={() => onOpen(entry)} /> : <ContentSearchResultRow key={`content:${entry.result.id}`} result={entry.result} query={query} active={index === activeIndex} onActive={() => onActiveIndex(index)} onOpen={() => onOpen(entry)} />)}
	</div>;
}

export function ExplorerPathDialog({ dialog, busy, error, onCancel, onSubmit }: { dialog: PathDialog; busy: boolean; error: string; onCancel: () => void; onSubmit: (name: string) => Promise<boolean> }) {
  const [name, setName] = useState(dialog.initialName ?? '');
  const title = dialog.kind === 'file' ? 'Create file' : dialog.kind === 'directory' ? 'Create directory' : 'Rename path';
  const modal = useModalDialog(onCancel, !busy);
  return <ModalLayer><div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onCancel(); }}><section ref={modal.ref as RefObject<HTMLElement>} className="explorer-path-dialog" role="dialog" aria-modal="true" aria-labelledby="explorer-path-dialog-title">
    <header><h2 id="explorer-path-dialog-title">{title}</h2><button className="icon-button" onClick={onCancel} aria-label="Close"><X size={16} /></button></header>
    <p>Parent: {dialog.parentPath || 'workspace root'}</p>
    <label>Name<input data-autofocus value={name} onChange={(event) => setName(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter' && name.trim()) void onSubmit(name.trim()); }} /></label>
    {error && <p className="error">{error}</p>}
    <footer><button className="ghost" disabled={busy} onClick={onCancel}>Cancel</button><button className="primary" disabled={busy || !name.trim()} onClick={() => void onSubmit(name.trim())}>{busy ? 'Saving...' : dialog.kind === 'rename' ? 'Rename' : 'Create'}</button></footer>
  </section></div></ModalLayer>;
}

export function ExplorerEmpty({ row }: { row?: VisibleExplorerRow }) {
  return <div className="document-empty"><Folder size={24} /><strong>{row ? row.node.name : 'Select a file'}</strong><span>{row?.node.type === 'directory' ? 'Expand this directory or choose a file.' : 'Choose a file from a workspace tree.'}</span></div>;
}

export function ExplorerDiff({ diff, onRevert, disabled }: { diff: string; onRevert: () => void; disabled: boolean }) {
  const files = useMemo(() => parseGitDiff(diff), [diff]);
  return <section className="diff-panel"><header className="diff-toolbar"><strong>{files.length ? `${files.length} changed file` : 'No local changes'}</strong><button className="danger-action" disabled={disabled || !diff} onClick={onRevert}><RotateCcw size={15} /> Revert File</button></header><pre className="diff-view">{diff || 'No local changes.'}</pre></section>;
}

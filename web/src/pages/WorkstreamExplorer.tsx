import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties } from 'react';
import {
  ChevronDown, ChevronRight, Clipboard, Code2, Eye, File, Folder, FolderGit2, GitCompare,
  FilePlus2, FolderPlus, KanbanSquare as WorkstreamIcon, PanelRightClose, PanelRightOpen, Pencil, RefreshCw, RotateCcw, Search, X
} from 'lucide-react';
import type { ExplorerLocation } from '../app/router';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { RecentGitActivity } from '../components/RecentGitActivity';
import { ContentViewer } from '../features/content-viewer/ContentViewer';
import { autoSaveLabel, useFileEditorSession } from '../features/file-editor/useFileEditorSession';
import { FileStateIcon } from '../features/file-tree/FileStateIcon';
import { treeKeyboardAction } from '../features/workstream-explorer/keyboard';
import { explorerNodeId } from '../features/workstream-explorer/tree';
import type { VisibleExplorerRow } from '../features/workstream-explorer/types';
import { useWorkstreamExplorer } from '../features/workstream-explorer/useWorkstreamExplorer';
import { WorkspaceBranchSelector } from '../features/workstream-explorer/WorkspaceBranchSelector';
import { useWorkspaceBranches } from '../features/workstream-explorer/useWorkspaceBranches';
import type { WorkspaceBranchState } from '../features/workstream-explorer/useWorkspaceBranches';
import { useWorkspacePathSearch } from '../features/workstream-explorer/useWorkspacePathSearch';
import { useWorkspacePathMutations } from '../features/workstream-explorer/useWorkspacePathMutations';
import { ApiError, api } from '../lib/api';
import type { GitActivityEntry, GitStatus, ItemSummary, WorkspaceConfig, WorkspaceHealth, WorkspacePathGitState, WorkspacePathSearchResult } from '../lib/types';
import { parseGitDiff } from '../shared/domain/diff';
import { ContentSearchResultRow, PathSearchResultRow } from '../features/content-search/ContentSearch';
import { useContentSearch } from '../features/content-search/useContentSearch';
import type { ContentSearchSelection, WorkspaceContentSearchResult } from '../lib/types';

type EditorTab = 'preview' | 'raw' | 'diff';
type PathDialog = { kind: 'file' | 'directory' | 'rename'; parentPath: string; currentPath?: string; initialName?: string };
type ExplorerUnifiedResult = { kind: 'path'; result: WorkspacePathSearchResult } | { kind: 'content'; result: WorkspaceContentSearchResult };

export function WorkstreamExplorer({ workspaces, location, onLocationChange, onOpenWorkstream }: {
  workspaces: WorkspaceConfig[];
  location?: ExplorerLocation;
  onLocationChange: (location?: ExplorerLocation) => void;
  onOpenWorkstream: (workspace: WorkspaceConfig, itemId?: string) => void;
}) {
  const explorer = useWorkstreamExplorer(workspaces, location, onLocationChange);
  const [tab, setTab] = useState<EditorTab>('preview');
  const [diff, setDiff] = useState('');
  const [error, setError] = useState('');
  const [recoveryHint, setRecoveryHint] = useState('');
  const [revertOpen, setRevertOpen] = useState(false);
  const [reverting, setReverting] = useState(false);
  const [inspectorOpen, setInspectorOpen] = useState(false);
  const [leftWidth, setLeftWidth] = useState(() => boundedNumber(localStorage.getItem('workstreamExplorer.leftWidth'), 340));
  const [rightWidth, setRightWidth] = useState(() => boundedNumber(localStorage.getItem('workstreamExplorer.rightWidth'), 300));
  const [searchIndex, setSearchIndex] = useState(0);
  const [pathDialog, setPathDialog] = useState<PathDialog | null>(null);
	const [matchContext, setMatchContext] = useState<ContentSearchSelection | null>(null);
	const treeRef = useRef<HTMLDivElement | null>(null);
  const gridRef = useRef<HTMLDivElement | null>(null);
  const workspace = workspaces.find((item) => item.id === location?.workspaceId);
  const selectedRow = explorer.rows.find((row) => explorerNodeId(row.workspaceId, row.node.path) === explorer.selection?.nodeId);
	const pathSearch = useWorkspacePathSearch({ workspaceId: location?.workspaceId, includeIgnored: explorer.showIgnored });
	const contentSearch = useContentSearch({ kind: 'explorer', mode: explorer.mode, workspaceId: location?.workspaceId, includeIgnored: explorer.showIgnored });
	const searchResults = useMemo<ExplorerUnifiedResult[]>(() => [
		...pathSearch.results.slice(0, 5).map((result) => ({ kind: 'path' as const, result })),
		...contentSearch.results.slice(0, 15).map((result) => ({ kind: 'content' as const, result }))
	], [contentSearch.results, pathSearch.results]);
  const mutations = useWorkspacePathMutations(async (result) => {
    await explorer.invalidateDirectories(result.workspaceId, result.invalidatedPaths);
    await explorer.expandToPath(result.workspaceId, result.path, result.type);
  });

  const editor = useFileEditorSession({
    save: (file, content) => api.saveWorkspaceFile(location?.workspaceId ?? '', { path: file.path, content, expectedHash: file.hash }).then((result) => result.file),
    onSaved: () => void loadDiff(),
    onError: (caught) => showError(caught, 'File save failed')
  });

  const refreshAfterBranchSwitch = useCallback(async (workspaceId: string) => {
    if (location?.workspaceId === workspaceId) {
      editor.open(null);
      setDiff('');
      onLocationChange({ workspaceId, mode: explorer.mode });
    }
    pathSearch.setQuery('');
    contentSearch.clear();
    setSearchIndex(0);
    setMatchContext(null);
    await explorer.refreshWorkspaceBranch(workspaceId);
  }, [contentSearch, editor, explorer, location?.workspaceId, onLocationChange, pathSearch]);
  const workspaceBranches = useWorkspaceBranches(workspaces, refreshAfterBranchSwitch);

  const switchWorkspaceBranch = async (workspace: WorkspaceConfig, branch: string) => {
    if (location?.workspaceId === workspace.id && editor.dirty && !(await editor.saveNow())) return;
    await workspaceBranches.switchBranch(workspace, branch);
  };

  const showError = (caught: unknown, fallback: string) => {
    setError(caught instanceof Error ? caught.message : fallback);
    setRecoveryHint(caught instanceof ApiError ? caught.recoveryHint ?? '' : '');
  };

  const loadFile = async () => {
    if (!location?.workspaceId || !location.path || selectedRow?.node.type !== 'file') {
      editor.open(null);
      setDiff('');
      return;
    }
    setError('');
    setRecoveryHint('');
    try {
      editor.open(await api.workspaceFile(location.workspaceId, location.path));
      await loadDiff();
    } catch (caught) {
      editor.open(null);
      showError(caught, 'File failed to load');
    }
  };

  const loadDiff = async () => {
    if (!location?.workspaceId || !location.path) return setDiff('');
    try {
      setDiff((await api.workspaceFileDiff(location.workspaceId, location.path)).diff ?? '');
    } catch {
      setDiff('');
    }
  };

  useEffect(() => { void loadFile(); }, [location?.workspaceId, location?.path, selectedRow?.node.type]);

  const selectRow = async (row: VisibleExplorerRow) => {
    if (editor.dirty && !(await editor.saveNow())) return;
		setMatchContext(null);
    explorer.select(row.workspaceId, row.node.path);
  };

  const openSearchResult = async (result: WorkspacePathSearchResult) => {
    if (editor.dirty && !(await editor.saveNow())) return;
    await explorer.expandToPath(result.workspaceId, result.path, result.type);
		pathSearch.setQuery('');
		contentSearch.clear();
    setSearchIndex(0);
  };

	const openContentResult = async (result: WorkspaceContentSearchResult) => {
		if (editor.dirty && !(await editor.saveNow())) return;
		setMatchContext({ workspaceId: result.workspaceId, path: result.path, lineNumber: result.lineNumber, columnStart: result.columnStart, columnEnd: result.columnEnd });
		await explorer.expandToPath(result.workspaceId, result.path, 'file');
		pathSearch.setQuery('');
		contentSearch.clear();
		setSearchIndex(0);
	};

	const setExplorerSearchQuery = (query: string) => {
		pathSearch.setQuery(query);
		contentSearch.setQuery(query);
		setMatchContext(null);
		setSearchIndex(0);
	};

	const openUnifiedSearchResult = (entry: ExplorerUnifiedResult) => {
		if (entry.kind === 'path') void openSearchResult(entry.result);
		else void openContentResult(entry.result);
	};

  const selectedParentPath = () => {
    if (!selectedRow || selectedRow.node.type === 'workspace') return '';
    if (selectedRow.node.type === 'directory') return selectedRow.node.path;
    const separator = selectedRow.node.path.lastIndexOf('/');
    return separator >= 0 ? selectedRow.node.path.slice(0, separator) : '';
  };

  const openRename = async () => {
    if (!selectedRow || selectedRow.node.type === 'workspace') return;
    if (editor.dirty && !(await editor.saveNow())) return;
    const separator = selectedRow.node.path.lastIndexOf('/');
    setPathDialog({
      kind: 'rename',
      parentPath: separator >= 0 ? selectedRow.node.path.slice(0, separator) : '',
      currentPath: selectedRow.node.path,
      initialName: selectedRow.node.name
    });
  };

  const submitPathDialog = async (name: string) => {
    if (!pathDialog || !location?.workspaceId) return false;
    const destinationPath = pathDialog.parentPath ? `${pathDialog.parentPath}/${name}` : name;
    const result = pathDialog.kind === 'file'
      ? await mutations.createFile(location.workspaceId, { parentPath: pathDialog.parentPath, name, content: '' })
      : pathDialog.kind === 'directory'
        ? await mutations.createDirectory(location.workspaceId, { parentPath: pathDialog.parentPath, name })
        : await mutations.rename(location.workspaceId, { path: pathDialog.currentPath ?? '', destinationPath });
    if (result) setPathDialog(null);
    return Boolean(result);
  };

  const toggleRow = (row: VisibleExplorerRow) => {
    if (row.node.type === 'workspace' || row.node.type === 'directory') explorer.toggleExpanded(row.workspaceId, row.node.path);
  };

  const onTreeKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    const result = treeKeyboardAction(event.key, explorer.rows, explorer.activeIndex, explorer.expandedNodeIds);
    if (result.activeIndex === explorer.activeIndex && !result.toggleNodeId && !result.select) return;
    event.preventDefault();
    explorer.setActiveIndex(result.activeIndex);
    if (result.toggleNodeId) {
      const row = explorer.rows[result.activeIndex];
      toggleRow(row);
    }
    if (result.select) void selectRow(result.select);
  };

  const revertFile = async () => {
    if (!editor.file || !location?.workspaceId) return;
    setReverting(true);
    try {
      const result = await api.revertWorkspaceFile(location.workspaceId, { path: editor.file.path });
      editor.open(result.file);
      await loadDiff();
      setError('');
    } catch (caught) {
      showError(caught, 'Revert failed');
    } finally {
      setReverting(false);
      setRevertOpen(false);
    }
  };

  const reveal = () => {
    if (!workspace) return;
    const path = location?.path ? `${workspace.path.replace(/\/$/, '')}/${location.path}` : workspace.path;
    void api.openPath(path).catch((caught) => showError(caught, 'Could not reveal path'));
  };

  const startResize = (side: 'left' | 'right', event: React.PointerEvent<HTMLButtonElement>) => {
    const start = event.clientX;
    const initial = side === 'left' ? leftWidth : rightWidth;
    let latest = initial;
    const move = (next: PointerEvent) => {
      const width = Math.min(520, Math.max(220, initial + (side === 'left' ? next.clientX - start : start - next.clientX)));
      latest = width;
      gridRef.current?.style.setProperty(side === 'left' ? '--explorer-left-width' : '--explorer-right-width', `${width}px`);
      if (side === 'left') setLeftWidth(width); else setRightWidth(width);
    };
    const up = () => {
      window.removeEventListener('pointermove', move);
      localStorage.setItem(side === 'left' ? 'workstreamExplorer.leftWidth' : 'workstreamExplorer.rightWidth', String(latest));
    };
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', up, { once: true });
  };

  const gridStyle = {
    '--explorer-left-width': `${leftWidth}px`,
    '--explorer-right-width': `${inspectorOpen ? rightWidth : 44}px`
  } as CSSProperties;

  return (
    <section className="workstream-explorer-page">
      <header className="explorer-header">
        <div><span className="eyebrow">{workspace?.name ?? 'No workspace selected'}</span><h1>Workstream Explorer</h1></div>
        <div className="explorer-header-actions">
          <button className="secondary" type="button" disabled={!workspace} onClick={() => setPathDialog({ kind: 'file', parentPath: selectedParentPath() })}><FilePlus2 size={15} /> New file</button>
          <button className="secondary" type="button" disabled={!workspace} onClick={() => setPathDialog({ kind: 'directory', parentPath: selectedParentPath() })}><FolderPlus size={15} /> New folder</button>
          <button className="secondary" type="button" disabled={!selectedRow || selectedRow.node.type === 'workspace'} onClick={() => void openRename()}><Pencil size={15} /> Rename</button>
          <button className="secondary" type="button" onClick={explorer.collapseAll}>Collapse all</button>
          <button className="secondary" type="button" onClick={explorer.refresh}><RefreshCw size={15} /> Refresh</button>
        </div>
      </header>
      <div className="explorer-grid" style={gridStyle} ref={gridRef}>
        <aside className="explorer-tree-panel">
		  <div className="explorer-panel-header">
			<strong>Files</strong>
			<select aria-label="Explorer tree mode" value={explorer.mode} onChange={(event) => explorer.setMode(event.target.value as 'sources' | 'all')}>
				<option value="sources">Planning folders</option>
				<option value="all">Entire workspace</option>
			</select>
		  </div>
		  <div className="explorer-search-context">Search in {workspace?.name ?? 'all workspaces'}</div>
		  <div className="explorer-toolbar">
			<label><Search size={15} /><input aria-label="Search files" value={pathSearch.query} onChange={(event) => setExplorerSearchQuery(event.target.value)} onKeyDown={(event) => {
			  if (event.key === 'ArrowDown' && searchResults.length) { event.preventDefault(); setSearchIndex((index) => Math.min(index + 1, searchResults.length - 1)); }
			  if (event.key === 'ArrowUp' && searchResults.length) { event.preventDefault(); setSearchIndex((index) => Math.max(index - 1, 0)); }
			  if (event.key === 'Enter' && searchResults.length) { event.preventDefault(); openUnifiedSearchResult(searchResults[searchIndex] ?? searchResults[0]); }
			  if (event.key === 'Escape') setExplorerSearchQuery('');
			}} placeholder="Search files and text" /></label>
		  </div>
		  {pathSearch.query.trim() && <ExplorerUnifiedSearchResults query={pathSearch.query} results={searchResults} loading={pathSearch.loading || contentSearch.loading} error={pathSearch.error || contentSearch.error} activeIndex={searchIndex} onActiveIndex={setSearchIndex} onOpen={openUnifiedSearchResult} />}
		  <div className="explorer-tree" ref={treeRef} role="tree" aria-label="Workspace files" tabIndex={0} onKeyDown={onTreeKeyDown}>
            {explorer.rows.map((row, index) => (
              <ExplorerTreeRow key={explorerNodeId(row.workspaceId, row.node.path)} row={row} gitState={row.node.type === 'file' ? explorer.gitStateByPath.get(explorerNodeId(row.workspaceId, row.node.path)) : undefined} branchState={workspaceBranches.states[row.workspaceId]} active={index === explorer.activeIndex} selected={explorer.selection?.nodeId === explorerNodeId(row.workspaceId, row.node.path)} expanded={explorer.expandedNodeIds.has(explorerNodeId(row.workspaceId, row.node.path))} onFocus={() => explorer.setActiveIndex(index)} onSelect={() => void selectRow(row)} onToggle={() => toggleRow(row)} onBranchChange={(workspace, branch) => void switchWorkspaceBranch(workspace, branch)} />
            ))}
            {explorer.rows.length === 0 && <p className="explorer-empty">No matching paths.</p>}
			{explorer.mode === 'sources' && workspaces.every((item) => item.sources.length === 0) && <button className="secondary" type="button" onClick={() => explorer.setMode('all')}>Browse All Files</button>}
          </div>
          <button className="explorer-resize-handle left" aria-label="Resize workspace tree" onPointerDown={(event) => startResize('left', event)} />
        </aside>
        <main className="workspace-file-editor">
          <div className="explorer-breadcrumbs">
            <span>{workspace?.name ?? 'Select a workspace'}</span>
            {(location?.path?.split('/') ?? []).map((part, index, parts) => <span key={`${part}-${index}`}>{index < parts.length && ' / '}{part}</span>)}
            <div>
              <button className="icon-button" type="button" title="Copy path" disabled={!workspace} onClick={() => void navigator.clipboard.writeText(location?.path ?? workspace?.path ?? '')}><Clipboard size={15} /></button>
              <button className="secondary" type="button" disabled={!workspace} onClick={reveal}>Reveal</button>
            </div>
          </div>
          <div className="tabs explorer-editor-tabs">
            <div className="tab-list">
              <button className={tab === 'preview' ? 'active' : ''} onClick={() => setTab('preview')}><Eye size={15} /> Preview</button>
              <button className={tab === 'raw' ? 'active' : ''} onClick={() => setTab('raw')}><Code2 size={15} /> Raw</button>
              <button className={tab === 'diff' ? 'active' : ''} onClick={() => setTab('diff')}><GitCompare size={15} /> Diff</button>
            </div>
            <span className={`autosave-state ${editor.state}`}>{autoSaveLabel(editor.state)}</span>
          </div>
		  {matchContext && <div className="content-match-context">Line {matchContext.lineNumber}, columns {matchContext.columnStart}–{matchContext.columnEnd}</div>}
          {error && <div className="operation-error"><p className="error">{error}</p>{recoveryHint && <p>{recoveryHint}</p>}<button className="secondary" onClick={() => void loadFile()}>Reload file</button></div>}
          {tab === 'preview' && (editor.file ? <ContentViewer file={editor.file} content={editor.content} /> : <ExplorerEmpty row={selectedRow} />)}
          {tab === 'raw' && <textarea className="raw-editor" value={editor.file ? editor.content : 'Select a file.'} disabled={!editor.file?.editable} onChange={(event) => editor.setContent(event.target.value)} spellCheck={false} />}
          {tab === 'diff' && <ExplorerDiff diff={diff} onRevert={() => setRevertOpen(true)} disabled={!editor.file || reverting} />}
        </main>
        <aside className={inspectorOpen ? 'explorer-inspector' : 'explorer-inspector collapsed'}>
          <div className="panel-header"><h2>Inspector</h2><button className="icon-button" type="button" aria-label={inspectorOpen ? 'Collapse inspector' : 'Expand inspector'} onClick={() => setInspectorOpen((value) => !value)}>{inspectorOpen ? <PanelRightClose size={16} /> : <PanelRightOpen size={16} />}</button></div>
          {inspectorOpen && <ExplorerInspector workspace={workspace} row={selectedRow} file={editor.file} onOpenWorkstream={onOpenWorkstream} />}
          {inspectorOpen && <button className="explorer-resize-handle right" aria-label="Resize inspector" onPointerDown={(event) => startResize('right', event)} />}
        </aside>
      </div>
      {revertOpen && editor.file && <ConfirmDialog title="Revert file" message={`Revert ${editor.file.path} to HEAD?`} confirmLabel={reverting ? 'Reverting...' : 'Revert File'} busy={reverting} danger onCancel={() => setRevertOpen(false)} onConfirm={revertFile} />}
      {pathDialog && <ExplorerPathDialog dialog={pathDialog} busy={Boolean(mutations.busy)} error={mutations.error} onCancel={() => { mutations.clearError(); setPathDialog(null); }} onSubmit={submitPathDialog} />}
    </section>
  );
}

function ExplorerTreeRow({ row, gitState, branchState, active, selected, expanded, onFocus, onSelect, onToggle, onBranchChange }: { row: VisibleExplorerRow; gitState?: WorkspacePathGitState; branchState?: WorkspaceBranchState; active: boolean; selected: boolean; expanded: boolean; onFocus: () => void; onSelect: () => void; onToggle: () => void; onBranchChange: (workspace: WorkspaceConfig, branch: string) => void }) {
  const expandable = row.node.type === 'workspace' || row.node.type === 'directory';
  const workspace = row.node.type === 'workspace' ? row.node.workspace : undefined;
  return <div className={`explorer-tree-row${selected ? ' selected' : ''}${active ? ' active' : ''}`} role="treeitem" aria-level={row.level + 1} aria-expanded={expandable ? expanded : undefined} aria-selected={selected} style={{ '--explorer-depth': row.level } as CSSProperties} onMouseEnter={onFocus}>
    <button className="explorer-row-toggle" type="button" tabIndex={-1} onClick={onToggle} disabled={!expandable} aria-label={expandable ? `${expanded ? 'Collapse' : 'Expand'} ${row.node.name}` : undefined}>{expandable ? (expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />) : null}</button>
    <button className="explorer-row-main" type="button" tabIndex={active ? 0 : -1} onFocus={onFocus} onClick={onSelect} onDoubleClick={expandable ? onToggle : undefined}>
      {row.node.type === 'workspace' ? <FolderGit2 className="explorer-node-icon" size={16} /> : row.node.type === 'directory' ? <Folder className="explorer-node-icon" size={16} /> : <File className="explorer-node-icon" size={16} />}
      <span className={`explorer-row-label ${row.node.type}`}>{row.node.name}</span>
      {row.node.type === 'file' && gitState && <FileStateIcon state={gitState.conflict ? 'conflicted' : gitState.status} />}
    </button>
    {workspace && <WorkspaceBranchSelector workspace={workspace} state={branchState} onChange={(branch) => onBranchChange(workspace, branch)} />}
  </div>;
}

function ExplorerUnifiedSearchResults({ query, results, loading, error, activeIndex, onActiveIndex, onOpen }: { query: string; results: ExplorerUnifiedResult[]; loading: boolean; error: string; activeIndex: number; onActiveIndex: (index: number) => void; onOpen: (result: ExplorerUnifiedResult) => void }) {
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

function ExplorerPathDialog({ dialog, busy, error, onCancel, onSubmit }: { dialog: PathDialog; busy: boolean; error: string; onCancel: () => void; onSubmit: (name: string) => Promise<boolean> }) {
  const [name, setName] = useState(dialog.initialName ?? '');
  const title = dialog.kind === 'file' ? 'Create file' : dialog.kind === 'directory' ? 'Create directory' : 'Rename path';
  return <div className="dialog-backdrop" role="presentation"><section className="explorer-path-dialog" role="dialog" aria-modal="true" aria-labelledby="explorer-path-dialog-title">
    <header><h2 id="explorer-path-dialog-title">{title}</h2><button className="icon-button" onClick={onCancel} aria-label="Close"><X size={16} /></button></header>
    <p>Parent: {dialog.parentPath || 'workspace root'}</p>
    <label>Name<input autoFocus value={name} onChange={(event) => setName(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter' && name.trim()) void onSubmit(name.trim()); }} /></label>
    {error && <p className="error">{error}</p>}
    <footer><button className="ghost" disabled={busy} onClick={onCancel}>Cancel</button><button className="primary" disabled={busy || !name.trim()} onClick={() => void onSubmit(name.trim())}>{busy ? 'Saving...' : dialog.kind === 'rename' ? 'Rename' : 'Create'}</button></footer>
  </section></div>;
}

function ExplorerEmpty({ row }: { row?: VisibleExplorerRow }) {
  return <div className="document-empty"><Folder size={24} /><strong>{row ? row.node.name : 'Select a file'}</strong><span>{row?.node.type === 'directory' ? 'Expand this directory or choose a file.' : 'Choose a file from a workspace tree.'}</span></div>;
}

function ExplorerDiff({ diff, onRevert, disabled }: { diff: string; onRevert: () => void; disabled: boolean }) {
  const files = useMemo(() => parseGitDiff(diff), [diff]);
  return <section className="diff-panel"><header className="diff-toolbar"><strong>{files.length ? `${files.length} changed file` : 'No local changes'}</strong><button className="danger-action" disabled={disabled || !diff} onClick={onRevert}><RotateCcw size={15} /> Revert File</button></header><pre className="diff-view">{diff || 'No local changes.'}</pre></section>;
}

function ExplorerInspector({ workspace, row, file, onOpenWorkstream }: { workspace?: WorkspaceConfig; row?: VisibleExplorerRow; file: ReturnType<typeof useFileEditorSession>['file']; onOpenWorkstream: (workspace: WorkspaceConfig, itemId?: string) => void }) {
  const [git, setGit] = useState<GitStatus | null>(null);
  const [activity, setActivity] = useState<GitActivityEntry[]>([]);
  const [activityLoading, setActivityLoading] = useState(false);
  const [activityOpen, setActivityOpen] = useState(() => readStoredToggle('explorer.inspector.gitActivityOpen'));
  const [health, setHealth] = useState<WorkspaceHealth | null>(null);
  const [item, setItem] = useState<ItemSummary | null>(null);
  const activityPath = row?.node.type === 'file' || row?.node.type === 'directory'
    ? row.node.path
    : file?.path || '';
  useEffect(() => {
    setGit(null); setHealth(null);
    if (!workspace) return;
    void api.gitStatus(workspace.id).then(setGit).catch(() => setGit(null));
    void api.workspaceHealth(workspace.id).then(setHealth).catch(() => setHealth(null));
  }, [workspace?.id]);
  useEffect(() => {
    if (!workspace) {
      setActivity([]);
      setActivityLoading(false);
      return;
    }
    let active = true;
    setActivityLoading(true);
    api.gitActivity(workspace.id, { path: activityPath || undefined, limit: 8 })
      .then((entries) => {
        if (active) setActivity(entries);
      })
      .catch(() => {
        if (active) setActivity([]);
      })
      .finally(() => {
        if (active) setActivityLoading(false);
      });
    return () => {
      active = false;
    };
  }, [workspace?.id, activityPath]);
  useEffect(() => {
    setItem(null);
    if (row?.item) void api.items(new URLSearchParams()).then((items) => setItem(items.find((candidate) => candidate.id === row.item?.itemId) ?? null));
  }, [row?.item?.itemId]);
  if (!workspace) return <p className="explorer-empty">Select a workspace or file.</p>;
  return <div className="inspector-content">
    <section><h3>Workspace</h3><dl><dt>Name</dt><dd>{workspace.name}</dd><dt>Branch</dt><dd>{git?.branch ?? workspace.baselineBranch}</dd><dt>Health</dt><dd>{health?.summary ?? 'Loading'}</dd><dt>Changes</dt><dd>{git?.changes.length ?? 0}</dd></dl><button className="secondary" onClick={() => onOpenWorkstream(workspace, row?.item?.itemId)}><WorkstreamIcon size={15} /> Open Workstream</button></section>
    {file && <section><h3>File</h3><dl><dt>Path</dt><dd>{file.path}</dd><dt>Kind</dt><dd>{file.kind}</dd><dt>Size</dt><dd>{file.sizeBytes.toLocaleString()} bytes</dd><dt>Editable</dt><dd>{file.editable ? 'Text' : 'Read only'}</dd></dl></section>}
    {row?.item && <section><h3>Item</h3><dl><dt>ID</dt><dd>{row.item.identifier}</dd><dt>Title</dt><dd>{row.item.title}</dd><dt>Status</dt><dd>{row.item.status}</dd><dt>Owner</dt><dd>{item?.owner || 'Unassigned'}</dd></dl><a className="secondary button-link" href={`/items/${encodeURIComponent(row.item.itemId)}`}>Open details</a></section>}
    <section>
      <details className="recent-activity-panel" open={activityOpen} onToggle={(event) => {
        const open = event.currentTarget.open;
        setActivityOpen(open);
        localStorage.setItem('explorer.inspector.gitActivityOpen', open ? '1' : '0');
      }}>
        <summary>
          <span>Recent Activity</span>
          <small>{activity.length} events</small>
        </summary>
        <RecentGitActivity entries={activity} loading={activityLoading} emptyLabel="No activity found for this selection." pathLabel={activityPath || 'workspace'} />
      </details>
    </section>
  </div>;
}

function boundedNumber(value: string | null, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? Math.min(520, Math.max(220, parsed)) : fallback;
}

function readStoredToggle(key: string): boolean {
  return localStorage.getItem(key) === '1';
}

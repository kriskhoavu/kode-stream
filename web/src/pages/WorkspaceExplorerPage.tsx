import { useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties } from 'react';
import {
  ChevronDown, ChevronRight, Clipboard, Code2, Eye, File, Folder, FolderGit2, GitCompare,
  KanbanSquare, PanelRightClose, PanelRightOpen, RefreshCw, RotateCcw, Search, Settings2
} from 'lucide-react';
import type { ExplorerLocation } from '../app/router';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { ContentViewer } from '../features/content-viewer/ContentViewer';
import { autoSaveLabel, useFileEditorSession } from '../features/file-editor/useFileEditorSession';
import { treeKeyboardAction } from '../features/workspace-explorer/keyboard';
import { explorerNodeId } from '../features/workspace-explorer/tree';
import type { VisibleExplorerRow } from '../features/workspace-explorer/types';
import { useWorkspaceExplorer } from '../features/workspace-explorer/useWorkspaceExplorer';
import { ApiError, api } from '../lib/api';
import type { GitStatus, ItemSummary, WorkspaceConfig, WorkspaceHealth } from '../lib/types';
import { parseGitDiff } from '../shared/domain/diff';

type EditorTab = 'preview' | 'raw' | 'diff';

export function WorkspaceExplorerPage({ workspaces, location, onLocationChange, onOpenKanban }: {
  workspaces: WorkspaceConfig[];
  location?: ExplorerLocation;
  onLocationChange: (location?: ExplorerLocation) => void;
  onOpenKanban: (workspace: WorkspaceConfig) => void;
}) {
  const explorer = useWorkspaceExplorer(workspaces, location, onLocationChange);
  const [tab, setTab] = useState<EditorTab>('preview');
  const [diff, setDiff] = useState('');
  const [error, setError] = useState('');
  const [recoveryHint, setRecoveryHint] = useState('');
  const [revertOpen, setRevertOpen] = useState(false);
  const [reverting, setReverting] = useState(false);
  const [inspectorOpen, setInspectorOpen] = useState(true);
  const [leftWidth, setLeftWidth] = useState(() => boundedNumber(localStorage.getItem('workspaceExplorer.leftWidth'), 340));
  const [rightWidth, setRightWidth] = useState(() => boundedNumber(localStorage.getItem('workspaceExplorer.rightWidth'), 300));
  const gridRef = useRef<HTMLDivElement | null>(null);
  const workspace = workspaces.find((item) => item.id === location?.workspaceId);
  const selectedRow = explorer.rows.find((row) => explorerNodeId(row.workspaceId, row.node.path) === explorer.selection?.nodeId);

  const editor = useFileEditorSession({
    save: (file, content) => api.saveWorkspaceFile(location?.workspaceId ?? '', { path: file.path, content, expectedHash: file.hash }).then((result) => result.file),
    onSaved: () => void loadDiff(),
    onError: (caught) => showError(caught, 'File save failed')
  });

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
    explorer.select(row.workspaceId, row.node.path);
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
    const move = (next: PointerEvent) => {
      const width = Math.min(520, Math.max(220, initial + (side === 'left' ? next.clientX - start : start - next.clientX)));
      gridRef.current?.style.setProperty(side === 'left' ? '--explorer-left-width' : '--explorer-right-width', `${width}px`);
      if (side === 'left') setLeftWidth(width); else setRightWidth(width);
    };
    const up = () => {
      window.removeEventListener('pointermove', move);
      localStorage.setItem(side === 'left' ? 'workspaceExplorer.leftWidth' : 'workspaceExplorer.rightWidth', String(side === 'left' ? leftWidth : rightWidth));
    };
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', up, { once: true });
  };

  const gridStyle = {
    '--explorer-left-width': `${leftWidth}px`,
    '--explorer-right-width': `${inspectorOpen ? rightWidth : 44}px`
  } as CSSProperties;

  return (
    <section className="workspace-explorer-page">
      <header className="explorer-header">
        <div><span className="eyebrow">All workspaces</span><h1>Workspace Explorer</h1></div>
        <div className="explorer-header-actions">
          <button className="secondary" type="button" onClick={explorer.collapseAll}>Collapse all</button>
          <button className="secondary" type="button" onClick={explorer.refresh}><RefreshCw size={15} /> Refresh</button>
        </div>
      </header>
      <div className="explorer-grid" style={gridStyle} ref={gridRef}>
        <aside className="explorer-tree-panel">
          <div className="explorer-toolbar">
            <label><Search size={15} /><input aria-label="Search loaded paths" value={explorer.filter} onChange={(event) => explorer.setFilter(event.target.value)} placeholder="Search loaded paths" /></label>
            <button className={explorer.showIgnored ? 'icon-button active' : 'icon-button'} type="button" title="Show ignored files" onClick={() => explorer.setShowIgnored(!explorer.showIgnored)}><Settings2 size={16} /></button>
          </div>
          <div className="explorer-tree" role="tree" aria-label="Workspace files" tabIndex={0} onKeyDown={onTreeKeyDown}>
            {explorer.rows.map((row, index) => (
              <ExplorerTreeRow key={explorerNodeId(row.workspaceId, row.node.path)} row={row} active={index === explorer.activeIndex} selected={explorer.selection?.nodeId === explorerNodeId(row.workspaceId, row.node.path)} expanded={explorer.expandedNodeIds.has(explorerNodeId(row.workspaceId, row.node.path))} onFocus={() => explorer.setActiveIndex(index)} onSelect={() => void selectRow(row)} onToggle={() => toggleRow(row)} />
            ))}
            {explorer.rows.length === 0 && <p className="explorer-empty">No matching paths.</p>}
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
          {error && <div className="operation-error"><p className="error">{error}</p>{recoveryHint && <p>{recoveryHint}</p>}<button className="secondary" onClick={() => void loadFile()}>Reload file</button></div>}
          {tab === 'preview' && (editor.file ? <ContentViewer file={editor.file} content={editor.content} /> : <ExplorerEmpty row={selectedRow} />)}
          {tab === 'raw' && <textarea className="raw-editor" value={editor.file ? editor.content : 'Select a file.'} disabled={!editor.file?.editable} onChange={(event) => editor.setContent(event.target.value)} spellCheck={false} />}
          {tab === 'diff' && <ExplorerDiff diff={diff} onRevert={() => setRevertOpen(true)} disabled={!editor.file || reverting} />}
        </main>
        <aside className={inspectorOpen ? 'explorer-inspector' : 'explorer-inspector collapsed'}>
          <div className="panel-header"><h2>Inspector</h2><button className="icon-button" onClick={() => setInspectorOpen((value) => !value)}>{inspectorOpen ? <PanelRightClose size={16} /> : <PanelRightOpen size={16} />}</button></div>
          {inspectorOpen && <ExplorerInspector workspace={workspace} row={selectedRow} file={editor.file} onOpenKanban={onOpenKanban} />}
          {inspectorOpen && <button className="explorer-resize-handle right" aria-label="Resize inspector" onPointerDown={(event) => startResize('right', event)} />}
        </aside>
      </div>
      {revertOpen && editor.file && <ConfirmDialog title="Revert file" message={`Revert ${editor.file.path} to HEAD?`} confirmLabel={reverting ? 'Reverting...' : 'Revert File'} busy={reverting} danger onCancel={() => setRevertOpen(false)} onConfirm={revertFile} />}
    </section>
  );
}

function ExplorerTreeRow({ row, active, selected, expanded, onFocus, onSelect, onToggle }: { row: VisibleExplorerRow; active: boolean; selected: boolean; expanded: boolean; onFocus: () => void; onSelect: () => void; onToggle: () => void }) {
  const expandable = row.node.type === 'workspace' || row.node.type === 'directory';
  return <div className={`explorer-tree-row${selected ? ' selected' : ''}${active ? ' active' : ''}`} role="treeitem" aria-level={row.level + 1} aria-expanded={expandable ? expanded : undefined} aria-selected={selected} style={{ '--explorer-depth': row.level } as CSSProperties} onMouseEnter={onFocus}>
    <button className="explorer-row-toggle" type="button" tabIndex={-1} onClick={onToggle} disabled={!expandable}>{expandable ? (expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />) : null}</button>
    <button className="explorer-row-main" type="button" tabIndex={active ? 0 : -1} onFocus={onFocus} onClick={onSelect}>
      {row.node.type === 'workspace' ? <FolderGit2 size={16} /> : row.node.type === 'directory' ? <Folder size={16} /> : <File size={16} />}
      <span><strong>{row.node.name}</strong>{row.item && <small>{row.item.identifier} · {row.item.title}</small>}</span>
      {row.item && <i className={`item-status-dot ${row.item.status}`} title={row.item.status} />}
    </button>
  </div>;
}

function ExplorerEmpty({ row }: { row?: VisibleExplorerRow }) {
  return <div className="document-empty"><Folder size={24} /><strong>{row ? row.node.name : 'Select a file'}</strong><span>{row?.node.type === 'directory' ? 'Expand this directory or choose a file.' : 'Choose a file from a workspace tree.'}</span></div>;
}

function ExplorerDiff({ diff, onRevert, disabled }: { diff: string; onRevert: () => void; disabled: boolean }) {
  const files = useMemo(() => parseGitDiff(diff), [diff]);
  return <section className="diff-panel"><header className="diff-toolbar"><strong>{files.length ? `${files.length} changed file` : 'No local changes'}</strong><button className="danger-action" disabled={disabled || !diff} onClick={onRevert}><RotateCcw size={15} /> Revert File</button></header><pre className="diff-view">{diff || 'No local changes.'}</pre></section>;
}

function ExplorerInspector({ workspace, row, file, onOpenKanban }: { workspace?: WorkspaceConfig; row?: VisibleExplorerRow; file: ReturnType<typeof useFileEditorSession>['file']; onOpenKanban: (workspace: WorkspaceConfig) => void }) {
  const [git, setGit] = useState<GitStatus | null>(null);
  const [health, setHealth] = useState<WorkspaceHealth | null>(null);
  const [item, setItem] = useState<ItemSummary | null>(null);
  useEffect(() => {
    setGit(null); setHealth(null);
    if (!workspace) return;
    void api.gitStatus(workspace.id).then(setGit).catch(() => setGit(null));
    void api.workspaceHealth(workspace.id).then(setHealth).catch(() => setHealth(null));
  }, [workspace?.id]);
  useEffect(() => {
    setItem(null);
    if (row?.item) void api.items(new URLSearchParams()).then((items) => setItem(items.find((candidate) => candidate.id === row.item?.itemId) ?? null));
  }, [row?.item?.itemId]);
  if (!workspace) return <p className="explorer-empty">Select a workspace or file.</p>;
  return <div className="inspector-content">
    <section><h3>Workspace</h3><dl><dt>Name</dt><dd>{workspace.name}</dd><dt>Branch</dt><dd>{git?.branch ?? workspace.baselineBranch}</dd><dt>Health</dt><dd>{health?.summary ?? 'Loading'}</dd><dt>Changes</dt><dd>{git?.changes.length ?? 0}</dd></dl><button className="secondary" onClick={() => onOpenKanban(workspace)}><KanbanSquare size={15} /> Open Kanban</button></section>
    {file && <section><h3>File</h3><dl><dt>Path</dt><dd>{file.path}</dd><dt>Kind</dt><dd>{file.kind}</dd><dt>Size</dt><dd>{file.sizeBytes.toLocaleString()} bytes</dd><dt>Editable</dt><dd>{file.editable ? 'Markdown' : 'Read only'}</dd></dl></section>}
    {row?.item && <section><h3>Item</h3><dl><dt>ID</dt><dd>{row.item.identifier}</dd><dt>Title</dt><dd>{row.item.title}</dd><dt>Status</dt><dd>{row.item.status}</dd><dt>Owner</dt><dd>{item?.owner || 'Unassigned'}</dd></dl><a className="secondary button-link" href={`/items/${encodeURIComponent(row.item.itemId)}`}>Open details</a></section>}
  </div>;
}

function boundedNumber(value: string | null, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? Math.min(520, Math.max(220, parsed)) : fallback;
}

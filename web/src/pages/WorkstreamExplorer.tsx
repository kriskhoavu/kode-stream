import { lastPathSegment, parentDirectoryPath } from '../shared/domain/paths';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import { CaseSensitive, Clipboard, Code2, Eye, GitCompare, FilePlus2, FolderPlus, PanelLeftClose, PanelLeftOpen, PanelRightClose, PanelRightOpen, Pencil, RefreshCw, Search, X } from 'lucide-react';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { readStringPreference, writePreference } from '../shared/preferences/store';
import { ContentViewer } from '../features/content-viewer/ContentViewer';
import { autoSaveLabel, useFileEditorSession } from '../features/file-editor/useFileEditorSession';
import { treeKeyboardAction } from '../features/workstream-explorer/keyboard';
import { explorerNodeId, normalizeExplorerPath } from '../features/workstream-explorer/tree';
import { sourcePaths } from '../lib/sources';
import type { ExplorerLocation, VisibleExplorerRow } from '../features/workstream-explorer/types';
import { useWorkstreamExplorer } from '../features/workstream-explorer/useWorkstreamExplorer';
import { useWorkspaceBranches } from '../features/workstream-explorer/useWorkspaceBranches';
import { useWorkspacePathSearch } from '../features/workstream-explorer/useWorkspacePathSearch';
import { useWorkspacePathMutations } from '../features/workstream-explorer/useWorkspacePathMutations';
import { ApiError, api } from '../shared/api';
import type { WorkspaceConfig, WorkspacePathSearchResult } from '../lib/types';
import { useContentSearch } from '../features/content-search/useContentSearch';
import type { ContentSearchSelection, WorkspaceContentSearchResult } from '../lib/types';
import { ExplorerInspector } from '../features/workstream-explorer/ExplorerInspector';
import { ExplorerTreeRow, ExplorerUnifiedSearchResults, ExplorerPathDialog, ExplorerEmpty, ExplorerDiff } from '../features/workstream-explorer/ExplorerParts';
import { type EditorTab, type PathDialog, type ExplorerUnifiedResult, type OpenWorkspaceFileTab, type ExplorerRightPanelProps } from '../features/workstream-explorer/explorerPageTypes';
import { boundedNumber, boundedLeftPanelWidth, boundedRightPanelWidth, tabKey, restrictRowsToRoot } from '../features/workstream-explorer/explorerLayout';

export function WorkstreamExplorer({ workspaces, location, onLocationChange, embedded = false, treeRootPath, showModeSelector = true, embeddedHeaderContent, leftPanelContent, mainContent, agentArea, rightPanel }: {
  workspaces: WorkspaceConfig[];
  location?: ExplorerLocation;
  onLocationChange: (location?: ExplorerLocation) => void;
  embedded?: boolean;
  treeRootPath?: string;
  showModeSelector?: boolean;
  embeddedHeaderContent?: ReactNode;
  leftPanelContent?: ReactNode;
  mainContent?: ReactNode;
  /**
   * Agent sessions shown in the main pane. Kept mounted while inactive, so a
   * held terminal stays connected; while active it hides the file editor,
   * which stays mounted too and keeps its open tabs.
   */
  agentArea?: { content: ReactNode; active: boolean };
  rightPanel?: ExplorerRightPanelProps;
}) {
  const explorer = useWorkstreamExplorer(workspaces, location, onLocationChange);
  const [tab, setTab] = useState<EditorTab>('preview');
  const [diff, setDiff] = useState('');
  const [error, setError] = useState('');
  const [refreshWarning, setRefreshWarning] = useState('');
  const [recoveryHint, setRecoveryHint] = useState('');
  const [revertOpen, setRevertOpen] = useState(false);
  const [reverting, setReverting] = useState(false);
  const [inspectorOpen, setInspectorOpen] = useState(false);
  const [leftCollapsed, setLeftCollapsed] = useState(false);
  const [leftWidth, setLeftWidth] = useState(() => boundedNumber(readStringPreference('workstreamExplorer.leftWidth'), 400, boundedLeftPanelWidth));
  const [rightWidth, setRightWidth] = useState(() => boundedNumber(readStringPreference('workstreamExplorer.rightWidth'), 300, boundedRightPanelWidth));
  const [openTabs, setOpenTabs] = useState<OpenWorkspaceFileTab[]>([]);
  const [activeTabKey, setActiveTabKey] = useState('');
  const [searchIndex, setSearchIndex] = useState(0);
  const [pathDialog, setPathDialog] = useState<PathDialog | null>(null);
	const [matchContext, setMatchContext] = useState<ContentSearchSelection | null>(null);
  const treeRef = useRef<HTMLDivElement | null>(null);
  const gridRef = useRef<HTMLDivElement | null>(null);
  const resizeCleanupRef = useRef<(() => void) | null>(null);
  const openTabsRef = useRef<OpenWorkspaceFileTab[]>([]);
  const workspace = workspaces.find((item) => item.id === location?.workspaceId);
  const rightPanelOpen = rightPanel ? !rightPanel.collapsed : inspectorOpen;
  const rightPanelTitle = rightPanel?.title ?? 'Inspector';
  const rightPanelExpandedLabel = rightPanel?.expandedLabel ?? 'Collapse inspector';
  const rightPanelCollapsedLabel = rightPanel?.collapsedLabel ?? 'Expand inspector';
  const normalizedTreeRootPath = normalizeExplorerPath(treeRootPath ?? '');
  const visibleRows = useMemo(() => restrictRowsToRoot(explorer.rows, location?.workspaceId, normalizedTreeRootPath), [explorer.rows, location?.workspaceId, normalizedTreeRootPath]);
  // A tree restricted to a root (the item's Plan view) always opens on that
  // root expanded: the expanded set is shared and remembered, so a root
  // collapsed once elsewhere would otherwise greet every visit closed.
  const { expandPath } = explorer;
  useEffect(() => {
    if (!location?.workspaceId || !normalizedTreeRootPath) return;
    void expandPath(location.workspaceId, normalizedTreeRootPath, 'directory');
  }, [expandPath, location?.workspaceId, normalizedTreeRootPath]);
  const selectedRow = visibleRows.find((row) => explorerNodeId(row.workspaceId, row.node.path) === explorer.selection?.nodeId);
  const activeTab = useMemo(() => openTabs.find((tab) => tabKey(tab.workspaceId, tab.path) === activeTabKey) ?? null, [activeTabKey, openTabs]);
	const pathSearch = useWorkspacePathSearch({ workspaceId: location?.workspaceId, includeIgnored: explorer.showIgnored });
	const contentSearch = useContentSearch({ kind: 'explorer', mode: explorer.mode, workspaceId: location?.workspaceId, includeIgnored: explorer.showIgnored });
	const searchResults = useMemo<ExplorerUnifiedResult[]>(() => [
		...pathSearch.results.slice(0, 5).map((result) => ({ kind: 'path' as const, result })),
		...contentSearch.results.slice(0, 15).map((result) => ({ kind: 'content' as const, result }))
	], [contentSearch.results, pathSearch.results]);
  const mutations = useWorkspacePathMutations(async (result) => {
    setRefreshWarning(result.refreshRequired ? (result.refreshError || 'The file change was saved, but workspace refresh is required.') : '');
    await explorer.invalidateDirectories(result.workspaceId, result.invalidatedPaths);
    await explorer.expandToPath(result.workspaceId, result.path, result.type);
  });

  const editor = useFileEditorSession({
    save: (file, content) => api.saveWorkspaceFile(location?.workspaceId ?? '', { path: file.path, content, expectedHash: file.hash }).then((result) => {
      setRefreshWarning(result.refreshRequired ? (result.refreshError || 'The file was saved, but workspace refresh is required.') : '');
      return result.file;
    }),
    onSaved: () => void loadDiff(),
    onError: (caught) => showError(caught, 'File save failed')
  });

  useEffect(() => {
    openTabsRef.current = openTabs;
  }, [openTabs]);

  const refreshAfterBranchSwitch = useCallback(async (workspaceId: string) => {
    if (location?.workspaceId === workspaceId) {
      const nextTabs = openTabsRef.current.filter((tab) => tab.workspaceId !== workspaceId);
      setOpenTabs(nextTabs);
      setActiveTabKey((current) => {
        if (!current.startsWith(`${workspaceId}:`)) return current;
        return nextTabs[0] ? tabKey(nextTabs[0].workspaceId, nextTabs[0].path) : '';
      });
      if (!nextTabs.some((tab) => tabKey(tab.workspaceId, tab.path) === activeTabKey)) {
        editor.open(null);
        setDiff('');
      }
      onLocationChange({ workspaceId, mode: explorer.mode });
    }
    pathSearch.setQuery('');
    contentSearch.clear();
    setSearchIndex(0);
    setMatchContext(null);
    await explorer.refreshWorkspaceBranch(workspaceId);
  }, [contentSearch, editor, explorer, location?.workspaceId, onLocationChange, pathSearch]);
  const workspaceBranches = useWorkspaceBranches(embedded ? [] : workspaces, refreshAfterBranchSwitch);

  const switchWorkspaceBranch = async (workspace: WorkspaceConfig, branch: string) => {
    if (location?.workspaceId === workspace.id && editor.dirty && !(await editor.saveNow())) return;
    await workspaceBranches.switchBranch(workspace, branch);
  };

  const showError = (caught: unknown, fallback: string) => {
    setError(caught instanceof Error ? caught.message : fallback);
    setRecoveryHint(caught instanceof ApiError ? caught.recoveryHint ?? '' : '');
  };

  const loadDiff = async (targetWorkspaceId = activeTab?.workspaceId, targetPath = activeTab?.path) => {
    if (!targetWorkspaceId || !targetPath) return setDiff('');
    try {
      setDiff((await api.workspaceFileDiff(targetWorkspaceId, targetPath)).diff ?? '');
    } catch {
      setDiff('');
    }
  };

  const rememberOpenTab = (file: { path: string; editable: boolean }, workspaceId: string) => {
    const nextTab = { workspaceId, path: file.path, editable: file.editable, name: lastPathSegment(file.path) };
    setOpenTabs((current) => {
      const existingIndex = current.findIndex((tab) => tab.workspaceId === workspaceId && tab.path === file.path);
      if (existingIndex >= 0) {
        const next = current.slice();
        next[existingIndex] = nextTab;
        return next;
      }
      return [...current, nextTab];
    });
    setActiveTabKey(tabKey(workspaceId, file.path));
  };

  const loadFile = async (targetWorkspaceId = activeTab?.workspaceId, targetPath = activeTab?.path) => {
    if (!targetWorkspaceId || !targetPath) {
      editor.open(null);
      setDiff('');
      return;
    }
    setError('');
    setRecoveryHint('');
    try {
      const nextFile = await api.workspaceFile(targetWorkspaceId, targetPath);
      editor.open(nextFile);
      rememberOpenTab(nextFile, targetWorkspaceId);
      await loadDiff(targetWorkspaceId, targetPath);
    } catch (caught) {
      editor.open(null);
      showError(caught, 'File failed to load');
    }
  };

  useEffect(() => {
    if (!location?.workspaceId || !location.path || selectedRow?.node.type !== 'file') return;
    const key = tabKey(location.workspaceId, location.path);
    if (key === activeTabKey && editor.file?.path === location.path) return;
    void loadFile(location.workspaceId, location.path);
  }, [activeTabKey, editor.file?.path, location?.path, location?.workspaceId, selectedRow?.node.type]);

  useEffect(() => {
    if (activeTabKey) return;
    editor.open(null);
    setDiff('');
  }, [activeTabKey, editor]);

  const selectRow = async (row: VisibleExplorerRow) => {
    if (editor.dirty && !(await editor.saveNow())) return;
		setMatchContext(null);
    explorer.select(row.workspaceId, row.node.path);
  };

  // A tree restricted to a root (the item's Plan view) opens on the last file
  // directly in that root, as the tree sorts it: a plan's README.md. Once per
  // root, and only while nothing else is open, so it never replaces a file
  // the user opened or one the URL names.
  const openedRootRef = useRef('');
  useEffect(() => {
    const workspaceId = location?.workspaceId;
    if (!workspaceId || !normalizedTreeRootPath) return;
    const rootKey = explorerNodeId(workspaceId, normalizedTreeRootPath);
    if (openedRootRef.current === rootKey) return;
    if (openTabs.length > 0 || (selectedRow && selectedRow.node.type === 'file')) {
      openedRootRef.current = rootKey;
      return;
    }
    const lastRootFile = [...visibleRows].reverse().find((row) => row.parentId === rootKey && row.node.type === 'file');
    if (!lastRootFile) return;
    openedRootRef.current = rootKey;
    void selectRow(lastRootFile);
  }, [location?.workspaceId, normalizedTreeRootPath, openTabs.length, selectedRow, visibleRows]);

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
    if (!selectedRow || selectedRow.node.type === 'workspace') return normalizedTreeRootPath;
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
    const result = treeKeyboardAction(event.key, visibleRows, explorer.activeIndex, explorer.expandedNodeIds);
    if (result.activeIndex === explorer.activeIndex && !result.toggleNodeId && !result.select) return;
    event.preventDefault();
    explorer.setActiveIndex(result.activeIndex);
    if (result.toggleNodeId) {
      const row = visibleRows[result.activeIndex];
      if (row) toggleRow(row);
    }
    if (result.select) void selectRow(result.select);
  };

  const revertFile = async () => {
    if (!editor.file || !location?.workspaceId) return;
    setReverting(true);
    try {
      const result = await api.revertWorkspaceFile(location.workspaceId, { path: editor.file.path });
      setRefreshWarning(result.refreshRequired ? (result.refreshError || 'The file was reverted, but workspace refresh is required.') : '');
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
    const path = activeTab?.path ? `${workspace.path.replace(/\/$/, '')}/${activeTab.path}` : workspace.path;
    void api.openPath(path).catch((caught) => showError(caught, 'Could not reveal path'));
  };

  const activateTab = async (tab: OpenWorkspaceFileTab) => {
    if (editor.dirty && !(await editor.saveNow())) return;
    setMatchContext(null);
    setActiveTabKey(tabKey(tab.workspaceId, tab.path));
    await explorer.expandToPath(tab.workspaceId, tab.path, 'file');
  };

  const closeTab = async (tab: OpenWorkspaceFileTab) => {
    const key = tabKey(tab.workspaceId, tab.path);
    if (key === activeTabKey && editor.dirty && !(await editor.saveNow())) return;
    const currentTabs = openTabsRef.current;
    const index = currentTabs.findIndex((item) => tabKey(item.workspaceId, item.path) === key);
    const remaining = currentTabs.filter((item) => tabKey(item.workspaceId, item.path) !== key);
    const nextTab = key === activeTabKey ? remaining[index] ?? remaining[index - 1] ?? null : activeTab;
    setOpenTabs(remaining);
    if (nextTab) {
      setActiveTabKey(tabKey(nextTab.workspaceId, nextTab.path));
      await explorer.expandToPath(nextTab.workspaceId, nextTab.path, 'file');
      return;
    }
    setActiveTabKey('');
    editor.open(null);
    setDiff('');
    if (location?.workspaceId === tab.workspaceId && location.path === tab.path) {
      const parentPath = parentDirectoryPath(tab.path);
      onLocationChange({ workspaceId: tab.workspaceId, path: parentPath || undefined, mode: explorer.mode });
    }
  };

  const startResize = (side: 'left' | 'right', event: React.PointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    resizeCleanupRef.current?.();
    const start = event.clientX;
    const initial = side === 'left' ? leftWidth : rightWidth;
    let latest = initial;
    const move = (next: PointerEvent) => {
      const width = side === 'left'
        ? boundedLeftPanelWidth(initial + next.clientX - start)
        : boundedRightPanelWidth(initial + start - next.clientX);
      latest = width;
      gridRef.current?.style.setProperty(side === 'left' ? '--explorer-left-width' : '--explorer-right-width', `${width}px`);
      if (side === 'left') setLeftWidth(width); else setRightWidth(width);
    };
    const cleanup = () => {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', up);
      writePreference(side === 'left' ? 'workstreamExplorer.leftWidth' : 'workstreamExplorer.rightWidth', String(latest));
      if (resizeCleanupRef.current === cleanup) resizeCleanupRef.current = null;
    };
    function up() { cleanup(); }
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', up, { once: true });
    resizeCleanupRef.current = cleanup;
  };

  useEffect(() => () => resizeCleanupRef.current?.(), []);

  const gridStyle = {
    '--explorer-left-width': `${leftCollapsed ? 44 : leftWidth}px`,
    '--explorer-right-width': `${rightPanelOpen ? rightWidth : 44}px`
  } as CSSProperties;

  const explorerContent = (
    <>
      {!embedded && (
        <header className="explorer-header">
          <div><span className="eyebrow">{workspace?.name ?? 'No workspace selected'}</span><h1>Workstream Explorer</h1></div>
          <div className="explorer-header-actions" />
        </header>
      )}
      <div className="explorer-grid" style={gridStyle} ref={gridRef}>
        <aside className={leftCollapsed ? 'explorer-tree-panel collapsed' : 'explorer-tree-panel'}>
		  <div className={embedded ? 'explorer-panel-header embedded' : 'explorer-panel-header'}>
			{!leftCollapsed && (embedded ? <div className="explorer-panel-title">{embeddedHeaderContent ?? <strong>Files</strong>}</div> : <strong>Files</strong>)}
            <div className={embedded ? 'explorer-panel-actions compact' : 'explorer-panel-actions'}>
              {!leftCollapsed && showModeSelector && (
                <select aria-label="Explorer tree mode" value={explorer.mode} onChange={(event) => explorer.setMode(event.target.value as 'sources' | 'all')}>
				<option value="sources">Source folders</option>
				<option value="all">Entire workspace</option>
			</select>
              )}
              {!leftCollapsed && !leftPanelContent && <button className={embedded ? 'icon-button explorer-action-button' : 'secondary'} type="button" aria-label="New file" title="New file" disabled={!workspace} onClick={() => setPathDialog({ kind: 'file', parentPath: selectedParentPath() })}>{embedded ? <FilePlus2 size={15} /> : <><FilePlus2 size={15} /> New file</>}</button>}
              {!leftCollapsed && !leftPanelContent && <button className={embedded ? 'icon-button explorer-action-button' : 'secondary'} type="button" aria-label="New folder" title="New folder" disabled={!workspace} onClick={() => setPathDialog({ kind: 'directory', parentPath: selectedParentPath() })}>{embedded ? <FolderPlus size={15} /> : <><FolderPlus size={15} /> New folder</>}</button>}
              {!leftCollapsed && !leftPanelContent && <button className={embedded ? 'icon-button explorer-action-button' : 'secondary'} type="button" aria-label="Rename selected path" title="Rename" disabled={!selectedRow || selectedRow.node.type === 'workspace'} onClick={() => void openRename()}>{embedded ? <Pencil size={15} /> : <><Pencil size={15} /> Rename</>}</button>}
              <button className={embedded ? 'icon-button explorer-action-button' : 'secondary'} type="button" aria-label={leftCollapsed ? 'Expand files panel' : 'Collapse files panel'} title={leftCollapsed ? 'Expand files panel' : 'Collapse files panel'} onClick={() => setLeftCollapsed((value) => !value)}>{embedded ? (leftCollapsed ? <PanelLeftOpen size={15} /> : <PanelLeftClose size={15} />) : <>{leftCollapsed ? <PanelLeftOpen size={15} /> : <PanelLeftClose size={15} />} {leftCollapsed ? 'Expand files' : 'Collapse files'}</>}</button>
              {(!embedded && !leftCollapsed) && <button className="secondary" type="button" aria-label="Refresh" title="Refresh" onClick={explorer.refresh}><RefreshCw size={15} /> Refresh</button>}
            </div>
		  </div>
		  {!leftCollapsed && leftPanelContent && <div className="explorer-left-panel-content">{leftPanelContent}</div>}
		  {!leftCollapsed && !leftPanelContent && !embedded && <div className="explorer-search-context">Search in {workspace?.name ?? 'all workspaces'}</div>}
		  {!leftCollapsed && !leftPanelContent && <div className="explorer-toolbar">
			<label><Search size={15} /><input aria-label="Search files" value={pathSearch.query} onChange={(event) => setExplorerSearchQuery(event.target.value)} onKeyDown={(event) => {
			  if (event.key === 'ArrowDown' && searchResults.length) { event.preventDefault(); setSearchIndex((index) => Math.min(index + 1, searchResults.length - 1)); }
			  if (event.key === 'ArrowUp' && searchResults.length) { event.preventDefault(); setSearchIndex((index) => Math.max(index - 1, 0)); }
			  if (event.key === 'Enter' && searchResults.length) { event.preventDefault(); openUnifiedSearchResult(searchResults[searchIndex] ?? searchResults[0]); }
			  if (event.key === 'Escape') setExplorerSearchQuery('');
			}} placeholder="Search files and text" /></label>
			<button type="button" aria-label="Match case" title="Match case" aria-pressed={contentSearch.caseSensitive} className={contentSearch.caseSensitive ? 'icon-button active' : 'icon-button'} onClick={() => { contentSearch.setCaseSensitive(!contentSearch.caseSensitive); setSearchIndex(0); }}><CaseSensitive size={17} /></button>
		  </div>}
		  {!leftCollapsed && !leftPanelContent && pathSearch.query.trim() && <ExplorerUnifiedSearchResults query={pathSearch.query} results={searchResults} loading={pathSearch.loading || contentSearch.loading} error={pathSearch.error || contentSearch.error} activeIndex={searchIndex} onActiveIndex={setSearchIndex} onOpen={openUnifiedSearchResult} />}
		  {!leftCollapsed && !leftPanelContent && <div className="explorer-tree" ref={treeRef} role="tree" aria-label="Workspace files" tabIndex={0} onKeyDown={onTreeKeyDown}>
            {visibleRows.map((row, index) => (
              <ExplorerTreeRow key={explorerNodeId(row.workspaceId, row.node.path)} row={row} gitState={row.node.type === 'file' ? explorer.gitStateByPath.get(explorerNodeId(row.workspaceId, row.node.path)) : undefined} branchState={workspaceBranches.states[row.workspaceId]} decision={workspaceBranches.decision} onDecision={(strategy, message) => void workspaceBranches.resolveDecision(strategy, message)} onCancelDecision={workspaceBranches.cancelDecision} showBranchSelector={!embedded} active={index === explorer.activeIndex} selected={explorer.selection?.nodeId === explorerNodeId(row.workspaceId, row.node.path)} expanded={explorer.expandedNodeIds.has(explorerNodeId(row.workspaceId, row.node.path))} onFocus={() => explorer.setActiveIndex(index)} onSelect={() => void selectRow(row)} onToggle={() => toggleRow(row)} onBranchChange={(workspace, branch) => void switchWorkspaceBranch(workspace, branch)} />
            ))}
            {visibleRows.length === 0 && <p className="explorer-empty">No matching paths.</p>}
			{showModeSelector && explorer.mode === 'sources' && workspaces.every((item) => sourcePaths(item).length === 0) && <button className="secondary" type="button" onClick={() => explorer.setMode('all')}>Browse All Files</button>}
          </div>}
          {!leftCollapsed && <button className="explorer-resize-handle left" aria-label="Resize workspace tree" onPointerDown={(event) => startResize('left', event)} />}
        </aside>
        <main className={agentArea?.active ? 'workspace-file-editor agent-active' : 'workspace-file-editor'}>
          {agentArea && <div className="explorer-agent-area" hidden={!agentArea.active}>{agentArea.content}</div>}
          {mainContent ?? <>
          <div className="explorer-breadcrumbs">
            <span>{workspace?.name ?? 'Select a workspace'}</span>
            {((activeTab?.path ?? location?.path)?.split('/') ?? []).filter(Boolean).map((part, index, parts) => <span key={`${part}-${index}`}>{index < parts.length && ' / '}{part}</span>)}
            <div>
              <button className="icon-button" type="button" title="Copy path" disabled={!workspace} onClick={() => void navigator.clipboard.writeText(activeTab?.path ?? location?.path ?? workspace?.path ?? '')}><Clipboard size={15} /></button>
              <button className="secondary" type="button" disabled={!workspace} onClick={reveal}>Reveal</button>
            </div>
          </div>
          <div className="editor-file-strip">
            <div className="tab-list file-tab-list" aria-label="Open files">
              {openTabs.length > 0 ? openTabs.map((openTab) => {
                const key = tabKey(openTab.workspaceId, openTab.path);
                return <div key={key} className={key === activeTabKey ? 'file-tab active' : 'file-tab'}>
                  <button type="button" className="file-tab-button" onClick={() => void activateTab(openTab)} title={openTab.path}>{openTab.name}</button>
                  <button type="button" className="file-tab-close" aria-label={`Close ${openTab.name}`} onClick={() => void closeTab(openTab)}><X size={13} /></button>
                </div>;
              }) : <span className="file-tab-placeholder">Open a file</span>}
            </div>
          </div>
          <div className="tabs explorer-editor-tabs">
            <div className="editor-toolbar-actions">
              <div className="editor-view-switch" role="tablist" aria-label="File view mode">
                <button className={tab === 'preview' ? 'active' : ''} onClick={() => setTab('preview')}><Eye size={15} /> Preview</button>
                <button className={tab === 'raw' ? 'active' : ''} onClick={() => setTab('raw')}><Code2 size={15} /> Source</button>
                <button className={tab === 'diff' ? 'active' : ''} onClick={() => setTab('diff')}><GitCompare size={15} /> Diff</button>
              </div>
              <span className={`autosave-state ${editor.state}`}>{autoSaveLabel(editor.state)}</span>
            </div>
          </div>
		  {matchContext && <div className="content-match-context">Line {matchContext.lineNumber}, columns {matchContext.columnStart}–{matchContext.columnEnd}</div>}
          {error && <div className="operation-error"><p className="error">{error}</p>{recoveryHint && <p>{recoveryHint}</p>}<button className="secondary" onClick={() => void loadFile()}>Reload file</button></div>}
          {refreshWarning && <div className="operation-error" role="status"><p>{refreshWarning}</p><button className="secondary" onClick={() => {
            if (!location?.workspaceId) return;
            void api.scan(location.workspaceId).then(() => { setRefreshWarning(''); explorer.refresh(); }).catch((caught) => showError(caught, 'Workspace refresh failed'));
          }}>Retry workspace refresh</button></div>}
		  {tab === 'preview' && (editor.file ? <ContentViewer file={editor.file} content={editor.content} selection={matchContext} modeSwitch={false} /> : <ExplorerEmpty row={selectedRow} />)}
          {tab === 'raw' && <textarea className="raw-editor" value={editor.file ? editor.content : 'Select a file.'} disabled={!editor.file?.editable} onChange={(event) => editor.setContent(event.target.value)} spellCheck={false} />}
          {tab === 'diff' && <ExplorerDiff diff={diff} onRevert={() => setRevertOpen(true)} disabled={!editor.file || reverting} />}
          </>}
        </main>
        <aside className={rightPanelOpen ? `explorer-inspector${rightPanel?.className ? ` ${rightPanel.className}` : ''}` : `explorer-inspector collapsed${rightPanel?.className ? ` ${rightPanel.className}` : ''}`}>
          <div className="panel-header"><h2>{rightPanelTitle}</h2><button className="icon-button" type="button" aria-label={rightPanelOpen ? rightPanelExpandedLabel : rightPanelCollapsedLabel} onClick={() => rightPanel?.onToggle ? rightPanel.onToggle() : setInspectorOpen((value) => !value)}>{rightPanelOpen ? <PanelRightClose size={16} /> : <PanelRightOpen size={16} />}</button></div>
          {rightPanelOpen && (rightPanel ? rightPanel.content : <ExplorerInspector workspace={workspace} row={selectedRow} file={editor.file} />)}
          {rightPanelOpen && <button className="explorer-resize-handle right" aria-label="Resize inspector" onPointerDown={(event) => startResize('right', event)} />}
        </aside>
      </div>
      {revertOpen && editor.file && <ConfirmDialog title="Revert file" message={`Revert ${editor.file.path} to HEAD?`} confirmLabel={reverting ? 'Reverting...' : 'Revert File'} busy={reverting} danger onCancel={() => setRevertOpen(false)} onConfirm={revertFile} />}
      {pathDialog && <ExplorerPathDialog dialog={pathDialog} busy={Boolean(mutations.busy)} error={mutations.error} onCancel={() => { mutations.clearError(); setPathDialog(null); }} onSubmit={submitPathDialog} />}
    </>
  );

  return embedded ? explorerContent : (
    <section className="workstream-explorer-page">
      {explorerContent}
    </section>
  );
}

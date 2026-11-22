import { useEffect, useMemo, useRef, useState } from 'react';
import { useHorizontalDrag } from '../components/useHorizontalDrag';
import { useOpenFileTabs } from '../features/item-workspace/useOpenFileTabs';
import type { CSSProperties, MutableRefObject } from 'react';
import { ArrowLeft, Bot, Code2, FilePlus2, FileText, FolderOpen, FolderPlus, GitCompare, GripVertical, Info, Ticket, PanelLeftClose, PanelLeftOpen, PanelRightClose, PanelRightOpen, Pencil, RefreshCw, X } from 'lucide-react';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { preferredFile } from '../shared/domain/fileTree';
import { CreateItemPathDialog, RenameItemPathDialog } from '../features/item-workspace/ItemPathDialogs';
import { ItemQualityPanel } from '../features/item-workspace/ItemQualityPanel';
import { useItemQuality } from '../features/item-workspace/useItemQuality';
import { ItemGitPanel } from '../features/item-git/ItemGitPanel';
import { useItemGit } from '../features/item-git/useItemGit';
import { StatusMenu } from '../components/StatusMenu';
import { ContentViewer } from '../features/content-viewer/ContentViewer';
import { ApiError, api } from '../shared/api';
import type { FileNode, GitChange, ItemDetail, ItemMetadataUpdateInput, ItemStatus, WorkspaceConfig } from '../lib/types';
import { isDocumentationMetadataSource, labels, metadataSourceLabel } from '../lib/vocabulary';
import { parseGitDiff } from '../shared/domain/diff';
import { notifyReliabilityChanged } from '../features/reliability/hooks';
import { autoSaveLabel, useFileEditorSession } from '../features/file-editor/useFileEditorSession';
import { ContentSearchInput, ContentSearchResults } from '../features/content-search/ContentSearch';
import { useContentSearch } from '../features/content-search/useContentSearch';
import type { ContentSearchSelection, WorkspaceContentSearchResult } from '../lib/types';
import { JiraItemPanel } from '../features/jira/JiraItemPanel';
import { WorkstreamExplorer } from './WorkstreamExplorer';
import { AgentPane, AgentsTab, useHeldAgentSessions, type AgentFocus, type AgentPaneHarness, type AgentsMode } from '../features/workstream/AgentsPanel';
import { HarnessRunTab } from '../features/workstream/HarnessRunTab';
import { takeOverByHand } from '../features/workstream/takeOver';
import { useBoardHarness } from '../features/workstream/useBoardHarness';
import { queueOrder, workForItem } from '../features/workstream/harnessJoin';
import { RunView } from '../features/harness/RunView';
import { RunDrawer } from '../features/harness/RunDrawer';
import { RunPagePath, RunWorkbench } from '../features/harness/RunWorkbench';
import type { HarnessRunTab as HarnessRunTabId, ItemView } from '../app/router';
import '../features/harness/harness.css';
import { useItemSessions } from '../features/workstream/useItemSessions';
import type { ExplorerLocation } from '../features/workstream-explorer/types';
import { BranchCheckoutPicker } from '../features/workstream/BranchCheckoutPicker';
import { useWorkspaceBranches } from '../features/workstream-explorer/useWorkspaceBranches';
import { type Tab, type RightPanelTab, type DiffMode, type PendingConfirm, type DetailViewMode, type OpenItemFileTab } from '../features/item-workspace/itemWorkspaceTypes';
import { fileDirectoryPaths, hasFile, currentGitPath, workspacePathForSelection, buildFileStateMap, normalizePath } from '../features/item-workspace/itemPaths';
import { parentDirectoryPath } from '../shared/domain/paths';
import { visibleItemWarnings } from '../features/item-workspace/itemFormat';
import { DiffPanel } from '../features/item-workspace/DiffPanel';
import { TreeNode } from '../features/item-workspace/FileTreeNode';
import { EmptyDocumentState, StatusBadge } from '../features/item-workspace/ItemBadges';

export function ItemWorkspacePage({ itemId, view, onViewChange, refreshKey, workspaces, onBack, onContentChanged, allowEmbeddedAISessions = true }: { itemId: string; /** `run` shows the item's harness run whole, full width. */ view?: ItemView; onViewChange?: (view?: ItemView) => void; refreshKey: number; workspaces: WorkspaceConfig[]; onBack: () => void; onContentChanged?: () => void | Promise<void>; allowEmbeddedAISessions?: boolean }) {
  const [plan, setPlan] = useState<ItemDetail | null>(null);
  const [files, setFiles] = useState<FileNode[]>([]);
  const [metadataDraft, setMetadataDraft] = useState<ItemMetadataUpdateInput>({});
  const [savingMetadata, setSavingMetadata] = useState(false);
  const [diff, setDiff] = useState('');
  const [diffMode, setDiffMode] = useState<DiffMode>('review');
  const [revertingFile, setRevertingFile] = useState(false);
  const [revertDialogOpen, setRevertDialogOpen] = useState(false);
  const [pendingConfirm, setPendingConfirm] = useState<PendingConfirm | null>(null);
  const [tab, setTab] = useState<Tab>('preview');
  const [rightPanelTab, setRightPanelTab] = useState<RightPanelTab>('info');
  // The item's agent sessions: listed in the Agents tab, shown in the main pane
  // while one is in focus, and held connected until the page leaves the item.
  const [agentFocus, setAgentFocus] = useState<AgentFocus>();
  const { byItem: itemAgents, reload: reloadAgents } = useItemSessions(plan?.workspaceId, plan?.branch ?? '');
  const agents = itemAgents.get(itemId);
  const { heldLiveSessionIds, release: releaseHeldSession } = useHeldAgentSessions(agents, agentFocus, itemId);
  // The harness's work on this item, for the Agents tab's "Harness run" mode and the full-width run view.
  const harness = useBoardHarness();
  const work = useMemo(() => workForItem(plan, harness.state), [plan, harness.state]);
  const [agentsMode, setAgentsMode] = useState<AgentsMode>('run');
  const [runTab, setRunTab] = useState<{ tab: HarnessRunTabId; agentRunId?: string } | undefined>();
  const harnessRun = work?.run ? <HarnessRunTab work={work} harness={harness} onOpenConversation={(id) => setAgentFocus({ kind: 'harness', id })} onOpenPlan={(id) => setAgentFocus({ kind: 'harness-plan', id })} onOpenFull={onViewChange ? () => onViewChange('run') : undefined} onTakeOver={() => void takeOverByHand(work, harness).then(({ agentRunId }) => { if (agentRunId) { setAgentFocus({ kind: 'harness', id: agentRunId }); } else { setAgentsMode('sessions'); setAgentFocus({ kind: 'launch' }); } })} /> : undefined;
  const paneHarness: AgentPaneHarness = { client: harness.client, run: work?.run, detail: work?.run ? harness.state.details[work.run.id] : undefined, reload: () => (work?.run ? harness.loadRun(work.run.id) : Promise.resolve()) };
  const [error, setError] = useState('');
  const [recoveryHint, setRecoveryHint] = useState('');
  const [leftCollapsed, setLeftCollapsed] = useState(false);
  const [rightCollapsed, setRightCollapsed] = useState(false);
  const [leftWidth, setLeftWidth] = useState(300);
  const [rightWidth, setRightWidth] = useState(300);
  const [detailViewMode, setDetailViewMode] = useState<DetailViewMode>('plan');
  const [createPathKind, setCreatePathKind] = useState<'file' | 'directory' | null>(null);
  const [selectedDirectoryPath, setSelectedDirectoryPath] = useState('');
  const [selectedTreeNode, setSelectedTreeNode] = useState<{ path: string; type: 'file' | 'directory' } | null>(null);
  const [renameOpen, setRenameOpen] = useState(false);
  const workspaceGridRef = useRef<HTMLDivElement | null>(null);
  const autoSaveRefreshTimerRef = useRef<number | null>(null);
  const startDrag = useHorizontalDrag();
  const [contentSearchIndex, setContentSearchIndex] = useState(0);
  const [matchContext, setMatchContext] = useState<ContentSearchSelection | null>(null);
  const fileTreeRef = useRef<HTMLDivElement | null>(null);
	const contentSearch = useContentSearch({ kind: 'item', itemId });
  const [workspaceExplorerLocation, setWorkspaceExplorerLocation] = useState<ExplorerLocation>();
  const fileTabs = useOpenFileTabs();
  const { openTabs, activeTabId } = fileTabs;

  const showOperationError = (caught: unknown, fallback: string) => {
    setError(caught instanceof Error ? caught.message : fallback);
    setRecoveryHint(caught instanceof ApiError ? caught.recoveryHint ?? '' : '');
  };

  const showGitResultError = (result: { message?: string; recoveryHint?: string }) => {
    if (!result.message) return;
    setError(result.message);
    setRecoveryHint(result.recoveryHint ?? '');
  };

  const createItemPath = async (createPathName: string): Promise<boolean> => {
    if (!plan?.itemPath || !createPathKind || !createPathName.trim()) return false;
    const parts = createPathName.trim().split('/');
    if (parts.some((part) => !part || part === '.' || part === '..')) {
      setError('Use a relative path without empty, ".", or ".." segments.');
      return false;
    }
    setError('');
    try {
      const directoryParts = createPathKind === 'file' ? parts.slice(0, -1) : parts;
      const existingDirectories = fileDirectoryPaths(files);
      let relativeParent = selectedDirectoryPath;
      for (const directory of directoryParts) {
        const relativePath = relativeParent ? `${relativeParent}/${directory}` : directory;
        if (!existingDirectories.has(relativePath)) {
          const parentPath = relativeParent ? `${plan.itemPath}/${relativeParent}` : plan.itemPath;
          await api.createWorkspaceDirectory(plan.workspaceId, { parentPath, name: directory });
          existingDirectories.add(relativePath);
        }
        relativeParent = relativePath;
      }
      if (createPathKind === 'file') {
        const parentPath = relativeParent ? `${plan.itemPath}/${relativeParent}` : plan.itemPath;
        await api.createWorkspaceFile(plan.workspaceId, { parentPath, name: parts.at(-1)!, content: '' });
      }
      setFiles(await api.files(itemId));
      await onContentChanged?.();
      notifyReliabilityChanged();
      return true;
    } catch (caught) {
      showOperationError(caught, `Could not create ${createPathKind}`);
      return false;
    }
  };

  const renameItemPath = async (renameName: string): Promise<boolean> => {
    if (!plan?.itemPath || !selectedTreeNode || !renameName.trim()) return false;
    const name = renameName.trim();
    if (name === '.' || name === '..' || name.includes('/') || name.includes('\\')) {
      setError('Rename must be a single file or directory name.');
      return false;
    }
    const separator = selectedTreeNode.path.lastIndexOf('/');
    const parent = separator >= 0 ? selectedTreeNode.path.slice(0, separator) : '';
    const destination = parent ? `${parent}/${name}` : name;
    setError('');
    try {
      await api.renameWorkspacePath(plan.workspaceId, {
        path: `${plan.itemPath}/${selectedTreeNode.path}`,
        destinationPath: `${plan.itemPath}/${destination}`
      });
      setFiles(await api.files(itemId));
      if (selectedTreeNode.type === 'file') editor.open(null);
      if (selectedDirectoryPath === selectedTreeNode.path) setSelectedDirectoryPath(destination);
      setSelectedTreeNode({ path: destination, type: selectedTreeNode.type });
      await onContentChanged?.();
      notifyReliabilityChanged();
      return true;
    } catch (caught) {
      showOperationError(caught, 'Could not rename path');
      return false;
    }
  };

  const editor = useFileEditorSession({
    save: (targetFile, content) => api.saveFile(itemId, targetFile.id, { content, expectedHash: targetFile.hash }),
    onSaved: () => {
      scheduleFileChangeRefresh();
      notifyReliabilityChanged();
    },
    onError: (caught) => showOperationError(caught, 'File save failed')
  });
  const { file, content: editorContent, setContent: setEditorContent, dirty: dirtyFile, state: autoSaveState } = editor;

  useEffect(() => {
    const controller = new AbortController();
    setError('');
    setRecoveryHint('');
    setPlan(null);
    setFiles([]);
    setDiff('');
    editor.open(null);
    fileTabs.reset();
    void (async () => {
      try {
        const nextPlan = await api.item(itemId, controller.signal);
        if (controller.signal.aborted) return;
        setPlan(nextPlan);
        const tree = await api.files(itemId, undefined, controller.signal);
        if (controller.signal.aborted) return;
        setFiles(tree);
        const first = preferredFile(tree);
        const diffPromise = api.diff(itemId, controller.signal)
          .then((nextDiff) => { if (!controller.signal.aborted) setDiff(nextDiff.diff); })
          .catch((err) => { if (!controller.signal.aborted) setError(err instanceof Error ? err.message : 'Diff failed to load'); });
        if (first) {
          setSelectedDirectoryPath('');
          setSelectedTreeNode({ path: first.path, type: 'file' });
          const nextFile = await api.file(itemId, first.id, undefined, controller.signal);
          if (controller.signal.aborted) return;
          editor.open(nextFile);
          fileTabs.remember(nextFile);
        } else {
          setSelectedDirectoryPath('');
          setSelectedTreeNode(null);
        }
        await diffPromise;
      } catch (err) {
        if (!controller.signal.aborted) setError(err instanceof Error ? err.message : 'Item failed to load');
      }
    })();
    return () => controller.abort();
  }, [itemId]);

  // A refreshKey bump is the app refreshing (on window focus, among others),
  // not a new item: refresh the item, its file tree and diff in place, and
  // leave the open tabs and any unsaved edit alone.
  const refreshKeyLoadedRef = useRef(refreshKey);
  useEffect(() => {
    if (refreshKeyLoadedRef.current === refreshKey) return;
    refreshKeyLoadedRef.current = refreshKey;
    const controller = new AbortController();
    api.item(itemId, controller.signal).then((nextPlan) => {
      if (!controller.signal.aborted) setPlan((current) => JSON.stringify(current) === JSON.stringify(nextPlan) ? current : nextPlan);
    }).catch(() => undefined);
    api.files(itemId, undefined, controller.signal).then((tree) => {
      if (!controller.signal.aborted) setFiles((current) => JSON.stringify(current) === JSON.stringify(tree) ? current : tree);
    }).catch(() => undefined);
    api.diff(itemId, controller.signal).then((nextDiff) => {
      if (!controller.signal.aborted) setDiff(nextDiff.diff);
    }).catch(() => undefined);
    return () => controller.abort();
  }, [refreshKey]);

  useEffect(() => {
    if (!plan) return;
    setMetadataDraft({
      title: plan.title,
      scope: plan.scope,
      identifier: plan.identifier,
      status: plan.status,
      owner: plan.owner ?? '',
      tags: plan.tags
    });
    void git.loadStatus();
  }, [plan]);



  useEffect(() => {
    if (!plan) {
      setWorkspaceExplorerLocation(undefined);
      return;
    }
    setWorkspaceExplorerLocation((current) => current?.workspaceId === plan.workspaceId
      ? current
      : {
          workspaceId: plan.workspaceId,
          path: normalizePath(plan.itemPath ?? '') || undefined,
          mode: 'all'
        });
  }, [plan?.workspaceId, plan?.itemPath]);


  // One listener for the page's life: it reads whether metadata is unsaved through a ref.
  const dirtyMetadataRef = useRef(false);
  useEffect(() => {
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      if (!dirtyMetadataRef.current) return;
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, []);

  const loadFile = async (fileId: string) => {
    try {
      const nextFile = await api.file(itemId, fileId);
      editor.open(nextFile);
      fileTabs.remember(nextFile);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'File failed to load');
    }
  };

  const openFile = async (fileId: string) => {
    if (dirtyMetadata) {
      setPendingConfirm({
        title: 'Discard changes',
        message: 'Discard unsaved metadata changes and open another file?',
        confirmLabel: 'Discard',
        danger: true,
        onConfirm: () => {
          setPendingConfirm(null);
          void loadFile(fileId);
        }
      });
      return;
    }
    if (dirtyFile && !(await editor.saveNow())) return;
    await loadFile(fileId);
  };

	const openTreeFile = (fileId: string) => {
		setMatchContext(null);
		void openFile(fileId);
	};

  const activateTab = async (tabToOpen: OpenItemFileTab) => {
    if (dirtyFile && !(await editor.saveNow())) return;
    setMatchContext(null);
    setSelectedDirectoryPath('');
    setSelectedTreeNode({ path: tabToOpen.path, type: 'file' });
    fileTabs.activate(tabToOpen.id);
    await loadFile(tabToOpen.id);
  };

  const closeTab = async (tabToClose: OpenItemFileTab) => {
    if (tabToClose.id === activeTabId && dirtyFile && !(await editor.saveNow())) return;
    const { next: nextTab } = fileTabs.close(tabToClose);
    if (nextTab) {
      setSelectedDirectoryPath('');
      setSelectedTreeNode({ path: nextTab.path, type: 'file' });
      await loadFile(nextTab.id);
      return;
    }
    editor.open(null);
    setSelectedTreeNode(parentDirectoryPath(tabToClose.path) ? { path: parentDirectoryPath(tabToClose.path), type: 'directory' } : null);
    setSelectedDirectoryPath(parentDirectoryPath(tabToClose.path));
  };

	const openContentResult = async (result: WorkspaceContentSearchResult) => {
		if (!result.fileId) return;
		if (dirtyFile && !(await editor.saveNow())) return;
		setSelectedDirectoryPath('');
		setSelectedTreeNode({ path: result.path, type: 'file' });
		setMatchContext({ workspaceId: result.workspaceId, itemId: result.itemId, path: result.path, fileId: result.fileId, lineNumber: result.lineNumber, columnStart: result.columnStart, columnEnd: result.columnEnd });
		await openFile(result.fileId);
	};

	useEffect(() => { setMatchContext(null); setContentSearchIndex(0); }, [contentSearch.query]);

  const dirtyMetadata = Boolean(plan) && (
    (metadataDraft.title ?? '') !== (plan?.title ?? '') ||
    (metadataDraft.scope ?? '') !== (plan?.scope ?? '') ||
    (metadataDraft.identifier ?? '') !== (plan?.identifier ?? '') ||
    (metadataDraft.status ?? '') !== (plan?.status ?? '') ||
    (metadataDraft.owner ?? '') !== (plan?.owner ?? '') ||
    (metadataDraft.tags ?? []).join('\n') !== (plan?.tags ?? []).join('\n')
  );
  dirtyMetadataRef.current = dirtyMetadata;
  const dirty = dirtyMetadata;
  const diffFiles = useMemo(() => parseGitDiff(diff), [diff]);
  const selectedGitPath = useMemo(() => currentGitPath(plan, file), [plan, file]);
  const activityPath = plan?.itemPath || '';
  const git = useItemGit({
    workspaceId: plan?.workspaceId,
    activityPath,
    onChanged: () => onContentChanged?.(),
    onProblem: (problem) => 'result' in problem ? showGitResultError(problem.result) : showOperationError(problem.caught, problem.fallback)
  });
  const gitStatus = git.status;
  const selectedFileHasDiff = Boolean(selectedGitPath && diffFiles.some((item) => item.path === selectedGitPath || item.oldPath === selectedGitPath));
  const hasFiles = useMemo(() => hasFile(files), [files]);
  const visibleWarnings = useMemo(() => visibleItemWarnings(plan), [plan]);
  const fileStateByPath = useMemo(() => buildFileStateMap(plan, gitStatus, file, dirtyFile), [plan, gitStatus, file, dirtyFile]);
  /*
   * Derived, not stored in state. An effect would resolve the workspace one
   * render after `plan` arrives, so the page would render the pre-explorer
   * fallback for a frame and then swap layouts. That swap replaces every node
   * in the right panel, including the Info/Jira/Quality tab bar, and a click
   * landing in the window is dispatched into a detached button and lost.
   */
  const workspaceConfig = useMemo(() => plan ? workspaces.find((workspace) => workspace.id === plan.workspaceId) ?? null : null, [plan, workspaces]);
  const quality = useItemQuality({ plan, workspaceConfig, workspaces });
  const explorerWorkspaces = useMemo(() => workspaceConfig ? [workspaceConfig] : [], [workspaceConfig]);
	const checkoutBranches = useWorkspaceBranches(explorerWorkspaces);
  const currentCheckoutBranch = gitStatus?.branch || plan?.branch || workspaceConfig?.baselineBranch || '';
  const gridStyle = {
    '--left-panel-width': `${leftCollapsed ? 44 : leftWidth}px`,
    '--right-panel-width': `${rightCollapsed ? 44 : rightWidth}px`,
  } as CSSProperties & Record<'--left-panel-width' | '--right-panel-width', string>;
  const currentWorkspacePath = workspacePathForSelection(plan, file);
  const itemRootPath = normalizePath(plan?.itemPath ?? '') || undefined;

  useEffect(() => () => clearTimer(autoSaveRefreshTimerRef), []);

  const startResize = (side: 'left' | 'right', event: React.PointerEvent<HTMLButtonElement>) => {
    const startingWidth = side === 'left' ? leftWidth : rightWidth;
    let latestWidth = startingWidth;
    startDrag(event, {
      move: (deltaX) => {
        latestWidth = Math.min(520, Math.max(220, side === 'left' ? startingWidth + deltaX : startingWidth - deltaX));
        workspaceGridRef.current?.style.setProperty(side === 'left' ? '--left-panel-width' : '--right-panel-width', `${latestWidth}px`);
      },
      end: () => (side === 'left' ? setLeftWidth : setRightWidth)(latestWidth),
    });
  };

  const loadDiff = async () => {
    try {
      const payload = await api.diff(itemId);
      setDiff(payload.diff || '');
    } catch {
      setDiff('');
    }
  };

  const refreshGitDetails = async () => {
    if (!plan) return;
    setError('');
    setRecoveryHint('');
    await Promise.all([
      git.loadStatus(),
      git.loadActivity(),
      loadDiff()
    ]);
  };

  const openGitChangePath = (change: GitChange) => {
    if (!plan) return;
    const path = normalizePath(change.path);
    if (!path) return;
    setWorkspaceExplorerLocation((current) => ({
      workspaceId: plan.workspaceId,
      path,
      mode: current?.mode ?? 'all'
    }));
  };

  const goBack = () => {
    if (dirtyFile) {
      void editor.saveNow().then((saved) => {
        if (!saved || dirtyMetadata) return;
        onBack();
      });
      if (!dirtyMetadata) return;
    }
    if (!dirty) return onBack();
    setPendingConfirm({
      title: 'Discard changes',
      message: 'Discard unsaved metadata changes and return to the board?',
      confirmLabel: 'Discard',
      danger: true,
      onConfirm: () => {
        setPendingConfirm(null);
        onBack();
      }
    });
  };

  const openWorkspaceView = () => {
    if (!plan) return;
    setWorkspaceExplorerLocation((current) => ({
      workspaceId: plan.workspaceId,
      path: currentWorkspacePath || current?.path,
      mode: current?.mode ?? 'all'
    }));
    setDetailViewMode('workspace');
  };

  const openPlanView = () => {
    if (!plan) return;
    const rootPath = normalizePath(plan.itemPath ?? '') || undefined;
    setWorkspaceExplorerLocation((current) => ({
      workspaceId: plan.workspaceId,
      path: current?.path && rootPath && current.path.startsWith(`${rootPath}/`) ? current.path : rootPath,
      mode: 'all'
    }));
    setDetailViewMode('plan');
  };

  const openGitView = () => {
    if (!plan) return;
    setWorkspaceExplorerLocation((current) => ({
      workspaceId: plan.workspaceId,
      path: currentWorkspacePath || current?.path,
      mode: current?.mode ?? 'all'
    }));
    setDetailViewMode('git');
  };

  const scheduleFileChangeRefresh = () => {
    clearTimer(autoSaveRefreshTimerRef);
    autoSaveRefreshTimerRef.current = window.setTimeout(() => {
      if (plan) void git.loadStatus();
      void loadDiff();
    }, 700);
  };

  const revertFile = async () => {
    if (!file || !plan) return;
    setRevertingFile(true);
    setError('');
    try {
      await api.revertFile(itemId, file.id);
      const updated = await api.file(itemId, file.id);
      editor.open(updated);
      await loadDiff();
      await git.loadStatus();
      await onContentChanged?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Revert failed');
    } finally {
      setRevertingFile(false);
      setRevertDialogOpen(false);
    }
  };

  const saveMetadata = async () => {
    if (!plan) return;
    setSavingMetadata(true);
    setError('');
    try {
		const result = await api.saveMetadata(itemId, { ...metadataDraft, expectedRevision: plan.metadataRevision });
		setPlan(result.item);
		if (result.refreshRequired) setRecoveryHint(result.refreshError || 'The change was committed; reload the item to refresh its index.');
      if (plan) await git.loadStatus();
      await onContentChanged?.();
      notifyReliabilityChanged();
    } catch (err) {
      showOperationError(err, 'Metadata save failed');
    } finally {
      setSavingMetadata(false);
    }
  };



  const gitPanel = (
    <ItemGitPanel git={git} className="git-panel git-side-panel" activityOpenPreference="item.details.gitActivityOpen" branchPlaceholder="New Branch" onOpenPath={openGitChangePath} unavailable={{ className: 'metadata-callout git-side-panel', hint: 'Refresh the workspace or scan the source to load Git information.' }} />
  );

  const workItemPanelContent = (
    <>
      <div className="side-panel-tabs" role="tablist" aria-label="Item side panel">
        <button type="button" className={rightPanelTab === 'info' ? 'active' : ''} onClick={() => setRightPanelTab('info')}>
          <Info size={14} /> Info
        </button>
        <button type="button" className={rightPanelTab === 'jira' ? 'active' : ''} onClick={() => setRightPanelTab('jira')}>
          <Ticket size={14} /> Jira
        </button>
        <button type="button" className={rightPanelTab === 'quality' ? 'active' : ''} onClick={() => setRightPanelTab('quality')}>
          <GitCompare size={14} /> Quality
        </button>
        <button type="button" className={rightPanelTab === 'agents' ? 'active' : ''} onClick={() => setRightPanelTab('agents')}>
          <Bot size={14} /> Agents{(agents?.live.length ?? 0) > 0 && <span className="side-tab-count" aria-label={`${agents?.live.length} live`}>{agents?.live.length}</span>}
        </button>
      </div>
      {rightPanelTab === 'info' && (
        <>
          {isDocumentationMetadataSource(plan?.metadataSource) && (
            <div className="metadata-callout">
              <strong>{metadataSourceLabel(plan?.metadataSource)}</strong>
              <span>This item is a documentation folder. It is browsable even though it does not use a structured source item layout.</span>
            </div>
          )}
          <dl>
            <dt>{labels.workspace}</dt><dd>{plan?.workspaceName}</dd>
            <dt>{labels.scope}</dt><dd>{plan?.scope}</dd>
            <dt>{labels.identifier}</dt><dd>{plan?.identifier}</dd>
            <dt>Branch</dt><dd>{plan?.branch}</dd>
            <dt>Status</dt><dd>{plan?.status && <StatusBadge status={plan.status} />}</dd>
            <dt>Metadata</dt><dd>{metadataSourceLabel(plan?.metadataSource)}</dd>
            <dt>Author</dt><dd>{plan?.author || plan?.owner || 'Unknown'}</dd>
            <dt>Files</dt><dd>{plan?.counts.files ?? files.length}</dd>
          </dl>
          {!isDocumentationMetadataSource(plan?.metadataSource) && (
            <div className="metadata-form">
              <label>Title<input value={metadataDraft.title ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, title: event.target.value }))} /></label>
              <label>{labels.scope}<input value={metadataDraft.scope ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, scope: event.target.value }))} /></label>
              <label>{labels.identifier}<input value={metadataDraft.identifier ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, identifier: event.target.value }))} /></label>
              <label>Status<StatusMenu value={(metadataDraft.status ?? 'draft') as ItemStatus} onChange={(status) => setMetadataDraft((draft) => ({ ...draft, status }))} /></label>
              <label>Owner<input value={metadataDraft.owner ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, owner: event.target.value }))} /></label>
              <label>Tags<input value={(metadataDraft.tags ?? []).join(', ')} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, tags: event.target.value.split(',').map((tag) => tag.trim()).filter(Boolean) }))} /></label>
            </div>
          )}
          <div className="workspace-actions">
            <button className="save-action save-metadata-action" type="button" disabled={!dirtyMetadata || savingMetadata || isDocumentationMetadataSource(plan?.metadataSource)} onClick={saveMetadata}>{savingMetadata ? 'Saving...' : 'Save Metadata'}</button>
          </div>
          <div className="tags">{(plan?.tags ?? []).map((tag) => <span key={tag}>{tag}</span>)}</div>
          {visibleWarnings.length ? (
            <div className="plan-warnings">
              <h3>Warnings</h3>
              {visibleWarnings.map((warning) => <p key={`${warning.itemPath ?? 'plan'}-${warning.message}`}>{warning.message}</p>)}
            </div>
          ) : null}
        </>
      )}
      {rightPanelTab === 'jira' && <JiraItemPanel itemId={itemId} />}
      {rightPanelTab === 'quality' && <ItemQualityPanel quality={quality} plan={plan} workspaceConfig={workspaceConfig} />}
      {rightPanelTab === 'agents' && <AgentsTab itemId={itemId} agents={agents} focus={agentFocus} onFocus={setAgentFocus} onError={setError} allowEmbedded={allowEmbeddedAISessions} harnessRun={harnessRun} mode={agentsMode} onModeChange={setAgentsMode} onLaunched={(result) => { void Promise.resolve(reloadAgents()).then(() => setAgentFocus({ kind: 'session', id: result.session.id })); }} />}
      {error && (
        <div className="operation-error">
          <p className="error">{error}</p>
          {recoveryHint && <p>{recoveryHint}</p>}
          {recoveryHint && file && (
            <div className="recovery-actions">
              <button className="secondary" type="button" onClick={() => void loadFile(file.id)}><RefreshCw size={14} /> Reload file</button>
              <button className="secondary" type="button" onClick={() => setTab('diff')}><GitCompare size={14} /> View diff</button>
            </div>
          )}
        </div>
      )}
    </>
  );

  /*
   * An empty workspace list means the app has not finished loading workspaces,
   * not that this item's workspace is missing: an item always belongs to one.
   * Keep showing the loading state until the lookup can actually succeed or
   * fail, so the layout gate below settles on its first render.
   */
  if (!plan || (!workspaceConfig && workspaces.length === 0)) {
    return <section className="empty-state"><button className="ghost" onClick={goBack}><ArrowLeft size={16} /> Back</button>{error ? <p className="error">{error}</p> : <p>Loading item...</p>}</section>;
  }

  if (view === 'run' && work?.run) {
    const run = work.run;
    return (
      <section className="workspace-page item-run-page hx" aria-label="Harness run, full width">
        <header className="workspace-header">
          <div className="workspace-header-main">
            <button className="ghost" type="button" onClick={() => (onViewChange ? onViewChange(undefined) : goBack())}><ArrowLeft size={16} /> Back to the card</button>
          </div>
          <div className="workspace-header-title">
            <h1>{plan.title}</h1>
            <RunPagePath run={harness.state.runs[run.id] ?? run} lead={<><span className="workspace-item-path-segment">{plan.scope}</span><span className="workspace-item-path-separator">/</span><span className="workspace-item-path-segment">{plan.identifier}</span></>} />
          </div>
          <div className="workspace-header-actions" />
        </header>
        <RunView state={harness.state} runId={run.id} onSelectRun={() => undefined} onRefresh={() => void harness.refresh()} loadRun={harness.loadRun} onApprove={harness.approve} onReject={harness.reject} onCancel={harness.cancelRun} onOpenTab={(tab, agentRunId) => setRunTab({ tab, agentRunId })} workbench={(detail, request) => <RunWorkbench client={harness.client} detail={detail} request={request} />} />
        {runTab && <RunDrawer state={harness.state} client={harness.client} taskId={work.task.id} tab={runTab.tab} agentRunId={runTab.agentRunId} queue={queueOrder(harness.state, work.task.workspaceId)} onTabChange={(tab) => setRunTab({ tab })} onClose={() => setRunTab(undefined)} onCancel={harness.cancelRun} loadRun={harness.loadRun} onApprove={harness.approve} onReject={harness.reject} onPin={harness.pin} onMove={() => undefined} onSetQuestions={harness.setQuestions} askDefault={harness.state.status?.askQuestions ?? true} />}
      </section>
    );
  }

  return (
    <section className="workspace-page">
      <header className="workspace-header">
        <div className="workspace-header-main">
          <button className="ghost" onClick={goBack}><ArrowLeft size={16} /> Back</button>
        </div>
        <div className="workspace-header-title">
          <h1>{plan?.title ?? 'Loading item'}</h1>
          <div className="workspace-item-path" aria-label="Item location">
            <span className="workspace-item-path-segment">{plan?.scope ?? '...'}</span>
            <span className="workspace-item-path-separator">/</span>
            {workspaceConfig ? <><span className="sr-only">Checkout: {currentCheckoutBranch || workspaceConfig.baselineBranch}</span><BranchCheckoutPicker workspaceId={workspaceConfig.id} currentCheckoutBranch={currentCheckoutBranch || workspaceConfig.baselineBranch} branches={checkoutBranches.states[workspaceConfig.id]?.branches ?? []} gitStatus={gitStatus} ariaLabel="Select checkout branch" listboxLabel="Checkout branches" onSwitched={async () => { await onContentChanged?.(); }} /></> : <span className="workspace-item-path-segment">Checkout: {currentCheckoutBranch || '...'}</span>}
            <span className="workspace-item-path-separator">/</span>
            <span className="workspace-item-path-segment">{plan?.identifier ?? '...'}</span>
          </div>
        </div>
        <div className="workspace-header-actions">
          <button
            className={`icon-button workspace-git-status-button${gitStatus?.dirty ? ' is-dirty' : ''}`}
            type="button"
            aria-label={gitStatus?.dirty ? 'Refresh Git status, activity, and diff; local changes present' : 'Refresh Git status, activity, and diff'}
            title={gitStatus?.dirty ? 'Local changes present. Refresh Git status, activity, and diff.' : 'Refresh Git status, activity, and diff'}
            disabled={git.statusLoading || !plan}
            onClick={() => void refreshGitDetails()}
          >
            <RefreshCw size={18} />
          </button>
        </div>
      </header>
      {plan && workspaceConfig ? (
        <WorkstreamExplorer
          embedded
          showModeSelector={false}
          treeRootPath={detailViewMode === 'plan' ? itemRootPath : undefined}
          leftPanelContent={detailViewMode === 'git' ? gitPanel : undefined}
          agentArea={agentFocus || heldLiveSessionIds.length > 0 ? {
            active: Boolean(agentFocus),
            content: <section className="drawer-main drawer-agent-main"><AgentPane focus={agentFocus} heldSessionIds={heldLiveSessionIds} itemId={itemId} workspaceId={plan?.workspaceId} agents={agents} harness={paneHarness} onBack={() => setAgentFocus(undefined)} onFocus={setAgentFocus} onRelease={releaseHeldSession} onReload={reloadAgents} /></section>
          } : undefined}
          rightPanel={{
            title: <><Info size={16} /> Work Item</>,
            content: workItemPanelContent,
            collapsed: rightCollapsed,
            onToggle: () => setRightCollapsed((value) => !value),
            className: 'metadata-panel side-panel',
            collapsedLabel: 'Expand item info',
            expandedLabel: 'Collapse item info'
          }}
          embeddedHeaderContent={
            <div className="segmented-control segmented-control-compact" role="tablist" aria-label="Item detail view mode">
              <button type="button" className={detailViewMode === 'plan' ? 'active' : ''} aria-selected={detailViewMode === 'plan'} onClick={openPlanView}>Plan</button>
              <button type="button" className={detailViewMode === 'workspace' ? 'active' : ''} aria-selected={detailViewMode === 'workspace'} onClick={openWorkspaceView}>Explorer</button>
              <button type="button" className={detailViewMode === 'git' ? 'active' : ''} aria-selected={detailViewMode === 'git'} onClick={openGitView}>Git</button>
            </div>
          }
          workspaces={explorerWorkspaces}
          location={workspaceExplorerLocation}
          onLocationChange={setWorkspaceExplorerLocation}
        />
      ) : (
      <div className="workspace-grid" style={gridStyle} ref={workspaceGridRef}>
        <aside className={leftCollapsed ? 'file-tree side-panel collapsed' : 'file-tree side-panel'}>
          <div className="panel-header">
            <button className={selectedTreeNode ? 'ghost' : 'ghost active'} type="button" title="Select item root" onClick={() => { setSelectedDirectoryPath(''); setSelectedTreeNode(null); }}><FolderOpen size={16} /> Files</button>
            <div className="workspace-header-actions">
              {!leftCollapsed && <button className="icon-button" type="button" aria-label="New file" title="New file" onClick={() => setCreatePathKind('file')}><FilePlus2 size={16} /></button>}
              {!leftCollapsed && <button className="icon-button" type="button" aria-label="New folder" title="New folder" onClick={() => setCreatePathKind('directory')}><FolderPlus size={16} /></button>}
              {!leftCollapsed && <button className="icon-button" type="button" aria-label="Rename selected path" title="Rename" disabled={!selectedTreeNode} onClick={() => setRenameOpen(true)}><Pencil size={16} /></button>}
              <button className="icon-button" type="button" title={leftCollapsed ? 'Expand files' : 'Collapse files'} onClick={() => setLeftCollapsed((value) => !value)}>
                {leftCollapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
              </button>
            </div>
          </div>
          {!leftCollapsed && (
			<>
				<ContentSearchInput label="Search inside this item" query={contentSearch.query} onQueryChange={contentSearch.setQuery} caseSensitive={contentSearch.caseSensitive} onCaseSensitiveChange={(value) => { setMatchContext(null); setContentSearchIndex(0); contentSearch.setCaseSensitive(value); }} />
				{contentSearch.query.trim().length >= 2 && <ContentSearchResults {...contentSearch} activeIndex={contentSearchIndex} onActiveIndex={setContentSearchIndex} onOpen={(result) => void openContentResult(result)} onEscape={contentSearch.clear} treeRef={fileTreeRef} showWorkspaceContext={false} />}
			</>
		  )}
		  {!leftCollapsed && (
			<div className="file-tree-list" ref={fileTreeRef} tabIndex={-1}>
              {files.map((node) => <TreeNode node={node} key={node.id} onOpen={openTreeFile} activePath={selectedTreeNode?.type === 'file' ? selectedTreeNode.path : undefined} depth={0} fileStateByPath={fileStateByPath} selectedDirectoryPath={selectedDirectoryPath} onSelectDirectory={(path) => { setSelectedDirectoryPath(path); setSelectedTreeNode({ path, type: 'directory' }); }} onSelectFile={(path) => { setSelectedDirectoryPath(''); setSelectedTreeNode({ path, type: 'file' }); }} />)}
            </div>
          )}
          {!leftCollapsed && (
            <button className="panel-resize-handle panel-resize-handle-left" type="button" aria-label="Resize files panel" onPointerDown={(event) => startResize('left', event)}>
              <GripVertical size={16} />
            </button>
          )}
        </aside>
        <div className="document-panel">
          <div className="editor-file-strip">
            <div className="tab-list file-tab-list" aria-label="Open files">
              {openTabs.length > 0 ? openTabs.map((openTab) => (
                <div key={openTab.id} className={openTab.id === activeTabId ? 'file-tab active' : 'file-tab'}>
                  <button type="button" className="file-tab-button" onClick={() => void activateTab(openTab)} title={openTab.path}>{openTab.name}</button>
                  <button type="button" className="file-tab-close" aria-label={`Close ${openTab.name}`} onClick={() => void closeTab(openTab)}><X size={13} /></button>
                </div>
              )) : <span className="file-tab-placeholder">Open a file</span>}
            </div>
          </div>
          <div className="tabs">
            <div className="editor-toolbar-actions">
              <div className="editor-view-switch" role="tablist" aria-label="File view mode">
                <button className={tab === 'preview' ? 'active' : ''} onClick={() => setTab('preview')}><FileText size={15} /> Preview</button>
                <button className={tab === 'raw' ? 'active' : ''} onClick={() => setTab('raw')}><Code2 size={15} /> Source</button>
                <button className={tab === 'diff' ? 'active' : ''} onClick={() => setTab('diff')}><GitCompare size={15} /> Diff</button>
              </div>
              <span className={`autosave-state ${autoSaveState}`}>{autoSaveLabel(autoSaveState)}</span>
            </div>
          </div>
		  {matchContext && <div className="content-match-context">Line {matchContext.lineNumber}, columns {matchContext.columnStart}–{matchContext.columnEnd}</div>}
          {(dirtyMetadata || dirtyFile || autoSaveState !== 'idle') && <div className="edit-state-banner">{dirtyMetadata ? 'Unsaved metadata changes' : autoSaveLabel(autoSaveState)}</div>}
		  {tab === 'preview' && (file ? <ContentViewer file={file} content={editorContent} selection={matchContext} modeSwitch={false} /> : <EmptyDocumentState hasFiles={hasFiles} />)}
          {tab === 'raw' && (
            <textarea
              className="raw-editor"
              value={file ? editorContent : (hasFiles ? 'Select a file.' : 'No files found in this plan.')}
              onChange={(event) => setEditorContent(event.target.value)}
              disabled={!file || !file.editable}
              spellCheck={false}
            />
          )}
          {tab === 'diff' && (
            <DiffPanel
              diff={diff}
              files={diffFiles}
              mode={diffMode}
              selectedPath={selectedGitPath}
              selectedFileHasDiff={selectedFileHasDiff}
              reverting={revertingFile}
              onModeChange={setDiffMode}
              onRevertFile={() => setRevertDialogOpen(true)}
            />
          )}
        </div>
        <aside className={rightCollapsed ? 'metadata-panel side-panel collapsed' : 'metadata-panel side-panel'}>
          <div className="panel-header">
            <h2><Info size={16} /> Work Item</h2>
            <button className="icon-button" type="button" title={rightCollapsed ? 'Expand item info' : 'Collapse item info'} onClick={() => setRightCollapsed((value) => !value)}>
              {rightCollapsed ? <PanelRightOpen size={16} /> : <PanelRightClose size={16} />}
            </button>
          </div>
          {!rightCollapsed && workItemPanelContent}
          {!rightCollapsed && (
            <button className="panel-resize-handle panel-resize-handle-right" type="button" aria-label="Resize item info panel" onPointerDown={(event) => startResize('right', event)}>
              <GripVertical size={16} />
            </button>
          )}
        </aside>
      </div>
      )}
      {createPathKind && <CreateItemPathDialog kind={createPathKind} parent={selectedDirectoryPath} onCancel={() => setCreatePathKind(null)} onCreate={createItemPath} />}
      {renameOpen && selectedTreeNode && <RenameItemPathDialog node={selectedTreeNode} onCancel={() => setRenameOpen(false)} onRename={renameItemPath} />}
      {revertDialogOpen && file && (
        <ConfirmDialog
          title="Revert file"
          message={dirtyFile ? `Discard unsaved editor changes and revert ${file.path} to HEAD?` : `Revert ${file.path} to HEAD?`}
          confirmLabel={revertingFile ? 'Reverting...' : 'Revert File'}
          busy={revertingFile}
          danger
          onCancel={() => setRevertDialogOpen(false)}
          onConfirm={revertFile}
        />
      )}
      {pendingConfirm && (
        <ConfirmDialog
          title={pendingConfirm.title}
          message={pendingConfirm.message}
          confirmLabel={pendingConfirm.confirmLabel}
          danger={pendingConfirm.danger}
          onCancel={() => setPendingConfirm(null)}
          onConfirm={pendingConfirm.onConfirm}
        />
      )}
    </section>
  );
}

function clearTimer(ref: MutableRefObject<number | null>) {
  if (ref.current === null) return;
  window.clearTimeout(ref.current);
  ref.current = null;
}

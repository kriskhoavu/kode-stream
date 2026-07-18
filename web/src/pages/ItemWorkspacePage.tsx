import { memo, useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, MutableRefObject } from 'react';
import {
  ArrowLeft,
  ChevronDown,
  Code2,
  File as FileIcon,
  FilePlus2,
  FileText,
  FolderOpen,
  FolderPlus,
  GitBranch,
  GitCompare,
  GripVertical,
  Info,
  Ticket,
  RotateCcw,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightClose,
  PanelRightOpen,
  Pencil,
  RefreshCw,
  X,
} from 'lucide-react';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { RecentGitActivity } from '../components/RecentGitActivity';
import { StatusMenu } from '../components/StatusMenu';
import { ContentViewer } from '../features/content-viewer/ContentViewer';
import { ApiError, api, statusLabels } from '../lib/api';
import type { FileContent, FileNode, GitActivityEntry, GitChange, GitStatus, ItemDetail, ItemMetadataUpdateInput, ItemStatus, ItemVerificationTests, VerificationJob, VerificationTestSelection, VerifyProfile, WorkspaceConfig, WorkspaceTreeEntry } from '../lib/types';
import { labels, metadataSourceLabel } from '../lib/vocabulary';
import { parseGitDiff } from '../shared/domain/diff';
import type { DiffFile } from '../shared/domain/diff';
import { notifyReliabilityChanged } from '../features/reliability/hooks';
import { autoSaveLabel, useFileEditorSession } from '../features/file-editor/useFileEditorSession';
import { FileStateIcon } from '../features/file-tree/FileStateIcon';
import type { TreeFileState } from '../features/file-tree/FileStateIcon';
import { ContentSearchInput, ContentSearchResults } from '../features/content-search/ContentSearch';
import { useContentSearch } from '../features/content-search/useContentSearch';
import type { ContentSearchSelection, WorkspaceContentSearchResult } from '../lib/types';
import { AISessionLaunchControl } from '../features/ai-session/AISessionLaunchControl';
import { JiraItemPanel } from '../features/jira/JiraItemPanel';
import { WorkstreamExplorer } from './WorkstreamExplorer';
import type { ExplorerLocation } from '../features/workstream-explorer/types';
import { useWorkspaceBranches } from '../features/workstream-explorer/useWorkspaceBranches';
import { BranchSnapshotPicker } from '../features/workstream/BranchSnapshotPicker';

type Tab = 'preview' | 'raw' | 'diff';
type RightPanelTab = 'info' | 'jira' | 'quality';
type DiffMode = 'review' | 'raw';
type PendingConfirm = { title: string; message: string; confirmLabel: string; danger?: boolean; onConfirm: () => void };
type DetailViewMode = 'plan' | 'workspace' | 'git';
type BranchViewState = { branch: string; currentCheckoutBranch: string; sourceMode: 'working_tree' | 'snapshot'; missing: true };
type OpenItemFileTab = { id: string; path: string; name: string; editable: boolean };

export function ItemWorkspacePage({ itemId, refreshKey, workspaces, onBack, onOpenItem, onContentChanged }: { itemId: string; refreshKey: number; workspaces: WorkspaceConfig[]; onBack: () => void; onOpenItem: (itemId: string) => void; onContentChanged?: () => void | Promise<void> }) {
  const [plan, setPlan] = useState<ItemDetail | null>(null);
  const [files, setFiles] = useState<FileNode[]>([]);
  const [metadataDraft, setMetadataDraft] = useState<ItemMetadataUpdateInput>({});
  const [savingMetadata, setSavingMetadata] = useState(false);
  const [gitStatus, setGitStatus] = useState<GitStatus | null>(null);
  const [gitActivity, setGitActivity] = useState<GitActivityEntry[]>([]);
  const [gitActivityLoading, setGitActivityLoading] = useState(false);
  const [gitLoading, setGitLoading] = useState(false);
  const [gitMessage, setGitMessage] = useState('');
  const [selectedGitPaths, setSelectedGitPaths] = useState<string[]>([]);
  const [branchName, setBranchName] = useState('');
  const [gitBusy, setGitBusy] = useState('');
  const [gitActivityOpen, setGitActivityOpen] = useState(() => readStoredToggle('item.details.gitActivityOpen'));
  const [diff, setDiff] = useState('');
  const [diffMode, setDiffMode] = useState<DiffMode>('review');
  const [revertingFile, setRevertingFile] = useState(false);
  const [revertDialogOpen, setRevertDialogOpen] = useState(false);
  const [pendingConfirm, setPendingConfirm] = useState<PendingConfirm | null>(null);
  const [tab, setTab] = useState<Tab>('preview');
  const [rightPanelTab, setRightPanelTab] = useState<RightPanelTab>('info');
  const [error, setError] = useState('');
  const [recoveryHint, setRecoveryHint] = useState('');
  const [branchLoading, setBranchLoading] = useState(false);
  const [branchView, setBranchView] = useState<BranchViewState | null>(null);
  const [leftCollapsed, setLeftCollapsed] = useState(false);
  const [rightCollapsed, setRightCollapsed] = useState(false);
  const [leftWidth, setLeftWidth] = useState(300);
  const [rightWidth, setRightWidth] = useState(300);
  const [detailViewMode, setDetailViewMode] = useState<DetailViewMode>('plan');
  const [aiLaunchMessage, setAILaunchMessage] = useState('');
  const [createPathKind, setCreatePathKind] = useState<'file' | 'directory' | null>(null);
  const [createPathName, setCreatePathName] = useState('');
  const [creatingPath, setCreatingPath] = useState(false);
  const [selectedDirectoryPath, setSelectedDirectoryPath] = useState('');
  const [selectedTreeNode, setSelectedTreeNode] = useState<{ path: string; type: 'file' | 'directory' } | null>(null);
  const [renameName, setRenameName] = useState('');
  const [renameOpen, setRenameOpen] = useState(false);
  const [renamingPath, setRenamingPath] = useState(false);
  const workspaceGridRef = useRef<HTMLDivElement | null>(null);
  const autoSaveRefreshTimerRef = useRef<number | null>(null);
  const [contentSearchIndex, setContentSearchIndex] = useState(0);
  const [matchContext, setMatchContext] = useState<ContentSearchSelection | null>(null);
  const fileTreeRef = useRef<HTMLDivElement | null>(null);
	const contentSearch = useContentSearch({ kind: 'item', itemId });
  const [verificationJob, setVerificationJob] = useState<VerificationJob | null>(null);
  const [verificationBusy, setVerificationBusy] = useState(false);
  const [verificationError, setVerificationError] = useState('');
  const [verificationTests, setVerificationTests] = useState<ItemVerificationTests | null>(null);
  const [verificationTestsBusy, setVerificationTestsBusy] = useState(false);
  const [automationLaunchBusy, setAutomationLaunchBusy] = useState(false);
  const [manualSpec, setManualSpec] = useState('');
  const [specPickerOpen, setSpecPickerOpen] = useState(false);
  const [specPickerPath, setSpecPickerPath] = useState('');
  const [specPickerEntries, setSpecPickerEntries] = useState<WorkspaceTreeEntry[]>([]);
  const [specPickerSelected, setSpecPickerSelected] = useState<string[]>([]);
  const [specPickerLoading, setSpecPickerLoading] = useState(false);
  const [specPickerError, setSpecPickerError] = useState('');
  const [workspaceConfig, setWorkspaceConfig] = useState<WorkspaceConfig | null>(null);
  const [artifactPreview, setArtifactPreview] = useState<{ title: string; path: string; content: string; loading: boolean; error: string } | null>(null);
  const [workspaceExplorerLocation, setWorkspaceExplorerLocation] = useState<ExplorerLocation>();
  const [openTabs, setOpenTabs] = useState<OpenItemFileTab[]>([]);
  const [activeTabId, setActiveTabId] = useState('');
  const openTabsRef = useRef<OpenItemFileTab[]>([]);

  const showOperationError = (caught: unknown, fallback: string) => {
    setError(caught instanceof Error ? caught.message : fallback);
    setRecoveryHint(caught instanceof ApiError ? caught.recoveryHint ?? '' : '');
  };

  const showGitResultError = (result: { message?: string; recoveryHint?: string }) => {
    if (!result.message) return;
    setError(result.message);
    setRecoveryHint(result.recoveryHint ?? '');
  };

  const createItemPath = async () => {
    if (!plan?.itemPath || !createPathKind || !createPathName.trim()) return;
    const parts = createPathName.trim().split('/');
    if (parts.some((part) => !part || part === '.' || part === '..')) {
      setError('Use a relative path without empty, ".", or ".." segments.');
      return;
    }
    setCreatingPath(true);
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
      setCreatePathKind(null);
      setCreatePathName('');
      await onContentChanged?.();
      notifyReliabilityChanged();
    } catch (caught) {
      showOperationError(caught, `Could not create ${createPathKind}`);
    } finally {
      setCreatingPath(false);
    }
  };

  const renameItemPath = async () => {
    if (!plan?.itemPath || !selectedTreeNode || !renameName.trim()) return;
    const name = renameName.trim();
    if (name === '.' || name === '..' || name.includes('/') || name.includes('\\')) {
      setError('Rename must be a single file or directory name.');
      return;
    }
    const separator = selectedTreeNode.path.lastIndexOf('/');
    const parent = separator >= 0 ? selectedTreeNode.path.slice(0, separator) : '';
    const destination = parent ? `${parent}/${name}` : name;
    setRenamingPath(true);
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
      setRenameName('');
      setRenameOpen(false);
      await onContentChanged?.();
      notifyReliabilityChanged();
    } catch (caught) {
      showOperationError(caught, 'Could not rename path');
    } finally {
      setRenamingPath(false);
    }
  };

  const editor = useFileEditorSession({
    save: (targetFile, content) => {
      const materializeConfirmed = confirmSnapshotMaterialization(plan, 'file');
      if (materializeConfirmed === null) throw new Error('Snapshot materialization canceled');
      return api.saveFile(itemId, targetFile.id, { content, expectedHash: targetFile.hash, materializeConfirmed });
    },
    onSaved: () => {
      scheduleFileChangeRefresh();
      notifyReliabilityChanged();
    },
    onError: (caught) => showOperationError(caught, 'File save failed')
  });
  const { file, content: editorContent, setContent: setEditorContent, dirty: dirtyFile, state: autoSaveState } = editor;

  useEffect(() => {
    openTabsRef.current = openTabs;
  }, [openTabs]);

  useEffect(() => {
    setError('');
    setRecoveryHint('');
    setBranchView(null);
    editor.open(null);
    setOpenTabs([]);
    setActiveTabId('');
    api.item(itemId).then(setPlan).catch((err: Error) => setError(err.message));
    api.files(itemId).then((tree) => {
      setFiles(tree);
      const first = preferredFile(tree);
      if (first) {
        setSelectedDirectoryPath('');
        setSelectedTreeNode({ path: first.path, type: 'file' });
        void openFile(first.id);
      } else {
        setSelectedDirectoryPath('');
        setSelectedTreeNode(null);
      }
    }).catch((err: Error) => setError(err.message));
    void loadDiff();
  }, [itemId, refreshKey]);

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
    void loadGitStatus(plan.workspaceId);
  }, [plan]);

  useEffect(() => {
    if (!plan) {
      setWorkspaceConfig(null);
      return;
    }
    setWorkspaceConfig(workspaces.find((workspace) => workspace.id === plan.workspaceId) ?? null);
  }, [plan, workspaces]);

  useEffect(() => {
    if (!plan) {
      setVerificationTests(null);
      return;
    }
    let active = true;
    setVerificationTestsBusy(true);
    const loadVerificationTests = api.itemVerificationTests;
    if (typeof loadVerificationTests !== 'function') {
      setVerificationTests(null);
      setVerificationTestsBusy(false);
      return;
    }
    void loadVerificationTests(plan.id)
      .then((tests) => { if (active) setVerificationTests(tests); })
      .catch(() => { if (active) setVerificationTests(null); })
      .finally(() => { if (active) setVerificationTestsBusy(false); });
    return () => { active = false; };
  }, [plan?.id]);

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

  useEffect(() => {
    if (!plan || !verificationJob || verificationJob.status === 'failed' || verificationJob.status === 'passed') return;
    let active = true;
    const timer = window.setInterval(() => {
      void api.verificationJob(plan.workspaceId, verificationJob.id)
        .then((job) => {
          if (!active) return;
          setVerificationJob(job);
        })
        .catch(() => undefined);
    }, 1000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [plan, verificationJob]);

  useEffect(() => {
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      if (!dirtyMetadata) return;
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  });

  const loadFile = async (fileId: string) => {
    try {
      const nextFile = await api.file(itemId, fileId);
      editor.open(nextFile);
      rememberOpenTab(nextFile);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'File failed to load');
    }
  };

  const rememberOpenTab = (nextFile: FileContent) => {
    const nextTab = { id: nextFile.id, path: nextFile.path, editable: nextFile.editable, name: lastPathSegment(nextFile.path) };
    setOpenTabs((current) => {
      const existingIndex = current.findIndex((tab) => tab.id === nextTab.id);
      if (existingIndex >= 0) {
        const next = current.slice();
        next[existingIndex] = nextTab;
        return next;
      }
      return [...current, nextTab];
    });
    setActiveTabId(nextTab.id);
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
    setActiveTabId(tabToOpen.id);
    await loadFile(tabToOpen.id);
  };

  const closeTab = async (tabToClose: OpenItemFileTab) => {
    if (tabToClose.id === activeTabId && dirtyFile && !(await editor.saveNow())) return;
    const currentTabs = openTabsRef.current;
    const index = currentTabs.findIndex((tab) => tab.id === tabToClose.id);
    const remaining = currentTabs.filter((tab) => tab.id !== tabToClose.id);
    const nextTab = tabToClose.id === activeTabId ? remaining[index] ?? remaining[index - 1] ?? null : currentTabs.find((tab) => tab.id === activeTabId) ?? null;
    setOpenTabs(remaining);
    if (nextTab) {
      setActiveTabId(nextTab.id);
      setSelectedDirectoryPath('');
      setSelectedTreeNode({ path: nextTab.path, type: 'file' });
      await loadFile(nextTab.id);
      return;
    }
    setActiveTabId('');
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
  const dirty = dirtyMetadata;
  const diffFiles = useMemo(() => parseGitDiff(diff), [diff]);
  const selectedGitPath = useMemo(() => currentGitPath(plan, file), [plan, file]);
  const activityPath = plan?.itemPath || '';
  const selectedFileHasDiff = Boolean(selectedGitPath && diffFiles.some((item) => item.path === selectedGitPath || item.oldPath === selectedGitPath));
  const hasFiles = useMemo(() => hasFile(files), [files]);
  const visibleWarnings = useMemo(() => visibleItemWarnings(plan), [plan]);
  const fileStateByPath = useMemo(() => buildFileStateMap(plan, gitStatus, file, dirtyFile), [plan, gitStatus, file, dirtyFile]);
  const explorerWorkspaces = useMemo(() => workspaceConfig ? [workspaceConfig] : [], [workspaceConfig]);
  const selectedDetailBranch = branchView?.branch ?? plan?.branch ?? '';
  const detailSourceMode = branchView?.sourceMode ?? plan?.sourceMode ?? 'working_tree';
  const itemWorkspaceBranches = useWorkspaceBranches(explorerWorkspaces);
  const currentCheckoutBranch = gitStatus?.branch || itemWorkspaceBranches.states[workspaceConfig?.id ?? '']?.current || workspaceConfig?.baselineBranch || '';
  const branchOptions = useMemo(() => unique([
    ...(workspaceConfig ? itemWorkspaceBranches.states[workspaceConfig.id]?.branches ?? [] : []),
    currentCheckoutBranch,
    workspaceConfig?.baselineBranch ?? '',
    selectedDetailBranch
  ]), [currentCheckoutBranch, itemWorkspaceBranches.states, selectedDetailBranch, workspaceConfig]);
  const switchItemBranch = async (branch: string) => {
    if (!workspaceConfig || !plan || branch === selectedDetailBranch) return;
    if (branch === plan.branch) {
      setBranchView(null);
      setError('');
      setRecoveryHint('');
      return;
    }
    if (dirtyMetadata) {
      setError('Save metadata changes before loading another branch snapshot.');
      return;
    }
    if (dirtyFile && !(await editor.saveNow())) return;
    setBranchLoading(true);
    setError('');
    setRecoveryHint('');
    try {
      const result = await api.loadWorkstreamBranch(workspaceConfig.id, { branch });
      const matched = matchingBranchItem(result.items, plan);
      if (!matched) {
        editor.open(null);
        setFiles([]);
        setDiff('');
        setMatchContext(null);
        setSelectedDirectoryPath('');
        setSelectedTreeNode(null);
        setBranchView({
          branch: result.branch || branch,
          currentCheckoutBranch: result.currentCheckoutBranch,
          sourceMode: result.sourceMode,
          missing: true
        });
        return;
      }
      setBranchView(null);
      onOpenItem(matched.id);
    } catch (caught) {
      showOperationError(caught, 'Branch snapshot failed to load');
    } finally {
      setBranchLoading(false);
    }
  };
  const gridStyle = {
    '--left-panel-width': `${leftCollapsed ? 44 : leftWidth}px`,
    '--right-panel-width': `${rightCollapsed ? 44 : rightWidth}px`,
  } as CSSProperties & Record<'--left-panel-width' | '--right-panel-width', string>;
  const currentWorkspacePath = workspacePathForSelection(plan, file);
  const itemRootPath = normalizePath(plan?.itemPath ?? '') || undefined;

  useEffect(() => () => {
    clearTimer(autoSaveRefreshTimerRef);
  }, []);

  const startResize = (side: 'left' | 'right', event: React.PointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const startingWidth = side === 'left' ? leftWidth : rightWidth;
    let latestWidth = startingWidth;

    const onPointerMove = (moveEvent: PointerEvent) => {
      const delta = moveEvent.clientX - startX;
      const nextWidth = side === 'left' ? startingWidth + delta : startingWidth - delta;
      const boundedWidth = Math.min(520, Math.max(220, nextWidth));
      latestWidth = boundedWidth;
      workspaceGridRef.current?.style.setProperty(side === 'left' ? '--left-panel-width' : '--right-panel-width', `${boundedWidth}px`);
    };

    const onPointerUp = () => {
      document.body.classList.remove('is-resizing-panel');
      window.removeEventListener('pointermove', onPointerMove);
      window.removeEventListener('pointerup', onPointerUp);
      if (side === 'left') {
        setLeftWidth(latestWidth);
      } else {
        setRightWidth(latestWidth);
      }
    };

    document.body.classList.add('is-resizing-panel');
    window.addEventListener('pointermove', onPointerMove);
    window.addEventListener('pointerup', onPointerUp, { once: true });
  };

  const loadGitStatus = async (workspaceId: string) => {
    setGitLoading(true);
    try {
      setGitStatus(await api.gitStatus(workspaceId));
    } catch {
      setGitStatus(null);
    } finally {
      setGitLoading(false);
    }
  };

  const loadGitActivity = async (workspaceId: string, path: string) => {
    setGitActivityLoading(true);
    try {
      setGitActivity(await api.gitActivity(workspaceId, { path: path || undefined, limit: 8 }));
    } catch {
      setGitActivity([]);
    } finally {
      setGitActivityLoading(false);
    }
  };

  useEffect(() => {
    if (!plan) {
      setGitActivity([]);
      setGitActivityLoading(false);
      return;
    }
    void loadGitActivity(plan.workspaceId, activityPath);
  }, [plan?.workspaceId, activityPath]);

  const loadDiff = async () => {
    try {
      const payload = await api.diff(itemId);
      setDiff(payload.diff || '');
    } catch {
      setDiff('');
    }
  };

  const runGitOperation = async (operation: 'fetch' | 'pull' | 'push') => {
    if (!plan) return;
    setGitBusy(operation);
    setError('');
    try {
      const confirm = operation === 'pull' && Boolean(gitStatus?.dirty);
      const result = operation === 'fetch'
        ? await api.gitFetch(plan.workspaceId)
        : operation === 'pull'
          ? await api.gitPull(plan.workspaceId, { confirm })
          : await api.gitPush(plan.workspaceId);
      setGitStatus(result.status);
      await loadGitActivity(plan.workspaceId, activityPath);
      if (operation === 'pull') await onContentChanged?.();
      if (!result.ok) showGitResultError(result);
      else notifyReliabilityChanged();
    } catch (err) {
      showOperationError(err, `${operation} failed`);
    } finally {
      setGitBusy('');
    }
  };

  const refreshGitDetails = async () => {
    if (!plan) return;
    setError('');
    setRecoveryHint('');
    await Promise.all([
      loadGitStatus(plan.workspaceId),
      loadGitActivity(plan.workspaceId, activityPath),
      loadDiff()
    ]);
  };

  const commitSelectedPaths = async () => {
    if (!plan) return;
    setGitBusy('commit');
    setError('');
    try {
      const result = await api.gitCommit(plan.workspaceId, { message: gitMessage, paths: selectedGitPaths });
      setGitStatus(result.status);
      await loadGitActivity(plan.workspaceId, activityPath);
      setGitMessage('');
      setSelectedGitPaths([]);
      await onContentChanged?.();
      if (!result.ok) showGitResultError(result);
      else notifyReliabilityChanged();
    } catch (err) {
      showOperationError(err, 'Commit failed');
    } finally {
      setGitBusy('');
    }
  };

  const createAndSwitchBranch = async () => {
    if (!plan || !branchName.trim()) return;
    setGitBusy('branch');
    setError('');
    try {
      const result = await api.createBranch(plan.workspaceId, { name: branchName.trim(), checkout: true });
      setGitStatus(result.status);
      await loadGitActivity(plan.workspaceId, activityPath);
      setBranchName('');
      await onContentChanged?.();
      if (!result.ok) showGitResultError(result);
      else notifyReliabilityChanged();
    } catch (err) {
      showOperationError(err, 'Branch operation failed');
    } finally {
      setGitBusy('');
    }
  };

  const toggleGitPath = (path: string) => {
    setSelectedGitPaths((current) => current.includes(path) ? current.filter((item) => item !== path) : [...current, path]);
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
      if (plan) void loadGitStatus(plan.workspaceId);
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
      await loadGitStatus(plan.workspaceId);
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
      const materializeConfirmed = confirmSnapshotMaterialization(plan, 'metadata');
      if (materializeConfirmed === null) return;
      const result = await api.saveMetadata(itemId, { ...metadataDraft, materializeConfirmed });
      setPlan(result.item);
      if (plan) await loadGitStatus(plan.workspaceId);
      await onContentChanged?.();
      notifyReliabilityChanged();
    } catch (err) {
      showOperationError(err, 'Metadata save failed');
    } finally {
      setSavingMetadata(false);
    }
  };

  const runVerification = async (profile: VerifyProfile) => {
    if (!plan) return;
    setVerificationBusy(true);
    setVerificationError('');
    try {
      const job = await api.createVerificationJob(plan.workspaceId, { profile, trigger: 'manual_checkpoint', terminalMode: 'embedded' });
      setVerificationJob(job);
    } catch (err) {
      setVerificationError(err instanceof Error ? err.message : 'Failed to start verification');
    } finally {
      setVerificationBusy(false);
    }
  };

  const saveVerificationTests = async (selection: VerificationTestSelection) => {
    if (!plan) return null;
    setVerificationTestsBusy(true);
    setVerificationError('');
    try {
      const tests = await api.saveItemVerificationTests(plan.id, selection);
      setVerificationTests(tests);
      return tests;
    } catch (err) {
      setVerificationError(err instanceof Error ? err.message : 'Failed to save automation tests');
      return null;
    } finally {
      setVerificationTestsBusy(false);
    }
  };

  const selectedSpecs = verificationTests?.selection.selectedSpecs ?? [];
  const automationEnvironment = verificationTests?.selection.environment || workspaceConfig?.runtime?.automation?.defaultEnvironment || 'local';
  const automationDisplayMode = verificationTests?.selection.displayMode === 'visible' ? 'visible' : 'silent';
  const automationJobActive = verificationJob?.mode === 'automation' && (verificationJob.status === 'queued' || verificationJob.status === 'running');
  const automationRunBusy = automationLaunchBusy || automationJobActive;
  const automationRepoPath = workspaceConfig?.runtime?.automation?.repositoryPath ?? '';
  const automationWorkspace = useMemo(() => {
    const target = normalizeLocalPath(automationRepoPath);
    if (!target) return undefined;
    return workspaces.find((workspace) => normalizeLocalPath(workspace.path) === target);
  }, [automationRepoPath, workspaces]);
  const setSelectedSpecs = async (specs: string[]) => {
    await saveVerificationTests({ selectedSpecs: specs, environment: automationEnvironment, displayMode: automationDisplayMode });
  };
  const addSelectedSpec = async (spec: string) => {
    const clean = spec.trim();
    if (!clean || selectedSpecs.includes(clean)) return;
    await setSelectedSpecs([...selectedSpecs, clean]);
  };
  const removeSelectedSpec = async (spec: string) => {
    await setSelectedSpecs(selectedSpecs.filter((candidate) => candidate !== spec));
  };
  const saveAutomationEnvironment = async (environment: string) => {
    await saveVerificationTests({ selectedSpecs, environment, displayMode: automationDisplayMode });
  };
  const saveAutomationDisplayMode = async (displayMode: 'silent' | 'visible') => {
    await saveVerificationTests({ selectedSpecs, environment: automationEnvironment, displayMode });
  };
  const addManualSpec = async () => {
    const clean = manualSpec.trim();
    if (!clean) return;
    await addSelectedSpec(clean);
    setManualSpec('');
  };
  const openSpecPicker = () => {
    setSpecPickerPath('');
    setSpecPickerSelected([]);
    setSpecPickerError('');
    setSpecPickerOpen(true);
  };
  const loadSpecPickerDirectory = async (path: string) => {
    if (!automationWorkspace) return;
    setSpecPickerLoading(true);
    setSpecPickerError('');
    try {
      const listing = await api.workspaceTree(automationWorkspace.id, path);
      setSpecPickerPath(listing.path);
      setSpecPickerEntries(listing.entries.filter((entry) => entry.type === 'directory' || isAutomationSpecPath(entry.path)));
    } catch (err) {
      setSpecPickerError(err instanceof Error ? err.message : 'Failed to browse automation specs');
    } finally {
      setSpecPickerLoading(false);
    }
  };
  const togglePickerSpec = (path: string) => {
    setSpecPickerSelected((current) => current.includes(path) ? current.filter((spec) => spec !== path) : [...current, path]);
  };
  const addPickedSpecs = async () => {
    const nextSpecs = [...selectedSpecs];
    specPickerSelected.forEach((spec) => {
      if (!nextSpecs.includes(spec)) nextSpecs.push(spec);
    });
    await setSelectedSpecs(nextSpecs);
    setSpecPickerOpen(false);
    setSpecPickerSelected([]);
  };

  useEffect(() => {
    if (!specPickerOpen || !automationWorkspace) return;
    void loadSpecPickerDirectory(specPickerPath);
  }, [specPickerOpen, automationWorkspace?.id, specPickerPath]);

  const runAutomationVerification = async () => {
    if (!plan) return;
    setVerificationBusy(true);
    setAutomationLaunchBusy(true);
    setVerificationError('');
    try {
      const tests = await saveVerificationTests({ selectedSpecs, environment: automationEnvironment, displayMode: automationDisplayMode });
      const specs = tests?.selection.selectedSpecs ?? selectedSpecs;
      const environment = tests?.selection.environment || automationEnvironment;
      const displayMode = tests?.selection.displayMode === 'visible' ? 'visible' : automationDisplayMode;
      const job = await api.createVerificationJob(plan.workspaceId, { mode: 'automation', environment, displayMode, selectedSpecs: specs, trigger: 'manual_checkpoint', terminalMode: 'embedded' });
      setVerificationJob(job);
    } catch (err) {
      setVerificationError(err instanceof Error ? err.message : 'Failed to start automation verification');
    } finally {
      setVerificationBusy(false);
      setAutomationLaunchBusy(false);
    }
  };

  const rerunVerification = async (profile?: VerifyProfile) => {
    if (!plan || !verificationJob) return;
    setVerificationBusy(true);
    setVerificationError('');
    try {
      const job = await api.rerunVerificationJob(plan.workspaceId, verificationJob.id, profile);
      setVerificationJob(job);
    } catch (err) {
      setVerificationError(err instanceof Error ? err.message : 'Failed to rerun verification');
    } finally {
      setVerificationBusy(false);
    }
  };

  const openArtifactPath = async (path: string) => {
    try {
      await api.openPath(path);
    } catch {
      setVerificationError(`Could not open artifact path: ${path}`);
    }
  };

  const previewArtifact = async (kind: string, absolutePath: string) => {
    if (!plan) return;
    const relativePath = toWorkspaceRelativePath(workspaceConfig?.path, absolutePath);
    setArtifactPreview({ title: kind, path: absolutePath, content: '', loading: true, error: '' });
    if (!relativePath) {
      setArtifactPreview({ title: kind, path: absolutePath, content: '', loading: false, error: 'Preview unavailable for this artifact path. Use Open to view it externally.' });
      return;
    }
    try {
      const file = await api.workspaceFile(plan.workspaceId, relativePath);
      setArtifactPreview({ title: kind, path: absolutePath, content: file.content, loading: false, error: '' });
    } catch (caught) {
      setArtifactPreview({ title: kind, path: absolutePath, content: '', loading: false, error: caught instanceof Error ? caught.message : 'Could not load artifact preview' });
    }
  };

  const qualityPanel = (
    <section className="metadata-callout quality-panel" aria-label="Quality">
      <div className="verification-header">
        <strong>Quality</strong>
        {verificationJob && <span className={`verification-trigger-badge ${verificationTriggerTone(verificationJob)}`}>{verificationTriggerLabel(verificationJob)}</span>}
      </div>
      <div className="verification-actions">
        <button className="secondary" type="button" onClick={() => void runVerification('smoke')} disabled={verificationBusy || !plan?.workspaceId || !workspaceConfig?.runtime}>Run smoke verify</button>
        <button className="secondary" type="button" onClick={() => void runVerification('critical')} disabled={verificationBusy || !plan?.workspaceId || !workspaceConfig?.runtime}>Run critical verify</button>
        <button className="secondary" type="button" onClick={() => void rerunVerification()} disabled={verificationBusy || !verificationJob}>Re-run latest</button>
      </div>
      {!plan?.workspaceId && <span className="verification-note">No workspace selected.</span>}
      {plan?.workspaceId && !workspaceConfig?.runtime && <span className="verification-note">Runtime not configured for this workspace.</span>}
      {workspaceConfig?.runtime && !workspaceConfig.runtime.automation?.enabled && <span className="verification-note">Automation tests are not configured for this workspace.</span>}
      {workspaceConfig?.runtime?.automation?.enabled && (
        <div className="automation-test-panel">
          <div className="quality-section-heading">
            <strong>Automation</strong>
            <span>Environment and card-linked specs</span>
          </div>
          <label className="repo-field">Automation environment<input value={automationEnvironment} onChange={(event) => void saveAutomationEnvironment(event.target.value)} placeholder="local" /></label>
          <div className="automation-mode-field">
            <span>Run mode</span>
            <div className="segmented-control automation-mode-toggle" role="tablist" aria-label="Automation run mode">
              <button type="button" className={automationDisplayMode === 'silent' ? 'active' : ''} aria-selected={automationDisplayMode === 'silent'} onClick={() => void saveAutomationDisplayMode('silent')}>Silent</button>
              <button type="button" className={automationDisplayMode === 'visible' ? 'active' : ''} aria-selected={automationDisplayMode === 'visible'} onClick={() => void saveAutomationDisplayMode('visible')}>Visible browser</button>
            </div>
          </div>
          <div className="quality-section-heading compact">
            <strong>Selected specs</strong>
            {!automationWorkspace && <span>Register the automation repository as a workspace to browse specs.</span>}
          </div>
          <div className="automation-spec-list" aria-label="Selected automation specs">
            {selectedSpecs.length ? selectedSpecs.map((spec) => <span className="automation-spec-chip" key={spec}>{spec}<button type="button" aria-label={`Remove ${spec}`} onClick={() => void removeSelectedSpec(spec)}>×</button></span>) : <span className="verification-note">No selected specs.</span>}
          </div>
          <div className="path-input-row manual-spec-row">
            <input aria-label="Manual automation spec" value={manualSpec} onChange={(event) => setManualSpec(event.target.value)} placeholder="Repo-relative spec path" />
            <button className="secondary" type="button" onClick={() => void addManualSpec()} disabled={!manualSpec.trim()}>Add spec</button>
            <button className="secondary browse-specs-action" type="button" onClick={openSpecPicker} disabled={!automationWorkspace}>Browse</button>
          </div>
          <div className="automation-action-row">
            <button className={automationRunBusy ? 'secondary automation-run-action progress-button active' : 'secondary automation-run-action'} type="button" onClick={() => void runAutomationVerification()} disabled={automationRunBusy || verificationTestsBusy || !plan?.workspaceId || !workspaceConfig?.runtime?.automation?.enabled || selectedSpecs.length === 0}>
              <span>{automationRunBusy ? (automationDisplayMode === 'visible' ? 'Starting browser...' : 'Running automation...') : 'Run automation tests'}</span>
              {automationRunBusy && <span className="button-progress-bar" aria-hidden="true" />}
            </button>
          </div>
          <div className="automation-suggestions">
            <div className="quality-subsection-heading">
              <strong>Suggested specs</strong>
              <span>From automation-test in plan.yaml</span>
            </div>
            {verificationTests?.discoveredSpecs?.length ? (
              <div className="automation-discovered-specs" aria-label="Suggested automation specs">
                {verificationTests.discoveredSpecs.map((spec) => {
                  const selected = selectedSpecs.includes(spec.path);
                  return (
                    <article className={selected ? 'automation-suggestion-card selected' : 'automation-suggestion-card'} key={`${spec.path}-${spec.sourcePath ?? ''}`}>
                      <div>
                        <strong>{spec.path}</strong>
                        <span>{spec.runner}{spec.sourcePath ? ` · ${spec.sourcePath}` : ''}</span>
                      </div>
                      <button className="secondary" type="button" disabled={selected} onClick={() => void addSelectedSpec(spec.path)}>
                        {selected ? 'Selected' : 'Select'}
                      </button>
                    </article>
                  );
                })}
              </div>
            ) : <span className="verification-note">No suggested specs from plan.yaml.</span>}
          </div>
        </div>
      )}
      {verificationBusy && <span className="verification-note">Starting verification...</span>}
      {verificationError && <span className="error" role="alert">{verificationError}</span>}
      {verificationJob && <span className="verification-status">{verificationJob.mode === 'automation' ? 'automation' : verificationJob.profile} · {verificationJob.status}{verificationJob.failureType ? ` (${verificationJob.failureType})` : ''}</span>}
      {verificationJob && <span className="verification-note">{verificationTriggerDescription(verificationJob)}</span>}
      {verificationJob?.steps?.length ? (
        <div className="verification-steps">
          {verificationJob.steps.map((step) => (
            <span className={`verification-step ${step.status === 'ok' ? 'ok' : 'failed'}`} key={`${step.step}-${step.at}`}>{step.step}: {step.status}</span>
          ))}
        </div>
      ) : null}
      {verificationJob?.artifacts?.length ? (
        <div className="verification-artifacts">
          {verificationJob.artifacts.map((artifact) => (
            <article className="verification-artifact-card" key={`${artifact.kind}-${artifact.path}`}>
              <div className="verification-artifact-meta">
                <strong>{formatArtifactKind(artifact.kind, artifact.path)}</strong>
                <span>{artifact.path}</span>
                <small>{formatBytes(artifact.sizeBytes)} · {formatAt(artifact.createdAt)}</small>
              </div>
              <div className="verification-artifact-actions">
                <button className="secondary" type="button" onClick={() => void previewArtifact(formatArtifactKind(artifact.kind, artifact.path), artifact.path)}>Preview</button>
                <button className="secondary" type="button" onClick={() => void openArtifactPath(artifact.path)}>Open</button>
              </div>
            </article>
          ))}
        </div>
      ) : null}
    </section>
  );

  const gitPanel = gitStatus ? (
    <section className="git-panel git-side-panel">
      <div className="git-summary">
        <span>{gitStatus.branch}</span>
        <span>{gitStatus.ahead} ahead</span>
        <span>{gitStatus.behind} behind</span>
      </div>
      <div className="workspace-actions">
        <button className="secondary" type="button" disabled={Boolean(gitBusy)} onClick={() => runGitOperation('fetch')}>{gitBusy === 'fetch' ? 'Fetching...' : 'Fetch'}</button>
        <button className="secondary" type="button" disabled={Boolean(gitBusy)} onClick={() => runGitOperation('pull')}>{gitBusy === 'pull' ? 'Pulling...' : 'Pull'}</button>
        <button className="secondary" type="button" disabled={Boolean(gitBusy)} onClick={() => runGitOperation('push')}>{gitBusy === 'push' ? 'Pushing...' : 'Push'}</button>
      </div>
      <div className="git-changes">
        {gitStatus.changes.length === 0 && <span>No local changes</span>}
        {gitStatus.changes.map((change) => (
          <div className="git-change-row" key={`${change.status}-${change.path}`}>
            <input type="checkbox" aria-label={`Select ${change.path} for commit`} checked={selectedGitPaths.includes(change.path)} onChange={() => toggleGitPath(change.path)} />
            <span>{change.status}</span>
            <button className="git-change-path" type="button" title={change.path} onClick={() => openGitChangePath(change)}>{change.path}</button>
          </div>
        ))}
      </div>
      <textarea className="commit-message" value={gitMessage} onChange={(event) => setGitMessage(event.target.value)} placeholder="Commit message" />
      <button className="primary" type="button" disabled={Boolean(gitBusy) || selectedGitPaths.length === 0 || !gitMessage.trim()} onClick={commitSelectedPaths}>
        {gitBusy === 'commit' ? 'Committing...' : 'Commit Selected'}
      </button>
      <div className="branch-create-row">
        <input value={branchName} onChange={(event) => setBranchName(event.target.value)} placeholder="New Branch" />
        <button className="secondary" type="button" disabled={Boolean(gitBusy) || !branchName.trim()} onClick={createAndSwitchBranch}>
          {gitBusy === 'branch' ? 'Creating...' : 'Create Branch'}
        </button>
      </div>
      <details className="recent-activity-panel" open={gitActivityOpen} onToggle={(event) => {
        const open = event.currentTarget.open;
        setGitActivityOpen(open);
        localStorage.setItem('item.details.gitActivityOpen', open ? '1' : '0');
      }}>
        <summary>
          <span>Recent Activity</span>
          <small>{gitActivity.length} events</small>
        </summary>
        <RecentGitActivity entries={gitActivity} loading={gitActivityLoading} emptyLabel="No activity found for this item." pathLabel={activityPath || 'workspace'} />
      </details>
    </section>
  ) : (
    <div className="metadata-callout git-side-panel">
      <strong>Git status unavailable</strong>
      <span>Refresh the workspace or scan the source to load Git information.</span>
    </div>
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
      </div>
      {rightPanelTab === 'info' && (
        <>
          {plan?.metadataSource === 'docs' && (
            <div className="metadata-callout">
              <strong>Docs</strong>
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
          {plan?.metadataSource !== 'docs' && (
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
            <button className="save-action save-metadata-action" type="button" disabled={!dirtyMetadata || savingMetadata || plan?.metadataSource === 'docs'} onClick={saveMetadata}>{savingMetadata ? 'Saving...' : 'Save Metadata'}</button>
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
      {rightPanelTab === 'quality' && qualityPanel}
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

  if (error && !plan) {
    return <section className="empty-state"><button className="ghost" onClick={goBack}><ArrowLeft size={16} /> Back</button><p className="error">{error}</p></section>;
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
            {workspaceConfig ? (
              <BranchSnapshotPicker
                selectedBranch={selectedDetailBranch}
                currentCheckoutBranch={currentCheckoutBranch}
                sourceMode={detailSourceMode}
                branches={branchOptions}
                disabled={branchLoading || itemWorkspaceBranches.states[workspaceConfig.id]?.switching}
                ariaLabel="Select item branch"
                listboxLabel="Item branches"
                onSelect={(branch) => void switchItemBranch(branch)}
              />
            ) : (
              <span className="workspace-item-path-segment">{plan?.branch ?? '...'}</span>
            )}
            <span className="workspace-item-path-separator">/</span>
            <span className="workspace-item-path-segment">{plan?.identifier ?? '...'}</span>
          </div>
        </div>
        <div className="workspace-header-actions">
          <AISessionLaunchControl itemId={itemId} buttonLabel="AI session" disabled={!plan} onLaunched={setAILaunchMessage} onError={(caught) => showOperationError(caught, 'AI session launch failed')} />
          <button
            className={`icon-button workspace-git-status-button${gitStatus?.dirty ? ' is-dirty' : ''}`}
            type="button"
            aria-label={gitStatus?.dirty ? 'Refresh Git status, activity, and diff; local changes present' : 'Refresh Git status, activity, and diff'}
            title={gitStatus?.dirty ? 'Local changes present. Refresh Git status, activity, and diff.' : 'Refresh Git status, activity, and diff'}
            disabled={gitLoading || !plan}
            onClick={() => void refreshGitDetails()}
          >
            <RefreshCw size={18} />
          </button>
        </div>
      </header>
      {aiLaunchMessage && <div className="operation-notice" role="status">{aiLaunchMessage}</div>}
      {branchView?.missing ? (
        <section className="workspace-branch-empty" role="status">
          <FileText size={28} />
          <strong>{plan?.identifier ?? 'This item'} is not on {branchView.branch}</strong>
          <span>The current checkout branch is {branchView.currentCheckoutBranch}. The selected branch was loaded as a snapshot, but no matching item exists there.</span>
        </section>
      ) : plan && workspaceConfig && detailSourceMode !== 'snapshot' ? (
        <WorkstreamExplorer
          embedded
          showModeSelector={false}
          treeRootPath={detailViewMode === 'plan' ? itemRootPath : undefined}
          leftPanelContent={detailViewMode === 'git' ? gitPanel : undefined}
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
              {!leftCollapsed && <button className="icon-button" type="button" aria-label="Rename selected path" title="Rename" disabled={!selectedTreeNode} onClick={() => { setRenameName(selectedTreeNode?.path.split('/').at(-1) ?? ''); setRenameOpen(true); }}><Pencil size={16} /></button>}
              <button className="icon-button" type="button" title={leftCollapsed ? 'Expand files' : 'Collapse files'} onClick={() => setLeftCollapsed((value) => !value)}>
                {leftCollapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
              </button>
            </div>
          </div>
          {!leftCollapsed && (
			<>
				<ContentSearchInput label="Search inside this item" query={contentSearch.query} onQueryChange={contentSearch.setQuery} />
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
                <button className={tab === 'preview' ? 'active' : ''} onClick={() => setTab('preview')}><FileText size={15} /> Rendered</button>
                <button className={tab === 'raw' ? 'active' : ''} onClick={() => setTab('raw')}><Code2 size={15} /> Source</button>
                <button className={tab === 'diff' ? 'active' : ''} onClick={() => setTab('diff')}><GitCompare size={15} /> Diff</button>
              </div>
              <span className={`autosave-state ${autoSaveState}`}>{autoSaveLabel(autoSaveState)}</span>
            </div>
          </div>
		  {matchContext && <div className="content-match-context">Line {matchContext.lineNumber}, columns {matchContext.columnStart}–{matchContext.columnEnd}</div>}
          {(dirtyMetadata || dirtyFile || autoSaveState !== 'idle') && <div className="edit-state-banner">{dirtyMetadata ? 'Unsaved metadata changes' : autoSaveLabel(autoSaveState)}</div>}
          {tab === 'preview' && (file ? <ContentViewer file={file} content={editorContent} /> : <EmptyDocumentState hasFiles={hasFiles} />)}
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
      {artifactPreview && (
        <section className="modal-backdrop" role="presentation" onClick={() => setArtifactPreview(null)}>
          <div className="modal-panel artifact-preview-modal" role="dialog" aria-modal="true" aria-label="Artifact preview" onClick={(event) => event.stopPropagation()}>
            <header>
              <div>
                <h2>{artifactPreview.title}</h2>
                <span>{artifactPreview.path}</span>
              </div>
            </header>
            {artifactPreview.loading && <p>Loading preview...</p>}
            {!artifactPreview.loading && artifactPreview.error && <p className="error">{artifactPreview.error}</p>}
            {!artifactPreview.loading && !artifactPreview.error && <pre className="artifact-preview-content">{artifactPreview.content || 'No text content available.'}</pre>}
            <div className="modal-actions">
              <button className="secondary" type="button" onClick={() => void openArtifactPath(artifactPreview.path)}>Open externally</button>
              <button className="primary" type="button" onClick={() => setArtifactPreview(null)}>Close</button>
            </div>
          </div>
        </section>
      )}
      {specPickerOpen && (
        <section className="modal-backdrop" role="presentation" onClick={() => setSpecPickerOpen(false)}>
          <div className="modal-panel spec-picker-modal" role="dialog" aria-modal="true" aria-label="Browse automation specs" onClick={(event) => event.stopPropagation()}>
            <header>
              <div>
                <h2>Browse automation specs</h2>
                <span>{automationWorkspace?.name ?? 'Automation repository'} / {specPickerPath || 'root'}</span>
              </div>
              <button className="icon-button" type="button" aria-label="Close spec browser" onClick={() => setSpecPickerOpen(false)}><X size={16} /></button>
            </header>
            {specPickerPath && (
              <button className="secondary spec-picker-up" type="button" onClick={() => setSpecPickerPath(parentPath(specPickerPath))}>Up one folder</button>
            )}
            {specPickerError && <p className="error" role="alert">{specPickerError}</p>}
            <div className="spec-picker-list" aria-label="Automation spec files">
              {specPickerLoading && <span className="verification-note">Loading specs...</span>}
              {!specPickerLoading && specPickerEntries.length === 0 && <span className="verification-note">No spec files in this folder.</span>}
              {!specPickerLoading && specPickerEntries.map((entry) => entry.type === 'directory' ? (
                <button className="spec-picker-row directory" type="button" key={entry.path} onClick={() => setSpecPickerPath(entry.path)}>
                  <FolderOpen size={15} /> <span>{entry.name}</span>
                </button>
              ) : (
                <label className="spec-picker-row" key={entry.path}>
                  <input type="checkbox" checked={specPickerSelected.includes(entry.path)} disabled={selectedSpecs.includes(entry.path)} onChange={() => togglePickerSpec(entry.path)} />
                  <span>{entry.path}</span>
                  {selectedSpecs.includes(entry.path) && <small>Selected</small>}
                </label>
              ))}
            </div>
            <footer className="modal-actions">
              <button className="secondary" type="button" onClick={() => setSpecPickerOpen(false)}>Cancel</button>
              <button className="primary" type="button" disabled={specPickerSelected.length === 0 || verificationTestsBusy} onClick={() => void addPickedSpecs()}>Add {specPickerSelected.length || ''} spec{specPickerSelected.length === 1 ? '' : 's'}</button>
            </footer>
          </div>
        </section>
      )}
      {createPathKind && (
        <div className="modal-backdrop" role="presentation">
          <section className="modal-panel" role="dialog" aria-modal="true" aria-label={`Create new ${createPathKind}`}>
            <header><h2>New {createPathKind === 'file' ? 'file' : 'folder'}</h2></header>
            <div className="metadata-form">
              <p>Parent: {selectedDirectoryPath || 'item root'}</p>
              <label>Relative path<input autoFocus value={createPathName} onChange={(event) => setCreatePathName(event.target.value)} placeholder={createPathKind === 'file' ? 'schema.json' : 'api'} /></label>
            </div>
            <footer className="modal-actions">
              <button className="ghost" type="button" disabled={creatingPath} onClick={() => { setCreatePathKind(null); setCreatePathName(''); }}>Cancel</button>
              <button className="primary" type="button" disabled={creatingPath || !createPathName.trim()} onClick={() => void createItemPath()}>{creatingPath ? 'Creating...' : 'Create'}</button>
            </footer>
          </section>
        </div>
      )}
      {renameOpen && selectedTreeNode && (
        <div className="modal-backdrop" role="presentation">
          <section className="modal-panel" role="dialog" aria-modal="true" aria-label="Rename path">
            <header><h2>Rename {selectedTreeNode.type}</h2></header>
            <div className="metadata-form">
              <p>Current: {selectedTreeNode.path}</p>
              <label>Name<input autoFocus value={renameName} onChange={(event) => setRenameName(event.target.value)} /></label>
            </div>
            <footer className="modal-actions">
              <button className="ghost" type="button" disabled={renamingPath} onClick={() => { setRenameOpen(false); setRenameName(''); }}>Cancel</button>
              <button className="primary" type="button" disabled={renamingPath || !renameName.trim()} onClick={() => void renameItemPath()}>{renamingPath ? 'Renaming...' : 'Rename'}</button>
            </footer>
          </section>
        </div>
      )}
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

function EmptyDocumentState({ hasFiles }: { hasFiles: boolean }) {
  return (
    <div className="document-empty">
      <FileText size={22} />
      <strong>{hasFiles ? 'Select a file' : 'No files found'}</strong>
      <span>{hasFiles ? 'Choose a file from the explorer to preview its content.' : 'This item folder does not contain any readable files yet.'}</span>
    </div>
  );
}

function StatusBadge({ status }: { status: ItemDetail['status'] }) {
  return <span className={`status-badge ${status}`}>{statusLabel(status)}</span>;
}

function statusLabel(status: ItemDetail['status']): string {
  return statusLabels[status] ?? status;
}

function clearTimer(ref: MutableRefObject<number | null>) {
  if (ref.current === null) return;
  window.clearTimeout(ref.current);
  ref.current = null;
}

function DiffPanel({ diff, files, mode, selectedPath, selectedFileHasDiff, reverting, onModeChange, onRevertFile }: {
  diff: string;
  files: DiffFile[];
  mode: DiffMode;
  selectedPath: string;
  selectedFileHasDiff: boolean;
  reverting: boolean;
  onModeChange: (mode: DiffMode) => void;
  onRevertFile: () => void;
}) {
  const shownFiles = selectedPath ? files.filter((item) => item.path === selectedPath || item.oldPath === selectedPath) : files;
  const reviewFiles = shownFiles.length > 0 ? shownFiles : files;
  return (
    <section className="diff-panel">
      <header className="diff-toolbar">
        <div className="diff-mode-switch" role="tablist" aria-label="Diff view mode">
          <button type="button" className={mode === 'review' ? 'active' : ''} onClick={() => onModeChange('review')}>Review</button>
          <button type="button" className={mode === 'raw' ? 'active' : ''} onClick={() => onModeChange('raw')}>Git</button>
        </div>
        <div className="diff-actions">
          <span>{files.length} changed file{files.length === 1 ? '' : 's'}</span>
          <button className="danger-action" type="button" disabled={!selectedFileHasDiff || reverting} onClick={onRevertFile}>
            <RotateCcw size={15} /> {reverting ? 'Reverting...' : 'Revert File'}
          </button>
        </div>
      </header>
      {mode === 'raw' && <pre className="diff-view">{diff || 'No local changes.'}</pre>}
      {mode === 'review' && (
        <div className="diff-review">
          {reviewFiles.length === 0 && <div className="document-empty"><GitCompare size={22} /><strong>No local changes</strong><span>The selected plan has no Git diff.</span></div>}
          {reviewFiles.map((item) => (
            <article className={item.path === selectedPath ? 'diff-file active' : 'diff-file'} key={`${item.oldPath ?? item.path}-${item.path}`}>
              <header>
                <strong>{item.path}</strong>
                {item.oldPath && item.oldPath !== item.path && <span>renamed from {item.oldPath}</span>}
                <div>
                  <span className="diff-add">+{item.additions}</span>
                  <span className="diff-delete">-{item.deletions}</span>
                </div>
              </header>
              <div className="diff-lines">
                {item.lines.map((line, index) => (
                  <div className={`diff-line ${line.type}`} key={`${item.path}-${index}`}>
                    <span className="line-number">{line.oldLine ?? ''}</span>
                    <span className="line-number">{line.newLine ?? ''}</span>
                    <code>{line.text || ' '}</code>
                  </div>
                ))}
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}

const TreeNode = memo(function TreeNode({ node, onOpen, activePath, depth, fileStateByPath, selectedDirectoryPath, onSelectDirectory, onSelectFile }: { node: FileNode; onOpen: (id: string) => void; activePath?: string; depth: number; fileStateByPath: Map<string, TreeFileState>; selectedDirectoryPath: string; onSelectDirectory: (path: string) => void; onSelectFile: (path: string) => void }) {
  const indent = { '--tree-indent': `${depth * 14}px` } as CSSProperties & Record<'--tree-indent', string>;
  const [expanded, setExpanded] = useState(true);

  if (node.type === 'directory') {
    return (
      <details open={expanded} className="tree-dir">
        <summary className={selectedDirectoryPath === node.path ? 'tree-row tree-row-dir active' : 'tree-row tree-row-dir'} style={indent} title={node.path} onClick={(event) => { event.preventDefault(); onSelectDirectory(node.path); }} onDoubleClick={(event) => { event.preventDefault(); setExpanded((value) => !value); }}>
          <ChevronDown className="tree-chevron" size={14} />
          <FolderOpen className="tree-icon" size={16} />
          <span className="tree-label">{node.name}</span>
        </summary>
        <div className="tree-children">
          {node.children?.map((child) => <TreeNode node={child} key={child.id} onOpen={onOpen} activePath={activePath} depth={depth + 1} fileStateByPath={fileStateByPath} selectedDirectoryPath={selectedDirectoryPath} onSelectDirectory={onSelectDirectory} onSelectFile={onSelectFile} />)}
        </div>
      </details>
    );
  }
  const state = fileStateByPath.get(normalizePath(node.path));
  return (
    <button className={activePath === node.path ? 'tree-row tree-file active' : 'tree-row tree-file'} style={indent} title={node.path} onClick={() => { onSelectFile(node.path); onOpen(node.id); }}>
      <span className="tree-spacer" />
      <FileIcon className="tree-icon" size={16} />
      <span className="tree-label">{node.name}</span>
      {state && <FileStateIcon state={state} />}
    </button>
  );
});

function firstFile(nodes: FileNode[]): FileNode | null {
  for (const node of nodes) {
    if (node.type === 'file') return node;
    const child = firstFile(node.children ?? []);
    if (child) return child;
  }
  return null;
}

function preferredFile(nodes: FileNode[]): FileNode | null {
	return findReadme(nodes, true) ?? findReadme(nodes, false) ?? firstFile(nodes);
}

function fileDirectoryPaths(nodes: FileNode[]): Set<string> {
  const paths = new Set<string>();
  const visit = (entries: FileNode[]) => entries.forEach((node) => {
    if (node.type !== 'directory') return;
    paths.add(node.path);
    visit(node.children ?? []);
  });
  visit(nodes);
  return paths;
}

function findReadme(nodes: FileNode[], rootOnly: boolean): FileNode | null {
	for (const node of nodes) {
		if (node.type === 'file' && node.name.toLowerCase() === 'readme.md') return node;
		if (!rootOnly && node.type === 'directory') {
			const child = findReadme(node.children ?? [], false);
			if (child) return child;
		}
	}
	return null;
}

function hasFile(nodes: FileNode[]): boolean {
  return firstFile(nodes) !== null;
}

function currentGitPath(plan: ItemDetail | null, file: FileContent | null): string {
  if (!plan?.itemPath || !file?.path) return '';
  return `${plan.itemPath.replace(/\/$/, '')}/${file.path.replace(/^\//, '')}`;
}

function workspacePathForSelection(plan: ItemDetail | null, file: FileContent | null): string | undefined {
  const itemPath = normalizePath(plan?.itemPath ?? '');
  const filePath = normalizePath(file?.path ?? '');
  if (itemPath && filePath) return `${itemPath}/${filePath}`;
  return itemPath || undefined;
}

function buildFileStateMap(plan: ItemDetail | null, gitStatus: GitStatus | null, file: FileContent | null, dirtyFile: boolean): Map<string, TreeFileState> {
  const stateByPath = new Map<string, TreeFileState>();
  const itemPath = normalizePath(plan?.itemPath ?? '');
  for (const change of gitStatus?.changes ?? []) {
    const localPath = localItemPath(itemPath, change);
    if (localPath) stateByPath.set(localPath, change.status);
  }
  if (dirtyFile && file?.path) {
    stateByPath.set(normalizePath(file.path), 'unsaved');
  }
  return stateByPath;
}

function localItemPath(itemPath: string, change: GitChange): string {
  const path = normalizePath(change.path);
  const oldPath = normalizePath(change.oldPath ?? '');
  return stripItemPath(path, itemPath) || stripItemPath(oldPath, itemPath);
}

function stripItemPath(path: string, itemPath: string): string {
  if (!path) return '';
  if (!itemPath) return path;
  if (path === itemPath) return '';
  return path.startsWith(`${itemPath}/`) ? path.slice(itemPath.length + 1) : '';
}

function normalizePath(path: string): string {
  return path.replace(/^\/+/, '').replace(/\/+$/, '');
}

function normalizeLocalPath(path: string): string {
  return path.trim().replace(/\/+$/, '');
}

function parentPath(path: string): string {
  const parts = normalizePath(path).split('/').filter(Boolean);
  parts.pop();
  return parts.join('/');
}

function isAutomationSpecPath(path: string): boolean {
  return /\.(?:cy|spec|test)\.(?:ts|tsx|js|jsx)$/i.test(normalizePath(path));
}

function lastPathSegment(path: string): string {
  const normalized = path.replace(/\/+$/, '');
  const separator = normalized.lastIndexOf('/');
  return separator >= 0 ? normalized.slice(separator + 1) : normalized;
}

function parentDirectoryPath(path: string): string {
  const normalized = path.replace(/\/+$/, '');
  const separator = normalized.lastIndexOf('/');
  return separator >= 0 ? normalized.slice(0, separator) : '';
}

function matchingBranchItem(items: { id: string; itemPath?: string; scope?: string; identifier?: string }[], current: ItemDetail) {
  const currentPath = normalizePath(current.itemPath ?? '');
  return items.find((item) => currentPath && normalizePath(item.itemPath ?? '') === currentPath)
    ?? items.find((item) => item.scope === current.scope && item.identifier === current.identifier);
}

function confirmSnapshotMaterialization(item: ItemDetail | null, operation: 'file' | 'metadata'): boolean | null {
  if (!item || item.sourceMode !== 'snapshot') return false;
  const copyTarget = item.metadataSource === 'docs'
    ? 'only this docs file'
    : `the whole plan at ${item.itemPath || item.identifier}`;
  const action = operation === 'metadata' ? 'edit its metadata' : 'edit it';
  const message = `This item is loaded from branch ${item.branch}. To ${action}, Kode Stream will copy ${copyTarget} into the current checkout branch, then apply your change there.`;
  return window.confirm(message) ? true : null;
}

function unique(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean))).sort((a, b) => a.localeCompare(b));
}

function toWorkspaceRelativePath(workspacePath: string | undefined, absolutePath: string): string {
  if (!workspacePath) return '';
  const normalizedWorkspace = workspacePath.replace(/\\/g, '/').replace(/\/+$/, '');
  const normalizedTarget = absolutePath.replace(/\\/g, '/');
  if (normalizedTarget === normalizedWorkspace) return '';
  if (!normalizedTarget.startsWith(`${normalizedWorkspace}/`)) return '';
  return normalizedTarget.slice(normalizedWorkspace.length + 1);
}

function readStoredToggle(key: string): boolean {
  return localStorage.getItem(key) === '1';
}

function visibleItemWarnings(plan: ItemDetail | null): { itemPath?: string; message: string }[] {
  if (!plan?.warnings?.length) return [];
  return plan.warnings.filter((warning) => !isIgnorableWarning(warning.message));
}

function isIgnorableWarning(message: string): boolean {
  const normalized = message.toLowerCase();
  return normalized.includes("plan.yaml") && normalized.includes("does not exist in");
}

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let size = value;
  let index = 0;
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024;
    index += 1;
  }
  return `${size.toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}

function formatAt(value?: string): string {
  if (!value) return 'Unknown time';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function formatArtifactKind(kind: string, path: string): string {
  const fileName = path.split('/').at(-1)?.toLowerCase() ?? '';
  if (kind === 'automation_log' || fileName === 'automation.log') return 'Automation log';
  if (kind === 'verify_log' || fileName === 'verify.log') return 'Verification log';
  if (kind === 'runtime_log' || fileName === 'runtime.log') return 'Runtime setup log';
  if (kind === 'playwright_trace') return 'Playwright trace';
  if (kind === 'playwright_video') return 'Playwright video';
  if (kind === 'playwright_screenshot') return 'Screenshot';
  if (kind === 'playwright_report') return 'Test report';
  return kind.replaceAll('_', ' ');
}

function verificationTriggerLabel(job: VerificationJob): string {
  const mode = job.terminalMode ? ` (${job.terminalMode})` : '';
  if (job.trigger?.startsWith('checkpoint:')) {
    const eventType = job.trigger.slice('checkpoint:'.length);
    return `${eventType}${mode}`;
  }
  if (job.trigger === 'rerun') {
    return `rerun${mode}`;
  }
  if (job.trigger === 'manual_checkpoint') {
    return `manual${mode}`;
  }
  return `${job.trigger || 'manual'}${mode}`;
}

function verificationTriggerDescription(job: VerificationJob): string {
  const provider = job.provider ? ` via ${providerLabel(job.provider)}` : '';
  if (job.trigger?.startsWith('checkpoint:')) {
    const eventType = job.trigger.slice('checkpoint:'.length).replaceAll('_', ' ');
    return `Auto verification from ${eventType}${provider}${job.sessionId ? ` (session ${job.sessionId})` : ''}.`;
  }
  if (job.trigger === 'rerun') {
    return `Re-run requested${provider}${job.sessionId ? ` (session ${job.sessionId})` : ''}.`;
  }
  return `Manual verification request${provider}.`;
}

function verificationTriggerTone(job: VerificationJob): 'manual' | 'rerun' | 'auto-embedded' | 'auto-external' | 'auto' {
  if (job.trigger === 'rerun') {
    return 'rerun';
  }
  if (job.trigger?.startsWith('checkpoint:')) {
    if (job.terminalMode === 'embedded') {
      return 'auto-embedded';
    }
    if (job.terminalMode === 'external') {
      return 'auto-external';
    }
    return 'auto';
  }
  return 'manual';
}

function providerLabel(id: string): string {
  return ({ claude: 'Claude', codex: 'Codex', copilot: 'Copilot', opencode: 'OpenCode' } as Record<string, string>)[id] ?? id;
}

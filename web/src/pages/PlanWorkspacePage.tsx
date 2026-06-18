import { useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties } from 'react';
import {
  ArrowLeft,
  ChevronDown,
  Code2,
  File as FileIcon,
  FileText,
  FilePlus2,
  FolderOpen,
  GitBranch,
  GitCompare,
  GripVertical,
  Info,
  PencilLine,
  TriangleAlert,
  RotateCcw,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightClose,
  PanelRightOpen,
  RefreshCw,
} from 'lucide-react';
import { marked } from 'marked';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { api } from '../lib/api';
import type { FileContent, FileNode, GitChange, GitChangeStatus, GitStatus, PlanDetail, PlanMetadataUpdateInput } from '../lib/types';

type Tab = 'preview' | 'raw' | 'diff';
type RightPanelTab = 'info' | 'git';
type DiffMode = 'review' | 'raw';
type DiffLine = { type: 'context' | 'add' | 'delete' | 'meta'; text: string; oldLine?: number; newLine?: number };
type DiffFile = { path: string; oldPath?: string; lines: DiffLine[]; additions: number; deletions: number };
type PendingConfirm = { title: string; message: string; confirmLabel: string; danger?: boolean; onConfirm: () => void };
type TreeFileState = GitChangeStatus | 'unsaved';

export function PlanWorkspacePage({ planId, refreshKey, onBack, onContentChanged }: { planId: string; refreshKey: number; onBack: () => void; onContentChanged?: () => void | Promise<void> }) {
  const [plan, setPlan] = useState<PlanDetail | null>(null);
  const [files, setFiles] = useState<FileNode[]>([]);
  const [file, setFile] = useState<FileContent | null>(null);
  const [editorContent, setEditorContent] = useState('');
  const [savedContent, setSavedContent] = useState('');
  const [savingFile, setSavingFile] = useState(false);
  const [metadataDraft, setMetadataDraft] = useState<PlanMetadataUpdateInput>({});
  const [savingMetadata, setSavingMetadata] = useState(false);
  const [gitStatus, setGitStatus] = useState<GitStatus | null>(null);
  const [gitLoading, setGitLoading] = useState(false);
  const [gitMessage, setGitMessage] = useState('');
  const [selectedGitPaths, setSelectedGitPaths] = useState<string[]>([]);
  const [branchName, setBranchName] = useState('');
  const [gitBusy, setGitBusy] = useState('');
  const [diff, setDiff] = useState('');
  const [diffMode, setDiffMode] = useState<DiffMode>('review');
  const [revertingFile, setRevertingFile] = useState(false);
  const [revertDialogOpen, setRevertDialogOpen] = useState(false);
  const [pendingConfirm, setPendingConfirm] = useState<PendingConfirm | null>(null);
  const [tab, setTab] = useState<Tab>('preview');
  const [rightPanelTab, setRightPanelTab] = useState<RightPanelTab>('info');
  const [error, setError] = useState('');
  const [leftCollapsed, setLeftCollapsed] = useState(false);
  const [rightCollapsed, setRightCollapsed] = useState(false);
  const [leftWidth, setLeftWidth] = useState(300);
  const [rightWidth, setRightWidth] = useState(300);
  const workspaceGridRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    setError('');
    setFile(null);
    api.plan(planId).then(setPlan).catch((err: Error) => setError(err.message));
    api.files(planId).then((tree) => {
      setFiles(tree);
      const first = firstFile(tree);
      if (first) void openFile(first.id);
    }).catch((err: Error) => setError(err.message));
    void loadDiff();
  }, [planId, refreshKey]);

  useEffect(() => {
    if (!plan) return;
    setMetadataDraft({
      title: plan.title,
      service: plan.service,
      ticket: plan.ticket,
      status: plan.status,
      owner: plan.owner ?? '',
      tags: plan.tags
    });
    void loadGitStatus(plan.repositoryId);
  }, [plan]);

  useEffect(() => {
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      if (!dirtyFile && !dirtyMetadata) return;
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  });

  const loadFile = async (fileId: string) => {
    try {
      const nextFile = await api.file(planId, fileId);
      setFile(nextFile);
      setEditorContent(nextFile.content);
      setSavedContent(nextFile.content);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'File failed to load');
    }
  };

  const openFile = async (fileId: string) => {
    if (dirty) {
      setPendingConfirm({
        title: 'Discard changes',
        message: 'Discard unsaved changes and open another file?',
        confirmLabel: 'Discard',
        danger: true,
        onConfirm: () => {
          setPendingConfirm(null);
          void loadFile(fileId);
        }
      });
      return;
    }
    await loadFile(fileId);
  };

  const dirtyFile = file !== null && editorContent !== savedContent;
  const dirtyMetadata = Boolean(plan) && (
    (metadataDraft.title ?? '') !== (plan?.title ?? '') ||
    (metadataDraft.service ?? '') !== (plan?.service ?? '') ||
    (metadataDraft.ticket ?? '') !== (plan?.ticket ?? '') ||
    (metadataDraft.status ?? '') !== (plan?.status ?? '') ||
    (metadataDraft.owner ?? '') !== (plan?.owner ?? '') ||
    (metadataDraft.tags ?? []).join('\n') !== (plan?.tags ?? []).join('\n')
  );
  const dirty = dirtyFile || dirtyMetadata;
  const preview = useMemo(() => ({ __html: marked.parse(editorContent || file?.content || '') as string }), [editorContent, file]);
  const diffFiles = useMemo(() => parseGitDiff(diff), [diff]);
  const selectedGitPath = useMemo(() => currentGitPath(plan, file), [plan, file]);
  const selectedFileHasDiff = Boolean(selectedGitPath && diffFiles.some((item) => item.path === selectedGitPath || item.oldPath === selectedGitPath));
  const hasFiles = useMemo(() => hasFile(files), [files]);
  const fileStateByPath = useMemo(() => buildFileStateMap(plan, gitStatus, file, dirtyFile), [plan, gitStatus, file, dirtyFile]);
  const gridStyle = {
    '--left-panel-width': `${leftCollapsed ? 44 : leftWidth}px`,
    '--right-panel-width': `${rightCollapsed ? 44 : rightWidth}px`,
  } as CSSProperties & Record<'--left-panel-width' | '--right-panel-width', string>;

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

  const loadGitStatus = async (repositoryId: string) => {
    setGitLoading(true);
    try {
      setGitStatus(await api.gitStatus(repositoryId));
    } catch {
      setGitStatus(null);
    } finally {
      setGitLoading(false);
    }
  };

  const loadDiff = async () => {
    try {
      const payload = await api.diff(planId);
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
        ? await api.gitFetch(plan.repositoryId)
        : operation === 'pull'
          ? await api.gitPull(plan.repositoryId, { confirm })
          : await api.gitPush(plan.repositoryId);
      setGitStatus(result.status);
      if (operation === 'pull') await onContentChanged?.();
      if (!result.ok && result.message) setError(result.message);
    } catch (err) {
      setError(err instanceof Error ? err.message : `${operation} failed`);
    } finally {
      setGitBusy('');
    }
  };

  const commitSelectedPaths = async () => {
    if (!plan) return;
    setGitBusy('commit');
    setError('');
    try {
      const result = await api.gitCommit(plan.repositoryId, { message: gitMessage, paths: selectedGitPaths });
      setGitStatus(result.status);
      setGitMessage('');
      setSelectedGitPaths([]);
      await onContentChanged?.();
      if (!result.ok && result.message) setError(result.message);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Commit failed');
    } finally {
      setGitBusy('');
    }
  };

  const createAndSwitchBranch = async () => {
    if (!plan || !branchName.trim()) return;
    setGitBusy('branch');
    setError('');
    try {
      const result = await api.createBranch(plan.repositoryId, { name: branchName.trim(), checkout: true });
      setGitStatus(result.status);
      setBranchName('');
      await onContentChanged?.();
      if (!result.ok && result.message) setError(result.message);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Branch operation failed');
    } finally {
      setGitBusy('');
    }
  };

  const toggleGitPath = (path: string) => {
    setSelectedGitPaths((current) => current.includes(path) ? current.filter((item) => item !== path) : [...current, path]);
  };

  const goBack = () => {
    if (!dirty) {
      onBack();
      return;
    }
    setPendingConfirm({
      title: 'Discard changes',
      message: 'Discard unsaved changes and return to the board?',
      confirmLabel: 'Discard',
      danger: true,
      onConfirm: () => {
        setPendingConfirm(null);
        onBack();
      }
    });
  };

  const saveFile = async () => {
    if (!file) return;
    setSavingFile(true);
    setError('');
    try {
      await api.saveFile(planId, file.id, { content: editorContent, expectedHash: file.hash });
      const updated = await api.file(planId, file.id);
      setFile(updated);
      setEditorContent(updated.content);
      setSavedContent(updated.content);
      if (plan) await loadGitStatus(plan.repositoryId);
      await loadDiff();
      await onContentChanged?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'File save failed');
    } finally {
      setSavingFile(false);
    }
  };

  const revertFile = async () => {
    if (!file || !plan) return;
    setRevertingFile(true);
    setError('');
    try {
      await api.revertFile(planId, file.id);
      const updated = await api.file(planId, file.id);
      setFile(updated);
      setEditorContent(updated.content);
      setSavedContent(updated.content);
      await loadDiff();
      await loadGitStatus(plan.repositoryId);
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
      const result = await api.saveMetadata(planId, metadataDraft);
      setPlan(result.plan);
      if (plan) await loadGitStatus(plan.repositoryId);
      await onContentChanged?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Metadata save failed');
    } finally {
      setSavingMetadata(false);
    }
  };

  if (error && !plan) {
    return <section className="empty-state"><button className="ghost" onClick={goBack}><ArrowLeft size={16} /> Back</button><p className="error">{error}</p></section>;
  }

  return (
    <section className="workspace-page">
      <header className="workspace-header">
        <button className="ghost" onClick={goBack}><ArrowLeft size={16} /> Back</button>
        <div>
          <h1>{plan?.title ?? 'Loading plan'}</h1>
          <span>{plan?.service} / {plan?.branch} / {plan?.ticket}</span>
        </div>
        <button className="secondary" disabled={gitLoading}><RefreshCw size={16} /> {gitStatus?.dirty ? 'Local changes' : 'Git status'}</button>
      </header>
      <div className="workspace-grid" style={gridStyle} ref={workspaceGridRef}>
        <aside className={leftCollapsed ? 'file-tree side-panel collapsed' : 'file-tree side-panel'}>
          <div className="panel-header">
            <h2><FolderOpen size={16} /> Files</h2>
            <button className="icon-button" type="button" title={leftCollapsed ? 'Expand files' : 'Collapse files'} onClick={() => setLeftCollapsed((value) => !value)}>
              {leftCollapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
            </button>
          </div>
          {!leftCollapsed && (
            <div className="file-tree-list">
              {files.map((node) => <TreeNode node={node} key={node.id} onOpen={openFile} activeId={file?.id} depth={0} fileStateByPath={fileStateByPath} />)}
            </div>
          )}
          {!leftCollapsed && (
            <button className="panel-resize-handle panel-resize-handle-left" type="button" aria-label="Resize files panel" onPointerDown={(event) => startResize('left', event)}>
              <GripVertical size={16} />
            </button>
          )}
        </aside>
        <div className="document-panel">
          <div className="tabs">
            <div className="tab-list">
              <button className={tab === 'preview' ? 'active' : ''} onClick={() => setTab('preview')}><FileText size={15} /> Preview</button>
              <button className={tab === 'raw' ? 'active' : ''} onClick={() => setTab('raw')}><Code2 size={15} /> Raw</button>
              <button className={tab === 'diff' ? 'active' : ''} onClick={() => setTab('diff')}><GitCompare size={15} /> Diff</button>
            </div>
            <button className="save-action save-file-tab-action" type="button" disabled={!dirtyFile || savingFile} onClick={saveFile}>
              {savingFile ? 'Saving...' : 'Save File'}
            </button>
          </div>
          {dirty && <div className="edit-state-banner">Unsaved changes</div>}
          {tab === 'preview' && (file ? <article className="markdown-preview" dangerouslySetInnerHTML={preview} /> : <EmptyDocumentState hasFiles={hasFiles} />)}
          {tab === 'raw' && (
            <textarea
              className="raw-editor"
              value={file ? editorContent : (hasFiles ? 'Select a file.' : 'No files found in this plan.')}
              onChange={(event) => setEditorContent(event.target.value)}
              disabled={!file}
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
            <button className="icon-button" type="button" title={rightCollapsed ? 'Expand plan info' : 'Collapse plan info'} onClick={() => setRightCollapsed((value) => !value)}>
              {rightCollapsed ? <PanelRightOpen size={16} /> : <PanelRightClose size={16} />}
            </button>
          </div>
          {!rightCollapsed && (
            <>
              <div className="side-panel-tabs" role="tablist" aria-label="Plan side panel">
                <button type="button" className={rightPanelTab === 'info' ? 'active' : ''} onClick={() => setRightPanelTab('info')}>
                  <Info size={14} /> Info
                </button>
                <button type="button" className={rightPanelTab === 'git' ? 'active' : ''} onClick={() => setRightPanelTab('git')}>
                  <GitBranch size={14} /> Git
                </button>
              </div>
              {rightPanelTab === 'info' && (
                <>
              {plan?.metadataSource === 'docs' && (
                <div className="metadata-callout">
                  <strong>Docs</strong>
                  <span>This item is a documentation folder. It is browsable even though it does not use the plan service/ticket structure.</span>
                </div>
              )}
              <dl>
                <dt>Repository</dt><dd>{plan?.repositoryName}</dd>
                <dt>Service</dt><dd>{plan?.service}</dd>
                <dt>Branch</dt><dd>{plan?.branch}</dd>
                <dt>Status</dt><dd>{plan?.status && <StatusBadge status={plan.status} />}</dd>
                <dt>Source</dt><dd>{sourceLabel(plan?.metadataSource)}</dd>
                <dt>Author</dt><dd>{plan?.author || plan?.owner || 'Unknown'}</dd>
                <dt>Files</dt><dd>{plan?.counts.files ?? files.length}</dd>
              </dl>
              {plan?.metadataSource !== 'docs' && (
                <div className="metadata-form">
                  <label>Title<input value={metadataDraft.title ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, title: event.target.value }))} /></label>
                  <label>Service<input value={metadataDraft.service ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, service: event.target.value }))} /></label>
                  <label>Ticket<input value={metadataDraft.ticket ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, ticket: event.target.value }))} /></label>
                  <label>Status<select value={metadataDraft.status ?? 'draft'} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, status: event.target.value as PlanDetail['status'] }))}>
                    <option value="ideas">Ideas</option>
                    <option value="draft">Draft</option>
                    <option value="in_progress">In Progress</option>
                    <option value="review">Review</option>
                    <option value="done">Done</option>
                  </select></label>
                  <label>Owner<input value={metadataDraft.owner ?? ''} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, owner: event.target.value }))} /></label>
                  <label>Tags<input value={(metadataDraft.tags ?? []).join(', ')} onChange={(event) => setMetadataDraft((draft) => ({ ...draft, tags: event.target.value.split(',').map((tag) => tag.trim()).filter(Boolean) }))} /></label>
                </div>
              )}
              <div className="workspace-actions">
                <button className="save-action save-metadata-action" type="button" disabled={!dirtyMetadata || savingMetadata || plan?.metadataSource === 'docs'} onClick={saveMetadata}>{savingMetadata ? 'Saving...' : 'Save Metadata'}</button>
              </div>
              <div className="tags">{(plan?.tags ?? []).map((tag) => <span key={tag}>{tag}</span>)}</div>
              {plan?.warnings?.length ? (
                <div className="plan-warnings">
                  <h3>Warnings</h3>
                  {plan.warnings.map((warning) => <p key={`${warning.planPath ?? 'plan'}-${warning.message}`}>{warning.message}</p>)}
                </div>
              ) : null}
                </>
              )}
              {rightPanelTab === 'git' && (
                gitStatus ? (
                <section className="git-panel">
                  <h3>Git</h3>
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
                      <label key={`${change.status}-${change.path}`}>
                        <input type="checkbox" checked={selectedGitPaths.includes(change.path)} onChange={() => toggleGitPath(change.path)} />
                        <span>{change.status}</span>
                        <strong>{change.path}</strong>
                      </label>
                    ))}
                  </div>
                  <textarea className="commit-message" value={gitMessage} onChange={(event) => setGitMessage(event.target.value)} placeholder="Commit message" />
                  <button className="primary" type="button" disabled={Boolean(gitBusy) || selectedGitPaths.length === 0 || !gitMessage.trim()} onClick={commitSelectedPaths}>
                    {gitBusy === 'commit' ? 'Committing...' : 'Commit Selected'}
                  </button>
                  <div className="branch-create-row">
                    <input value={branchName} onChange={(event) => setBranchName(event.target.value)} placeholder="new-branch-name" />
                    <button className="secondary" type="button" disabled={Boolean(gitBusy) || !branchName.trim()} onClick={createAndSwitchBranch}>
                      {gitBusy === 'branch' ? 'Creating...' : 'Create Branch'}
                    </button>
                  </div>
                </section>
                ) : (
                  <div className="metadata-callout">
                    <strong>Git status unavailable</strong>
                    <span>Refresh the workspace or scan the repository to load Git information.</span>
                  </div>
                )
              )}
              {error && <p className="error">{error}</p>}
            </>
          )}
          {!rightCollapsed && (
            <button className="panel-resize-handle panel-resize-handle-right" type="button" aria-label="Resize plan info panel" onPointerDown={(event) => startResize('right', event)}>
              <GripVertical size={16} />
            </button>
          )}
        </aside>
      </div>
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
      <span>{hasFiles ? 'Choose a file from the explorer to preview its content.' : 'This plan folder does not contain any readable files yet.'}</span>
    </div>
  );
}

function StatusBadge({ status }: { status: PlanDetail['status'] }) {
  return <span className={`status-badge ${status}`}>{statusLabel(status)}</span>;
}

function statusLabel(status: PlanDetail['status']): string {
  switch (status) {
    case 'ideas':
      return 'Ideas';
    case 'draft':
      return 'Draft';
    case 'in_progress':
      return 'In Progress';
    case 'review':
      return 'Review';
    case 'done':
      return 'Done';
    default:
      return status;
  }
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

function TreeNode({ node, onOpen, activeId, depth, fileStateByPath }: { node: FileNode; onOpen: (id: string) => void; activeId?: string; depth: number; fileStateByPath: Map<string, TreeFileState> }) {
  const indent = { '--tree-indent': `${depth * 14}px` } as CSSProperties & Record<'--tree-indent', string>;

  if (node.type === 'directory') {
    return (
      <details open className="tree-dir">
        <summary className="tree-row tree-row-dir" style={indent} title={node.path}>
          <ChevronDown className="tree-chevron" size={14} />
          <FolderOpen className="tree-icon" size={16} />
          <span className="tree-label">{node.name}</span>
        </summary>
        <div className="tree-children">
          {node.children?.map((child) => <TreeNode node={child} key={child.id} onOpen={onOpen} activeId={activeId} depth={depth + 1} fileStateByPath={fileStateByPath} />)}
        </div>
      </details>
    );
  }
  const state = fileStateByPath.get(normalizePath(node.path));
  return (
    <button className={activeId === node.id ? 'tree-row tree-file active' : 'tree-row tree-file'} style={indent} title={node.path} onClick={() => onOpen(node.id)}>
      <span className="tree-spacer" />
      <FileIcon className="tree-icon" size={16} />
      <span className="tree-label">{node.name}</span>
      {state && <FileStateIcon state={state} />}
    </button>
  );
}

function FileStateIcon({ state }: { state: TreeFileState }) {
  const label = treeFileStateLabel(state);
  if (state === 'added' || state === 'untracked') {
    return <FilePlus2 className={`tree-state-icon ${state}`} size={14} aria-label={label} />;
  }
  if (state === 'conflicted') {
    return <TriangleAlert className={`tree-state-icon ${state}`} size={14} aria-label={label} />;
  }
  return <PencilLine className={`tree-state-icon ${state}`} size={14} aria-label={label} />;
}

function firstFile(nodes: FileNode[]): FileNode | null {
  for (const node of nodes) {
    if (node.type === 'file') return node;
    const child = firstFile(node.children ?? []);
    if (child) return child;
  }
  return null;
}

function hasFile(nodes: FileNode[]): boolean {
  return firstFile(nodes) !== null;
}

function sourceLabel(source?: string): string {
  return source === 'docs' ? 'Docs' : 'Plan';
}

function currentGitPath(plan: PlanDetail | null, file: FileContent | null): string {
  if (!plan?.planRoot || !file?.path) return '';
  return `${plan.planRoot.replace(/\/$/, '')}/${file.path.replace(/^\//, '')}`;
}

function buildFileStateMap(plan: PlanDetail | null, gitStatus: GitStatus | null, file: FileContent | null, dirtyFile: boolean): Map<string, TreeFileState> {
  const stateByPath = new Map<string, TreeFileState>();
  const planRoot = normalizePath(plan?.planRoot ?? '');
  for (const change of gitStatus?.changes ?? []) {
    const localPath = localPlanPath(planRoot, change);
    if (localPath) stateByPath.set(localPath, change.status);
  }
  if (dirtyFile && file?.path) {
    stateByPath.set(normalizePath(file.path), 'unsaved');
  }
  return stateByPath;
}

function localPlanPath(planRoot: string, change: GitChange): string {
  const path = normalizePath(change.path);
  const oldPath = normalizePath(change.oldPath ?? '');
  return stripPlanRoot(path, planRoot) || stripPlanRoot(oldPath, planRoot);
}

function stripPlanRoot(path: string, planRoot: string): string {
  if (!path) return '';
  if (!planRoot) return path;
  if (path === planRoot) return '';
  return path.startsWith(`${planRoot}/`) ? path.slice(planRoot.length + 1) : '';
}

function normalizePath(path: string): string {
  return path.replace(/^\/+/, '').replace(/\/+$/, '');
}

function treeFileStateLabel(state: TreeFileState): string {
  switch (state) {
    case 'added':
    case 'untracked':
      return 'New file not committed';
    case 'unsaved':
      return 'Unsaved editor changes';
    case 'conflicted':
      return 'File has conflicts';
    case 'deleted':
      return 'Deleted file';
    case 'renamed':
      return 'Renamed file';
    case 'copied':
      return 'Copied file';
    case 'modified':
    default:
      return 'Modified file not committed';
  }
}

function parseGitDiff(diff: string): DiffFile[] {
  const files: DiffFile[] = [];
  let current: DiffFile | null = null;
  let oldLine = 0;
  let newLine = 0;
  for (const rawLine of diff.split('\n')) {
    if (rawLine.startsWith('diff --git ')) {
      const match = rawLine.match(/^diff --git a\/(.+?) b\/(.+)$/);
      current = {
        oldPath: match?.[1],
        path: match?.[2] ?? match?.[1] ?? rawLine.replace(/^diff --git\s+/, ''),
        lines: [],
        additions: 0,
        deletions: 0
      };
      files.push(current);
      oldLine = 0;
      newLine = 0;
      continue;
    }
    if (!current) continue;
    if (rawLine.startsWith('--- ')) {
      const oldPath = rawLine.replace(/^---\s+a\//, '').replace(/^---\s+/, '');
      if (oldPath !== '/dev/null') current.oldPath = oldPath;
      continue;
    }
    if (rawLine.startsWith('+++ ')) {
      const path = rawLine.replace(/^\+\+\+\s+b\//, '').replace(/^\+\+\+\s+/, '');
      if (path !== '/dev/null') current.path = path;
      continue;
    }
    if (rawLine.startsWith('@@')) {
      const match = rawLine.match(/^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@(.*)$/);
      oldLine = match ? Number(match[1]) : 0;
      newLine = match ? Number(match[2]) : 0;
      current.lines.push({ type: 'meta', text: rawLine });
      continue;
    }
    if (rawLine.startsWith('+')) {
      current.additions += 1;
      current.lines.push({ type: 'add', text: rawLine.slice(1), newLine });
      newLine += 1;
      continue;
    }
    if (rawLine.startsWith('-')) {
      current.deletions += 1;
      current.lines.push({ type: 'delete', text: rawLine.slice(1), oldLine });
      oldLine += 1;
      continue;
    }
    if (rawLine.startsWith('index ') || rawLine.startsWith('new file ') || rawLine.startsWith('deleted file ')) {
      current.lines.push({ type: 'meta', text: rawLine });
      continue;
    }
    current.lines.push({ type: 'context', text: rawLine.startsWith(' ') ? rawLine.slice(1) : rawLine, oldLine, newLine });
    oldLine += 1;
    newLine += 1;
  }
  return files.filter((item) => item.lines.length > 0 || item.additions > 0 || item.deletions > 0);
}

import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import {
  ArrowLeft,
  ChevronDown,
  Code2,
  File as FileIcon,
  FileText,
  FolderOpen,
  GitCompare,
  GripVertical,
  Info,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightClose,
  PanelRightOpen,
  RefreshCw,
} from 'lucide-react';
import { marked } from 'marked';
import { api } from '../lib/api';
import type { FileContent, FileNode, GitStatus, PlanDetail, PlanMetadataUpdateInput } from '../lib/types';

type Tab = 'preview' | 'raw' | 'diff';

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
  const [diff, setDiff] = useState('');
  const [tab, setTab] = useState<Tab>('preview');
  const [error, setError] = useState('');
  const [leftCollapsed, setLeftCollapsed] = useState(false);
  const [rightCollapsed, setRightCollapsed] = useState(false);
  const [leftWidth, setLeftWidth] = useState(300);
  const [rightWidth, setRightWidth] = useState(300);

  useEffect(() => {
    setError('');
    setFile(null);
    api.plan(planId).then(setPlan).catch((err: Error) => setError(err.message));
    api.files(planId).then((tree) => {
      setFiles(tree);
      const first = firstFile(tree);
      if (first) void openFile(first.id);
    }).catch((err: Error) => setError(err.message));
    api.diff(planId).then((payload) => setDiff(payload.diff || 'No local changes.')).catch(() => setDiff('No diff available.'));
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

  const openFile = async (fileId: string) => {
    if (!confirmDiscardChanges()) return;
    try {
      const nextFile = await api.file(planId, fileId);
      setFile(nextFile);
      setEditorContent(nextFile.content);
      setSavedContent(nextFile.content);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'File failed to load');
    }
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
  const hasFiles = useMemo(() => hasFile(files), [files]);
  const gridStyle = {
    '--left-panel-width': `${leftCollapsed ? 44 : leftWidth}px`,
    '--right-panel-width': `${rightCollapsed ? 44 : rightWidth}px`,
  } as CSSProperties & Record<'--left-panel-width' | '--right-panel-width', string>;

  const startResize = (side: 'left' | 'right', event: React.PointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const startingWidth = side === 'left' ? leftWidth : rightWidth;

    const onPointerMove = (moveEvent: PointerEvent) => {
      const delta = moveEvent.clientX - startX;
      const nextWidth = side === 'left' ? startingWidth + delta : startingWidth - delta;
      const boundedWidth = Math.min(520, Math.max(220, nextWidth));
      if (side === 'left') {
        setLeftWidth(boundedWidth);
      } else {
        setRightWidth(boundedWidth);
      }
    };

    const onPointerUp = () => {
      document.body.classList.remove('is-resizing-panel');
      window.removeEventListener('pointermove', onPointerMove);
      window.removeEventListener('pointerup', onPointerUp);
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

  const confirmDiscardChanges = () => {
    if (!dirty) return true;
    return window.confirm('Discard unsaved changes?');
  };

  const goBack = () => {
    if (confirmDiscardChanges()) onBack();
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
      await onContentChanged?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'File save failed');
    } finally {
      setSavingFile(false);
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
      <div className="workspace-grid" style={gridStyle}>
        <aside className={leftCollapsed ? 'file-tree side-panel collapsed' : 'file-tree side-panel'}>
          <div className="panel-header">
            <h2><FolderOpen size={16} /> Files</h2>
            <button className="icon-button" type="button" title={leftCollapsed ? 'Expand files' : 'Collapse files'} onClick={() => setLeftCollapsed((value) => !value)}>
              {leftCollapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
            </button>
          </div>
          {!leftCollapsed && (
            <div className="file-tree-list">
              {files.map((node) => <TreeNode node={node} key={node.id} onOpen={openFile} activeId={file?.id} depth={0} />)}
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
            <button className={tab === 'preview' ? 'active' : ''} onClick={() => setTab('preview')}><FileText size={15} /> Preview</button>
            <button className={tab === 'raw' ? 'active' : ''} onClick={() => setTab('raw')}><Code2 size={15} /> Raw</button>
            <button className={tab === 'diff' ? 'active' : ''} onClick={() => setTab('diff')}><GitCompare size={15} /> Diff</button>
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
          {tab === 'diff' && <pre className="diff-view">{diff}</pre>}
        </div>
        <aside className={rightCollapsed ? 'metadata-panel side-panel collapsed' : 'metadata-panel side-panel'}>
          <div className="panel-header">
            <h2><Info size={16} /> Plan Info</h2>
            <button className="icon-button" type="button" title={rightCollapsed ? 'Expand plan info' : 'Collapse plan info'} onClick={() => setRightCollapsed((value) => !value)}>
              {rightCollapsed ? <PanelRightOpen size={16} /> : <PanelRightClose size={16} />}
            </button>
          </div>
          {!rightCollapsed && (
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
                <dt>Status</dt><dd>{plan?.status}</dd>
                <dt>Git</dt><dd>{gitStatus ? `${gitStatus.branch}${gitStatus.dirty ? ' · dirty' : ''}` : 'Unknown'}</dd>
                <dt>Source</dt><dd>{sourceLabel(plan?.metadataSource)}</dd>
                <dt>Author</dt><dd>{plan?.author || plan?.owner || 'Unknown'}</dd>
                <dt>Files</dt><dd>{plan?.counts.files ?? files.length}</dd>
              </dl>
              <div className="workspace-actions">
                <button className="primary" type="button" disabled={!dirtyFile || savingFile} onClick={saveFile}>{savingFile ? 'Saving...' : 'Save File'}</button>
                <button className="secondary" type="button" disabled={!dirtyMetadata || savingMetadata || plan?.metadataSource === 'docs'} onClick={saveMetadata}>{savingMetadata ? 'Saving...' : 'Save Metadata'}</button>
              </div>
              <div className="tags">{(plan?.tags ?? []).map((tag) => <span key={tag}>{tag}</span>)}</div>
              {plan?.description && <p>{plan.description}</p>}
              {plan?.warnings?.length ? (
                <div className="plan-warnings">
                  <h3>Warnings</h3>
                  {plan.warnings.map((warning) => <p key={`${warning.planPath ?? 'plan'}-${warning.message}`}>{warning.message}</p>)}
                </div>
              ) : null}
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

function TreeNode({ node, onOpen, activeId, depth }: { node: FileNode; onOpen: (id: string) => void; activeId?: string; depth: number }) {
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
          {node.children?.map((child) => <TreeNode node={child} key={child.id} onOpen={onOpen} activeId={activeId} depth={depth + 1} />)}
        </div>
      </details>
    );
  }
  return (
    <button className={activeId === node.id ? 'tree-row tree-file active' : 'tree-row tree-file'} style={indent} title={node.path} onClick={() => onOpen(node.id)}>
      <span className="tree-spacer" />
      <FileIcon className="tree-icon" size={16} />
      <span className="tree-label">{node.name}</span>
    </button>
  );
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

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
import type { FileContent, FileNode, PlanDetail } from '../lib/types';

type Tab = 'preview' | 'raw' | 'diff';

export function PlanWorkspacePage({ planId, refreshKey, onBack }: { planId: string; refreshKey: number; onBack: () => void }) {
  const [plan, setPlan] = useState<PlanDetail | null>(null);
  const [files, setFiles] = useState<FileNode[]>([]);
  const [file, setFile] = useState<FileContent | null>(null);
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

  const openFile = async (fileId: string) => {
    try {
      setFile(await api.file(planId, fileId));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'File failed to load');
    }
  };

  const preview = useMemo(() => ({ __html: marked.parse(file?.content ?? '') as string }), [file]);
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

  if (error && !plan) {
    return <section className="empty-state"><button className="ghost" onClick={onBack}><ArrowLeft size={16} /> Back</button><p className="error">{error}</p></section>;
  }

  return (
    <section className="workspace-page">
      <header className="workspace-header">
        <button className="ghost" onClick={onBack}><ArrowLeft size={16} /> Back</button>
        <div>
          <h1>{plan?.title ?? 'Loading plan'}</h1>
          <span>{plan?.service} / {plan?.branch} / {plan?.ticket}</span>
        </div>
        <button className="secondary" disabled><RefreshCw size={16} /> Pull</button>
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
          {tab === 'preview' && (file ? <article className="markdown-preview" dangerouslySetInnerHTML={preview} /> : <EmptyDocumentState hasFiles={hasFiles} />)}
          {tab === 'raw' && <pre className="raw-markdown">{file?.content ?? (hasFiles ? 'Select a file.' : 'No files found in this plan.')}</pre>}
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
                <dt>Source</dt><dd>{sourceLabel(plan?.metadataSource)}</dd>
                <dt>Author</dt><dd>{plan?.author || plan?.owner || 'Unknown'}</dd>
                <dt>Files</dt><dd>{plan?.counts.files ?? files.length}</dd>
              </dl>
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

import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { ArrowLeft, ChevronDown, Code2, File as FileIcon, FileText, FolderOpen, GitCompare, Info, RefreshCw } from 'lucide-react';
import { marked } from 'marked';
import { api } from '../lib/api';
import type { FileContent, FileNode, PlanDetail } from '../lib/types';

type Tab = 'preview' | 'raw' | 'diff';

export function PlanWorkspacePage({ planId, onBack }: { planId: string; onBack: () => void }) {
  const [plan, setPlan] = useState<PlanDetail | null>(null);
  const [files, setFiles] = useState<FileNode[]>([]);
  const [file, setFile] = useState<FileContent | null>(null);
  const [diff, setDiff] = useState('');
  const [tab, setTab] = useState<Tab>('preview');
  const [error, setError] = useState('');

  useEffect(() => {
    setError('');
    api.plan(planId).then(setPlan).catch((err: Error) => setError(err.message));
    api.files(planId).then((tree) => {
      setFiles(tree);
      const first = firstFile(tree);
      if (first) void openFile(first.id);
    }).catch((err: Error) => setError(err.message));
    api.diff(planId).then((payload) => setDiff(payload.diff || 'No local changes.')).catch(() => setDiff('No diff available.'));
  }, [planId]);

  const openFile = async (fileId: string) => {
    try {
      setFile(await api.file(planId, fileId));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'File failed to load');
    }
  };

  const preview = useMemo(() => ({ __html: marked.parse(file?.content ?? '') as string }), [file]);

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
      <div className="workspace-grid">
        <aside className="file-tree">
          <h2>Files</h2>
          <div className="file-tree-list">
            {files.map((node) => <TreeNode node={node} key={node.id} onOpen={openFile} activeId={file?.id} depth={0} />)}
          </div>
        </aside>
        <div className="document-panel">
          <div className="tabs">
            <button className={tab === 'preview' ? 'active' : ''} onClick={() => setTab('preview')}><FileText size={15} /> Preview</button>
            <button className={tab === 'raw' ? 'active' : ''} onClick={() => setTab('raw')}><Code2 size={15} /> Raw</button>
            <button className={tab === 'diff' ? 'active' : ''} onClick={() => setTab('diff')}><GitCompare size={15} /> Diff</button>
          </div>
          {tab === 'preview' && <article className="markdown-preview" dangerouslySetInnerHTML={preview} />}
          {tab === 'raw' && <pre className="raw-markdown">{file?.content ?? 'Select a file.'}</pre>}
          {tab === 'diff' && <pre className="diff-view">{diff}</pre>}
        </div>
        <aside className="metadata-panel">
          <h2><Info size={16} /> Plan Info</h2>
          <dl>
            <dt>Repository</dt><dd>{plan?.repositoryName}</dd>
            <dt>Service</dt><dd>{plan?.service}</dd>
            <dt>Branch</dt><dd>{plan?.branch}</dd>
            <dt>Status</dt><dd>{plan?.status}</dd>
            <dt>Author</dt><dd>{plan?.author || plan?.owner || 'Unknown'}</dd>
            <dt>Files</dt><dd>{plan?.counts.files ?? files.length}</dd>
          </dl>
          <div className="tags">{plan?.tags.map((tag) => <span key={tag}>{tag}</span>)}</div>
          {plan?.description && <p>{plan.description}</p>}
          {error && <p className="error">{error}</p>}
        </aside>
      </div>
    </section>
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

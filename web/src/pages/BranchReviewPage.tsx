import { useEffect, useMemo, useState } from 'react';
import { ArrowLeft, Download, FileText, GitBranch, RefreshCw } from 'lucide-react';
import type { ReviewLocation } from '../app/router';
import { ContentViewer } from '../features/content-viewer/ContentViewer';
import { useWorkspaceBranches } from '../features/workstream-explorer/useWorkspaceBranches';
import { api } from '../lib/api';
import type { FileContent, FileNode, ItemSummary, WorkspaceConfig, WorkstreamBranchLoadResult } from '../lib/types';
import { isDocumentationMetadataSource } from '../lib/vocabulary';

export function BranchReviewPage({ workspace, location, onLocationChange, onExit, onImported, onCheckoutSwitched }: {
  workspace?: WorkspaceConfig;
  location?: ReviewLocation;
  onLocationChange: (location: ReviewLocation) => void;
  onExit: () => void;
  onImported: (itemId: string) => void;
  onCheckoutSwitched: () => void | Promise<void>;
}) {
  const [review, setReview] = useState<WorkstreamBranchLoadResult | null>(null);
  const [selectedItemId, setSelectedItemId] = useState('');
  const [files, setFiles] = useState<FileNode[]>([]);
  const [file, setFile] = useState<FileContent | null>(null);
  const [loading, setLoading] = useState(false);
  const [fileLoading, setFileLoading] = useState(false);
  const [importing, setImporting] = useState(false);
  const [error, setError] = useState('');
  const branchState = useWorkspaceBranches(workspace ? [workspace] : [], async () => {
    await onCheckoutSwitched();
  });
  const workspaceState = workspace ? branchState.states[workspace.id] : undefined;
  const branch = location?.branch ?? '';
  const branchOptions = useMemo(
    () => (workspaceState?.branches ?? []).filter((candidate) => candidate !== workspaceState?.current),
    [workspaceState?.branches, workspaceState?.current]
  );
  const selectedItem = review?.items.find((item) => item.id === selectedItemId);
  const canImportSelected = Boolean(selectedItem && !isDocumentationMetadataSource(selectedItem.metadataSource));

  const loadReview = async (force = false) => {
    if (!workspace || !branch) return;
    if (branch === workspaceState?.current) {
      setReview(null);
      setError('This branch is already checked out. Exit review to use the operational pages.');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const result = await api.loadBranchReview(workspace.id, { branch, force });
      setReview(result);
      setSelectedItemId((current) => result.items.some((item) => item.id === current) ? current : result.items[0]?.id ?? '');
    } catch (caught) {
      setReview(null);
      setError(caught instanceof Error ? caught.message : 'Branch review failed to load');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setReview(null);
    setSelectedItemId('');
    setFiles([]);
    setFile(null);
    if (branch) void loadReview(false);
  }, [workspace?.id, branch, workspaceState?.current]);

  useEffect(() => {
    setFiles([]);
    setFile(null);
    if (!selectedItemId) return;
    let active = true;
    setFileLoading(true);
    api.files(selectedItemId).then((tree) => {
      if (!active) return;
      setFiles(tree);
      const first = flattenFiles(tree)[0];
      if (!first) return;
      return api.file(selectedItemId, first.id).then((content) => {
        if (active) setFile(content);
      });
    }).catch((caught) => {
      if (active) setError(caught instanceof Error ? caught.message : 'Reviewed files failed to load');
    }).finally(() => {
      if (active) setFileLoading(false);
    });
    return () => { active = false; };
  }, [selectedItemId]);

  const openFile = async (node: FileNode) => {
    if (!selectedItemId || node.type !== 'file') return;
    setFileLoading(true);
    setError('');
    try {
      setFile(await api.file(selectedItemId, node.id));
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'Reviewed file failed to load');
    } finally {
      setFileLoading(false);
    }
  };

  const importPlan = async () => {
    if (!workspace || !review || !selectedItem) return;
    const confirmed = window.confirm(`Import ${selectedItem.identifier} from ${review.branch} at ${shortCommit(review.commit)} into checkout ${review.currentCheckoutBranch}?\n\nThe source snapshot stays unchanged. Import fails if the target plan already exists.`);
    if (!confirmed) return;
    setImporting(true);
    setError('');
    try {
      const result = await api.importReviewedPlan(workspace.id, { sourceBranch: review.branch, expectedCommit: review.commit, expectedCheckoutBranch: review.currentCheckoutBranch, itemId: selectedItem.id });
      onImported(result.item.id);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'Reviewed plan import failed');
    } finally {
      setImporting(false);
    }
  };

  const switchCheckout = async () => {
    if (!workspace || !review) return;
    await branchState.switchBranch(workspace, review.branch);
  };

  if (!workspace) {
    return <section className="empty-state"><h1>Branch Review</h1><p>Select a workspace before reviewing another branch.</p><button type="button" onClick={onExit}>Back to Workstream</button></section>;
  }

  return (
    <section className="branch-review-page" aria-label="Branch Review">
      <header className="branch-review-header">
        <div>
          <button className="ghost" type="button" onClick={onExit}><ArrowLeft size={16} /> Exit review</button>
          <h1>Branch Review</h1>
          <p>Read-only committed snapshot. Operational pages remain on the current checkout.</p>
        </div>
        <div className="branch-review-context">
          <span className="branch-context-chip"><GitBranch size={14} /><span>Checkout</span><strong>{workspaceState?.current ?? workspace.baselineBranch}</strong></span>
          {review && <span className="branch-context-chip review"><span>Review</span><strong>{review.branch}</strong><code>{shortCommit(review.commit)}</code></span>}
        </div>
      </header>

      <div className="branch-review-toolbar">
        <label>Branch
          <select aria-label="Review branch" value={branch} onChange={(event) => onLocationChange({ workspaceId: workspace.id, branch: event.target.value || undefined })}>
            <option value="">Choose a branch</option>
            {branchOptions.map((candidate) => <option key={candidate} value={candidate}>{candidate}</option>)}
          </select>
        </label>
        {review && <button className="secondary" type="button" disabled={loading} onClick={() => void loadReview(true)}><RefreshCw size={15} /> Refresh snapshot</button>}
        {review && <button className="secondary" type="button" disabled={loading || workspaceState?.switching} onClick={() => void switchCheckout()}><GitBranch size={15} /> {workspaceState?.switching ? 'Switching…' : 'Switch workspace to this branch'}</button>}
        {review && <button className="primary" type="button" disabled={!canImportSelected || importing} title={selectedItem && !canImportSelected ? 'Only structured plans can be imported' : undefined} onClick={() => void importPlan()}><Download size={15} /> {importing ? 'Importing…' : 'Import selected plan'}</button>}
      </div>

      {loading && <div className="operation-notice" role="status">Loading committed snapshot…</div>}
      {error && <div className="operation-notice error" role="alert">{error}</div>}
      {!branch && <div className="branch-review-empty"><GitBranch size={28} /><strong>Choose a non-checkout branch</strong><span>The branch is read at a pinned commit without changing the workspace.</span></div>}
      {review && review.items.length === 0 && <div className="branch-review-empty"><FileText size={28} /><strong>No plans on this branch</strong><span>The committed snapshot contains no items in the configured sources.</span></div>}
      {review && review.items.length > 0 && (
        <div className="branch-review-grid">
          <aside className="branch-review-plans" aria-label="Reviewed plans">
            <h2>Plans <span>{review.itemCount}</span></h2>
            {review.items.map((item) => <ReviewItemButton key={item.id} item={item} selected={item.id === selectedItemId} onSelect={() => setSelectedItemId(item.id)} />)}
          </aside>
          <aside className="branch-review-files" aria-label="Reviewed files">
            <h2>Files</h2>
            {flattenFiles(files).map((node) => <button key={node.id} type="button" className={file?.id === node.id ? 'active' : ''} onClick={() => void openFile(node)}><FileText size={14} /> {node.path}</button>)}
            {!fileLoading && files.length === 0 && <span>No committed files found.</span>}
          </aside>
          <main className="branch-review-preview" aria-label="Reviewed file preview">
            {fileLoading && <div className="drawer-empty" role="status">Loading file…</div>}
            {!fileLoading && file && <><div className="branch-review-file-heading"><strong>{file.path}</strong><span>Read only</span></div><ContentViewer file={file} content={file.content} /></>}
            {!fileLoading && !file && <div className="drawer-empty">Select a committed file to preview it.</div>}
          </main>
        </div>
      )}
    </section>
  );
}

function ReviewItemButton({ item, selected, onSelect }: { item: ItemSummary; selected: boolean; onSelect: () => void }) {
  return <button type="button" className={selected ? 'active' : ''} onClick={onSelect}><strong>{item.identifier}</strong><span>{item.title}</span><small>{item.scope} · {item.status}</small></button>;
}

function flattenFiles(nodes: FileNode[]): FileNode[] {
  return nodes.flatMap((node) => node.type === 'file' ? [node] : flattenFiles(node.children ?? []));
}

function shortCommit(commit: string): string {
  return commit.slice(0, 8) || 'unknown';
}

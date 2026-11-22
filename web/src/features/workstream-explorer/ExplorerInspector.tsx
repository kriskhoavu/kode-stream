import { useEffect, useState } from 'react';
import { writePreference, readTogglePreference } from '../../shared/preferences/store';
import { RecentGitActivity } from '../../components/RecentGitActivity';
import { useFileEditorSession } from '../file-editor/useFileEditorSession';
import type { VisibleExplorerRow } from './types';
import { api } from '../../shared/api';
import type { GitActivityEntry, GitStatus, ItemSummary, WorkspaceConfig, WorkspaceHealth } from '../../lib/types';

export function ExplorerInspector({ workspace, row, file }: { workspace?: WorkspaceConfig; row?: VisibleExplorerRow; file: ReturnType<typeof useFileEditorSession>['file'] }) {
  const [git, setGit] = useState<GitStatus | null>(null);
  const [activity, setActivity] = useState<GitActivityEntry[]>([]);
  const [activityLoading, setActivityLoading] = useState(false);
  const [activityOpen, setActivityOpen] = useState(() => readTogglePreference('explorer.inspector.gitActivityOpen'));
  const [health, setHealth] = useState<WorkspaceHealth | null>(null);
  const [item, setItem] = useState<ItemSummary | null>(null);
  const activityPath = row?.node.type === 'file' || row?.node.type === 'directory'
    ? row.node.path
    : file?.path || '';
  useEffect(() => {
    setGit(null); setHealth(null);
    if (!workspace) return;
    const controller = new AbortController();
    void api.gitStatus(workspace.id, controller.signal).then((value) => { if (!controller.signal.aborted) setGit(value); }).catch(() => { if (!controller.signal.aborted) setGit(null); });
    void api.workspaceHealth(workspace.id, controller.signal).then((value) => { if (!controller.signal.aborted) setHealth(value); }).catch(() => { if (!controller.signal.aborted) setHealth(null); });
    return () => controller.abort();
  }, [workspace?.id]);
  useEffect(() => {
    if (!workspace) {
      setActivity([]);
      setActivityLoading(false);
      return;
    }
    const controller = new AbortController();
    setActivityLoading(true);
    api.gitActivity(workspace.id, { path: activityPath || undefined, limit: 8, signal: controller.signal })
      .then((entries) => {
        if (!controller.signal.aborted) setActivity(entries);
      })
      .catch(() => {
        if (!controller.signal.aborted) setActivity([]);
      })
      .finally(() => {
        if (!controller.signal.aborted) setActivityLoading(false);
      });
    return () => controller.abort();
  }, [workspace?.id, activityPath]);
  useEffect(() => {
    setItem(null);
    if (!row?.item) return;
    const controller = new AbortController();
    void api.items(new URLSearchParams(), controller.signal)
      .then((items) => { if (!controller.signal.aborted) setItem(items.find((candidate) => candidate.id === row.item?.itemId) ?? null); })
      .catch(() => undefined);
    return () => controller.abort();
  }, [row?.item?.itemId]);
  if (!workspace) return <p className="explorer-empty">Select a workspace or file.</p>;
  return <div className="inspector-content">
    <section><h3>Workspace</h3><dl><dt>Name</dt><dd>{workspace.name}</dd><dt>Branch</dt><dd>{git?.branch ?? workspace.baselineBranch}</dd><dt>Health</dt><dd>{health?.summary ?? 'Loading'}</dd><dt>Changes</dt><dd>{git?.changes.length ?? 0}</dd></dl></section>
    {file && <section><h3>File</h3><dl><dt>Path</dt><dd>{file.path}</dd><dt>Kind</dt><dd>{file.kind}</dd><dt>Size</dt><dd>{file.sizeBytes.toLocaleString()} bytes</dd><dt>Editable</dt><dd>{file.editable ? 'Text' : 'Read only'}</dd></dl></section>}
    {row?.item && <section><h3>Item</h3><dl><dt>ID</dt><dd>{row.item.identifier}</dd><dt>Title</dt><dd>{row.item.title}</dd><dt>Status</dt><dd>{row.item.status}</dd><dt>Owner</dt><dd>{item?.owner || 'Unassigned'}</dd></dl><a className="secondary button-link" href={`/items/${encodeURIComponent(row.item.itemId)}`}>Open details</a></section>}
    <section>
      <details className="recent-activity-panel" open={activityOpen} onToggle={(event) => {
        const open = event.currentTarget.open;
        setActivityOpen(open);
        writePreference('explorer.inspector.gitActivityOpen', open ? '1' : '0');
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

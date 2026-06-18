import { useEffect, useMemo, useState } from 'react';
import { GitBranch, FolderGit2, Search } from 'lucide-react';
import { api, statusLabels, statusOrder } from '../lib/api';
import type { ItemStatus, ItemSummary, WorkspaceConfig } from '../lib/types';

type BranchGroup = {
  branch: string;
  count: number;
  sources: string[];
  statuses: Record<ItemStatus, number>;
  latest?: string;
};

export function BranchesPage({ workspace, refreshKey, onOpenBranch }: {
  workspace?: WorkspaceConfig;
  refreshKey: number;
  onOpenBranch: (branch: string) => void;
}) {
  const [items, setPlans] = useState<ItemSummary[]>([]);
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!workspace) {
      setPlans([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    setError('');
    api.items(new URLSearchParams({ workspaceId: workspace.id }))
      .then(setPlans)
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [workspace, refreshKey]);

  const branches = useMemo(() => {
    const groups = new Map<string, BranchGroup>();
    items.forEach((plan) => {
      const branch = plan.branch || 'unknown';
      const current = groups.get(branch) ?? {
        branch,
        count: 0,
        sources: [],
        statuses: Object.fromEntries(statusOrder.map((status) => [status, 0])) as Record<ItemStatus, number>
      };
      current.count += 1;
      current.statuses[plan.status] += 1;
      const source = sourceRoot(plan, workspace);
      if (source && !current.sources.includes(source)) current.sources.push(source);
      if (plan.updatedAt && (!current.latest || new Date(plan.updatedAt) > new Date(current.latest))) {
        current.latest = plan.updatedAt;
      }
      groups.set(branch, current);
    });
    return Array.from(groups.values()).sort((a, b) => a.branch.localeCompare(b.branch, undefined, { numeric: true, sensitivity: 'base' }));
  }, [items, workspace]);

  const filteredBranches = useMemo(() => {
    const text = query.trim().toLowerCase();
    if (!text) return branches;
    return branches.filter((branch) => [branch.branch, ...branch.sources].join(' ').toLowerCase().includes(text));
  }, [branches, query]);

  if (!workspace && !loading) {
    return (
      <section className="empty-state">
        <h1>Branches</h1>
        <p>Register a local Git workspace to browse branch summaries.</p>
      </section>
    );
  }

  return (
    <section className="list-page">
      <div className="page-title list-title">
        <div>
          <h1><GitBranch size={22} /> Branches</h1>
          <span><FolderGit2 size={15} /> {workspace?.name ?? 'No workspace selected'}</span>
        </div>
      </div>
      <label className="filter-input list-search">
        <Search size={15} />
        <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search branches..." />
      </label>
      <div className="filter-summary">
        <span>{filteredBranches.length} of {branches.length} branches</span>
      </div>
      {error && <p className="error">{error}</p>}
      <div className="branch-grid" aria-busy={loading}>
        {loading && Array.from({ length: 6 }).map((_, index) => <div className="branch-card skeleton" key={index} />)}
        {!loading && filteredBranches.map((branch) => (
          <button type="button" className="branch-card" key={branch.branch} onClick={() => onOpenBranch(branch.branch)}>
            <div className="branch-card-header">
              <strong><GitBranch size={16} /> {branch.branch}</strong>
              <span>{branch.count} item{branch.count === 1 ? '' : 's'}</span>
            </div>
            <div className="branch-sources">
              {branch.sources.map((source) => <span key={source}>{source}</span>)}
            </div>
            <div className="branch-statuses">
              {statusOrder.map((status) => (
                <span key={status}>{statusLabels[status]} <strong>{branch.statuses[status]}</strong></span>
              ))}
            </div>
            <small>{branch.latest ? `Updated ${new Date(branch.latest).toLocaleDateString()}` : 'No update time'}</small>
          </button>
        ))}
        {!loading && filteredBranches.length === 0 && <div className="empty-list">No branches match the current search.</div>}
      </div>
    </section>
  );
}

function sourceRoot(plan: ItemSummary, workspace?: WorkspaceConfig): string {
  const root = plan.itemPath ?? '';
  const directories = workspace?.sources ?? [];
  return directories.find((directory) => root === directory || root.startsWith(`${directory}/`)) ?? root.split('/')[0] ?? 'items';
}

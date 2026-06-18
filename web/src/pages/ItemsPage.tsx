import { useEffect, useMemo, useState } from 'react';
import { FileText, FolderGit2, Search } from 'lucide-react';
import { api, statusLabels } from '../lib/api';
import type { ItemSummary, WorkspaceConfig } from '../lib/types';
import { labels } from '../lib/vocabulary';

export function ItemsPage({ workspace, refreshKey, onOpenPlan }: {
  workspace?: WorkspaceConfig;
  refreshKey: number;
  onOpenPlan: (itemId: string) => void;
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

  const filteredPlans = useMemo(() => {
    const text = query.trim().toLowerCase();
    if (!text) return items;
    return items.filter((plan) => [
      plan.title,
      plan.identifier,
      plan.scope,
      plan.branch,
      plan.author,
      plan.owner,
      plan.description,
      sourceRoot(plan, workspace)
    ].filter(Boolean).join(' ').toLowerCase().includes(text));
  }, [items, query, workspace]);

  if (!workspace && !loading) {
    return (
      <section className="empty-state">
        <h1>{labels.items}</h1>
        <p>Register a local Git workspace to browse items.</p>
      </section>
    );
  }

  return (
    <section className="list-page">
      <div className="page-title list-title">
        <div>
          <h1><FileText size={22} /> {labels.items}</h1>
          <span><FolderGit2 size={15} /> {workspace?.name ?? 'No workspace selected'}</span>
        </div>
      </div>
      <label className="filter-input list-search">
        <Search size={15} />
        <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search items..." />
      </label>
      <div className="filter-summary">
        <span>{filteredPlans.length} of {items.length} items</span>
      </div>
      {error && <p className="error">{error}</p>}
      <div className="plan-list" aria-busy={loading}>
        {loading && Array.from({ length: 5 }).map((_, index) => <div className="plan-list-row skeleton" key={index} />)}
        {!loading && filteredPlans.map((plan) => (
          <button type="button" className="plan-list-row" key={plan.id} onClick={() => onOpenPlan(plan.id)}>
            <div>
              <strong>{plan.title}</strong>
              <span>{plan.identifier} · {plan.scope || 'docs'} · {plan.branch}</span>
            </div>
            <p>{plan.description || 'No description'}</p>
            <div className="plan-list-meta">
              <span>{sourceRoot(plan, workspace)}</span>
              <span>{statusLabels[plan.status]}</span>
              <span>{plan.author || plan.owner || 'Unknown'}</span>
            </div>
          </button>
        ))}
        {!loading && filteredPlans.length === 0 && <div className="empty-list">No items match the current search.</div>}
      </div>
    </section>
  );
}

function sourceRoot(plan: ItemSummary, workspace?: WorkspaceConfig): string {
  const root = plan.itemPath ?? '';
  const directories = workspace?.sources ?? [];
  return directories.find((directory) => root === directory || root.startsWith(`${directory}/`)) ?? root.split('/')[0] ?? 'items';
}

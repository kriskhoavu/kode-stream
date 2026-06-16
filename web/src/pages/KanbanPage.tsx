import { useEffect, useMemo, useState } from 'react';
import { Filter, Plus, RotateCw, Search } from 'lucide-react';
import { api, statusLabels, statusOrder } from '../lib/api';
import type { PlanStatus, PlanSummary, RepositoryConfig } from '../lib/types';

export function KanbanPage({ repositories, query, onOpenPlan, onRepositoriesChanged }: {
  repositories: RepositoryConfig[];
  query: string;
  onOpenPlan: (planId: string) => void;
  onRepositoriesChanged: () => void;
}) {
  const [repositoryId, setRepositoryId] = useState('');
  const [branch, setBranch] = useState('');
  const [status, setStatus] = useState('');
  const [localQuery, setLocalQuery] = useState('');
  const [plans, setPlans] = useState<PlanSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [scanState, setScanState] = useState('');
  const text = query || localQuery;

  useEffect(() => {
    const params = new URLSearchParams();
    if (repositoryId) params.set('repositoryId', repositoryId);
    if (branch) params.set('branch', branch);
    if (status) params.set('status', status);
    if (text) params.set('q', text);
    setLoading(true);
    api.plans(params)
      .then(setPlans)
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [repositoryId, branch, status, text]);

  const branches = useMemo(() => Array.from(new Set(plans.map((plan) => plan.branch))).sort(), [plans]);
  const grouped = useMemo(() => {
    const map = new Map<PlanStatus, PlanSummary[]>();
    statusOrder.forEach((item) => map.set(item, []));
    plans.forEach((plan) => map.get(plan.status)?.push(plan));
    return map;
  }, [plans]);

  const scan = async () => {
    const target = repositoryId || repositories[0]?.id;
    if (!target) return;
    setScanState('Scanning');
    try {
      const result = await api.scan(target);
      setScanState(`${result.planCount} plans indexed`);
      onRepositoriesChanged();
      const params = new URLSearchParams(repositoryId ? { repositoryId } : {});
      setPlans(await api.plans(params));
    } catch (err) {
      setScanState(err instanceof Error ? err.message : 'Scan failed');
    }
  };

  if (repositories.length === 0 && !loading) {
    return (
      <section className="empty-state">
        <h1>Kanban</h1>
        <p>Register a local Git repository to scan plan directories.</p>
      </section>
    );
  }

  return (
    <section className="kanban-page">
      <div className="page-title">
        <h1>Kanban</h1>
        <button className="primary" disabled>
          <Plus size={16} /> New Plan
        </button>
      </div>
      <div className="board-toolbar">
        <select value={repositoryId} onChange={(event) => setRepositoryId(event.target.value)}>
          <option value="">All Repositories</option>
          {repositories.map((repo) => <option key={repo.id} value={repo.id}>{repo.name}</option>)}
        </select>
        <select value={branch} onChange={(event) => setBranch(event.target.value)}>
          <option value="">All Branches</option>
          {branches.map((item) => <option key={item}>{item}</option>)}
        </select>
        <select value={status} onChange={(event) => setStatus(event.target.value)}>
          <option value="">All Status</option>
          {statusOrder.map((item) => <option key={item} value={item}>{statusLabels[item]}</option>)}
        </select>
        <label className="filter-input">
          <Search size={15} />
          <input value={localQuery} onChange={(event) => setLocalQuery(event.target.value)} placeholder="Filter plans..." />
        </label>
        <button className="secondary" onClick={scan}>
          <RotateCw size={16} /> Scan
        </button>
        <span className="scan-state">{scanState}</span>
      </div>
      {error && <p className="error">{error}</p>}
      <div className="kanban-board" aria-busy={loading}>
        {statusOrder.map((column) => (
          <div className={`kanban-column ${column}`} key={column}>
            <header>
              <h2>{statusLabels[column]}</h2>
              <span>{grouped.get(column)?.length ?? 0}</span>
              <Filter size={14} />
            </header>
            <div className="card-stack">
              {loading && Array.from({ length: 3 }).map((_, index) => <div className="plan-card skeleton" key={index} />)}
              {!loading && grouped.get(column)?.map((plan) => <PlanCard key={plan.id} plan={plan} onOpen={() => onOpenPlan(plan.id)} />)}
              {!loading && (grouped.get(column)?.length ?? 0) === 0 && <div className="column-empty">No plans</div>}
            </div>
            <button className="add-plan" disabled><Plus size={14} /> Add Plan</button>
          </div>
        ))}
      </div>
    </section>
  );
}

function PlanCard({ plan, onOpen }: { plan: PlanSummary; onOpen: () => void }) {
  return (
    <button className="plan-card" onClick={onOpen}>
      <strong>{plan.title}</strong>
      <span>{plan.service} / {plan.branch}</span>
      <p>{plan.description || plan.ticket}</p>
      <footer>
        <span className="avatar">{(plan.author || plan.owner || '?').slice(0, 1).toUpperCase()}</span>
        <span>{plan.author || plan.owner || 'Unknown'}</span>
        <time>{plan.updatedAt ? new Date(plan.updatedAt).toLocaleDateString() : 'No date'}</time>
      </footer>
      {plan.tags.length > 0 && <div className="tags">{plan.tags.slice(0, 3).map((tag) => <span key={tag}>{tag}</span>)}</div>}
    </button>
  );
}

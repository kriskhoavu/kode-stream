import { useEffect, useMemo, useRef, useState } from 'react';
import { ChevronDown, Filter, Plus, RotateCw, Search, X } from 'lucide-react';
import { api, statusLabels, statusOrder } from '../lib/api';
import type { PlanStatus, PlanSummary, RepositoryConfig } from '../lib/types';

type FilterKey = 'statuses' | 'branches' | 'authors';

type Filters = Record<FilterKey, string[]>;

type FacetOption = { value: string; label: string };

const emptyFilters: Filters = {
  statuses: [],
  branches: [],
  authors: []
};

export function KanbanPage({ repository, refreshKey, onOpenPlan, onRepositoriesChanged }: {
  repository?: RepositoryConfig;
  refreshKey: number;
  onOpenPlan: (planId: string) => void;
  onRepositoriesChanged: () => void | Promise<void>;
}) {
  const [filters, setFilters] = useState<Filters>(emptyFilters);
  const [query, setQuery] = useState('');
  const [plans, setPlans] = useState<PlanSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [scanState, setScanState] = useState('');
  const [openFacet, setOpenFacet] = useState<FilterKey | ''>('');
  const text = query;

  useEffect(() => {
    if (!repository) {
      setPlans([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    api.plans(new URLSearchParams({ repositoryId: repository.id }))
      .then(setPlans)
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [repository, refreshKey]);

  const filteredPlans = useMemo(() => filterPlans(plans, filters, text), [plans, filters, text]);
  const branches = useMemo(() => unique(plans.map((plan) => plan.branch)), [plans]);
  const authors = useMemo(() => unique(plans.map((plan) => plan.author || plan.owner || 'Unknown')), [plans]);
  const facetConfig: { key: FilterKey; title: string; options: FacetOption[] }[] = [
    { key: 'statuses', title: 'Status', options: statusOrder.map((item) => ({ value: item, label: statusLabels[item] })) },
    { key: 'authors', title: 'Authors', options: authors.map((author) => ({ value: author, label: author })) },
    { key: 'branches', title: 'Branches', options: branches.map((branch) => ({ value: branch, label: branch })) }
  ];
  const activeFilterCount = Object.values(filters).reduce((sum, values) => sum + values.length, 0) + (text ? 1 : 0);
  const grouped = useMemo(() => {
    const map = new Map<PlanStatus, PlanSummary[]>();
    statusOrder.forEach((item) => map.set(item, []));
    filteredPlans.forEach((plan) => map.get(plan.status)?.push(plan));
    return map;
  }, [filteredPlans]);

  const scan = async () => {
    if (!repository) return;
    setScanState('Scanning');
    try {
      const result = await api.scan(repository.id);
      setScanState(`${result.planCount} plans indexed`);
      onRepositoriesChanged();
      setPlans(await api.plans(new URLSearchParams({ repositoryId: repository.id })));
    } catch (err) {
      setScanState(err instanceof Error ? err.message : 'Scan failed');
    }
  };

  const toggleFilter = (key: FilterKey, value: string) => {
    setFilters((current) => {
      const values = current[key];
      return {
        ...current,
        [key]: values.includes(value) ? values.filter((item) => item !== value) : [...values, value]
      };
    });
  };

  const clearFilters = () => {
    setFilters(emptyFilters);
    setQuery('');
  };

  if (!repository && !loading) {
    return (
      <section className="empty-state">
        <h1>Kanban</h1>
        <p>Register a local Git repository to create a workspace.</p>
      </section>
    );
  }

  return (
    <section className="kanban-page">
      <div className="page-title">
        <div>
          <h1>{repository?.name ?? 'Kanban'}</h1>
          <span>{repository ? `${repository.baselineBranch} workspace` : 'No workspace selected'}</span>
        </div>
        <button className="primary" disabled>
          <Plus size={16} /> New Plan
        </button>
      </div>
      <div className="board-toolbar">
        <label className="filter-input plan-search">
          <Search size={15} />
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search plans..." />
        </label>
        <button className="secondary" onClick={scan}>
          <RotateCw size={16} /> Scan
        </button>
        <button className="secondary" onClick={clearFilters} disabled={activeFilterCount === 0}>
          <X size={16} /> Clear
        </button>
        <span className="scan-state">{scanState}</span>
      </div>
      <div className="facet-bar">
        {facetConfig.map((facet) => (
          <FacetMenu
            key={facet.key}
            title={facet.title}
            options={facet.options}
            selected={filters[facet.key]}
            open={openFacet === facet.key}
            onOpen={() => setOpenFacet(openFacet === facet.key ? '' : facet.key)}
            onToggle={(value) => toggleFilter(facet.key, value)}
            onClear={() => setFilters((current) => ({ ...current, [facet.key]: [] }))}
          />
        ))}
      </div>
      <SelectedFilters facets={facetConfig} filters={filters} onRemove={toggleFilter} />
      <div className="filter-summary">
        <span>{filteredPlans.length} of {plans.length} items</span>
        {activeFilterCount > 0 && <span>{activeFilterCount} active filters</span>}
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

function FacetMenu({ title, options, selected, open, onOpen, onToggle, onClear }: {
  title: string;
  options: FacetOption[];
  selected: string[];
  open: boolean;
  onOpen: () => void;
  onToggle: (value: string) => void;
  onClear: () => void;
}) {
  const [optionQuery, setOptionQuery] = useState('');
  const menuRef = useRef<HTMLElement | null>(null);
  const visibleOptions = options.filter((option) => option.value);
  const searchedOptions = visibleOptions.filter((option) => option.label.toLowerCase().includes(optionQuery.toLowerCase()));

  useEffect(() => {
    if (!open) return;
    const closeOnOutsideClick = (event: PointerEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        onOpen();
      }
    };
    document.addEventListener('pointerdown', closeOnOutsideClick);
    return () => document.removeEventListener('pointerdown', closeOnOutsideClick);
  }, [onOpen, open]);

  if (visibleOptions.length === 0) return null;
  return (
    <section className="facet-menu" ref={menuRef}>
      <button type="button" className={selected.length > 0 ? 'facet-trigger active' : 'facet-trigger'} onClick={onOpen}>
        <span>{title}</span>
        <span className="facet-trigger-right">
          {selected.length > 0 && <strong>{selected.length}</strong>}
          <ChevronDown className={open ? 'facet-chevron open' : 'facet-chevron'} size={15} />
        </span>
      </button>
      {open && (
        <div className="facet-popover">
          <div className="facet-popover-header">
            <strong>{title}</strong>
            <button type="button" onClick={onClear} disabled={selected.length === 0}>Clear</button>
          </div>
          {visibleOptions.length > 8 && (
            <label className="facet-search">
              <Search size={14} />
              <input value={optionQuery} onChange={(event) => setOptionQuery(event.target.value)} placeholder={`Find ${title.toLowerCase()}...`} />
            </label>
          )}
          <div className="facet-option-list">
            {searchedOptions.map((option) => (
              <label className="facet-option" key={option.value}>
                <input type="checkbox" checked={selected.includes(option.value)} onChange={() => onToggle(option.value)} />
                <span>{option.label}</span>
              </label>
            ))}
            {searchedOptions.length === 0 && <span className="facet-empty">No matches</span>}
          </div>
        </div>
      )}
    </section>
  );
}

function SelectedFilters({ facets, filters, onRemove }: { facets: { key: FilterKey; title: string; options: FacetOption[] }[]; filters: Filters; onRemove: (key: FilterKey, value: string) => void }) {
  const chips = facets.flatMap((facet) => filters[facet.key].map((value) => ({
    key: facet.key,
    value,
    title: facet.title,
    label: facet.options.find((option) => option.value === value)?.label ?? value
  })));
  if (chips.length === 0) return null;
  return (
    <div className="selected-filters">
      {chips.map((chip) => (
        <button type="button" key={`${chip.key}-${chip.value}`} onClick={() => onRemove(chip.key, chip.value)}>
          <span>{chip.title}: {chip.label}</span>
          <X size={13} />
        </button>
      ))}
    </div>
  );
}

function PlanCard({ plan, onOpen }: { plan: PlanSummary; onOpen: () => void }) {
  const docs = plan.metadataSource === 'docs';
  return (
    <button className={docs ? 'plan-card docs-plan' : 'plan-card'} onClick={onOpen}>
      <div className="plan-card-title">
        <strong>{plan.title}</strong>
        {docs && <span className="metadata-badge docs">Docs</span>}
      </div>
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

export function filterPlans(plans: PlanSummary[], filters: Filters, text: string): PlanSummary[] {
  const query = text.trim().toLowerCase();
  return plans.filter((plan) => {
    if (filters.statuses.length > 0 && !filters.statuses.includes(plan.status)) return false;
    if (filters.branches.length > 0 && !filters.branches.includes(plan.branch)) return false;
    const author = plan.author || plan.owner || 'Unknown';
    if (filters.authors.length > 0 && !filters.authors.includes(author)) return false;
    if (query && !planSearchText(plan).includes(query)) return false;
    return true;
  });
}

function planSearchText(plan: PlanSummary): string {
  return [
    plan.title,
    plan.ticket,
    plan.service,
    plan.branch,
    plan.repositoryName,
    plan.author,
    plan.owner,
    plan.description,
    plan.metadataSource,
    ...plan.tags
  ].filter(Boolean).join(' ').toLowerCase();
}

function unique(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean))).sort((a, b) => a.localeCompare(b));
}
